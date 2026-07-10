package backup

import (
	"path"
	"strings"
)

// Excluded returns true if relPath matches any exclude glob.
// Globs use / separators; patterns like "*.log" match the base name.
func Excluded(relPath string, globs []string) bool {
	if len(globs) == 0 {
		return false
	}
	relPath = strings.TrimPrefix(strings.Trim(relPath, "/"), "./")
	base := path.Base(relPath)
	for _, g := range globs {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		g = strings.ReplaceAll(g, "\\", "/")
		if strings.Contains(g, "/") {
			if matchPath(relPath, g) {
				return true
			}
			// also match if path is under excluded dir
			prefix := strings.TrimSuffix(g, "/**")
			prefix = strings.TrimSuffix(prefix, "/*")
			if prefix != g && strings.HasPrefix(relPath, prefix+"/") {
				return true
			}
			if strings.HasSuffix(g, "/**") && strings.HasPrefix(relPath, strings.TrimSuffix(g, "/**")+"/") {
				return true
			}
			continue
		}
		if ok, _ := path.Match(g, base); ok {
			return true
		}
		if ok, _ := path.Match(g, relPath); ok {
			return true
		}
	}
	return false
}

func matchPath(p, pattern string) bool {
	if ok, _ := path.Match(pattern, p); ok {
		return true
	}
	// ** support: cache/** matches cache/foo/bar
	if strings.HasSuffix(pattern, "/**") {
		root := strings.TrimSuffix(pattern, "/**")
		return p == root || strings.HasPrefix(p, root+"/")
	}
	return false
}
