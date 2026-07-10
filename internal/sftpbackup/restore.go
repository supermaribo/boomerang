package sftpbackup

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/boomerang-backup/boomerang/internal/archive"
	"github.com/boomerang-backup/boomerang/internal/crypto"
	"github.com/boomerang-backup/boomerang/internal/remote"
	"github.com/pkg/sftp"
)

// RestoreSelected extracts paths from backup archive chain and writes via SFTP.
func RestoreSelected(box *crypto.Box, target remote.FileTarget, versionDir string, paths []string, log Logger) (int, error) {
	if log == nil {
		log = func(string) {}
	}
	want := normalizeWant(paths)
	if len(want) == 0 {
		return 0, fmt.Errorf("no paths selected")
	}

	client, err := remote.DialSSH(target.Host, target.Port, target.Username, target.AuthMode, target.Secret, remote.HostKeyTrust{
		KnownFingerprint: target.SSHHostKey,
		Pin:              target.PinHostKey,
	})
	if err != nil {
		return 0, err
	}
	defer client.Close()
	sc, err := sftp.NewClient(client)
	if err != nil {
		return 0, fmt.Errorf("sftp: %w", err)
	}
	defer sc.Close()

	root := strings.TrimSuffix(target.RemoteRoot, "/")
	if root == "" {
		root = "/"
	}

	restored := map[string]bool{}
	total := 0
	blobPath := archive.FilesBlobPath(versionDir)
	rc, zr, err := archive.OpenZstd(box, blobPath)
	if err != nil {
		return 0, err
	}
	defer rc.Close()
	defer zr.Close()
	n, err := restoreFromTar(tar.NewReader(zr), sc, root, want, restored, log)
	if err != nil {
		return total, err
	}
	total += n

	// incremental chain: try base versions for paths not yet restored
	m, err := ReadManifest(versionDir)
	if err == nil && m.BaseVersionID != "" {
		log(fmt.Sprintf("note: incremental restore may need base version %s for unchanged files", m.BaseVersionID))
	}

	log(fmt.Sprintf("done: %d entries restored", total))
	return total, nil
}

func restoreFromTar(tr *tar.Reader, sc *sftp.Client, root string, want, restored map[string]bool, log Logger) (int, error) {
	count := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, err
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		name = strings.TrimSuffix(name, "/")
		if name == "" || name == "." {
			continue
		}
		if !pathWanted(name, want) {
			continue
		}
		remotePath := joinRemote(root, name)
		if hdr.Typeflag == tar.TypeDir || strings.HasSuffix(hdr.Name, "/") {
			if err := sc.MkdirAll(remotePath); err != nil {
				log(fmt.Sprintf("mkdir %s: %v", remotePath, err))
				continue
			}
			restored[name] = true
			count++
			continue
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue
		}
		parent := path.Dir(remotePath)
		if err := sc.MkdirAll(parent); err != nil {
			return count, fmt.Errorf("mkdir %s: %w", parent, err)
		}
		wf, err := sc.Create(remotePath)
		if err != nil {
			_ = sc.Remove(remotePath)
			wf, err = sc.Create(remotePath)
			if err != nil {
				return count, fmt.Errorf("create %s: %w", remotePath, err)
			}
		}
		_, err = io.Copy(wf, tr)
		_ = wf.Close()
		if err != nil {
			return count, fmt.Errorf("write %s: %w", remotePath, err)
		}
		if hdr.Mode != 0 {
			_ = sc.Chmod(remotePath, os.FileMode(hdr.Mode))
		}
		restored[name] = true
		count++
		log(fmt.Sprintf("restored %s", name))
	}
	return count, nil
}

func normalizeWant(paths []string) map[string]bool {
	out := map[string]bool{}
	for _, p := range paths {
		p = strings.TrimPrefix(p, "./")
		p = strings.Trim(p, "/")
		if p == "" || p == "." {
			continue
		}
		out[p] = true
	}
	return out
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

func joinRemote(root, rel string) string {
	rel = strings.TrimPrefix(rel, "/")
	if root == "/" {
		return "/" + rel
	}
	return root + "/" + rel
}
