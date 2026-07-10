package mysqlbackup

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/boomerang-backup/boomerang/internal/remote"
)

const (
	mysqlListTablesTimeout = 45 * time.Second
	mysqlConnectTimeoutSec = "15"
)

// mysqlClientArgs returns flags shared by mysql invocations.
func mysqlClientArgs(bin string) []string {
	args := sslDisableArgs(bin)
	if supportsMySQLFlag(bin, "--connect-timeout") {
		args = append(args, "--connect-timeout="+mysqlConnectTimeoutSec)
	}
	return args
}

func supportsMySQLFlag(bin, flag string) bool {
	out, err := exec.Command(bin, "--help").Output()
	return err == nil && strings.Contains(string(out), flag)
}

func mysqlConn(t Target, log Logger) (host string, port int, cleanup func(), err error) {
	cleanup = func() {}
	if log == nil {
		log = func(string) {}
	}
	host = t.MysqlHost
	port = t.MysqlPort
	if !t.UseTunnel {
		return host, port, cleanup, nil
	}
	log("opening SSH tunnel for MySQL")
	tunnel, err := remote.DialSSH(t.SSHHost, t.SSHPort, t.SSHUser, t.SSHAuth, t.SSHSecret, remote.HostKeyTrust{
		KnownFingerprint: t.SSHHostKey,
	})
	if err != nil {
		return "", 0, cleanup, fmt.Errorf("ssh tunnel: %w", err)
	}
	localListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = tunnel.Close()
		return "", 0, cleanup, err
	}
	localPort := localListener.Addr().(*net.TCPAddr).Port
	remoteMySQL := net.JoinHostPort(t.MysqlHost, fmt.Sprintf("%d", t.MysqlPort))
	go forward(tunnel, localListener, remoteMySQL, log)
	cleanup = func() {
		_ = localListener.Close()
		_ = tunnel.Close()
	}
	time.Sleep(200 * time.Millisecond)
	return "127.0.0.1", localPort, cleanup, nil
}

func wrapMySQLExecError(op string, err error, out []byte) error {
	if err == nil {
		return nil
	}
	msg := strings.TrimSpace(string(out))
	hint := "check MySQL host/port, username/password, tunnel mode, and firewall from this appliance"
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "killed") {
		if msg != "" {
			return fmt.Errorf("%s: timed out (%s) — %s", op, msg, hint)
		}
		return fmt.Errorf("%s: timed out — %s", op, hint)
	}
	if msg != "" {
		return fmt.Errorf("%s: %w (%s)", op, err, msg)
	}
	return fmt.Errorf("%s: %w", op, err)
}
