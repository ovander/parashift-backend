package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/repo"
	"github.com/ovander/parashift/internal/service"
	"github.com/ovander/parashift/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newAdminService builds a service with a nil *gorm.DB which is safe as long as
// the test doesn't trigger actor name resolution (logs empty, or the nil-guard fires).
func newAdminService(auditRepo *testutil.MockAuditLogRepo) *service.AdminService {
	return service.NewAdminService(auditRepo, nil, newTestLogger())
}

// newAuditLog returns a minimal *model.AuditLog for use in tests.
func newAuditLog(tenantID, actorID uuid.UUID) *model.AuditLog {
	return &model.AuditLog{
		ID:           uuid.New(),
		TenantID:     tenantID,
		ActorID:      actorID,
		Action:       "leave.updated",
		ResourceType: "leave_request",
		ResourceID:   uuid.New().String(),
		CreatedAt:    time.Now(),
	}
}

// ─── ListAuditLogs — filter forwarding ────────────────────────────────────────

func TestAdminService_ListAuditLogs_NoFilters(t *testing.T) {
	tenantID := uuid.New()
	var capturedFilter repo.AuditLogFilter

	auditRepo := &testutil.MockAuditLogRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, f repo.AuditLogFilter, _, _ int) ([]*model.AuditLog, int64, error) {
			capturedFilter = f
			return nil, 0, nil
		},
	}
	svc := newAdminService(auditRepo)

	_, _, err := svc.ListAuditLogs(context.Background(), tenantID, service.AuditLogFilter{}, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, "", capturedFilter.Action)
	assert.Equal(t, "", capturedFilter.ResourceType)
	assert.Nil(t, capturedFilter.From)
	assert.Nil(t, capturedFilter.To)
}

func TestAdminService_ListAuditLogs_ActionFilter(t *testing.T) {
	tenantID := uuid.New()
	var capturedFilter repo.AuditLogFilter

	auditRepo := &testutil.MockAuditLogRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, f repo.AuditLogFilter, _, _ int) ([]*model.AuditLog, int64, error) {
			capturedFilter = f
			return nil, 0, nil
		},
	}
	svc := newAdminService(auditRepo)

	filter := service.AuditLogFilter{Action: "leave.updated"}
	_, _, err := svc.ListAuditLogs(context.Background(), tenantID, filter, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, "leave.updated", capturedFilter.Action)
}

func TestAdminService_ListAuditLogs_ResourceTypeFilter(t *testing.T) {
	tenantID := uuid.New()
	var capturedFilter repo.AuditLogFilter

	auditRepo := &testutil.MockAuditLogRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, f repo.AuditLogFilter, _, _ int) ([]*model.AuditLog, int64, error) {
			capturedFilter = f
			return nil, 0, nil
		},
	}
	svc := newAdminService(auditRepo)

	filter := service.AuditLogFilter{ResourceType: "shift"}
	_, _, err := svc.ListAuditLogs(context.Background(), tenantID, filter, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, "shift", capturedFilter.ResourceType)
}

func TestAdminService_ListAuditLogs_DateRangeFilter(t *testing.T) {
	tenantID := uuid.New()
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to   := time.Date(2026, 4, 7, 23, 59, 59, 0, time.UTC)
	var capturedFilter repo.AuditLogFilter

	auditRepo := &testutil.MockAuditLogRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, f repo.AuditLogFilter, _, _ int) ([]*model.AuditLog, int64, error) {
			capturedFilter = f
			return nil, 0, nil
		},
	}
	svc := newAdminService(auditRepo)

	filter := service.AuditLogFilter{From: &from, To: &to}
	_, _, err := svc.ListAuditLogs(context.Background(), tenantID, filter, 1, 20)
	require.NoError(t, err)
	require.NotNil(t, capturedFilter.From)
	require.NotNil(t, capturedFilter.To)
	assert.Equal(t, from, *capturedFilter.From)
	assert.Equal(t, to, *capturedFilter.To)
}

