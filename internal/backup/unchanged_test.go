package backup

import "testing"

func TestFileManifestsEqual(t *testing.T) {
	a := &FileManifest{Entries: []ManifestEntry{
		{Path: "a.txt", Size: 1, Mtime: "2026-01-01T00:00:00Z"},
		{Path: "dir", IsDir: true},
	}}
	b := &FileManifest{Entries: []ManifestEntry{
		{Path: "a.txt", Size: 1, Mtime: "2026-01-01T00:00:00Z"},
	}}
	if !FileManifestsEqual(a, b) {
		t.Fatal("expected equal manifests")
	}
	c := &FileManifest{Entries: []ManifestEntry{
		{Path: "a.txt", Size: 2, Mtime: "2026-01-01T00:00:00Z"},
	}}
	if FileManifestsEqual(a, c) {
		t.Fatal("expected different size to differ")
	}
}

func TestFileBackupUnchangedIncremental(t *testing.T) {
	m := &FileManifest{Kind: "incremental"}
	ok, err := FileBackupUnchanged(m, 0, 0, "")
	if err != nil || !ok {
		t.Fatalf("expected unchanged incremental, ok=%v err=%v", ok, err)
	}
	ok, err = FileBackupUnchanged(m, 1, 10, "")
	if err != nil || ok {
		t.Fatalf("expected changed incremental, ok=%v err=%v", ok, err)
	}
}
