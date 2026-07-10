package pathutil

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SafeDataPath joins rel under base and rejects path traversal.
func SafeDataPath(base, rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", fmt.Errorf("empty relative path")
	}
	rel = filepath.FromSlash(rel)
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute path not allowed: %s", rel)
	}
	clean := filepath.Clean(rel)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal not allowed: %s", rel)
	}
	dest := filepath.Join(base, clean)
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return "", err
	}
	if destAbs != baseAbs && !strings.HasPrefix(destAbs, baseAbs+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes data directory: %s", rel)
	}
	return dest, nil
}
