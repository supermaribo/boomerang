package setup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureAndValidateToken(t *testing.T) {
	dir := t.TempDir()
	secrets := filepath.Join(dir, "secrets")
	if err := os.MkdirAll(secrets, 0o700); err != nil {
		t.Fatal(err)
	}
	tok, err := EnsureToken(dir)
	if err != nil || tok == "" {
		t.Fatalf("EnsureToken: %v", err)
	}
	if !ValidateToken(dir, tok) {
		t.Fatal("expected valid token")
	}
	if ValidateToken(dir, "wrong") {
		t.Fatal("expected invalid token")
	}
	ClearToken(dir)
	if ValidateToken(dir, tok) {
		t.Fatal("expected cleared token")
	}
}
