package mysqlbackup

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ListTables returns table names in the target database.
func ListTables(t Target, log Logger) ([]string, error) {
	if log == nil {
		log = func(string) {}
	}
	mysqlBin, err := exec.LookPath("mysql")
	if err != nil {
		return nil, fmt.Errorf("mysql client not found on appliance — install default-mysql-client")
	}
	host, port, cleanup, err := mysqlConn(t, log)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	defaults, cleanupDefaults, err := defaultsExtraFile(host, port, t.MysqlUser, t.MysqlPass)
	if err != nil {
		return nil, err
	}
	defer cleanupDefaults()

	cmd := exec.CommandContext(ctx, mysqlBin, append([]string{
		defaults,
	}, append(sslDisableArgs(mysqlBin), "-N", "-B", "-e", "SHOW TABLES", t.MysqlDB)...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("list tables: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	var tables []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			tables = append(tables, line)
		}
	}
	return tables, nil
}
