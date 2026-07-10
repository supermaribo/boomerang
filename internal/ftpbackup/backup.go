package ftpbackup

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/boomerang-backup/boomerang/internal/archive"
	"github.com/boomerang-backup/boomerang/internal/backup"
	"github.com/boomerang-backup/boomerang/internal/crypto"
	"github.com/boomerang-backup/boomerang/internal/remote"
	"github.com/jlaffaye/ftp"
	"github.com/klauspost/compress/zstd"
)

type Logger func(string)

type Options struct {
	ExcludePaths  []string
	BaseManifest  *backup.FileManifest
	BaseVersionID string
}

type Result struct {
	Bytes    int64
	Files    int
	Manifest backup.FileManifest
}

func Backup(target remote.FileTarget, outDir string, opt Options, log Logger) (*Result, error) {
	if log == nil {
		log = func(string) {}
	}
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return nil, err
	}
	c, err := dialFTP(target)
	if err != nil {
		return nil, err
	}
	defer c.Quit()

	tarPath := archive.FilesBlobPath(outDir)
	f, err := os.Create(tarPath)
	if err != nil {
		return nil, err
	}
	zw, err := zstd.NewWriter(f)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	tw := tar.NewWriter(zw)

	roots := normalizeRoots(target)
	base := archiveBase(target.RemoteRoot, roots)
	baseIdx := backup.EntryIndex(opt.BaseManifest)
	kind := "full"
	if opt.BaseManifest != nil && opt.BaseVersionID != "" {
		kind = "incremental"
	}
	manifest := backup.FileManifest{
		Root: base, Paths: roots, Kind: kind, BaseVersionID: opt.BaseVersionID,
		Entries: []backup.ManifestEntry{},
	}
	var total int64
	files := 0
	skipped := 0

	var walkDir func(remotePath, relPrefix string)
	walkDir = func(remotePath, relPrefix string) {
		entries, err := c.List(remotePath)
		if err != nil {
			log(fmt.Sprintf("walk warn: %s: %v", remotePath, err))
			return
		}
		for _, e := range entries {
			name := e.Name
			if name == "." || name == ".." {
				continue
			}
			child := path.Join(remotePath, name)
			rel := name
			if relPrefix != "" {
				rel = relPrefix + "/" + name
			}
			if backup.Excluded(rel, opt.ExcludePaths) {
				continue
			}
			if e.Type == ftp.EntryTypeFolder {
				walkDir(child, rel)
				continue
			}
			entry := backup.ManifestEntry{
				Path: rel, Size: int64(e.Size), Mode: "regular",
				Mtime: e.Time.UTC().Format(time.RFC3339), IsDir: false,
			}
			if kind == "incremental" && !backup.Changed(entry, baseIdx) {
				continue
			}
			manifest.Entries = append(manifest.Entries, entry)
			hdr := &tar.Header{Name: rel, Mode: 0o644, Size: int64(e.Size), ModTime: e.Time}
			if err := tw.WriteHeader(hdr); err != nil {
				return
			}
			r, err := c.Retr(child)
			if err != nil {
				log(fmt.Sprintf("skip %s: %v", rel, err))
				skipped++
				continue
			}
			n, err := io.Copy(tw, r)
			_ = r.Close()
			if err != nil {
				log(fmt.Sprintf("skip copy %s: %v", rel, err))
				skipped++
				continue
			}
			total += n
			files++
		}
	}

	for _, root := range roots {
		log(fmt.Sprintf("backing up %s", root))
		walkDir(root, strings.TrimPrefix(path.Clean(root), "/"))
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	if skipped > 0 {
		return nil, fmt.Errorf("backup incomplete: %d file(s) skipped", skipped)
	}
	if err := backup.WriteFileManifest(outDir, &manifest); err != nil {
		return nil, err
	}
	meta, _ := json.MarshalIndent(map[string]any{
		"protocol": target.Protocol, "host": target.Host,
		"kind": kind, "files": files, "bytes": total,
		"finished": time.Now().UTC().Format(time.RFC3339),
	}, "", "  ")
	_ = os.WriteFile(filepath.Join(outDir, "meta.json"), meta, 0o600)
	log(fmt.Sprintf("done: %d files, %d bytes", files, total))
	return &Result{Bytes: total, Files: files, Manifest: manifest}, nil
}

func RestoreSelected(box *crypto.Box, target remote.FileTarget, versionDir string, paths []string, log Logger) (int, error) {
	if log == nil {
		log = func(string) {}
	}
	want := map[string]bool{}
	for _, p := range paths {
		p = strings.Trim(strings.TrimPrefix(p, "./"), "/")
		if p != "" {
			want[p] = true
		}
	}
	c, err := dialFTP(target)
	if err != nil {
		return 0, err
	}
	defer c.Quit()

	root := strings.TrimSuffix(target.RemoteRoot, "/")
	if root == "" {
		root = "/"
	}

	blobPath := archive.FilesBlobPath(versionDir)
	rc, zr, err := archive.OpenZstd(box, blobPath)
	if err != nil {
		return 0, err
	}
	defer rc.Close()
	defer zr.Close()
	tr := tar.NewReader(zr)
	restored := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return restored, err
		}
		name := strings.Trim(strings.TrimPrefix(hdr.Name, "./"), "/")
		if !pathWanted(name, want) {
			continue
		}
		if hdr.Typeflag == tar.TypeDir {
			continue
		}
		remotePath := joinRemote(root, name)
		parent := path.Dir(remotePath)
		_ = c.MakeDir(parent)
		if err := c.Stor(remotePath, tr); err != nil {
			return restored, err
		}
		restored++
		log(fmt.Sprintf("restored %s", name))
	}
	return restored, nil
}

func dialFTP(target remote.FileTarget) (*ftp.ServerConn, error) {
	addr := net.JoinHostPort(target.Host, fmt.Sprintf("%d", target.Port))
	opts := []ftp.DialOption{ftp.DialWithTimeout(15 * time.Second)}
	if target.Protocol == "ftps" {
		opts = append(opts, ftp.DialWithExplicitTLS(nil))
	}
	c, err := ftp.Dial(addr, opts...)
	if err != nil {
		return nil, err
	}
	if err := c.Login(target.Username, target.Secret.Password); err != nil {
		_ = c.Quit()
		return nil, err
	}
	return c, nil
}

func normalizeRoots(t remote.FileTarget) []string {
	var roots []string
	for _, p := range t.IncludePaths {
		p = path.Clean(strings.TrimSpace(p))
		if p != "" && p != "." {
			roots = append(roots, p)
		}
	}
	if len(roots) == 0 {
		r := t.RemoteRoot
		if r == "" {
			r = "/"
		}
		roots = []string{path.Clean(r)}
	}
	return roots
}

func archiveBase(remoteRoot string, roots []string) string {
	if len(roots) == 1 {
		return roots[0]
	}
	return path.Clean(remoteRoot)
}

func pathWanted(name string, want map[string]bool) bool {
	if want[name] {
		return true
	}
	for w := range want {
		if strings.HasPrefix(name, w+"/") || strings.HasPrefix(w, name+"/") {
			return true
		}
	}
	return false
}

func joinRemote(root, rel string) string {
	if root == "/" {
		return "/" + rel
	}
	return root + "/" + rel
}
