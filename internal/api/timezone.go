package api

import (
	"encoding/json"
	"net/http"

	"github.com/boomerang-backup/boomerang/internal/tzutil"
)

func (s *Server) handleListTimezones(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"timezone": tzutil.Name(s.store),
		"common":   tzutil.Common,
	})
}

func (s *Server) handlePutTimezone(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Timezone string `json:"timezone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	name, err := tzutil.Normalize(req.Timezone)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.SetMeta(tzutil.MetaKey, name); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if s.sched != nil {
		s.sched.Reload()
	}
	_ = s.store.Audit("settings_update", "timezone:"+name)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "timezone": name})
}
