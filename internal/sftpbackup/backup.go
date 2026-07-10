package sftpbackup

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/boomerang-backup/boomerang/internal/archive"
	"github.com/boomerang-backup/boomerang/internal/backup"
	"github.com/boomerang-backup/boomerang/internal/remote"
	"github.com/klauspost/compress/zstd"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
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

	client, err := dial(target)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	sc, err := sftp.NewClient(client)
	if err != nil {
		return nil, fmt.Errorf("sftp: %w", err)
	}
	defer sc.Close()

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
	incremental := opt.BaseManifest != nil && opt.BaseVersionID != ""
	kind := "full"
	if incremental {
		kind = "incremental"
		log(fmt.Sprintf("incremental backup (base %s)", opt.BaseVersionID))
	}

	manifest := backup.FileManifest{
		Root: base, Paths: roots, Kind: kind, BaseVersionID: opt.BaseVersionID,
		Entries: []backup.ManifestEntry{},
	}
	var total int64
	files := 0
	skipped := 0
	var walkErr error

	var walkDir func(full string)
	walkDir = func(full string) {
		if walkErr != nil {
			return
		}
		entries, err := sc.ReadDir(full)
		if err != nil {
			log(fmt.Sprintf("walk warn: %s: %v", full, err))
			return
		}
		for _, fi := range entries {
			name := fi.Name()
			if name == "." || name == ".." {
				continue
			}
			child := path.Join(full, name)
			rel, err := relPath(base, child)
			if err != nil {
				continue
			}
			if rel == "." || rel == "" {
				rel = name
			}
			if backup.Excluded(rel, opt.ExcludePaths) {
				continue
			}
			if !fi.IsDir() && !fi.Mode().IsRegular() {
				log(fmt.Sprintf("skip non-regular: %s (%s)", rel, fi.Mode().String()))
				continue
			}

			entry := backup.ManifestEntry{
				Path:  rel,
				Size:  fi.Size(),
				Mode:  fi.Mode().String(),
				Mtime: fi.ModTime().UTC().Format(time.RFC3339),
				IsDir: fi.IsDir(),
			}

			if fi.IsDir() {
				manifest.Entries = append(manifest.Entries, entry)
				walkDir(child)
				continue
			}

			if incremental && !backup.Changed(entry, baseIdx) {
				continue
			}

			manifest.Entries = append(manifest.Entries, entry)
			hdr, err := tar.FileInfoHeader(fi, "")
			if err != nil {
				log(fmt.Sprintf("skip header %s: %v", rel, err))
				continue
			}
			hdr.Name = rel
			if err := tw.WriteHeader(hdr); err != nil {
				walkErr = err
				return
			}
			rf, err := sc.Open(child)
			if err != nil {
				log(fmt.Sprintf("skip open %s: %v", rel, err))
				skipped++
				continue
			}
			n, err := io.Copy(tw, rf)
			_ = rf.Close()
			if err != nil {
				log(fmt.Sprintf("skip copy %s: %v", rel, err))
				skipped++
				continue
			}
			total += n
			files++
			if files%50 == 0 {
				log(fmt.Sprintf("copied %d files…", files))
			}
		}
	}

	seen := map[string]bool{}
	for _, root := range roots {
		root = path.Clean(root)
		if seen[root] {
			continue
		}
		seen[root] = true
		log(fmt.Sprintf("backing up %s", root))
		fi, err := sc.Stat(root)
		if err != nil {
			log(fmt.Sprintf("skip missing %s: %v", root, err))
			continue
		}
		if fi.IsDir() {
			walkDir(root)
		} else if fi.Mode().IsRegular() {
			rel, _ := relPath(base, root)
			if rel == "" {
				rel = path.Base(root)
			}
			if backup.Excluded(rel, opt.ExcludePaths) {
				continue
			}
			entry := backup.ManifestEntry{
				Path: rel, Size: fi.Size(), Mode: fi.Mode().String(),
				Mtime: fi.ModTime().UTC().Format(time.RFC3339), IsDir: false,
			}
			if incremental && !backup.Changed(entry, baseIdx) {
				continue
			}
			manifest.Entries = append(manifest.Entries, entry)
			hdr, _ := tar.FileInfoHeader(fi, "")
			hdr.Name = rel
			_ = tw.WriteHeader(hdr)
			rf, err := sc.Open(root)
			if err == nil {
				n, _ := io.Copy(tw, rf)
				_ = rf.Close()
				total += n
				files++
			}
		}
	}
	if walkErr != nil {
		return nil, walkErr
	}
	if skipped > 0 {
		return nil, fmt.Errorf("backup incomplete: %d file(s) skipped", skipped)
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

	if err := backup.WriteFileManifest(outDir, &manifest); err != nil {
		return nil, err
	}
	meta, _ := json.MarshalIndent(map[string]any{
		"protocol": target.Protocol, "host": target.Host, "root": base, "paths": roots,
		"kind": kind, "baseVersionId": opt.BaseVersionID,
		"files": files, "bytes": total, "finished": time.Now().UTC().Format(time.RFC3339),
	}, "", "  ")
	_ = os.WriteFile(filepath.Join(outDir, "meta.json"), meta, 0o600)

	log(fmt.Sprintf("done: %d files, %d bytes (%s)", files, total, kind))
	return &Result{Bytes: total, Files: files, Manifest: manifest}, nil
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
		root := t.RemoteRoot
		if root == "" {
			root = "/"
		}
		roots = []string{path.Clean(root)}
	}
	return roots
}

func archiveBase(remoteRoot string, roots []string) string {
	if len(roots) == 1 {
		return roots[0]
	}
	base := path.Clean(remoteRoot)
	for _, r := range roots {
		if r != base && !strings.HasPrefix(r, strings.TrimSuffix(base, "/")+"/") {
			return commonParent(roots)
		}
	}
	if base == "" {
		return "/"
	}
	return base
}

func commonParent(paths []string) string {
	if len(paths) == 0 {
		return "/"
	}
	parts := strings.Split(path.Clean(paths[0]), "/")
	for _, p := range paths[1:] {
		other := strings.Split(path.Clean(p), "/")
		n := len(parts)
		if len(other) < n {
			n = len(other)
		}
		i := 0
		for i < n && parts[i] == other[i] {
			i++
		}
		parts = parts[:i]
	}
	joined := strings.Join(parts, "/")
	if joined == "" {
		return "/"
	}
	return joined
}

func dial(t remote.FileTarget) (*ssh.Client, error) {
	return remote.DialSSH(t.Host, t.Port, t.Username, t.AuthMode, t.Secret, remote.HostKeyTrust{
		KnownFingerprint: t.SSHHostKey,
		Pin:              t.PinHostKey,
	})
}

func relPath(root, full string) (string, error) {
	root = path.Clean(root)
	full = path.Clean(full)
	if root == "/" {
		return strings.TrimPrefix(full, "/"), nil
	}
	if full == root {
		return ".", nil
	}
	prefix := root + "/"
	if !strings.HasPrefix(full, prefix) {
		return "", fmt.Errorf("outside root")
	}
	return strings.TrimPrefix(full, prefix), nil
}

// ReadManifest loads manifest.json (delegates to backup package).
func ReadManifest(versionDir string) (*backup.FileManifest, error) {
	return backup.ReadFileManifest(versionDir)
}
