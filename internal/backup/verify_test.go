package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boomerang-backup/boomerang/internal/archive"
	"github.com/boomerang-backup/boomerang/internal/crypto"
)

func TestVerifyFileBackupMatchesManifest(t *testing.T) {
	dir := t.TempDir()
	staging := filepath.Join(dir, "staging")
	if err := os.MkdirAll(filepath.Join(staging, "a"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "a", "hello.txt"), []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "version")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := archive.TarDirectory(staging, archive.FilesBlobPath(outDir)); err != nil {
		t.Fatal(err)
	}
	m := &FileManifest{
		Root: staging,
		Kind: "full",
		Entries: []ManifestEntry{
			{Path: "a/hello.txt", Size: 2},
		},
	}
	if err := WriteFileManifest(outDir, m); err != nil {
		t.Fatal(err)
	}
	box, err := crypto.NewBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyFileBackup(outDir, box); err != nil {
		t.Fatal(err)
	}

	m.Entries[0].Size = 99
	if err := WriteFileManifest(outDir, m); err != nil {
		t.Fatal(err)
	}
	if err := VerifyFileBackup(outDir, box); err == nil {
		t.Fatal("expected size mismatch")
	}

	m.Entries = []ManifestEntry{{Path: "a/missing.txt", Size: 1}}
	if err := WriteFileManifest(outDir, m); err != nil {
		t.Fatal(err)
	}
	if err := VerifyFileBackup(outDir, box); err == nil {
		t.Fatal("expected missing file")
	}
}

func TestVerifyTransferComplete(t *testing.T) {
	dir := t.TempDir()
	if err := VerifyTransferComplete(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, SkippedLogFile), []byte("storage/private/\nvendor/foo 2.svg\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := VerifyTransferComplete(dir)
	if err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("got %v", err)
	}
}

func TestSampleJoin(t *testing.T) {
	if SampleJoin([]string{"a", "b"}, 8) != "a, b" {
		t.Fatal(SampleJoin([]string{"a", "b"}, 8))
	}
	got := SampleJoin([]string{"a", "b", "c"}, 2)
	if !strings.Contains(got, "and 1 more") {
		t.Fatal(got)
	}
}
