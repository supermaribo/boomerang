//go:build linux

package agentstats

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTailFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "access.log")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\nfour\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := tailFile(path, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got != "three\nfour\n" {
		t.Fatalf("tailFile = %q, want %q", got, "three\nfour\n")
	}
}

func TestCandidateLogName(t *testing.T) {
	for _, name := range []string{"access.log", "error.log.1", "access_log", "error_log.1"} {
		if !candidateLogName(name) {
			t.Errorf("expected candidate %q", name)
		}
	}
	for _, name := range []string{"access.log.gz", "../access.log", "notes.txt"} {
		if candidateLogName(name) {
			t.Errorf("unexpected candidate %q", name)
		}
	}
}
