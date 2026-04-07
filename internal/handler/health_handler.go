package handler

import (
	"context"
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/ovander/parashift/internal/pkg"
)

// HealthHandler provides health and readiness check endpoints.
type HealthHandler struct {
	db *gorm.DB
}

// NewHealthHandler creates a new HealthHandler.
// db may be nil — the readiness check will report "db_unavailable" in that case.
func NewHealthHandler(db *gorm.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

// Check is the liveness probe: returns 200 as long as the process is running.
// Registered at GET /healthz.
// The reverse proxy (Caddy) uses this to decide whether to restart the container,
// NOT to decide whether to route traffic — use /readyz for that.
func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	pkg.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready is the readiness probe: returns 200 only when all dependencies are healthy.
// Registered at GET /readyz.
// A load balancer, systemd watchdog, or Kubernetes readiness probe should call this.
// Returns 503 if the database is unreachable so traffic is not routed to a broken instance.
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		pkg.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "error",
			"checks": map[string]string{"db": "not_configured"},
		})
		return
	}

	sqlDB, err := h.db.DB()
	if err != nil {
		pkg.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "error",
			"checks": map[string]string{"db": "driver_error: " + err.Error()},
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		pkg.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "error",
			"checks": map[string]string{"db": "unreachable: " + err.Error()},
		})
		return
	}

	pkg.WriteJSON(w, http.StatusOK, map[string]any{
		"status": "ready",
		"checks": map[string]string{"db": "ok"},
	})
}
