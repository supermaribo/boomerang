package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

type Box struct {
	gcm cipher.AEAD
}

func NewBox(masterKey []byte) (*Box, error) {
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes")
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{gcm: gcm}, nil
}

func (b *Box) Seal(plain []byte) ([]byte, error) {
	nonce := make([]byte, b.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return b.gcm.Seal(nonce, nonce, plain, nil), nil
}

func (b *Box) Open(sealed []byte) ([]byte, error) {
	ns := b.gcm.NonceSize()
	if len(sealed) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}
	return b.gcm.Open(nil, sealed[:ns], sealed[ns:], nil)
}
