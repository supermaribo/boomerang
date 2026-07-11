package mysqlbackup

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

const mysqlChecksumTimeout = 2 * time.Minute

// UnchangedSince compares live table checksums to the previous backup manifest.
func UnchangedSince(t Target, prevVersionDir string, log Logger) (bool, error) {
	if log == nil {
		log = func(string) {}
	}
	prev, err := readDBManifest(prevVersionDir)
	if err != nil {
		return false, err
	}
	if len(prev.Checksums) == 0 {
		return false, nil
	}
	tables := prev.Tables
	if len(t.IncludeTables) > 0 {
		tables = append([]string(nil), t.IncludeTables...)
	}
	if len(tables) == 0 {
		tables = append([]string(nil), prev.Tables...)
	}
	if len(tables) == 0 {
		return false, nil
	}
	cur, err := tableChecksums(t, tables, log)
	if err != nil {
		return false, err
	}
	if len(cur) != len(prev.Checksums) {
		return false, nil
	}
	for name, sum := range prev.Checksums {
		if cur[name] != sum {
			return false, nil
		}
	}
	log("checksums match previous backup — database unchanged")
	return true, nil
}

func tableChecksums(t Target, tables []string, log Logger) (map[string]uint32, error) {
	mysqlBin, err := exec.LookPath("mysql")
	if err != nil {
		return nil, fmt.Errorf("mysql client not found on appliance — install default-mysql-client")
	}
	host, port, cleanup, err := mysqlConn(t, log)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	defaults, cleanupDefaults, err := defaultsExtraFile(host, port, t.MysqlUser, t.MysqlPass)
	if err != nil {
		return nil, err
	}
	defer cleanupDefaults()

	sort.Strings(tables)
	quoted := make([]string, len(tables))
	for i, name := range tables {
		quoted[i] = "`" + strings.ReplaceAll(name, "`", "``") + "`"
	}
	q := "CHECKSUM TABLE " + strings.Join(quoted, ", ")

	ctx, cancel := context.WithTimeout(context.Background(), mysqlChecksumTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, mysqlBin, append([]string{
		defaults,
	}, append(mysqlClientArgs(mysqlBin), "-N", "-B", "-e", q, t.MysqlDB)...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, wrapMySQLExecError("checksum table", err, out)
	}

	outMap := map[string]uint32{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		table := strings.TrimPrefix(parts[0], t.MysqlDB+".")
		sum, err := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 32)
		if err != nil {
			continue
		}
		outMap[table] = uint32(sum)
	}
	return outMap, nil
}
