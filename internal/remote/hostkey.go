package remote

import (
	"fmt"
	"net"

	"golang.org/x/crypto/ssh"
)

// HostKeyTrust pins or verifies SSH host keys (SHA256 fingerprint).
type HostKeyTrust struct {
	KnownFingerprint string
	Pin              func(fingerprint string) error
}

func (t HostKeyTrust) callback() ssh.HostKeyCallback {
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		fp := ssh.FingerprintSHA256(key)
		if t.KnownFingerprint != "" {
			if fp != t.KnownFingerprint {
				return fmt.Errorf("ssh host key changed (got %s, expected %s)", fp, t.KnownFingerprint)
			}
			return nil
		}
		if t.Pin != nil {
			return t.Pin(fp)
		}
		return nil
	}
}
