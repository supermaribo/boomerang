package filebackup

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/boomerang-backup/boomerang/internal/archive"
	"github.com/boomerang-backup/boomerang/internal/backup"
	"github.com/boomerang-backup/boomerang/internal/crypto"
	"github.com/boomerang-backup/boomerang/internal/ftpbackup"
	"github.com/boomerang-backup/boomerang/internal/remote"
	"github.com/boomerang-backup/boomerang/internal/rsyncbackup"
	"github.com/boomerang-backup/boomerang/internal/sftpbackup"
	"github.com/boomerang-backup/boomerang/internal/store"
)

type Logger func(string)

type Options struct {
	Box           *crypto.Box
	ExcludePaths  []string
	BaseManifest  *backup.FileManifest
	BaseVersionID string
}

type Result struct {
	Bytes    int64
	Files    int
	Manifest backup.FileManifest
	Skipped  bool
}

func Backup(target remote.FileTarget, outDir string, opt Options, log Logger) (*Result, error) {
	if log == nil {
		log = func(string) {}
	}
	var res *Result
	var err error
	switch target.Protocol {
	case "sftp":
		sr, e := sftpbackup.Backup(target, outDir, sftpbackup.Options{
			ExcludePaths: opt.ExcludePaths, BaseManifest: opt.BaseManifest, BaseVersionID: opt.BaseVersionID,
		}, sftpbackup.Logger(log))
		if e != nil {
			return nil, e
		}
		res = &Result{Bytes: sr.Bytes, Files: sr.Files, Manifest: sr.Manifest}
	case "rsync":
		sr, e := rsyncbackup.Backup(target, outDir, rsyncbackup.Options{
			ExcludePaths: opt.ExcludePaths, BaseManifest: opt.BaseManifest, BaseVersionID: opt.BaseVersionID,
		}, rsyncbackup.Logger(log))
		if e != nil {
			return nil, e
		}
		res = &Result{Bytes: sr.Bytes, Files: sr.Files, Manifest: sr.Manifest, Skipped: sr.Skipped}
	case "ftp", "ftps":
		sr, e := ftpbackup.Backup(target, outDir, ftpbackup.Options{
			ExcludePaths: opt.ExcludePaths, BaseManifest: opt.BaseManifest, BaseVersionID: opt.BaseVersionID,
		}, ftpbackup.Logger(log))
		if e != nil {
			return nil, e
		}
		res = &Result{Bytes: sr.Bytes, Files: sr.Files, Manifest: sr.Manifest}
	default:
		return nil, fmt.Errorf("unsupported protocol %q", target.Protocol)
	}

	if res.Skipped {
		return res, nil
	}

	if opt.Box != nil {
		plain := archive.FilesBlobPath(outDir)
		if err := opt.Box.EncryptFile(plain, crypto.EncryptedPath(plain)); err != nil {
			return nil, fmt.Errorf("encrypt backup: %w", err)
		}
		_ = os.Remove(plain)
		res.Manifest.Encrypted = true
		if err := backup.WriteFileManifest(outDir, &res.Manifest); err != nil {
			return nil, err
		}
	}
	return res, err
}

func RestoreSelected(st *store.Store, box *crypto.Box, target remote.FileTarget, versionID, versionDir string, paths []string, log Logger) (int, error) {
	chain, err := VersionChain(st, versionID, versionDir)
	if err != nil {
		return 0, err
	}
	// oldest (full) first
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	total := 0
	for _, vid := range chain {
		dir := versionDir
		if vid != versionID {
			v, err := st.GetVersion(vid)
			if err != nil || v == nil {
				continue
			}
			dir = v.PathOnDisk
		}
		n, err := restoreOne(box, target, dir, paths, log)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func restoreOne(box *crypto.Box, target remote.FileTarget, versionDir string, paths []string, log Logger) (int, error) {
	switch target.Protocol {
	case "sftp", "rsync":
		return sftpbackup.RestoreSelected(box, target, versionDir, paths, sftpbackup.Logger(log))
	case "ftp", "ftps":
		return ftpbackup.RestoreSelected(box, target, versionDir, paths, ftpbackup.Logger(log))
	default:
		return 0, fmt.Errorf("unsupported protocol %q", target.Protocol)
	}
}

// LoadMergedManifest walks incremental chain for browse/restore.
func LoadMergedManifest(st *store.Store, targetType, targetID, versionID, versionDir string) (*backup.FileManifest, error) {
	merged := map[string]backup.ManifestEntry{}
	var root string
	var paths []string
	seen := map[string]bool{}
	curID := versionID
	for curID != "" && !seen[curID] {
		seen[curID] = true
		dir := versionDir
		if curID != versionID {
			v, err := st.GetVersion(curID)
			if err != nil || v == nil {
				break
			}
			dir = v.PathOnDisk
		}
		m, err := backup.ReadFileManifest(dir)
		if err != nil {
			return nil, err
		}
		if root == "" {
			root = m.Root
			paths = m.Paths
		}
		for _, e := range m.Entries {
			if e.IsDir {
				continue
			}
			if _, ok := merged[e.Path]; !ok {
				merged[e.Path] = e
			}
		}
		curID = m.BaseVersionID
	}
	entries := make([]backup.ManifestEntry, 0, len(merged))
	for _, e := range merged {
		entries = append(entries, e)
	}
	m, _ := backup.ReadFileManifest(versionDir)
	kind := "full"
	base := ""
	if m != nil {
		kind = m.Kind
		base = m.BaseVersionID
	}
	return &backup.FileManifest{
		Root: root, Paths: paths, Kind: kind, BaseVersionID: base,
		Entries: entries,
	}, nil
}

// VersionChain returns version IDs from tip back to full root.
func VersionChain(st *store.Store, versionID, versionDir string) ([]string, error) {
	var chain []string
	seen := map[string]bool{}
	curID := versionID
	dir := versionDir
	for curID != "" && !seen[curID] {
		seen[curID] = true
		chain = append(chain, curID)
		m, err := backup.ReadFileManifest(dir)
		if err != nil || m.BaseVersionID == "" {
			break
		}
		v, err := st.GetVersion(m.BaseVersionID)
		if err != nil || v == nil {
			break
		}
		curID = v.ID
		dir = v.PathOnDisk
	}
	return chain, nil
}

func ArchivePath(versionDir string) string {
	return filepath.Join(versionDir, archive.FilesArchive)
}
