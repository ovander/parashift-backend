package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/handler"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/repo"
	"github.com/ovander/parashift/internal/service"
	"github.com/ovander/parashift/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// adminCtx returns a context with admin role for use in audit log handler tests.
func adminCtx(tenantID uuid.UUID) context.Context {
	return managerCtx(tenantID) // re-use manager helper — tenant scoping is identical
}

func newAdminHandler(auditRepo *testutil.MockAuditLogRepo) *handler.AdminHandler {
	svc := service.NewAdminService(auditRepo, nil, newTestLogger())
	return handler.NewAdminHandler(svc)
}

func newAuditLog(tenantID, actorID uuid.UUID) *model.AuditLog {
	after, _ := json.Marshal(map[string]string{"status": "approved"})
	return &model.AuditLog{
		ID:           uuid.New(),
		TenantID:     tenantID,
		ActorID:      actorID,
		Action:       "leave.updated",
		ResourceType: "leave_request",
		ResourceID:   uuid.New().String(),
		After:        datatypes.JSON(after),
		CreatedAt:    time.Now(),
	}
}

// ── ListAuditLogs — happy path ────────────────────────────────────────────────

func TestAdminHandler_ListAuditLogs_OK(t *testing.T) {
	tenantID := uuid.New()
	actorID  := uuid.New()
	log1     := newAuditLog(tenantID, actorID)

	auditRepo := &testutil.MockAuditLogRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, _ repo.AuditLogFilter, _, _ int) ([]*model.AuditLog, int64, error) {
			return []*model.AuditLog{log1}, 1, nil
		},
	}
	h := newAdminHandler(auditRepo)

	req := httptest.NewRequest(http.MethodGet, "/admin/audit-logs", nil)
	req = req.WithContext(adminCtx(tenantID))
	rr := httptest.NewRecorder()
	h.ListAuditLogs(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Data  []map[string]interface{} `json:"data"`
		Total int                      `json:"total"`
	}
	require.NoError(t, decodeJSON(rr, &resp))
	require.Len(t, resp.Data, 1)
	assert.Equal(t, "leave.updated", resp.Data[0]["action"])
	assert.Equal(t, "leave_request", resp.Data[0]["resource_type"])
	assert.Equal(t, log1.ResourceID, resp.Data[0]["resource_id"])
	// actor_name is empty (nil db) but must be present in the JSON
	_, hasActorName := resp.Data[0]["actor_name"]
	assert.True(t, hasActorName, "actor_name field must always be present in response")
}

func TestAdminHandler_ListAuditLogs_EmptyResult(t *testing.T) {
	tenantID := uuid.New()
	auditRepo := &testutil.MockAuditLogRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, _ repo.AuditLogFilter, _, _ int) ([]*model.AuditLog, int64, error) {
			return nil, 0, nil
		},
	}
	h := newAdminHandler(auditRepo)

	req := httptest.NewRequest(http.MethodGet, "/admin/audit-logs", nil)
	req = req.WithContext(adminCtx(tenantID))
	rr := httptest.NewRecorder()
	h.ListAuditLogs(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp struct {
		Data  []interface{} `json:"data"`
		Total int           `json:"total"`
	}
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Equal(t, 0, resp.Total)
}

// ── ListAuditLogs — filter query params forwarded ─────────────────────────────

func TestAdminHandler_ListAuditLogs_ActionFilter(t *testing.T) {
	tenantID := uuid.New()
	var capturedFilter repo.AuditLogFilter

	auditRepo := &testutil.MockAuditLogRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, f repo.AuditLogFilter, _, _ int) ([]*model.AuditLog, int64, error) {
			capturedFilter = f
			return nil, 0, nil
		},
	}
	h := newAdminHandler(auditRepo)

	req := httptest.NewRequest(http.MethodGet, "/admin/audit-logs?action=shift.created", nil)
	req = req.WithContext(adminCtx(tenantID))
	rr := httptest.NewRecorder()
	h.ListAuditLogs(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "shift.created", capturedFilter.Action)
}

func TestAdminHandler_ListAuditLogs_ResourceTypeFilter(t *testing.T) {
	tenantID := uuid.New()
	var capturedFilter repo.AuditLogFilter

	auditRepo := &testutil.MockAuditLogRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, f repo.AuditLogFilter, _, _ int) ([]*model.AuditLog, int64, error) {
			capturedFilter = f
			return nil, 0, nil
		},
	}
	h := newAdminHandler(auditRepo)

	req := httptest.NewRequest(http.MethodGet, "/admin/audit-logs?resource_type=shift", nil)
	req = req.WithContext(adminCtx(tenantID))
	rr := httptest.NewRecorder()
	h.ListAuditLogs(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "shift", capturedFilter.ResourceType)
}

