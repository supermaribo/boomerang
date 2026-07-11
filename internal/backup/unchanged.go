package backup

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"sort"
)

// FileManifestsEqual reports whether two file snapshots have the same regular files
// (path, size, mtime). Directory-only differences are ignored.
func FileManifestsEqual(a, b *FileManifest) bool {
	if a == nil || b == nil {
		return false
	}
	ai := EntryIndex(a)
	bi := EntryIndex(b)
	if len(ai) != len(bi) {
		return false
	}
	for path, ae := range ai {
		be, ok := bi[path]
		if !ok || ae.Size != be.Size || ae.Mtime != be.Mtime {
			return false
		}
	}
	return true
}

// FileBackupUnchanged decides whether a completed file backup can be skipped.
// Incremental runs with no copied file data are unchanged; full/rsync snapshots
// are compared against the previous successful manifest on disk.
func FileBackupUnchanged(new *FileManifest, filesCopied int, bytesCopied int64, prevVersionDir string) (bool, error) {
	if new == nil {
		return false, nil
	}
	if new.Kind == "incremental" {
		return filesCopied == 0 && bytesCopied == 0, nil
	}
	prev, err := ReadFileManifest(prevVersionDir)
	if err != nil {
		return false, err
	}
	return FileManifestsEqual(new, prev), nil
}

// FileSHA256 returns the hex SHA-256 digest of a file.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// StringSlicesEqual compares two string slices regardless of order.
func StringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aa := append([]string(nil), a...)
	bb := append([]string(nil), b...)
	sort.Strings(aa)
	sort.Strings(bb)
	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}
