package crypto

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// Blob format: magic "BRNG\x01" + repeated chunks of:
// uint32_be ciphertext_len + nonce(12) + ciphertext

var blobMagic = []byte{'B', 'R', 'N', 'G', 1}

const blobChunkPlain = 1 << 20 // 1 MiB

func EncryptedPath(plain string) string {
	return plain + ".enc"
}

func (b *Box) EncryptFile(plainPath, encPath string) error {
	in, err := os.Open(plainPath)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(encPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := out.Write(blobMagic); err != nil {
		return err
	}
	buf := make([]byte, blobChunkPlain)
	for {
		n, err := in.Read(buf)
		if n > 0 {
			sealed, err := b.Seal(buf[:n])
			if err != nil {
				return err
			}
			var hdr [4]byte
			binary.BigEndian.PutUint32(hdr[:], uint32(len(sealed)))
			if _, err := out.Write(hdr[:]); err != nil {
				return err
			}
			if _, err := out.Write(sealed); err != nil {
				return err
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// OpenBlob opens an encrypted (.enc) or legacy plaintext backup blob.
func (b *Box) OpenBlob(path string) (io.ReadCloser, error) {
	if enc := EncryptedPath(path); fileExists(enc) {
		return b.openEncrypted(enc)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func (b *Box) openEncrypted(encPath string) (io.ReadCloser, error) {
	f, err := os.Open(encPath)
	if err != nil {
		return nil, err
	}
	magic := make([]byte, len(blobMagic))
	if _, err := io.ReadFull(f, magic); err != nil {
		_ = f.Close()
		return nil, err
	}
	for i := range blobMagic {
		if magic[i] != blobMagic[i] {
			_ = f.Close()
			return nil, fmt.Errorf("invalid encrypted blob header")
		}
	}
	return &blobReader{box: b, f: f}, nil
}

type blobReader struct {
	box    *Box
	f      *os.File
	plain  []byte
	off    int
	closed bool
}

func (r *blobReader) Read(p []byte) (int, error) {
	for {
		if r.off < len(r.plain) {
			n := copy(p, r.plain[r.off:])
			r.off += n
			return n, nil
		}
		if r.closed {
			return 0, io.EOF
		}
		var hdr [4]byte
		if _, err := io.ReadFull(r.f, hdr[:]); err != nil {
			if err == io.EOF {
				return 0, io.EOF
			}
			return 0, err
		}
		n := binary.BigEndian.Uint32(hdr[:])
		if n == 0 || n > 16*1024*1024 {
			return 0, fmt.Errorf("invalid blob chunk size %d", n)
		}
		sealed := make([]byte, n)
		if _, err := io.ReadFull(r.f, sealed); err != nil {
			return 0, err
		}
		plain, err := r.box.Open(sealed)
		if err != nil {
			return 0, err
		}
		r.plain = plain
		r.off = 0
	}
}

func (r *blobReader) Close() error {
	r.closed = true
	return r.f.Close()
}
