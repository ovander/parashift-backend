package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/service"
	"github.com/ovander/parashift/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSwapService(
	swapRepo *testutil.MockSwapRequestRepo,
	shiftRepo *testutil.MockShiftInstanceRepo,
	assignRepo *testutil.MockShiftAssignmentRepo,
	empRepo *testutil.MockEmployeeRepo,
) *service.SwapService {
	return service.NewSwapService(swapRepo, shiftRepo, assignRepo, empRepo, nil, newTestEmitter(), newTestLogger())
}

// ─── CreateSwapRequest ────────────────────────────────────────────────────────

func TestSwapService_CreateSwapRequest_Success(t *testing.T) {
	tenantID := uuid.New()
	requesterID := uuid.New()
	shift := testutil.NewShiftInstance(tenantID)
	assignment := testutil.NewShiftAssignment(tenantID, shift.ID, requesterID)
	created := false

	shiftRepo := &testutil.MockShiftInstanceRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.ShiftInstance, error) {
			return shift, nil
		},
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByShiftFn: func(_ context.Context, _, _ uuid.UUID) ([]*model.ShiftAssignment, error) {
			return []*model.ShiftAssignment{assignment}, nil
		},
	}
	swapRepo := &testutil.MockSwapRequestRepo{
		CreateFn: func(_ context.Context, sr *model.SwapRequest) error {
			created = true
			assert.Equal(t, tenantID, sr.TenantID)
			assert.Equal(t, requesterID, sr.RequesterID)
			assert.Equal(t, shift.ID, sr.ShiftInstanceID)
			return nil
		},
	}
	svc := newSwapService(swapRepo, shiftRepo, assignRepo, &testutil.MockEmployeeRepo{})

	req := dto.CreateSwapRequest{ShiftInstanceID: shift.ID}
	got, err := svc.CreateSwapRequest(authCtx(tenantID), tenantID, requesterID, req)
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, model.SwapStatusPending, got.Status)
}

func TestSwapService_CreateSwapRequest_ShiftNotFound(t *testing.T) {
	shiftRepo := &testutil.MockShiftInstanceRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.ShiftInstance, error) {
			return nil, nil
		},
	}
	svc := newSwapService(&testutil.MockSwapRequestRepo{}, shiftRepo, &testutil.MockShiftAssignmentRepo{}, &testutil.MockEmployeeRepo{})

	_, err := svc.CreateSwapRequest(authCtx(uuid.New()), uuid.New(), uuid.New(), dto.CreateSwapRequest{ShiftInstanceID: uuid.New()})
	require.Error(t, err)
}

func TestSwapService_CreateSwapRequest_NoAssignment(t *testing.T) {
	tenantID := uuid.New()
	shift := testutil.NewShiftInstance(tenantID)

	shiftRepo := &testutil.MockShiftInstanceRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.ShiftInstance, error) {
			return shift, nil
		},
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByShiftFn: func(_ context.Context, _, _ uuid.UUID) ([]*model.ShiftAssignment, error) {
			return []*model.ShiftAssignment{}, nil // no assignments
		},
	}
	svc := newSwapService(&testutil.MockSwapRequestRepo{}, shiftRepo, assignRepo, &testutil.MockEmployeeRepo{})

	_, err := svc.CreateSwapRequest(authCtx(tenantID), tenantID, uuid.New(), dto.CreateSwapRequest{ShiftInstanceID: shift.ID})
	require.Error(t, err)
}

func TestSwapService_CreateSwapRequest_RepoError(t *testing.T) {
	tenantID := uuid.New()
	requesterID := uuid.New()
	shift := testutil.NewShiftInstance(tenantID)
	assignment := testutil.NewShiftAssignment(tenantID, shift.ID, requesterID)

	shiftRepo := &testutil.MockShiftInstanceRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.ShiftInstance, error) {
			return shift, nil
		},
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByShiftFn: func(_ context.Context, _, _ uuid.UUID) ([]*model.ShiftAssignment, error) {
			return []*model.ShiftAssignment{assignment}, nil
		},
	}
	swapRepo := &testutil.MockSwapRequestRepo{
		CreateFn: func(_ context.Context, _ *model.SwapRequest) error {
			return errors.New("db error")
		},
	}
	svc := newSwapService(swapRepo, shiftRepo, assignRepo, &testutil.MockEmployeeRepo{})

	_, err := svc.CreateSwapRequest(authCtx(tenantID), tenantID, requesterID, dto.CreateSwapRequest{ShiftInstanceID: shift.ID})
	require.Error(t, err)
}

