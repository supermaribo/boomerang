package remote

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"golang.org/x/crypto/ssh"
)

// VerifyHostFingerprint checks ssh-keyscan output matches the pinned fingerprint.
func VerifyHostFingerprint(host string, port int, wantFP string) error {
	if wantFP == "" {
		return nil
	}
	cmd := exec.Command("ssh-keyscan", "-p", fmt.Sprintf("%d", port), host)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("ssh-keyscan: %w", err)
	}
	for _, line := range bytes.Split(out, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		parts := strings.Fields(string(line))
		if len(parts) < 3 {
			continue
		}
		keyBytes, err := ssh.ParsePublicKey([]byte(parts[1] + " " + parts[2]))
		if err != nil {
			continue
		}
		if ssh.FingerprintSHA256(keyBytes) == wantFP {
			return nil
		}
	}
	return fmt.Errorf("ssh host key %s not found for %s:%d", wantFP, host, port)
}
