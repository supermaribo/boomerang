package mysqlbackup

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// defaultsExtraFile writes a short-lived my.cnf for mysql/mysqldump clients.
// Avoids MYSQL_PWD (MariaDB warns and changes SSL behaviour) and disables SSL
// for typical internal-network targets without TLS.
func defaultsExtraFile(host string, port int, user, password string) (flag string, cleanup func(), err error) {
	f, err := os.CreateTemp("", "boomerang-mysql-*.cnf")
	if err != nil {
		return "", func() {}, err
	}
	path := f.Name()
	_, err = fmt.Fprintf(f, "[client]\nhost=%s\nport=%d\nuser=%s\npassword=%s\n",
		cnfQuote(host), port, cnfQuote(user), cnfQuote(password))
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(path)
		return "", func() {}, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = os.Remove(path)
		return "", func() {}, err
	}
	return "--defaults-extra-file=" + path, func() { _ = os.Remove(path) }, nil
}

// sslDisableArgs returns flags to skip TLS for internal MySQL/MariaDB targets.
func sslDisableArgs(bin string) []string {
	out, err := exec.Command(bin, "--help").Output()
	if err == nil && strings.Contains(string(out), "--ssl-mode") {
		return []string{"--ssl-mode=DISABLED"}
	}
	return []string{"--ssl=false"}
}

func cnfQuote(s string) string {
	if s == "" {
		return `""`
	}
	if !strings.ContainsAny(s, " \t#;\"'\\") {
		return s
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
