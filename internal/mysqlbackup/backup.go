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
	"github.com/klauspost/compress/zstd"
	"golang.org/x/crypto/ssh"
)

type Logger func(string)

type Target struct {
	MysqlHost string
	MysqlPort int
	MysqlDB   string
	MysqlUser string
	MysqlPass string
	// IncludeTables limits backup to named tables; empty means all tables.
	IncludeTables []string
	// SSH tunnel (optional)
	UseTunnel bool
	SSHHost   string
	SSHPort   int
	SSHUser   string
	SSHAuth       string
	SSHSecret     remote.AuthSecret
	SSHHostKey    string
}

type Result struct {
	Bytes  int64
	Tables []string
}

func Backup(t Target, outDir string, log Logger) (*Result, error) {
	if log == nil {
		log = func(string) {}
	}
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return nil, err
	}
	mysqldump, err := exec.LookPath("mysqldump")
	if err != nil {
		return nil, fmt.Errorf("mysqldump not found on appliance — install default-mysql-client")
	}

	host := t.MysqlHost
	port := t.MysqlPort
	var tunnelCleanup func()
	if t.UseTunnel {
		var err error
		host, port, tunnelCleanup, err = mysqlConn(t, log)
		if err != nil {
			return nil, err
		}
		defer tunnelCleanup()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	defaults, cleanupDefaults, err := defaultsExtraFile(host, port, t.MysqlUser, t.MysqlPass)
	if err != nil {
		return nil, err
	}
	defer cleanupDefaults()

	args := []string{
		defaults,
	}
	args = append(args, mysqlClientArgs(mysqldump)...)
	args = append(args,
		"--single-transaction",
		"--routines",
		"--triggers",
		t.MysqlDB,
	)
	if len(t.IncludeTables) > 0 {
		args = append(args, t.IncludeTables...)
		log(fmt.Sprintf("dumping %d table(s)", len(t.IncludeTables)))
	} else {
		log("dumping all tables")
	}
	cmd := exec.CommandContext(ctx, mysqldump, args...)

	outPath := filepath.Join(outDir, "full.sql.zst")
	out, err := os.Create(outPath)
	if err != nil {
		return nil, err
	}
	zw, err := zstd.NewWriter(out)
	if err != nil {
		_ = out.Close()
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = zw.Close()
		_ = out.Close()
		return nil, err
	}
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		_ = zw.Close()
		_ = out.Close()
		return nil, err
	}
	errBuf := &strings.Builder{}
	go func() { _, _ = io.Copy(errBuf, stderr) }()

	n, copyErr := io.Copy(zw, stdout)
	waitErr := cmd.Wait()
	_ = zw.Close()
	_ = out.Close()
	if copyErr != nil {
		return nil, copyErr
	}
	if waitErr != nil {
		return nil, fmt.Errorf("mysqldump: %w (%s)", waitErr, strings.TrimSpace(errBuf.String()))
	}

	tables, err := extractTables(nil, outPath)
	if err != nil {
		return nil, err
	}
	if len(t.IncludeTables) > 0 {
		tables = append([]string(nil), t.IncludeTables...)
	}
	manifest, _ := json.MarshalIndent(map[string]any{
		"database": t.MysqlDB,
		"tables":   tables,
		"bytes":    n,
		"finished": time.Now().UTC().Format(time.RFC3339),
	}, "", "  ")
	_ = os.WriteFile(filepath.Join(outDir, "manifest.json"), manifest, 0o600)
	log(fmt.Sprintf("done: %d bytes, %d tables", n, len(tables)))
	return &Result{Bytes: n, Tables: tables}, nil
}

func forward(client *ssh.Client, local net.Listener, remoteAddr string, log Logger) {
	for {
		loc, err := local.Accept()
		if err != nil {
			return
		}
		go func(c net.Conn) {
			defer c.Close()
			rmt, err := client.Dial("tcp", remoteAddr)
			if err != nil {
				log(fmt.Sprintf("tunnel dial: %v", err))
				return
			}
			defer rmt.Close()
			go func() { _, _ = io.Copy(rmt, c) }()
			_, _ = io.Copy(c, rmt)
		}(loc)
	}
}

func extractTables(box *crypto.Box, zstPath string) ([]string, error) {
	rc, zr, err := archive.OpenZstd(box, zstPath)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	defer zr.Close()
	var tables []string
	seen := map[string]bool{}
	sc := bufio.NewScanner(zr)
	buf := make([]byte, 0, 1024*1024)
	sc.Buffer(buf, 16*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "CREATE TABLE ") || strings.HasPrefix(line, "CREATE TABLE IF NOT EXISTS ") {
			name := parseTableName(line)
			if name != "" && !seen[name] {
				seen[name] = true
				tables = append(tables, name)
			}
		}
	}
	return tables, sc.Err()
}

func parseTableName(line string) string {
	line = strings.TrimPrefix(line, "CREATE TABLE IF NOT EXISTS ")
	line = strings.TrimPrefix(line, "CREATE TABLE ")
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	if line[0] == '`' {
		end := strings.Index(line[1:], "`")
		if end >= 0 {
			return line[1 : 1+end]
		}
	}
	parts := strings.Fields(line)
	if len(parts) > 0 {
		return strings.Trim(parts[0], "`")
	}
	return ""
}