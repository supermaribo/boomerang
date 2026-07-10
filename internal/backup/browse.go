package backup

import "strings"

// DefaultBrowsePrefix returns the manifest path prefix to open on first browse.
// RSYNC archives store paths like home/user/htdocs/file; SFTP uses paths relative to include root.
func DefaultBrowsePrefix(m *FileManifest) string {
	if m == nil || len(m.Paths) == 0 {
		return ""
	}
	prefix := strings.Trim(m.Paths[0], "/")
	if prefix == "" {
		return ""
	}
	for _, e := range m.Entries {
		p := strings.Trim(e.Path, "/")
		if p == "" {
			continue
		}
		if p == prefix || strings.HasPrefix(p, prefix+"/") {
			return prefix
		}
		return ""
	}
	return prefix
}

// RemoteBrowsePath returns the configured remote path for breadcrumbs.
func RemoteBrowsePath(m *FileManifest) string {
	if m == nil || len(m.Paths) == 0 {
		return ""
	}
	p := strings.TrimSpace(m.Paths[0])
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
}
