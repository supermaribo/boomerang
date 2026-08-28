package backup

import (
	"archive/tar"
	"fmt"
	"io"
	"strings"

	"github.com/boomerang-backup/boomerang/internal/archive"
	"github.com/boomerang-backup/boomerang/internal/crypto"
)

// VerifyFileBackup checks every manifest file is present in the archive with
// a matching size. It decrypts and streams the tar locally; it never contacts
// a remote host.
func VerifyFileBackup(versionDir string, box *crypto.Box) error {
	m, err := ReadFileManifest(versionDir)
	if err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	want := map[string]int64{}
	for _, e := range m.Entries {
		if e.IsDir {
			continue
		}
		p := NormalizeRelPath(e.Path)
		if p == "" {
			continue
		}
		want[p] = e.Size
	}

	rc, zr, err := archive.OpenZstd(box, archive.FilesBlobPath(versionDir))
	if err != nil {
		return err
	}
	defer rc.Close()
	defer zr.Close()

	found := map[string]int64{}
	tr := tar.NewReader(zr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			if _, err := io.Copy(io.Discard, tr); err != nil {
				return fmt.Errorf("read %s: %w", hdr.Name, err)
			}
			continue
		}
		name := NormalizeRelPath(hdr.Name)
		n, err := io.Copy(io.Discard, tr)
		if err != nil {
			return fmt.Errorf("read %s: %w", hdr.Name, err)
		}
		if name != "" {
			found[name] = n
		}
		if hdr.Size >= 0 && n != hdr.Size {
			return fmt.Errorf("archive file %s: tar header size %d, bytes read %d", name, hdr.Size, n)
		}
	}

	var missing []string
	var sizeMismatch []string
	for p, size := range want {
		got, ok := found[p]
		if !ok {
			missing = append(missing, p)
			continue
		}
		if got != size {
			sizeMismatch = append(sizeMismatch, fmt.Sprintf("%s (manifest %d, archive %d)", p, size, got))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("archive missing %d file(s) from manifest, e.g. %s", len(missing), SampleJoin(missing, 8))
	}
	if len(sizeMismatch) > 0 {
		return fmt.Errorf("archive size mismatch for %d file(s), e.g. %s", len(sizeMismatch), SampleJoin(sizeMismatch, 6))
	}
	return nil
}

// VerifyTransferComplete fails if rsync recorded unread remote paths.
func VerifyTransferComplete(versionDir string) error {
	skipped, err := ReadSkippedLog(versionDir)
	if err != nil {
		return err
	}
	if len(skipped) == 0 {
		return nil
	}
	return fmt.Errorf("backup is incomplete: %d remote path(s) were not readable, e.g. %s", len(skipped), SampleJoin(skipped, 8))
}

// NormalizeRelPath strips ./ and leading/trailing slashes from archive paths.
func NormalizeRelPath(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimPrefix(name, "./")
	name = strings.Trim(name, "/")
	return name
}

// SampleJoin joins items with a cap for error messages.
func SampleJoin(items []string, max int) string {
	if len(items) == 0 {
		return ""
	}
	if max <= 0 || len(items) <= max {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:max], ", ") + fmt.Sprintf(" (and %d more)", len(items)-max)
}
