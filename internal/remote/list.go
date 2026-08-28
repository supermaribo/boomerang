package remote

import (
	"fmt"
	"net"
	"os"
	"path"
	"strings"

	"github.com/jlaffaye/ftp"
	"github.com/pkg/sftp"

	"github.com/boomerang-backup/boomerang/internal/backup"
)

// RemoteFile is a regular file found while walking a backup target.
type RemoteFile struct {
	Rel  string
	Size int64
}

// FileList is a remote walk used to check backup completeness.
type FileList struct {
	Files       []RemoteFile
	Unreadable  []string
	WalkedRoots []string
}

// ListRegularFiles walks include paths and returns files the backup user can
// read, using the same relative-path scheme as the backup protocol.
func ListRegularFiles(t FileTarget, excludes []string) (FileList, error) {
	switch t.Protocol {
	case "sftp", "rsync":
		return listSFTP(t, excludes)
	case "ftp", "ftps":
		return listFTP(t, excludes)
	default:
		return FileList{}, fmt.Errorf("unsupported protocol %q", t.Protocol)
	}
}

func listRoots(t FileTarget) []string {
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
	if base == "" {
		return "/"
	}
	return base
}

func relForProtocol(protocol, base, full string) (string, error) {
	full = path.Clean(full)
	if protocol == "rsync" {
		return strings.TrimPrefix(full, "/"), nil
	}
	base = path.Clean(base)
	if base == "/" {
		return strings.TrimPrefix(full, "/"), nil
	}
	if full == base {
		return ".", nil
	}
	prefix := strings.TrimSuffix(base, "/") + "/"
	if !strings.HasPrefix(full, prefix) {
		return "", fmt.Errorf("outside root")
	}
	return strings.TrimPrefix(full, prefix), nil
}

func listSFTP(t FileTarget, excludes []string) (FileList, error) {
	client, err := DialSSH(t.Host, t.Port, t.Username, t.AuthMode, t.Secret, HostKeyTrust{
		KnownFingerprint: t.SSHHostKey,
		Pin:              t.PinHostKey,
	})
	if err != nil {
		return FileList{}, err
	}
	defer client.Close()
	sc, err := sftp.NewClient(client)
	if err != nil {
		return FileList{}, fmt.Errorf("sftp: %w", err)
	}
	defer sc.Close()

	roots := listRoots(t)
	base := archiveBase(t.RemoteRoot, roots)
	out := FileList{WalkedRoots: roots}
	seen := map[string]bool{}
	unread := map[string]bool{}

	var walk func(full string)
	walk = func(full string) {
		entries, err := sc.ReadDir(full)
		if err != nil {
			rel, relErr := relForProtocol(t.Protocol, base, full)
			if relErr != nil {
				rel = full
			}
			if !unread[rel] {
				unread[rel] = true
				out.Unreadable = append(out.Unreadable, rel+"/")
			}
			return
		}
		for _, fi := range entries {
			name := fi.Name()
			if name == "." || name == ".." {
				continue
			}
			child := path.Join(full, name)
			rel, err := relForProtocol(t.Protocol, base, child)
			if err != nil || rel == "." {
				continue
			}
			if backup.Excluded(rel, excludes) {
				continue
			}
			if fi.IsDir() {
				walk(child)
				continue
			}
			if !fi.Mode().IsRegular() {
				continue
			}
			if seen[rel] {
				continue
			}
			seen[rel] = true
			out.Files = append(out.Files, RemoteFile{Rel: rel, Size: fi.Size()})
		}
	}

	for _, root := range roots {
		fi, err := sc.Stat(root)
		if err != nil {
			if os.IsNotExist(err) {
				out.Unreadable = append(out.Unreadable, root+"/")
				continue
			}
			rel, relErr := relForProtocol(t.Protocol, base, root)
			if relErr != nil {
				rel = root
			}
			out.Unreadable = append(out.Unreadable, rel+"/")
			continue
		}
		if fi.IsDir() {
			walk(root)
			continue
		}
		if fi.Mode().IsRegular() {
			rel, err := relForProtocol(t.Protocol, base, root)
			if err == nil && rel != "." && !backup.Excluded(rel, excludes) && !seen[rel] {
				seen[rel] = true
				out.Files = append(out.Files, RemoteFile{Rel: rel, Size: fi.Size()})
			}
		}
	}
	return out, nil
}

func listFTP(t FileTarget, excludes []string) (FileList, error) {
	port := t.Port
	if port == 0 {
		port = 21
	}
	c, err := ftp.Dial(net.JoinHostPort(t.Host, fmt.Sprintf("%d", port)), ftpDialOpts(t.Protocol == "ftps")...)
	if err != nil {
		return FileList{}, err
	}
	defer c.Quit()
	if err := c.Login(t.Username, t.Secret.Password); err != nil {
		return FileList{}, fmt.Errorf("login: %w", err)
	}

	roots := listRoots(t)
	base := archiveBase(t.RemoteRoot, roots)
	out := FileList{WalkedRoots: roots}
	seen := map[string]bool{}

	var walk func(full string)
	walk = func(full string) {
		entries, err := c.List(full)
		if err != nil {
			rel, relErr := relForProtocol(t.Protocol, base, full)
			if relErr != nil {
				rel = full
			}
			out.Unreadable = append(out.Unreadable, rel+"/")
			return
		}
		for _, e := range entries {
			name := e.Name
			if name == "." || name == ".." {
				continue
			}
			child := path.Join(full, name)
			rel, err := relForProtocol(t.Protocol, base, child)
			if err != nil || rel == "." {
				continue
			}
			if backup.Excluded(rel, excludes) {
				continue
			}
			if e.Type == ftp.EntryTypeFolder {
				walk(child)
				continue
			}
			if seen[rel] {
				continue
			}
			seen[rel] = true
			out.Files = append(out.Files, RemoteFile{Rel: rel, Size: int64(e.Size)})
		}
	}
	for _, root := range roots {
		walk(root)
	}
	return out, nil
}
