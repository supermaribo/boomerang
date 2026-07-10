package remote

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

func hostPortLabel(host string, port int) string {
	if port == 0 {
		port = 22
	}
	if port == 22 {
		return host
	}
	return fmt.Sprintf("[%s]:%d", host, port)
}

func matchFingerprint(key ssh.PublicKey, wantFP string) bool {
	return ssh.FingerprintSHA256(key) == strings.TrimSpace(wantFP)
}

func parseKeyscanLine(line []byte) (ssh.PublicKey, error) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 || line[0] == '#' {
		return nil, fmt.Errorf("skip")
	}
	parts := strings.Fields(string(line))
	if len(parts) < 3 {
		return nil, fmt.Errorf("bad line")
	}
	return ssh.ParsePublicKey([]byte(parts[1] + " " + parts[2]))
}

func keyscanHost(host string, port int) ([]ssh.PublicKey, error) {
	if port == 0 {
		port = 22
	}
	cmd := exec.Command("ssh-keyscan", "-p", fmt.Sprintf("%d", port), "-T", "15",
		"-t", "ed25519,ecdsa,rsa", host)
	out, err := cmd.CombinedOutput()
	if err != nil && len(bytes.TrimSpace(out)) == 0 {
		return nil, fmt.Errorf("ssh-keyscan: %w", err)
	}
	var keys []ssh.PublicKey
	seen := map[string]bool{}
	for _, raw := range bytes.Split(out, []byte("\n")) {
		key, err := parseKeyscanLine(raw)
		if err != nil {
			continue
		}
		fp := ssh.FingerprintSHA256(key)
		if seen[fp] {
			continue
		}
		seen[fp] = true
		keys = append(keys, key)
	}
	return keys, nil
}

func probeHostKeys(host string, port int) ([]ssh.PublicKey, error) {
	if port == 0 {
		port = 22
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	var keys []ssh.PublicKey
	seen := map[string]bool{}
	cfg := &ssh.ClientConfig{
		User: "boomerang-probe",
		Auth: []ssh.AuthMethod{},
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			fp := ssh.FingerprintSHA256(key)
			if !seen[fp] {
				seen[fp] = true
				keys = append(keys, key)
			}
			return nil
		},
		Timeout: 15 * time.Second,
	}
	conn, err := ssh.Dial("tcp", addr, cfg)
	if conn != nil {
		_ = conn.Close()
	}
	if len(keys) == 0 {
		if err != nil {
			return nil, fmt.Errorf("ssh probe: %w", err)
		}
		return nil, fmt.Errorf("ssh probe: no host keys received")
	}
	return keys, nil
}

func collectHostKeys(host string, port int) ([]ssh.PublicKey, error) {
	keys, err := keyscanHost(host, port)
	if err == nil && len(keys) > 0 {
		return keys, nil
	}
	return probeHostKeys(host, port)
}

// KnownHostsFile writes a temp known_hosts containing only the pinned host key for rsync/ssh CLI.
func KnownHostsFile(host string, port int, wantFP string) (path string, cleanup func(), err error) {
	noop := func() {}
	wantFP = strings.TrimSpace(wantFP)
	if wantFP == "" {
		return "", noop, nil
	}
	if port == 0 {
		port = 22
	}
	keys, err := collectHostKeys(host, port)
	if err != nil {
		return "", noop, err
	}
	var match ssh.PublicKey
	for _, k := range keys {
		if matchFingerprint(k, wantFP) {
			match = k
			break
		}
	}
	if match == nil {
		return "", noop, fmt.Errorf("ssh host key %s not found for %s:%d", wantFP, host, port)
	}
	label := hostPortLabel(host, port)
	line := label + " " + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(match)))
	f, err := os.CreateTemp("", "boomerang-known-hosts-*")
	if err != nil {
		return "", noop, err
	}
	if _, err := f.WriteString(line + "\n"); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", noop, err
	}
	_ = f.Chmod(0o600)
	_ = f.Close()
	return f.Name(), func() { _ = os.Remove(f.Name()) }, nil
}

// VerifyHostFingerprint checks ssh-keyscan / probe output matches the pinned fingerprint.
func VerifyHostFingerprint(host string, port int, wantFP string) error {
	wantFP = strings.TrimSpace(wantFP)
	if wantFP == "" {
		return nil
	}
	keys, err := collectHostKeys(host, port)
	if err != nil {
		return err
	}
	for _, k := range keys {
		if matchFingerprint(k, wantFP) {
			return nil
		}
	}
	return fmt.Errorf("ssh host key %s not found for %s:%d", wantFP, host, port)
}
