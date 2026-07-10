package rsyncbackup

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/boomerang-backup/boomerang/internal/archive"
	"github.com/boomerang-backup/boomerang/internal/backup"
	"github.com/boomerang-backup/boomerang/internal/remote"
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
	if _, err := exec.LookPath("rsync"); err != nil {
		return nil, fmt.Errorf("rsync not found on appliance")
	}
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return nil, err
	}

	staging := filepath.Join(outDir, ".staging")
	_ = os.RemoveAll(staging)
	if err := os.MkdirAll(staging, 0o700); err != nil {
		return nil, err
	}
	defer os.RemoveAll(staging)

	sshArgs, cleanup, err := sshRsyncArgs(target)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	roots := normalizeRoots(target)
	if opt.BaseManifest != nil && opt.BaseVersionID != "" {
		log("note: RSYNC always captures a full snapshot (not incremental on disk)")
	}
	var hadPartial bool
	var rsyncOutput strings.Builder
	for _, root := range roots {
		src := fmt.Sprintf("%s@%s:%s/", target.Username, target.Host, strings.TrimSuffix(root, "/")+"/")
		args := []string{"-az", "--numeric-ids", "--ignore-errors", "-e", strings.Join(sshArgs, " ")}
		for _, ex := range opt.ExcludePaths {
			ex = strings.TrimSpace(ex)
			if ex != "" {
				args = append(args, "--exclude="+ex)
			}
		}
		dest := filepath.Join(staging, strings.TrimPrefix(root, "/"))
		if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
			return nil, err
		}
		args = append(args, src, dest+"/")
		log(fmt.Sprintf("rsync %s", root))
		cmd := exec.Command("rsync", args...)
		partial, out, err := runRsync(cmd)
		if err != nil {
			return nil, err
		}
		if partial {
			hadPartial = true
			rsyncOutput.WriteString(out)
			log(summarizeRsyncWarnings(out, root))
		}
	}
	if hadPartial {
		n, err := stagingFileCount(staging)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			return nil, fmt.Errorf("rsync: no files could be read (permission denied on remote)")
		}
		if skipped, err := writeSkippedLog(outDir, rsyncOutput.String(), roots[0]); err != nil {
			log(fmt.Sprintf("warning: could not write skipped log: %v", err))
		} else if skipped > 0 {
			log(fmt.Sprintf("skipped: %d path(s) listed in %s", skipped, backup.SkippedLogFile))
		}
	}

	tarPath := archive.FilesBlobPath(outDir)
	if err := archive.TarDirectory(staging, tarPath); err != nil {
		return nil, err
	}

	// Build manifest via SFTP-style walk of staging (local)
	manifest, files, bytes, err := manifestFromStaging(staging, target, opt)
	if err != nil {
		return nil, err
	}
	if err := backup.WriteFileManifest(outDir, manifest); err != nil {
		return nil, err
	}
	meta, _ := json.MarshalIndent(map[string]any{
		"protocol": "rsync", "host": target.Host, "paths": roots,
		"kind": manifest.Kind, "baseVersionId": opt.BaseVersionID,
		"files": files, "bytes": bytes, "finished": time.Now().UTC().Format(time.RFC3339),
	}, "", "  ")
	_ = os.WriteFile(filepath.Join(outDir, "meta.json"), meta, 0o600)
	log(fmt.Sprintf("done: %d files, %d bytes", files, bytes))
	if hadPartial {
		log("status: partial — some remote paths were not readable (see skipped.log if present)")
	} else {
		log("status: complete")
	}
	return &Result{Bytes: bytes, Files: files, Manifest: *manifest}, nil
}

func sshRsyncArgs(target remote.FileTarget) ([]string, func(), error) {
	var cleanups []func()
	cleanup := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}
	port := target.Port
	if port == 0 {
		port = 22
	}
	sshCmd := []string{"ssh", "-p", fmt.Sprintf("%d", port)}
	if target.SSHHostKey != "" {
		kh, khCleanup, err := remote.KnownHostsFile(target.Host, port, target.SSHHostKey)
		if err != nil {
			return nil, cleanup, err
		}
		cleanups = append(cleanups, khCleanup)
		sshCmd = append(sshCmd, "-o", "StrictHostKeyChecking=yes", "-o", "UserKnownHostsFile="+kh)
	} else {
		sshCmd = append(sshCmd, "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null")
	}
	if target.AuthMode == "key" && target.Secret.PrivateKey != "" {
		f, err := os.CreateTemp("", "boomerang-rsync-key-*")
		if err != nil {
			return nil, cleanup, err
		}
		if _, err := f.WriteString(target.Secret.PrivateKey); err != nil {
			_ = f.Close()
			return nil, cleanup, err
		}
		_ = f.Chmod(0o600)
		_ = f.Close()
		cleanups = append(cleanups, func() { _ = os.Remove(f.Name()) })
		sshCmd = append(sshCmd, "-i", f.Name())
	}
	return sshCmd, cleanup, nil
}

func normalizeRoots(t remote.FileTarget) []string {
	// reuse sftp logic via include paths
	ft := t
	if len(ft.IncludePaths) == 0 && ft.RemoteRoot != "" {
		ft.IncludePaths = []string{ft.RemoteRoot}
	}
	if len(ft.IncludePaths) == 0 {
		ft.IncludePaths = []string{"/"}
	}
	return ft.IncludePaths
}

func manifestFromStaging(staging string, target remote.FileTarget, opt Options) (*backup.FileManifest, int, int64, error) {
	kind := "full"
	roots := normalizeRoots(target)
	manifest := &backup.FileManifest{
		Root: staging, Paths: roots, Kind: kind,
		Entries: []backup.ManifestEntry{},
	}
	var files int
	var bytes int64
	_ = filepath.Walk(staging, func(full string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		if !fi.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(staging, full)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if backup.Excluded(rel, opt.ExcludePaths) {
			return nil
		}
		entry := backup.ManifestEntry{
			Path: rel, Size: fi.Size(), Mode: fi.Mode().String(),
			Mtime: fi.ModTime().UTC().Format(time.RFC3339), IsDir: false,
		}
		manifest.Entries = append(manifest.Entries, entry)
		files++
		bytes += fi.Size()
		return nil
	})
	return manifest, files, bytes, nil
}
