package api

import (
	"net/http"

	"github.com/boomerang-backup/boomerang/internal/update"
	"github.com/boomerang-backup/boomerang/internal/version"
)

func (s *Server) handleUpdateCheck(w http.ResponseWriter, _ *http.Request) {
	canApply, reason := update.ApplyCapability()
	res := update.CheckWithReason(version.Version, canApply, reason)
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleUpdateApply(w http.ResponseWriter, r *http.Request) {
	if !update.CanApply() {
		writeErr(w, http.StatusBadRequest, "in-place updates are not configured on this host — "+update.UpgradeHint())
		return
	}
	check := update.Check(version.Version, true)
	if check.CheckError != "" {
		writeErr(w, http.StatusBadGateway, check.CheckError)
		return
	}
	if !check.UpdateAvailable {
		writeErr(w, http.StatusBadRequest, "already on the latest release")
		return
	}
	if check.AssetURL == "" || check.AssetName == "" {
		writeErr(w, http.StatusBadGateway, "release has no downloadable asset for this system")
		return
	}

	path, err := update.DownloadAsset(s.cfg.DataDir, check.AssetURL, check.AssetName)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if err := update.ApplyDownloaded(path); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.store.Audit("app_update", check.CurrentVersion+" -> "+check.LatestVersion)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"previousVersion": check.CurrentVersion,
		"newVersion":      check.LatestVersion,
		"restarting":      true,
	})
}
