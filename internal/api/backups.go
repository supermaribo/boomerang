package api

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/boomerang-backup/boomerang/internal/archive"
	"github.com/boomerang-backup/boomerang/internal/backup"
	"github.com/boomerang-backup/boomerang/internal/filebackup"
	"github.com/boomerang-backup/boomerang/internal/store"
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

type deleteVersionReq struct {
	ConfirmName string `json:"confirmName"`
}

func (s *Server) handleDeleteFileVersion(w http.ResponseWriter, r *http.Request) {
	fsID := chi.URLParam(r, "id")
	vid := chi.URLParam(r, "vid")
	fs, err := s.store.GetFileServer(fsID)
	if err != nil || fs == nil {
		writeErr(w, http.StatusNotFound, "file server not found")
		return
	}
	var req deleteVersionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.ConfirmName != fs.Name {
		writeErr(w, http.StatusBadRequest, "type the file server name to confirm delete")
		return
	}
	if err := s.store.DeleteVersion("file", fsID, vid); err != nil {
		if errors.Is(err, store.ErrVersionNotFound) {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		var inUse store.ErrVersionInUse
		if errors.As(err, &inUse) {
			writeErr(w, http.StatusConflict, inUse.Error())
			return
		}
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.store.Audit("file_version_delete", fsID+":"+vid)
	s.scheduleOffsiteMirror()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleVerifyFileVersion(w http.ResponseWriter, r *http.Request) {
	if s.runner == nil {
		writeErr(w, http.StatusServiceUnavailable, "backup runner unavailable")
		return
	}
	fsID := chi.URLParam(r, "id")
	vid := chi.URLParam(r, "vid")
	jobID, err := s.runner.StartFileVerify(fsID, vid)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.store.Audit("file_verify", fsID+":"+vid)
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

func (s *Server) handleFileVersionLogs(w http.ResponseWriter, r *http.Request) {
	s.writeVersionLogs(w, r, "file")
}

func (s *Server) handleDBVersionLogs(w http.ResponseWriter, r *http.Request) {
	s.writeVersionLogs(w, r, "db")
}

func (s *Server) writeVersionLogs(w http.ResponseWriter, r *http.Request, targetType string) {
	tid := chi.URLParam(r, "id")
	vid := chi.URLParam(r, "vid")
	v, err := s.store.GetVersion(vid)
	if err != nil || v == nil || v.TargetType != targetType || v.TargetID != tid {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	lines, err := backup.ReadVersionLog(v.PathOnDisk)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	skipped, _ := backup.ReadSkippedLog(v.PathOnDisk)
	if len(skipped) > 0 && !backup.LogHasMissedPaths(lines) {
		lines = append(lines, backup.SkippedLogLines(skipped, 0)...)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"lines":   lines,
		"skipped": skipped,
	})
}