// ─── GetSwapRequest ───────────────────────────────────────────────────────────

func TestSwapService_GetSwapRequest_Found(t *testing.T) {
	tenantID := uuid.New()
	sr := testutil.NewSwapRequest(tenantID, uuid.New(), uuid.New())

	swapRepo := &testutil.MockSwapRequestRepo{
		GetByIDFn: func(_ context.Context, tID, id uuid.UUID) (*model.SwapRequest, error) {
			assert.Equal(t, tenantID, tID)
			assert.Equal(t, sr.ID, id)
			return sr, nil
		},
	}
	svc := newSwapService(swapRepo, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockEmployeeRepo{})

	got, err := svc.GetSwapRequest(context.Background(), tenantID, sr.ID)
	require.NoError(t, err)
	assert.Equal(t, sr.ID, got.ID)
}

func TestSwapService_GetSwapRequest_NotFound(t *testing.T) {
	swapRepo := &testutil.MockSwapRequestRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.SwapRequest, error) {
			return nil, nil
		},
	}
	svc := newSwapService(swapRepo, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockEmployeeRepo{})

	_, err := svc.GetSwapRequest(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
}

// ─── ReviewSwapRequest ────────────────────────────────────────────────────────

func TestSwapService_ReviewSwapRequest_Reject(t *testing.T) {
	tenantID := uuid.New()
	sr := testutil.NewSwapRequest(tenantID, uuid.New(), uuid.New())
	updated := false

	swapRepo := &testutil.MockSwapRequestRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.SwapRequest, error) {
			return sr, nil
		},
		UpdateFn: func(_ context.Context, s *model.SwapRequest) error {
			updated = true
			assert.Equal(t, model.SwapStatusRejected, s.Status)
			return nil
		},
	}
	svc := newSwapService(swapRepo, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockEmployeeRepo{})

	got, err := svc.ReviewSwapRequest(authCtx(tenantID), tenantID, sr.ID, dto.ReviewSwapRequest{Status: model.SwapStatusRejected})
	require.NoError(t, err)
	assert.True(t, updated)
	assert.Equal(t, model.SwapStatusRejected, got.Status)
}

func TestSwapService_ReviewSwapRequest_InvalidStatus(t *testing.T) {
	tenantID := uuid.New()
	sr := testutil.NewSwapRequest(tenantID, uuid.New(), uuid.New())

	swapRepo := &testutil.MockSwapRequestRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.SwapRequest, error) {
			return sr, nil
		},
	}
	svc := newSwapService(swapRepo, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockEmployeeRepo{})

	_, err := svc.ReviewSwapRequest(authCtx(tenantID), tenantID, sr.ID, dto.ReviewSwapRequest{Status: "bogus"})
	require.Error(t, err)
}

func TestSwapService_ReviewSwapRequest_NotFound(t *testing.T) {
	swapRepo := &testutil.MockSwapRequestRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.SwapRequest, error) {
			return nil, nil
		},
	}
	svc := newSwapService(swapRepo, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockEmployeeRepo{})

	_, err := svc.ReviewSwapRequest(authCtx(uuid.New()), uuid.New(), uuid.New(), dto.ReviewSwapRequest{Status: model.SwapStatusRejected})
	require.Error(t, err)
}

// ─── ListByStore ──────────────────────────────────────────────────────────────

func TestSwapService_ListByStore(t *testing.T) {
	tenantID := uuid.New()
	srs := []*model.SwapRequest{testutil.NewSwapRequest(tenantID, uuid.New(), uuid.New())}

	swapRepo := &testutil.MockSwapRequestRepo{
		ListByStoreFn: func(_ context.Context, tID uuid.UUID, status string, _, _ int) ([]*model.SwapRequest, int64, error) {
			assert.Equal(t, tenantID, tID)
			return srs, 1, nil
		},
	}
	svc := newSwapService(swapRepo, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockEmployeeRepo{})

	got, total, err := svc.ListByStore(context.Background(), tenantID, "", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, got, 1)
}
