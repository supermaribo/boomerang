package mysqlbackup

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/boomerang-backup/boomerang/internal/archive"
	"github.com/boomerang-backup/boomerang/internal/crypto"
	"github.com/boomerang-backup/boomerang/internal/remote"
)

// RestoreFull imports full.sql.zst into the target database over optional SSH tunnel.
func RestoreFull(box *crypto.Box, t Target, versionDir string, log Logger) error {
	return restoreSQL(box, t, versionDir, nil, log)
}

// RestoreTables imports only the named tables from the dump.
func RestoreTables(box *crypto.Box, t Target, versionDir string, tables []string, log Logger) error {
	if len(tables) == 0 {
		return RestoreFull(box, t, versionDir, log)
	}
	return restoreSQL(box, t, versionDir, tables, log)
}

func restoreSQL(box *crypto.Box, t Target, versionDir string, onlyTables []string, log Logger) error {
	if log == nil {
		log = func(string) {}
	}
	mysqlBin, err := exec.LookPath("mysql")
	if err != nil {
		return fmt.Errorf("mysql client not found on appliance — install default-mysql-client")
	}

	host, port, cleanup, err := mysqlConn(t, log)
	if err != nil {
		return err
	}
	defer cleanup()

	rc, zr, err := archive.OpenZstd(box, archive.SQLBlobPath(versionDir))
	if err != nil {
		return err
	}
	defer rc.Close()
	defer zr.Close()

	want := map[string]bool{}
	for _, tbl := range onlyTables {
		want[tbl] = true
	}

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		if len(want) == 0 {
			_, _ = io.Copy(pw, zr)
			return
		}
		if err := copyTables(zr, pw, want); err != nil {
			_ = pw.CloseWithError(err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()
	cmd := exec.CommandContext(ctx, mysqlBin,
		"-h", host,
		"-P", fmt.Sprintf("%d", port),
		"-u", t.MysqlUser,
		t.MysqlDB,
	)
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+t.MysqlPass)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return err
	}
	errCh := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(stderr)
		errCh <- string(b)
	}()
	log("importing SQL…")
	go func() { _, _ = io.Copy(stdin, pr); _ = stdin.Close() }()
	waitErr := cmd.Wait()
	errOut := <-errCh
	if waitErr != nil {
		return fmt.Errorf("mysql import: %w (%s)", waitErr, errOut)
	}
	log("database restore complete")
	return nil
}

func copyTables(zr io.Reader, out io.Writer, want map[string]bool) error {
	sc := bufio.NewScanner(zr)
	buf := make([]byte, 0, 1024*1024)
	sc.Buffer(buf, 16*1024*1024)
	var cur string
	var in bool
	for sc.Scan() {
		line := sc.Text()
		if tbl := tableFromDumpLine(line); tbl != "" {
			cur = tbl
			in = want[cur]
		}
		if in {
			if _, err := out.Write([]byte(line + "\n")); err != nil {
				return err
			}
		}
	}
	return sc.Err()
}

func tableFromDumpLine(line string) string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "-- Table structure for table ") {
		return strings.Trim(strings.TrimPrefix(line, "-- Table structure for table "), "`")
	}
	if strings.HasPrefix(line, "CREATE TABLE ") {
		return parseTableName(line)
	}
	return ""
}

func mysqlConn(t Target, log Logger) (host string, port int, cleanup func(), err error) {
	cleanup = func() {}
	host = t.MysqlHost
	port = t.MysqlPort
	if !t.UseTunnel {
		return host, port, cleanup, nil
	}
	log("opening SSH tunnel for MySQL restore")
	tunnel, err := remote.DialSSH(t.SSHHost, t.SSHPort, t.SSHUser, t.SSHAuth, t.SSHSecret)
	if err != nil {
		return "", 0, cleanup, fmt.Errorf("ssh tunnel: %w", err)
	}
	localListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = tunnel.Close()
		return "", 0, cleanup, err
	}
	localPort := localListener.Addr().(*net.TCPAddr).Port
	remoteMySQL := net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", t.MysqlPort))
	go forward(tunnel, localListener, remoteMySQL, log)
	cleanup = func() {
		_ = localListener.Close()
		_ = tunnel.Close()
	}
	time.Sleep(200 * time.Millisecond)
	return "127.0.0.1", localPort, cleanup, nil
}

func ReadManifestTables(versionDir string) ([]string, error) {
	path := filepath.Join(versionDir, "manifest.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m struct {
		Tables []string `json:"tables"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m.Tables, nil
}
