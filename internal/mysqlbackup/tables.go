package mysqlbackup

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
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

	ctx, cancel := context.WithTimeout(context.Background(), mysqlListTablesTimeout)
	defer cancel()

	defaults, cleanupDefaults, err := defaultsExtraFile(host, port, t.MysqlUser, t.MysqlPass)
	if err != nil {
		return nil, err
	}
	defer cleanupDefaults()

	cmd := exec.CommandContext(ctx, mysqlBin, append([]string{
		defaults,
	}, append(mysqlClientArgs(mysqlBin), "-N", "-B", "-e", "SHOW TABLES", t.MysqlDB)...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, wrapMySQLExecError("list tables", err, out)
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
