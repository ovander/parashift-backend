package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/handler"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/service"
	"github.com/ovander/parashift/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSwapHandler(
	swapRepo *testutil.MockSwapRequestRepo,
	shiftRepo *testutil.MockShiftInstanceRepo,
	assignRepo *testutil.MockShiftAssignmentRepo,
) *handler.SwapHandler {
	svc := service.NewSwapService(
		swapRepo, shiftRepo, assignRepo,
		&testutil.MockEmployeeRepo{},
		nil,
		newTestEmitter(), newTestLogger(),
	)
	return handler.NewSwapHandler(svc)
}

// ─── Create ───────────────────────────────────────────────────────────────────

func TestSwapHandler_Create_OK(t *testing.T) {
	storeID := uuid.New()
	requesterID := uuid.New()
	shift := testutil.NewShiftInstance(storeID)
	created := false

	shiftRepo := &testutil.MockShiftInstanceRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.ShiftInstance, error) {
			return shift, nil
		},
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByShiftFn: func(_ context.Context, _, _ uuid.UUID) ([]*model.ShiftAssignment, error) {
			// Return an assignment for the requester so the service is happy
			return []*model.ShiftAssignment{
				testutil.NewShiftAssignment(storeID, shift.ID, requesterID),
			}, nil
		},
	}
	swapRepo := &testutil.MockSwapRequestRepo{
		CreateFn: func(_ context.Context, _ *model.SwapRequest) error {
			created = true
			return nil
		},
	}

	svc := service.NewSwapService(swapRepo, shiftRepo, assignRepo, &testutil.MockEmployeeRepo{}, nil, newTestEmitter(), newTestLogger())
	h := handler.NewSwapHandler(svc)

	body := jsonBody(dto.CreateSwapRequest{
		ShiftInstanceID: shift.ID,
	})
	req := httptest.NewRequest(http.MethodPost, "/swaps", body)
	req.Header.Set("Content-Type", "application/json")
	// employeeCtx sets tenantID=storeID, userID=requesterID — the handler reads userID for the requester
	req = req.WithContext(employeeCtx(storeID, requesterID, "sub-requester"))
	req = withChiURLParam(req, "storeId", storeID.String())

	rr := httptest.NewRecorder()
	h.Create(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.True(t, created)
	var resp dto.SwapRequestResponse
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Equal(t, shift.ID, resp.ShiftInstanceID)
}

func TestSwapHandler_Create_BadJSON(t *testing.T) {
	storeID := uuid.New()
	h := newSwapHandler(&testutil.MockSwapRequestRepo{}, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{})

	req := httptest.NewRequest(http.MethodPost, "/swaps", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestSwapHandler_Create_MissingShiftID(t *testing.T) {
	storeID := uuid.New()
	h := newSwapHandler(&testutil.MockSwapRequestRepo{}, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{})

	// ShiftInstanceID is zero (uuid.Nil)
	body := jsonBody(dto.CreateSwapRequest{})
	req := httptest.NewRequest(http.MethodPost, "/swaps", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ─── Get ──────────────────────────────────────────────────────────────────────

func TestSwapHandler_Get_OK(t *testing.T) {
	storeID := uuid.New()
	requesterID := uuid.New()
	shiftID := uuid.New()
	sr := testutil.NewSwapRequest(storeID, requesterID, shiftID)

	swapRepo := &testutil.MockSwapRequestRepo{
		GetByIDFn: func(_ context.Context, tID, id uuid.UUID) (*model.SwapRequest, error) {
			assert.Equal(t, storeID, tID)
			assert.Equal(t, sr.ID, id)
			return sr, nil
		},
	}
	h := newSwapHandler(swapRepo, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{})

	req := httptest.NewRequest(http.MethodGet, "/swaps/"+sr.ID.String(), nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "swapId": sr.ID.String()})
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp dto.SwapRequestResponse
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Equal(t, sr.ID, resp.ID)
	assert.Equal(t, requesterID, resp.RequesterID)
}

func TestSwapHandler_Get_InvalidSwapID(t *testing.T) {
	storeID := uuid.New()
	h := newSwapHandler(&testutil.MockSwapRequestRepo{}, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{})

	req := httptest.NewRequest(http.MethodGet, "/swaps/bad", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "swapId": "bad"})
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ─── List ─────────────────────────────────────────────────────────────────────

func TestSwapHandler_List_OK(t *testing.T) {
	storeID := uuid.New()
	requesterID := uuid.New()
	shiftID := uuid.New()
	sr := testutil.NewSwapRequest(storeID, requesterID, shiftID)

	swapRepo := &testutil.MockSwapRequestRepo{
		ListByStoreFn: func(_ context.Context, _ uuid.UUID, _ string, _, _ int) ([]*model.SwapRequest, int64, error) {
			return []*model.SwapRequest{sr}, 1, nil
		},
	}
	h := newSwapHandler(swapRepo, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{})

	req := httptest.NewRequest(http.MethodGet, "/swaps", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.List(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

// ─── Review ───────────────────────────────────────────────────────────────────

func TestSwapHandler_Review_Reject(t *testing.T) {
	storeID := uuid.New()
	requesterID := uuid.New()
	shiftID := uuid.New()
	sr := testutil.NewSwapRequest(storeID, requesterID, shiftID)

	swapRepo := &testutil.MockSwapRequestRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.SwapRequest, error) {
			return sr, nil
		},
		UpdateFn: func(_ context.Context, s *model.SwapRequest) error {
			s.Status = model.SwapStatusRejected
			return nil
		},
	}
	// Use rejected so performSwap is NOT called (no assignment mock needed)
	h := newSwapHandler(swapRepo, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{})

	body := jsonBody(dto.ReviewSwapRequest{Status: model.SwapStatusRejected})
	req := httptest.NewRequest(http.MethodPut, "/swaps/"+sr.ID.String()+"/review", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "swapId": sr.ID.String()})
	rr := httptest.NewRecorder()
	h.Review(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp dto.SwapRequestResponse
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Equal(t, model.SwapStatusRejected, resp.Status)
}

func TestSwapHandler_Review_MissingStatus(t *testing.T) {
	storeID := uuid.New()
	swapID := uuid.New()
	h := newSwapHandler(&testutil.MockSwapRequestRepo{}, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{})

	body := jsonBody(dto.ReviewSwapRequest{Status: ""})
	req := httptest.NewRequest(http.MethodPut, "/swaps/"+swapID.String()+"/review", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "swapId": swapID.String()})
	rr := httptest.NewRecorder()
	h.Review(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestSwapHandler_Review_InvalidSwapID(t *testing.T) {
	storeID := uuid.New()
	h := newSwapHandler(&testutil.MockSwapRequestRepo{}, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{})

	body := jsonBody(dto.ReviewSwapRequest{Status: model.SwapStatusAccepted})
	req := httptest.NewRequest(http.MethodPut, "/swaps/bad/review", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "swapId": "bad"})
	rr := httptest.NewRecorder()
	h.Review(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
