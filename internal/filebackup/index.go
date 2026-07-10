package filebackup

import (
	"path"
	"strings"

	"github.com/boomerang-backup/boomerang/internal/backup"
	"github.com/boomerang-backup/boomerang/internal/store"
)

func EnsureManifestIndexed(st *store.Store, versionID, versionDir string) error {
	ok, err := st.HasManifestIndex(versionID)
	if err != nil || ok {
		return err
	}
	m, err := backup.ReadFileManifest(versionDir)
	if err != nil {
		return err
	}
	return st.ReplaceManifestIndex(versionID, m.Entries)
}

func IndexChain(st *store.Store, versionID, versionDir string) error {
	chain, err := VersionChain(st, versionID, versionDir)
	if err != nil {
		return err
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	for _, vid := range chain {
		dir := versionDir
		if vid != versionID {
			v, err := st.GetVersion(vid)
			if err != nil || v == nil {
				continue
			}
			dir = v.PathOnDisk
		}
		if err := EnsureManifestIndexed(st, vid, dir); err != nil {
			return err
		}
	}
	return nil
}

type PreviewFile struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

func RestorePreview(st *store.Store, versionID, versionDir string, paths []string) ([]PreviewFile, int64, int, error) {
	chain, err := VersionChain(st, versionID, versionDir)
	if err != nil {
		return nil, 0, 0, err
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	want := map[string]bool{}
	for _, p := range paths {
		p = strings.Trim(path.Clean("/"+strings.TrimPrefix(p, "/")), "/")
		if p != "" && p != "." {
			want[p] = true
		}
	}
	merged := map[string]PreviewFile{}
	for _, vid := range chain {
		dir := versionDir
		if vid != versionID {
			v, err := st.GetVersion(vid)
			if err != nil || v == nil {
				continue
			}
			dir = v.PathOnDisk
		}
		m, err := backup.ReadFileManifest(dir)
		if err != nil {
			continue
		}
		for _, e := range m.Entries {
			if e.IsDir {
				continue
			}
			p := strings.Trim(e.Path, "/")
			if !matchRestorePath(p, want) {
				continue
			}
			merged[p] = PreviewFile{Path: p, Size: e.Size}
		}
	}
	out := make([]PreviewFile, 0, len(merged))
	var total int64
	for _, f := range merged {
		out = append(out, f)
		total += f.Size
	}
	return out, total, len(out), nil
}

func matchRestorePath(p string, want map[string]bool) bool {
	if len(want) == 0 {
		return false
	}
	for w := range want {
		if p == w || strings.HasPrefix(p, w+"/") || strings.HasPrefix(w, p+"/") {
			return true
		}
	}
	return false
}
