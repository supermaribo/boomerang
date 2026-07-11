package update

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpectedSHA256FromSums(t *testing.T) {
	data := "abc123  boomerang-linux-amd64\ndef456  boomerang-linux-arm64\n"
	got, ok := ExpectedSHA256FromSums(data, "boomerang-linux-amd64")
	if !ok || got != "abc123" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestVerifyFileSHA256(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bin")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	sum, err := FileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyFileSHA256(path, sum); err != nil {
		t.Fatal(err)
	}
	if err := VerifyFileSHA256(path, "deadbeef"); err == nil {
		t.Fatal("expected mismatch")
	}
}
