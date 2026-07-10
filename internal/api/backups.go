package api

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/boomerang-backup/boomerang/internal/archive"
	"github.com/boomerang-backup/boomerang/internal/filebackup"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleGetFileVersion(w http.ResponseWriter, r *http.Request) {
	fsID := chi.URLParam(r, "id")
	vid := chi.URLParam(r, "vid")
	v, err := s.store.GetVersion(vid)
	if err != nil || v == nil || v.TargetType != "file" || v.TargetID != fsID {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, v)
}

type treeNode struct {
	Name     string     `json:"name"`
	Path     string     `json:"path"`
	IsDir    bool       `json:"isDir"`
	Size     int64      `json:"size,omitempty"`
	Children []treeNode `json:"children,omitempty"`
}

func (s *Server) handleFileVersionTree(w http.ResponseWriter, r *http.Request) {
	fsID := chi.URLParam(r, "id")
	vid := chi.URLParam(r, "vid")
	v, err := s.store.GetVersion(vid)
	if err != nil || v == nil || v.TargetType != "file" || v.TargetID != fsID {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	m, err := filebackup.LoadMergedManifest(s.store, "file", fsID, vid, v.PathOnDisk)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	prefix := strings.Trim(r.URL.Query().Get("path"), "/")
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))

	if q != "" {
		var hits []map[string]any
		for _, e := range m.Entries {
			p := strings.Trim(e.Path, "/")
			if p == "" || p == "." {
				continue
			}
			if !strings.Contains(strings.ToLower(p), q) {
				continue
			}
			hits = append(hits, map[string]any{
				"path": p, "isDir": e.IsDir, "size": e.Size, "mtime": e.Mtime,
			})
			if len(hits) >= 500 {
				break
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"mode": "search", "query": q, "entries": hits})
		return
	}

	children := map[string]treeNode{}
	for _, e := range m.Entries {
		p := strings.Trim(e.Path, "/")
		if p == "" || p == "." {
			continue
		}
		var rest string
		if prefix == "" {
			rest = p
		} else {
			if p == prefix {
				continue
			}
			if !strings.HasPrefix(p, prefix+"/") {
				continue
			}
			rest = strings.TrimPrefix(p, prefix+"/")
		}
		name, more, _ := strings.Cut(rest, "/")
		full := name
		if prefix != "" {
			full = prefix + "/" + name
		}
		if more != "" {
			if _, ok := children[name]; !ok {
				children[name] = treeNode{Name: name, Path: full, IsDir: true}
			} else {
				n := children[name]
				n.IsDir = true
				children[name] = n
			}
			continue
		}
		children[name] = treeNode{Name: name, Path: full, IsDir: e.IsDir, Size: e.Size}
	}
	list := make([]treeNode, 0, len(children))
	for _, n := range children {
		list = append(list, n)
	}
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			ai, aj := list[i], list[j]
			if ai.IsDir != aj.IsDir {
				if !ai.IsDir && aj.IsDir {
					list[i], list[j] = list[j], list[i]
				}
				continue
			}
			if strings.ToLower(ai.Name) > strings.ToLower(aj.Name) {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"mode":    "browse",
		"path":    prefix,
		"root":    m.Root,
		"kind":    m.Kind,
		"total":   len(m.Entries),
		"entries": list,
	})
}

type restoreReq struct {
	Paths       []string `json:"paths"`
	ConfirmName string   `json:"confirmName"`
}

func (s *Server) handleRestoreFileVersion(w http.ResponseWriter, r *http.Request) {
	fsID := chi.URLParam(r, "id")
	vid := chi.URLParam(r, "vid")
	fs, err := s.store.GetFileServer(fsID)
	if err != nil || fs == nil {
		writeErr(w, http.StatusNotFound, "file server not found")
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
	if req.ConfirmName != fs.Name {
		writeErr(w, http.StatusBadRequest, "type the file server name to confirm restore")
		return
	}
	jobID, err := s.runner.StartFileRestore(fsID, vid, clean)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.store.Audit("file_restore", fsID+":"+vid)
	writeJSON(w, http.StatusAccepted, map[string]string{"jobId": jobID})
}

type downloadReq struct {
	Paths []string `json:"paths"`
}

func (s *Server) handleDownloadFileVersion(w http.ResponseWriter, r *http.Request) {
	fsID := chi.URLParam(r, "id")
	vid := chi.URLParam(r, "vid")
	v, err := s.store.GetVersion(vid)
	if err != nil || v == nil || v.TargetType != "file" || v.TargetID != fsID {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if v.Status != "succeeded" {
		writeErr(w, http.StatusBadRequest, "version is not a successful backup")
		return
	}
	var req downloadReq
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
	want := map[string]bool{}
	for _, p := range clean {
		want[p] = true
	}

	chain, err := filebackup.VersionChain(s.store, vid, v.PathOnDisk)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}

	filename := fmt.Sprintf("boomerang-%s-%s.zip", fsID[:8], time.Now().UTC().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	zw := zip.NewWriter(w)
	defer zw.Close()

	written := 0
	for _, cid := range chain {
		dir := v.PathOnDisk
		if cid != vid {
			cv, err := s.store.GetVersion(cid)
			if err != nil || cv == nil {
				continue
			}
			dir = cv.PathOnDisk
		}
		n, err := archive.WriteZipPaths(s.box, archive.FilesBlobPath(dir), want, zw)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		written += n
	}
	if written == 0 {
		writeErr(w, http.StatusBadRequest, "no matching files in backup")
		return
	}
}

func sanitizePaths(paths []string) ([]string, error) {
	clean := make([]string, 0, len(paths))
	for _, p := range paths {
		p = path.Clean("/" + strings.TrimPrefix(p, "/"))
		p = strings.TrimPrefix(p, "/")
		if p == "" || p == "." || strings.Contains(p, "..") {
			continue
		}
		clean = append(clean, p)
	}
	return clean, nil
}

func (s *Server) handleBackupDatabase(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	jobID, err := s.runner.StartDBBackup(id)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.store.Audit("db_backup", id)
	writeJSON(w, http.StatusAccepted, map[string]string{"jobId": jobID})
}

func (s *Server) handleListDBVersions(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListVersions("db", chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}
