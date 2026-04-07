package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/handler"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/service"
	"github.com/ovander/parashift/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCoverageHandler(covRepo *testutil.MockCoverageRequirementRepo) *handler.CoverageHandler {
	svc := service.NewCoverageService(
		covRepo,
		&testutil.MockShiftInstanceRepo{},
		&testutil.MockShiftAssignmentRepo{},
		&testutil.MockEmployeeRepo{},
		newTestEmitter(), newTestLogger(),
	)
	return handler.NewCoverageHandler(svc)
}

// ─── ListRequirements ─────────────────────────────────────────────────────────

func TestCoverageHandler_ListRequirements_OK(t *testing.T) {
	storeID := uuid.New()
	reqs := []*model.CoverageRequirement{testutil.NewCoverageRequirement(storeID)}

	covRepo := &testutil.MockCoverageRequirementRepo{
		ListFn: func(_ context.Context, _ uuid.UUID) ([]*model.CoverageRequirement, error) {
			return reqs, nil
		},
	}
	h := newCoverageHandler(covRepo)

	req := httptest.NewRequest(http.MethodGet, "/coverage", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.ListRequirements(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp []dto.CoverageRequirementResponse
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Len(t, resp, 1)
}

func TestCoverageHandler_ListRequirements_Forbidden(t *testing.T) {
	storeID := uuid.New()
	otherStore := uuid.New()
	h := newCoverageHandler(&testutil.MockCoverageRequirementRepo{})

	req := httptest.NewRequest(http.MethodGet, "/coverage", nil)
	req = req.WithContext(managerCtx(otherStore)) // tenantID != storeID, not admin
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.ListRequirements(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

// ─── CreateRequirement ────────────────────────────────────────────────────────

func TestCoverageHandler_CreateRequirement_OK(t *testing.T) {
	storeID := uuid.New()
	created := false

	covRepo := &testutil.MockCoverageRequirementRepo{
		CreateFn: func(_ context.Context, cr *model.CoverageRequirement) error {
			created = true
			assert.Equal(t, 1, cr.DayOfWeek)
			return nil
		},
	}
	h := newCoverageHandler(covRepo)

	role := "pharmacist"
	body := jsonBody(dto.CoverageRequirementRequest{
		DayOfWeek:    1,
		StartTime:    "09:00",
		EndTime:      "17:00",
		MinStaff:     2,
		RequiredRole: &role,
	})
	req := httptest.NewRequest(http.MethodPost, "/coverage", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.CreateRequirement(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.True(t, created)
	var resp dto.CoverageRequirementResponse
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Equal(t, 1, resp.DayOfWeek)
}

func TestCoverageHandler_CreateRequirement_BadJSON(t *testing.T) {
	storeID := uuid.New()
	h := newCoverageHandler(&testutil.MockCoverageRequirementRepo{})

	req := httptest.NewRequest(http.MethodPost, "/coverage", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.CreateRequirement(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ─── UpdateRequirement ────────────────────────────────────────────────────────

func TestCoverageHandler_UpdateRequirement_OK(t *testing.T) {
	storeID := uuid.New()
	existing := testutil.NewCoverageRequirement(storeID)
	updated := false

	covRepo := &testutil.MockCoverageRequirementRepo{
		ListFn: func(_ context.Context, _ uuid.UUID) ([]*model.CoverageRequirement, error) {
			return []*model.CoverageRequirement{existing}, nil
		},
		UpdateFn: func(_ context.Context, _ *model.CoverageRequirement) error {
			updated = true
			return nil
		},
	}
	h := newCoverageHandler(covRepo)

	body := jsonBody(dto.CoverageRequirementRequest{
		DayOfWeek: existing.DayOfWeek,
		StartTime: existing.StartTime,
		EndTime:   existing.EndTime,
		MinStaff:  5,
	})
	req := httptest.NewRequest(http.MethodPut, "/coverage/"+existing.ID.String(), body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "reqId": existing.ID.String()})
	rr := httptest.NewRecorder()
	h.UpdateRequirement(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, updated)
}

// ─── DeleteRequirement ────────────────────────────────────────────────────────

func TestCoverageHandler_DeleteRequirement_OK(t *testing.T) {
	storeID := uuid.New()
	reqID := uuid.New()
	deleted := false

	covRepo := &testutil.MockCoverageRequirementRepo{
		DeleteFn: func(_ context.Context, _, _ uuid.UUID) error {
			deleted = true
			return nil
		},
	}
	h := newCoverageHandler(covRepo)

	req := httptest.NewRequest(http.MethodDelete, "/coverage/"+reqID.String(), nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "reqId": reqID.String()})
	rr := httptest.NewRecorder()
	h.DeleteRequirement(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.True(t, deleted)
}

func TestCoverageHandler_DeleteRequirement_InvalidReqID(t *testing.T) {
	storeID := uuid.New()
	h := newCoverageHandler(&testutil.MockCoverageRequirementRepo{})

	req := httptest.NewRequest(http.MethodDelete, "/coverage/bad", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "reqId": "bad"})
	rr := httptest.NewRecorder()
	h.DeleteRequirement(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ─── GetCoverage ──────────────────────────────────────────────────────────────

func TestCoverageHandler_GetCoverage_OK(t *testing.T) {
	storeID := uuid.New()
	covRepo := &testutil.MockCoverageRequirementRepo{
		ListFn: func(_ context.Context, _ uuid.UUID) ([]*model.CoverageRequirement, error) {
			return []*model.CoverageRequirement{}, nil
		},
	}
	shiftRepo := &testutil.MockShiftInstanceRepo{
		ListByDateFn: func(_ context.Context, _ uuid.UUID, _ time.Time) ([]*model.ShiftInstance, error) {
			return nil, nil
		},
	}
	svc := service.NewCoverageService(covRepo, shiftRepo, &testutil.MockShiftAssignmentRepo{}, &testutil.MockEmployeeRepo{}, newTestEmitter(), newTestLogger())
	h := handler.NewCoverageHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/coverage/analysis?from=2026-03-01&to=2026-03-31", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.GetCoverage(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestCoverageHandler_GetCoverage_MissingParams(t *testing.T) {
	storeID := uuid.New()
	h := newCoverageHandler(&testutil.MockCoverageRequirementRepo{})

	req := httptest.NewRequest(http.MethodGet, "/coverage/analysis", nil) // no from/to
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.GetCoverage(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
