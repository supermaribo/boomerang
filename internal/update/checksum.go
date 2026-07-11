package update

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

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

// ExpectedSHA256FromSums parses a line from SHA256SUMS for the given filename.
func ExpectedSHA256FromSums(data, filename string) (string, bool) {
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimPrefix(parts[1], "*")
		if name == filename {
			return parts[0], true
		}
	}
	return "", false
}

func VerifyFileSHA256(path, expected string) error {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if expected == "" {
		return fmt.Errorf("missing checksum")
	}
	got, err := FileSHA256(path)
	if err != nil {
		return err
	}
	if got != expected {
		return fmt.Errorf("checksum mismatch")
	}
	return nil
}
