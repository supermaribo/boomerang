package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/boomerang-backup/boomerang/internal/filebackup"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleFileVersionTree(w http.ResponseWriter, r *http.Request) {
	fsID := chi.URLParam(r, "id")
	vid := chi.URLParam(r, "vid")
	v, err := s.store.GetVersion(vid)
	if err != nil || v == nil || v.TargetType != "file" || v.TargetID != fsID {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err := filebackup.IndexChain(s.store, vid, v.PathOnDisk); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	chain, err := filebackup.VersionChain(s.store, vid, v.PathOnDisk)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}

	prefix := strings.Trim(r.URL.Query().Get("path"), "/")
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))

	if q != "" {
		hits, err := s.store.SearchManifestChain(chain, q, 500)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		entries := make([]map[string]any, 0, len(hits))
		for _, h := range hits {
			entries = append(entries, map[string]any{
				"path": h.Path, "isDir": h.IsDir, "size": h.Size, "mtime": h.Mtime,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"mode": "search", "query": q, "entries": entries})
		return
	}

	children, err := s.store.BrowseManifestChain(chain, prefix)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	total, _ := s.store.CountManifestChain(chain)
	list := make([]treeNode, 0, len(children))
	for _, c := range children {
		name := c.Path
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		list = append(list, treeNode{Name: name, Path: c.Path, IsDir: c.IsDir, Size: c.Size})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].IsDir != list[j].IsDir {
			return list[i].IsDir
		}
		return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
	})

	m, _ := filebackup.LoadMergedManifest(s.store, "file", fsID, vid, v.PathOnDisk)
	root := ""
	kind := "full"
	if m != nil {
		root = m.Root
		kind = m.Kind
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"mode":    "browse",
		"path":    prefix,
		"root":    root,
		"kind":    kind,
		"total":   total,
		"entries": list,
	})
}

func (s *Server) handleRestoreFilePreview(w http.ResponseWriter, r *http.Request) {
	fsID := chi.URLParam(r, "id")
	vid := chi.URLParam(r, "vid")
	v, err := s.store.GetVersion(vid)
	if err != nil || v == nil || v.TargetType != "file" || v.TargetID != fsID {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	var req restoreReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	clean, err := sanitizePaths(req.Paths)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(clean) == 0 {
		writeErr(w, http.StatusBadRequest, "select at least one path")
		return
	}
	files, totalBytes, count, err := filebackup.RestorePreview(s.store, vid, v.PathOnDisk, clean)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"files":       files,
		"totalFiles":  count,
		"totalBytes":  totalBytes,
		"overwrite":   true,
		"message":     "These paths will be written to the live server, overwriting existing files with the same names.",
	})
}
