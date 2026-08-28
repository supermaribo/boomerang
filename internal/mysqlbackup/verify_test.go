package mysqlbackup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestInspectSQLDump(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "full.sql.zst")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw, err := zstd.NewWriter(f)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = zw.Write([]byte("-- MySQL dump\nCREATE TABLE `users` (\n  id int\n);\nCREATE TABLE `orders` (\n  id int\n);\n-- Dump completed on 2026-08-28 12:00:00\n"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := inspectSQLDump(nil, path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Completed {
		t.Fatal("expected completed footer")
	}
	if len(info.Tables) != 2 || info.Tables[0] != "users" || info.Tables[1] != "orders" {
		t.Fatalf("tables: %#v", info.Tables)
	}
}

func TestMissingStrings(t *testing.T) {
	got := missingStrings([]string{"a", "b", "c"}, []string{"a", "c"})
	if len(got) != 1 || got[0] != "b" {
		t.Fatalf("%v", got)
	}
}
