package handler

import (
	"net/http"

	"github.com/ovander/parashift/internal/pkg"
)

// VersionHandler serves build metadata at GET /api/version.
// The endpoint is public (no auth) so the frontend can display it
// without requiring the user to be logged in.
type VersionHandler struct {
	version   string
	commit    string
	buildTime string
}

// NewVersionHandler creates a VersionHandler with the values injected at link time.
func NewVersionHandler(version, commit, buildTime string) *VersionHandler {
	return &VersionHandler{version: version, commit: commit, buildTime: buildTime}
}

type versionResponse struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
}

// Get handles GET /api/version.
func (h *VersionHandler) Get(w http.ResponseWriter, r *http.Request) {
	pkg.WriteJSON(w, http.StatusOK, versionResponse{
		Version:   h.version,
		Commit:    h.commit,
		BuildTime: h.buildTime,
	})
}
