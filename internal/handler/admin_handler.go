package handler

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/backendkit/pagination"
	"github.com/ovander/parashift/internal/pkg"
	"github.com/ovander/parashift/internal/service"
)

// AdminHandler handles admin-only HTTP requests.
type AdminHandler struct {
	svc *service.AdminService
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(svc *service.AdminService) *AdminHandler {
	return &AdminHandler{svc: svc}
}

// auditLogResponse is the JSON shape returned for a single audit log entry.
type auditLogResponse struct {
	ID           uuid.UUID `json:"id"`
	TenantID     uuid.UUID `json:"tenant_id"`
	ActorID      uuid.UUID `json:"actor_id"`
	ActorName    string    `json:"actor_name"`   // resolved employee name
	Action       string    `json:"action"`
	ResourceType string    `json:"resource_type"`
	ResourceID   string    `json:"resource_id"`
	After        []byte    `json:"after,omitempty"`
	Before       []byte    `json:"before,omitempty"`
	CreatedAt    string    `json:"created_at"`
}

func toAuditLogResponse(e *service.AuditLogEntry) auditLogResponse {
	return auditLogResponse{
		ID:           e.ID,
		TenantID:     e.TenantID,
		ActorID:      e.ActorID,
		ActorName:    e.ActorName,
		Action:       e.Action,
		ResourceType: e.ResourceType,
		ResourceID:   e.ResourceID,
		After:        []byte(e.After),
		Before:       []byte(e.Before),
		CreatedAt:    e.CreatedAt.Format(time.RFC3339),
	}
}

// GetStats returns platform-wide aggregate metrics for the admin dashboard.
func (h *AdminHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.GetStats(r.Context())
	if err != nil {
		pkg.WriteError(w, err)
		return
	}
	pkg.WriteJSON(w, http.StatusOK, stats)
}

// ListAuditLogs returns paginated audit log entries for the calling tenant.
//
// Query parameters:
//
//	action        — filter on the action string (e.g. "leave.updated")
//	resource_type — filter on resource type (e.g. "leave_request", "shift")
//	store_id      — UUID of a specific store; overrides the caller's tenant scope (super-admin)
//	from          — YYYY-MM-DD lower bound on created_at (inclusive)
//	to            — YYYY-MM-DD upper bound on created_at (inclusive, end of day)
//	page, per_page — pagination
func (h *AdminHandler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	filter, err := parseAuditFilter(r)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}
	// Super-admin can scope the query to a specific store.
	if filter.StoreID != nil {
		tenantID = *filter.StoreID
	}
	params := pagination.Parse(r)

	logs, total, err := h.svc.ListAuditLogs(ctx, tenantID, filter, params.Page, params.PerPage)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	responses := make([]auditLogResponse, len(logs))
	for i, l := range logs {
		responses[i] = toAuditLogResponse(l)
	}
	pkg.WriteJSON(w, http.StatusOK, pagination.NewPagedResponse(responses, params, total))
}

// ListAuditLogsForTenant returns audit logs for an explicit tenant_id (super-admin use).
func (h *AdminHandler) ListAuditLogsForTenant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tenantIDStr := r.URL.Query().Get("tenant_id")
	if tenantIDStr == "" {
		tenantIDStr = ctxutil.GetTenantID(ctx).String()
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid tenant_id").WithKey("errors.invalidInput"))
		return
	}

	filter, err := parseAuditFilter(r)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}
	params := pagination.Parse(r)

	logs, total, err := h.svc.ListAuditLogs(ctx, tenantID, filter, params.Page, params.PerPage)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	responses := make([]auditLogResponse, len(logs))
	for i, l := range logs {
		responses[i] = toAuditLogResponse(l)
	}
	pkg.WriteJSON(w, http.StatusOK, pagination.NewPagedResponse(responses, params, total))
}

// parseAuditFilter reads shared query parameters used by both audit log endpoints.
func parseAuditFilter(r *http.Request) (service.AuditLogFilter, error) {
	q := r.URL.Query()
	filter := service.AuditLogFilter{
		Action:       q.Get("action"),
		ResourceType: q.Get("resource_type"),
	}

	if storeIDStr := q.Get("store_id"); storeIDStr != "" {
		id, err := uuid.Parse(storeIDStr)
		if err != nil {
			return filter, apierror.BadRequest("invalid 'store_id' (must be a UUID)").WithKey("errors.invalidInput")
		}
		filter.StoreID = &id
	}

	if fromStr := q.Get("from"); fromStr != "" {
		t, err := time.Parse("2006-01-02", fromStr)
		if err != nil {
			t, err = time.Parse(time.RFC3339, fromStr)
			if err != nil {
				return filter, apierror.BadRequest("invalid 'from' date (use YYYY-MM-DD)").WithKey("errors.invalidInput")
			}
		}
		filter.From = &t
	}
	if toStr := q.Get("to"); toStr != "" {
		t, err := time.Parse("2006-01-02", toStr)
		if err != nil {
			t, err = time.Parse(time.RFC3339, toStr)
			if err != nil {
				return filter, apierror.BadRequest("invalid 'to' date (use YYYY-MM-DD)").WithKey("errors.invalidInput")
			}
		}
		// Extend to end of day so "to=2026-04-07" includes all events that day.
		endOfDay := t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		filter.To = &endOfDay
	}

	return filter, nil
}
