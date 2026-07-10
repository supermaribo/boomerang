package mysqlbackup

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/boomerang-backup/boomerang/internal/archive"
	"github.com/boomerang-backup/boomerang/internal/crypto"
)

// VerifyDBBackup checks the manifest and encrypted SQL dump open cleanly.
func VerifyDBBackup(versionDir string, box *crypto.Box) error {
	tables, err := ReadManifestTables(versionDir)
	if err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	rc, zr, err := archive.OpenZstd(box, archive.SQLBlobPath(versionDir))
	if err != nil {
		return err
	}
	defer rc.Close()
	defer zr.Close()

	got := 0
	sc := bufio.NewScanner(zr)
	buf := make([]byte, 0, 256*1024)
	sc.Buffer(buf, 16*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "CREATE TABLE ") {
			got++
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("read dump: %w", err)
	}
	want := len(tables)
	if want == 0 {
		// Legacy backup without manifest table list — ensure dump is non-empty.
		if got == 0 {
			return fmt.Errorf("dump contains no CREATE TABLE statements")
		}
		return nil
	}
	if got < want {
		return fmt.Errorf("dump has %d tables, manifest expects at least %d", got, want)
	}
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
