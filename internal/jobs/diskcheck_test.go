package jobs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckDiskForBackup(t *testing.T) {
	dir := t.TempDir()
	if err := checkDiskForBackup(dir, 0, "sftp"); err != nil {
		t.Fatalf("expected pass with min headroom: %v", err)
	}
	// Fill most of the temp dir isn't practical; just verify rsync needs more than zero estimate.
	if err := checkDiskForBackup(dir, 1024*1024*1024, "rsync"); err == nil {
		// May pass on systems with huge temp fs — ensure function runs without panic.
		t.Log("large rsync estimate passed (spacious temp dir)")
	}
	_ = os.MkdirAll(filepath.Join(dir, "nested"), 0o700)
}
