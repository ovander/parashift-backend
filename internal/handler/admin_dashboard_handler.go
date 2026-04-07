package handler

import (
	"net/http"

	"github.com/ovander/parashift/internal/pkg"
	"github.com/ovander/parashift/internal/service"
)

// AdminDashboardHandler serves the admin dashboard endpoint.
type AdminDashboardHandler struct {
	svc *service.AdminDashboardService
}

// NewAdminDashboardHandler creates a new AdminDashboardHandler.
func NewAdminDashboardHandler(svc *service.AdminDashboardService) *AdminDashboardHandler {
	return &AdminDashboardHandler{svc: svc}
}

// GetDashboard returns the full admin dashboard payload.
//
//	GET /api/v1/admin/dashboard
func (h *AdminDashboardHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	dashboard, err := h.svc.GetDashboard(r.Context())
	if err != nil {
		pkg.WriteError(w, err)
		return
	}
	pkg.WriteJSON(w, http.StatusOK, dashboard)
}
