package archive

import (
	"archive/tar"
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/boomerang-backup/boomerang/internal/crypto"
	"github.com/klauspost/compress/zstd"
)

const FilesArchive = "files.tar.zst"
const SQLArchive = "full.sql.zst"

func FilesBlobPath(versionDir string) string {
	return filepath.Join(versionDir, FilesArchive)
}

func SQLBlobPath(versionDir string) string {
	return filepath.Join(versionDir, SQLArchive)
}

func OpenZstd(box *crypto.Box, blobPath string) (io.ReadCloser, *zstd.Decoder, error) {
	rc, err := box.OpenBlob(blobPath)
	if err != nil {
		return nil, nil, err
	}
	zr, err := zstd.NewReader(rc)
	if err != nil {
		_ = rc.Close()
		return nil, nil, err
	}
	return rc, zr, nil
}

// TarDirectory writes srcDir contents into a zstd-compressed tar at outPath.
func TarDirectory(srcDir, outPath string) error {
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	zw, err := zstd.NewWriter(f)
	if err != nil {
		_ = f.Close()
		return err
	}
	tw := tar.NewWriter(zw)
	err = filepath.Walk(srcDir, func(full string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, full)
		if err != nil || rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		hdr, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		hdr.Name = rel
		if fi.IsDir() {
			hdr.Name = strings.TrimSuffix(rel, "/") + "/"
			return tw.WriteHeader(hdr)
		}
		if !fi.Mode().IsRegular() {
			return nil
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		rf, err := os.Open(full)
		if err != nil {
			return err
		}
		_, err = io.Copy(tw, rf)
		_ = rf.Close()
		return err
	})
	if err != nil {
		_ = tw.Close()
		_ = zw.Close()
		_ = f.Close()
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return f.Close()
}

// WriteZipPaths streams selected paths from a tar.zst blob into a zip writer.
func WriteZipPaths(box *crypto.Box, blobPath string, want map[string]bool, zw *zip.Writer) (int, error) {
	rc, zr, err := OpenZstd(box, blobPath)
	if err != nil {
		return 0, err
	}
	defer rc.Close()
	defer zr.Close()
	tr := tar.NewReader(zr)
	written := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return written, err
		}
		name := strings.TrimPrefix(strings.TrimSuffix(hdr.Name, "/"), "./")
		if name == "" || name == "." {
			continue
		}
		if !pathWanted(name, want) {
			continue
		}
		if hdr.Typeflag == tar.TypeDir || strings.HasSuffix(hdr.Name, "/") {
			continue
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue
		}
		w, err := zw.Create(name)
		if err != nil {
			return written, err
		}
		if _, err := io.Copy(w, tr); err != nil {
			return written, err
		}
		written++
	}
	return written, nil
}

func pathWanted(name string, want map[string]bool) bool {
	name = strings.Trim(name, "/")
	if want[name] {
		return true
	}
	for w := range want {
		if name == w || strings.HasPrefix(name, w+"/") || strings.HasPrefix(w, name+"/") {
			return true
		}
	}
	return false
}
