package remote

import (
	"fmt"
	"net"
	"path"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/pkg/sftp"
)

type PathWarning struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// CheckPathsAccess verifies read access to selected backup paths and reports permission problems.
func CheckPathsAccess(t FileTarget, paths []string) ([]PathWarning, error) {
	switch t.Protocol {
	case "sftp", "rsync":
		return checkPathsSFTP(t, paths)
	case "ftp", "ftps":
		return checkPathsFTP(t, paths)
	default:
		return nil, fmt.Errorf("unsupported protocol %q", t.Protocol)
	}
}

func checkPathsSFTP(t FileTarget, paths []string) ([]PathWarning, error) {
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

	var warnings []PathWarning
	seen := map[string]bool{}
	for _, raw := range paths {
		p := path.Clean(strings.TrimSpace(raw))
		if p == "" || p == "." {
			continue
		}
		if !path.IsAbs(p) {
			wd, _ := sc.Getwd()
			if wd == "" {
				wd = "/"
			}
			p = path.Join(wd, p)
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		warnings = append(warnings, checkOneSFTP(sc, p)...)
	}
	return warnings, nil
}

func checkOneSFTP(sc *sftp.Client, p string) []PathWarning {
	fi, err := sc.Stat(p)
	if err != nil {
		return []PathWarning{{Path: p, Message: accessErrMessage(err)}}
	}
	if !fi.IsDir() {
		f, err := sc.Open(p)
		if err != nil {
			return []PathWarning{{Path: p, Message: accessErrMessage(err)}}
		}
		_ = f.Close()
		return nil
	}
	entries, err := sc.ReadDir(p)
	if err != nil {
		return []PathWarning{{Path: p, Message: accessErrMessage(err)}}
	}
	var warnings []PathWarning
	for _, e := range entries {
		name := e.Name()
		if name == "." || name == ".." || !e.IsDir() {
			continue
		}
		child := path.Join(p, name)
		if _, err := sc.Stat(child); err != nil {
			warnings = append(warnings, PathWarning{Path: child, Message: accessErrMessage(err)})
			continue
		}
		if _, err := sc.ReadDir(child); err != nil {
			warnings = append(warnings, PathWarning{Path: child, Message: accessErrMessage(err)})
		}
	}
	return warnings
}

func checkPathsFTP(t FileTarget, paths []string) ([]PathWarning, error) {
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

	var warnings []PathWarning
	seen := map[string]bool{}
	for _, raw := range paths {
		p := path.Clean(strings.TrimSpace(raw))
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		entries, err := c.List(p)
		if err != nil {
			warnings = append(warnings, PathWarning{Path: p, Message: accessErrMessage(err)})
			continue
		}
		for _, e := range entries {
			if e.Type != ftp.EntryTypeFolder {
				continue
			}
			child := path.Join(p, e.Name)
			if _, err := c.List(child); err != nil {
				warnings = append(warnings, PathWarning{Path: child, Message: accessErrMessage(err)})
			}
		}
	}
	return warnings, nil
}

func accessErrMessage(err error) string {
	if err == nil {
		return "not accessible"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "permission denied"):
		return "permission denied — this account cannot read here"
	case strings.Contains(msg, "no such file"):
		return "path not found"
	default:
		return err.Error()
	}
}
