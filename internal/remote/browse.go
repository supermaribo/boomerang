package remote

import (
	"fmt"
	"net"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/pkg/sftp"
)

type DirEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

type BrowseResult struct {
	Path    string     `json:"path"`
	Parent  string     `json:"parent"`
	Entries []DirEntry `json:"entries"`
}

// Browse lists a remote directory for path selection during setup.
func Browse(t FileTarget, remotePath string) (*BrowseResult, error) {
	switch t.Protocol {
	case "sftp", "rsync":
		return browseSFTP(t, remotePath)
	case "ftp", "ftps":
		return browseFTP(t, remotePath)
	default:
		return nil, fmt.Errorf("unsupported protocol %q", t.Protocol)
	}
}

func browseSFTP(t FileTarget, remotePath string) (*BrowseResult, error) {
	client, err := DialSSH(t.Host, t.Port, t.Username, t.AuthMode, t.Secret, HostKeyTrust{
		KnownFingerprint: t.SSHHostKey,
		Pin:              t.PinHostKey,
	})
	if err != nil {
		return nil, err
	}
	defer client.Close()
	sc, err := sftp.NewClient(client)
	if err != nil {
		return nil, fmt.Errorf("sftp: %w", err)
	}
	defer sc.Close()

	p := strings.TrimSpace(remotePath)
	if p == "" || p == "." {
		wd, err := sc.Getwd()
		if err != nil || wd == "" {
			wd = "/"
		}
		p = wd
	}
	p = path.Clean(p)
	if !path.IsAbs(p) {
		wd, _ := sc.Getwd()
		if wd == "" {
			wd = "/"
		}
		p = path.Join(wd, p)
	}

	fi, err := sc.Stat(p)
	if err != nil {
		return nil, fmt.Errorf("path %q: %w", p, err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", p)
	}

	entries, err := sc.ReadDir(p)
	if err != nil {
		return nil, err
	}
	out := make([]DirEntry, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if name == "." || name == ".." {
			continue
		}
		child := path.Join(p, name)
		out = append(out, DirEntry{
			Name:  name,
			Path:  child,
			IsDir: e.IsDir(),
			Size:  e.Size(),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	parent := path.Dir(p)
	if parent == p {
		parent = ""
	}
	return &BrowseResult{Path: p, Parent: parent, Entries: out}, nil
}

func browseFTP(t FileTarget, remotePath string) (*BrowseResult, error) {
	addr := net.JoinHostPort(t.Host, fmt.Sprintf("%d", t.Port))
	opts := []ftp.DialOption{ftp.DialWithTimeout(15 * time.Second)}
	if t.Protocol == "ftps" {
		opts = append(opts, ftp.DialWithExplicitTLS(nil))
	}
	c, err := ftp.Dial(addr, opts...)
	if err != nil {
		return nil, err
	}
	defer c.Quit()
	if err := c.Login(t.Username, t.Secret.Password); err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}

	p := strings.TrimSpace(remotePath)
	if p == "" || p == "." {
		cur, err := c.CurrentDir()
		if err != nil || cur == "" {
			cur = "/"
		}
		p = cur
	}
	p = path.Clean(p)
	entries, err := c.List(p)
	if err != nil {
		return nil, fmt.Errorf("list %q: %w", p, err)
	}
	out := make([]DirEntry, 0, len(entries))
	for _, e := range entries {
		name := e.Name
		if name == "." || name == ".." {
			continue
		}
		child := path.Join(p, name)
		out = append(out, DirEntry{
			Name:  name,
			Path:  child,
			IsDir: e.Type == ftp.EntryTypeFolder,
			Size:  int64(e.Size),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	parent := path.Dir(p)
	if parent == p {
		parent = ""
	}
	return &BrowseResult{Path: p, Parent: parent, Entries: out}, nil
}