func TestAdminHandler_ListAuditLogs_DateRangeFilter(t *testing.T) {
	tenantID := uuid.New()
	var capturedFilter repo.AuditLogFilter

	auditRepo := &testutil.MockAuditLogRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, f repo.AuditLogFilter, _, _ int) ([]*model.AuditLog, int64, error) {
			capturedFilter = f
			return nil, 0, nil
		},
	}
	h := newAdminHandler(auditRepo)

	req := httptest.NewRequest(http.MethodGet, "/admin/audit-logs?from=2026-04-01&to=2026-04-07", nil)
	req = req.WithContext(adminCtx(tenantID))
	rr := httptest.NewRecorder()
	h.ListAuditLogs(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, capturedFilter.From)
	require.NotNil(t, capturedFilter.To)
	assert.Equal(t, 2026, capturedFilter.From.Year())
	assert.Equal(t, time.April, capturedFilter.From.Month())
	assert.Equal(t, 1, capturedFilter.From.Day())
	assert.Equal(t, 7, capturedFilter.To.Day())
	// "to" must be extended to end-of-day.
	assert.Equal(t, 23, capturedFilter.To.Hour())
	assert.Equal(t, 59, capturedFilter.To.Minute())
}

func TestAdminHandler_ListAuditLogs_AllFiltersCombined(t *testing.T) {
	tenantID := uuid.New()
	var capturedFilter repo.AuditLogFilter

	auditRepo := &testutil.MockAuditLogRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, f repo.AuditLogFilter, _, _ int) ([]*model.AuditLog, int64, error) {
			capturedFilter = f
			return nil, 0, nil
		},
	}
	h := newAdminHandler(auditRepo)

	req := httptest.NewRequest(http.MethodGet,
		"/admin/audit-logs?action=leave.updated&resource_type=leave_request&from=2026-04-01&to=2026-04-30",
		nil,
	)
	req = req.WithContext(adminCtx(tenantID))
	rr := httptest.NewRecorder()
	h.ListAuditLogs(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "leave.updated", capturedFilter.Action)
	assert.Equal(t, "leave_request", capturedFilter.ResourceType)
	assert.NotNil(t, capturedFilter.From)
	assert.NotNil(t, capturedFilter.To)
}

// ── ListAuditLogs — invalid filter params ─────────────────────────────────────

func TestAdminHandler_ListAuditLogs_InvalidFromDate(t *testing.T) {
	tenantID := uuid.New()
	h := newAdminHandler(&testutil.MockAuditLogRepo{})

	req := httptest.NewRequest(http.MethodGet, "/admin/audit-logs?from=not-a-date", nil)
	req = req.WithContext(adminCtx(tenantID))
	rr := httptest.NewRecorder()
	h.ListAuditLogs(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdminHandler_ListAuditLogs_InvalidToDate(t *testing.T) {
	tenantID := uuid.New()
	h := newAdminHandler(&testutil.MockAuditLogRepo{})

	req := httptest.NewRequest(http.MethodGet, "/admin/audit-logs?to=32-13-2026", nil)
	req = req.WithContext(adminCtx(tenantID))
	rr := httptest.NewRecorder()
	h.ListAuditLogs(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ── Response shape ─────────────────────────────────────────────────────────────

func TestAdminHandler_ListAuditLogs_ResponseIncludesAfterPayload(t *testing.T) {
	tenantID := uuid.New()
	log1     := newAuditLog(tenantID, uuid.New())

	auditRepo := &testutil.MockAuditLogRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, _ repo.AuditLogFilter, _, _ int) ([]*model.AuditLog, int64, error) {
			return []*model.AuditLog{log1}, 1, nil
		},
	}
	h := newAdminHandler(auditRepo)

	req := httptest.NewRequest(http.MethodGet, "/admin/audit-logs", nil)
	req = req.WithContext(adminCtx(tenantID))
	rr := httptest.NewRecorder()
	h.ListAuditLogs(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp struct {
		Data []map[string]interface{} `json:"data"`
	}
	require.NoError(t, decodeJSON(rr, &resp))
	require.Len(t, resp.Data, 1)
	// "after" must be present (non-nil payload from newAuditLog)
	assert.NotNil(t, resp.Data[0]["after"])
}

func TestAdminHandler_ListAuditLogs_ActorIDAlwaysPresent(t *testing.T) {
	tenantID := uuid.New()
	actorID  := uuid.New()
	log1     := newAuditLog(tenantID, actorID)

	auditRepo := &testutil.MockAuditLogRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, _ repo.AuditLogFilter, _, _ int) ([]*model.AuditLog, int64, error) {
			return []*model.AuditLog{log1}, 1, nil
		},
	}
	h := newAdminHandler(auditRepo)

	req := httptest.NewRequest(http.MethodGet, "/admin/audit-logs", nil)
	req = req.WithContext(adminCtx(tenantID))
	rr := httptest.NewRecorder()
	h.ListAuditLogs(rr, req)

	var resp struct {
		Data []map[string]interface{} `json:"data"`
	}
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Equal(t, actorID.String(), resp.Data[0]["actor_id"])
}
