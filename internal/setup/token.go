package setup

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const tokenFile = "setup.token"

func TokenPath(dataDir string) string {
	return filepath.Join(dataDir, "secrets", tokenFile)
}

// EnsureToken creates a setup token file when missing and returns the token.
func EnsureToken(dataDir string) (string, error) {
	path := TokenPath(dataDir)
	if raw, err := os.ReadFile(path); err == nil {
		tok := strings.TrimSpace(string(raw))
		if tok != "" {
			return tok, nil
		}
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(b)
	if err := os.WriteFile(path, []byte(tok+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("write setup token: %w", err)
	}
	return tok, nil
}

func ValidateToken(dataDir, provided string) bool {
	provided = strings.TrimSpace(provided)
	if provided == "" {
		return false
	}
	raw, err := os.ReadFile(TokenPath(dataDir))
	if err != nil {
		return false
	}
	expected := strings.TrimSpace(string(raw))
	if expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) == 1
}

func ClearToken(dataDir string) {
	_ = os.Remove(TokenPath(dataDir))
}