func TestAdminService_ListAuditLogs_ForwardsPageParams(t *testing.T) {
	tenantID := uuid.New()
	var capturedPage, capturedPageSize int

	auditRepo := &testutil.MockAuditLogRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, _ repo.AuditLogFilter, page, pageSize int) ([]*model.AuditLog, int64, error) {
			capturedPage     = page
			capturedPageSize = pageSize
			return nil, 0, nil
		},
	}
	svc := newAdminService(auditRepo)

	_, _, err := svc.ListAuditLogs(context.Background(), tenantID, service.AuditLogFilter{}, 3, 50)
	require.NoError(t, err)
	assert.Equal(t, 3, capturedPage)
	assert.Equal(t, 50, capturedPageSize)
}

// ─── ListAuditLogs — result mapping ───────────────────────────────────────────

func TestAdminService_ListAuditLogs_ReturnsEntries(t *testing.T) {
	tenantID := uuid.New()
	actorID  := uuid.New()
	log1     := newAuditLog(tenantID, actorID)
	log2     := newAuditLog(tenantID, actorID)

	auditRepo := &testutil.MockAuditLogRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, _ repo.AuditLogFilter, _, _ int) ([]*model.AuditLog, int64, error) {
			return []*model.AuditLog{log1, log2}, 2, nil
		},
	}
	// nil db — actor name resolution skipped gracefully
	svc := newAdminService(auditRepo)

	entries, total, err := svc.ListAuditLogs(context.Background(), tenantID, service.AuditLogFilter{}, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, entries, 2)
	assert.Equal(t, log1.ID, entries[0].ID)
	assert.Equal(t, log2.ID, entries[1].ID)
}

func TestAdminService_ListAuditLogs_ActorNameEmptyWhenDBNil(t *testing.T) {
	tenantID := uuid.New()
	log1     := newAuditLog(tenantID, uuid.New())

	auditRepo := &testutil.MockAuditLogRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, _ repo.AuditLogFilter, _, _ int) ([]*model.AuditLog, int64, error) {
			return []*model.AuditLog{log1}, 1, nil
		},
	}
	// nil db — resolver must not panic, actor_name should be empty
	svc := newAdminService(auditRepo)

	entries, _, err := svc.ListAuditLogs(context.Background(), tenantID, service.AuditLogFilter{}, 1, 20)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "", entries[0].ActorName, "actor_name must be empty when db is nil (graceful degradation)")
}

func TestAdminService_ListAuditLogs_PreservesResourceFields(t *testing.T) {
	tenantID   := uuid.New()
	resourceID := uuid.New().String()
	log1 := newAuditLog(tenantID, uuid.New())
	log1.ResourceType = "shift"
	log1.ResourceID   = resourceID

	auditRepo := &testutil.MockAuditLogRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, _ repo.AuditLogFilter, _, _ int) ([]*model.AuditLog, int64, error) {
			return []*model.AuditLog{log1}, 1, nil
		},
	}
	svc := newAdminService(auditRepo)

	entries, _, err := svc.ListAuditLogs(context.Background(), tenantID, service.AuditLogFilter{}, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, "shift", entries[0].ResourceType)
	assert.Equal(t, resourceID, entries[0].ResourceID)
}

// ─── ListAuditLogs — error handling ──────────────────────────────────────────

func TestAdminService_ListAuditLogs_RepoError(t *testing.T) {
	auditRepo := &testutil.MockAuditLogRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, _ repo.AuditLogFilter, _, _ int) ([]*model.AuditLog, int64, error) {
			return nil, 0, errors.New("db connection lost")
		},
	}
	svc := newAdminService(auditRepo)

	_, _, err := svc.ListAuditLogs(context.Background(), uuid.New(), service.AuditLogFilter{}, 1, 20)
	require.Error(t, err)
}

func TestAdminService_ListAuditLogs_EmptyResult(t *testing.T) {
	auditRepo := &testutil.MockAuditLogRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, _ repo.AuditLogFilter, _, _ int) ([]*model.AuditLog, int64, error) {
			return nil, 0, nil
		},
	}
	svc := newAdminService(auditRepo)

	entries, total, err := svc.ListAuditLogs(context.Background(), uuid.New(), service.AuditLogFilter{}, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, entries)
}
