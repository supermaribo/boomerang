package setup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateTokenConstantTime(t *testing.T) {
	dir := t.TempDir()
	secrets := filepath.Join(dir, "secrets")
	if err := os.MkdirAll(secrets, 0o700); err != nil {
		t.Fatal(err)
	}
	tok, err := EnsureToken(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ValidateToken(dir, tok) {
		t.Fatal("expected valid token")
	}
	if ValidateToken(dir, tok+"x") {
		t.Fatal("expected invalid token")
	}
	if ValidateToken(dir, "") {
		t.Fatal("expected empty token invalid")
	}
	_ = filepath.Join(dir, "secrets")
}
