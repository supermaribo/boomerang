package mysqlbackup

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/boomerang-backup/boomerang/internal/archive"
	"github.com/boomerang-backup/boomerang/internal/backup"
	"github.com/boomerang-backup/boomerang/internal/crypto"
)

type sqlDumpInfo struct {
	Tables    []string
	Completed bool
}

func inspectSQLDump(box *crypto.Box, zstPath string) (sqlDumpInfo, error) {
	rc, zr, err := archive.OpenZstd(box, zstPath)
	if err != nil {
		return sqlDumpInfo{}, err
	}
	defer rc.Close()
	defer zr.Close()

	var info sqlDumpInfo
	seen := map[string]bool{}
	sc := bufio.NewScanner(zr)
	buf := make([]byte, 0, 256*1024)
	sc.Buffer(buf, 16*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.Contains(line, "Dump completed") {
			info.Completed = true
		}
		if strings.HasPrefix(line, "CREATE TABLE ") || strings.HasPrefix(line, "CREATE TABLE IF NOT EXISTS ") {
			name := parseTableName(line)
			if name != "" && !seen[name] {
				seen[name] = true
				info.Tables = append(info.Tables, name)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return info, fmt.Errorf("read dump: %w", err)
	}
	return info, nil
}

// VerifyDBBackup checks the dump opens, contains every manifest table, and was not truncated.
func VerifyDBBackup(versionDir string, box *crypto.Box) error {
	tables, err := ReadManifestTables(versionDir)
	if err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	info, err := inspectSQLDump(box, archive.SQLBlobPath(versionDir))
	if err != nil {
		return err
	}
	if len(tables) == 0 {
		if len(info.Tables) == 0 {
			return fmt.Errorf("dump contains no CREATE TABLE statements")
		}
		if !info.Completed {
			return fmt.Errorf("SQL dump looks truncated (missing \"Dump completed\" footer)")
		}
		return nil
	}
	got := map[string]bool{}
	for _, name := range info.Tables {
		got[name] = true
	}
	var missing []string
	for _, name := range tables {
		if !got[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("dump is missing %d table(s) from the manifest, e.g. %s", len(missing), backup.SampleJoin(missing, 8))
	}
	if !info.Completed {
		return fmt.Errorf("SQL dump looks truncated (missing \"Dump completed\" footer)")
	}
	return nil
}

// VerifyAgainstSource re-checks live table list and CHECKSUM TABLE against the stored backup.
func VerifyAgainstSource(t Target, versionDir string, box *crypto.Box, log Logger) error {
	if log == nil {
		log = func(string) {}
	}
	if err := VerifyDBBackup(versionDir, box); err != nil {
		return err
	}
	log("SQL dump integrity OK")

	man, err := readDBManifest(versionDir)
	if err != nil {
		return err
	}
	live, err := ListBaseTables(t, log)
	if err != nil {
		return fmt.Errorf("list live tables: %w", err)
	}
	if len(t.IncludeTables) > 0 {
		allow := map[string]bool{}
		for _, name := range t.IncludeTables {
			allow[name] = true
		}
		filtered := live[:0]
		for _, name := range live {
			if allow[name] {
				filtered = append(filtered, name)
			}
		}
		live = filtered
	}

	inBackup := map[string]bool{}
	for _, name := range man.Tables {
		inBackup[name] = true
	}
	var missingFromBackup []string
	for _, name := range live {
		if !inBackup[name] {
			missingFromBackup = append(missingFromBackup, name)
		}
	}
	if len(missingFromBackup) > 0 {
		return fmt.Errorf("live database has %d table(s) not in this backup, e.g. %s", len(missingFromBackup), backup.SampleJoin(missingFromBackup, 8))
	}
	log(fmt.Sprintf("live table list matches backup (%d table(s))", len(man.Tables)))

	if len(man.Checksums) == 0 {
		log("note: this backup has no stored table checksums; skipped live checksum compare")
		return nil
	}
	cur, err := tableChecksums(t, man.Tables, log)
	if err != nil {
		return fmt.Errorf("live checksums: %w", err)
	}
	var changed []string
	for name, sum := range man.Checksums {
		if cur[name] != sum {
			changed = append(changed, name)
		}
	}
	sort.Strings(changed)
	if len(changed) > 0 {
		return fmt.Errorf("live CHECKSUM TABLE differs for %d table(s) — database has changed since this backup, e.g. %s", len(changed), backup.SampleJoin(changed, 8))
	}
	log("live CHECKSUM TABLE matches this backup")
	return nil
}

// StreamSQL writes decrypted decompressed SQL to out.
func StreamSQL(box *crypto.Box, versionDir string, out io.Writer) error {
	rc, zr, err := archive.OpenZstd(box, archive.SQLBlobPath(versionDir))
	if err != nil {
		return err
	}
	defer rc.Close()
	defer zr.Close()
	_, err = io.Copy(out, zr)
	return err
}
