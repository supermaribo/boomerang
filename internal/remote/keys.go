package remote

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"

	"golang.org/x/crypto/ssh"
)

func GenerateEd25519Keypair() (privatePEM string, publicAuthorized string, err error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	sshPriv, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		return "", "", err
	}
	pub, err := ssh.NewPublicKey(priv.Public().(ed25519.PublicKey))
	if err != nil {
		return "", "", fmt.Errorf("public key: %w", err)
	}
	return string(pem.EncodeToMemory(sshPriv)), string(ssh.MarshalAuthorizedKey(pub)), nil
}

func PublicKeyFromPrivate(privatePEM, passphrase string) (string, error) {
	var signer ssh.Signer
	var err error
	if passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(privatePEM), []byte(passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey([]byte(privatePEM))
	}
	if err != nil {
		return "", err
	}
	return string(ssh.MarshalAuthorizedKey(signer.PublicKey())), nil
}
