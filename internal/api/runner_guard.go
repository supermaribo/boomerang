package api

import "net/http"

func (s *Server) requireRunner(w http.ResponseWriter) bool {
	if s.runner == nil {
		writeErr(w, http.StatusServiceUnavailable, "backup runner unavailable")
		return false
	}
	return true
}
