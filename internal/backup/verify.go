package backup

import (
	"archive/tar"
	"fmt"
	"io"

	"github.com/boomerang-backup/boomerang/internal/archive"
	"github.com/boomerang-backup/boomerang/internal/crypto"
)

// VerifyFileBackup checks manifest entries against the encrypted archive.
func VerifyFileBackup(versionDir string, box *crypto.Box) error {
	m, err := ReadFileManifest(versionDir)
	if err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	rc, zr, err := archive.OpenZstd(box, archive.FilesBlobPath(versionDir))
	if err != nil {
		return err
	}
	defer rc.Close()
	defer zr.Close()

	wantFiles := 0
	for _, e := range m.Entries {
		if !e.IsDir {
			wantFiles++
		}
	}
	got := 0
	tr := tar.NewReader(zr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		if hdr.Typeflag == tar.TypeReg {
			got++
		}
	}
	if got < wantFiles {
		return fmt.Errorf("archive has %d files, manifest expects at least %d", got, wantFiles)
	}
	return nil
}
