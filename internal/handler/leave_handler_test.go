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

func newLeaveHandler(
	leaveRepo *testutil.MockLeaveRequestRepo,
	empRepo *testutil.MockEmployeeRepo,
) *handler.LeaveHandler {
	svc := service.NewLeaveService(
		leaveRepo, empRepo,
		&testutil.MockShiftAssignmentRepo{},
		&testutil.MockShiftInstanceRepo{},
		newTestEmitter(), newTestLogger(),
	)
	return handler.NewLeaveHandler(svc)
}

// ─── Create ───────────────────────────────────────────────────────────────────

func TestLeaveHandler_Create_OK(t *testing.T) {
	storeID := uuid.New()
	employeeID := uuid.New()
	reason := "Holiday"

	empRepo := &testutil.MockEmployeeRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Employee, error) {
			return testutil.NewEmployee(storeID), nil
		},
	}
	leaveRepo := &testutil.MockLeaveRequestRepo{
		HasActiveLeaveFn: func(_ context.Context, _, _ uuid.UUID, _, _ time.Time) (bool, error) { return false, nil },
		CreateFn:         func(_ context.Context, _ *model.LeaveRequest) error { return nil },
	}
	h := newLeaveHandler(leaveRepo, empRepo)

	body := jsonBody(map[string]any{
		"employee_id": employeeID.String(),
		"start_date":  time.Now().Format("2006-01-02"),
		"end_date":    time.Now().AddDate(0, 0, 5).Format("2006-01-02"),
		"type":        "vacation",
		"reason":      reason,
	})
	req := httptest.NewRequest(http.MethodPost, "/leaves", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp dto.LeaveRequestResponse
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Equal(t, model.LeaveStatusPending, resp.Status)
}

func TestLeaveHandler_Create_BadJSON(t *testing.T) {
	storeID := uuid.New()
	h := newLeaveHandler(&testutil.MockLeaveRequestRepo{}, &testutil.MockEmployeeRepo{})

	req := httptest.NewRequest(http.MethodPost, "/leaves", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestLeaveHandler_Create_MissingEmployeeID(t *testing.T) {
	storeID := uuid.New()
	h := newLeaveHandler(&testutil.MockLeaveRequestRepo{}, &testutil.MockEmployeeRepo{})

	// employee_id not provided — handler falls back to current user's UUID.
	// The mock empRepo has no GetByIDFn, so service returns NotFound → 404.
	body := jsonBody(map[string]any{
		"start_date": time.Now().Format("2006-01-02"),
		"end_date":   time.Now().AddDate(0, 0, 3).Format("2006-01-02"),
		"type":       "vacation",
	})
	req := httptest.NewRequest(http.MethodPost, "/leaves", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ─── Get ──────────────────────────────────────────────────────────────────────

func TestLeaveHandler_Get_OK(t *testing.T) {
	storeID := uuid.New()
	lr := testutil.NewLeaveRequest(storeID, uuid.New())

	leaveRepo := &testutil.MockLeaveRequestRepo{
		GetByIDFn: func(_ context.Context, tID, id uuid.UUID) (*model.LeaveRequest, error) {
			assert.Equal(t, storeID, tID)
			return lr, nil
		},
	}
	h := newLeaveHandler(leaveRepo, &testutil.MockEmployeeRepo{})

	req := httptest.NewRequest(http.MethodGet, "/leaves/"+lr.ID.String(), nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "leaveId": lr.ID.String()})
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp dto.LeaveRequestResponse
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Equal(t, lr.ID, resp.ID)
}

func TestLeaveHandler_Get_InvalidLeaveID(t *testing.T) {
	storeID := uuid.New()
	h := newLeaveHandler(&testutil.MockLeaveRequestRepo{}, &testutil.MockEmployeeRepo{})

	req := httptest.NewRequest(http.MethodGet, "/leaves/bad", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "leaveId": "bad"})
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ─── Review ───────────────────────────────────────────────────────────────────

func TestLeaveHandler_Review_Approve(t *testing.T) {
	storeID := uuid.New()
	lr := testutil.NewLeaveRequest(storeID, uuid.New())

	leaveRepo := &testutil.MockLeaveRequestRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.LeaveRequest, error) {
			return lr, nil
		},
		UpdateFn: func(_ context.Context, l *model.LeaveRequest) error {
			l.Status = model.LeaveStatusApproved
			return nil
		},
	}
	h := newLeaveHandler(leaveRepo, &testutil.MockEmployeeRepo{})

	body := jsonBody(dto.ReviewLeaveRequest{Status: model.LeaveStatusApproved})
	req := httptest.NewRequest(http.MethodPut, "/leaves/"+lr.ID.String()+"/review", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "leaveId": lr.ID.String()})
	rr := httptest.NewRecorder()
	h.Review(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp dto.LeaveRequestResponse
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Equal(t, model.LeaveStatusApproved, resp.Status)
}

func TestLeaveHandler_Review_InvalidStatus(t *testing.T) {
	storeID := uuid.New()
	lr := testutil.NewLeaveRequest(storeID, uuid.New())

	leaveRepo := &testutil.MockLeaveRequestRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.LeaveRequest, error) {
			return lr, nil
		},
	}
	h := newLeaveHandler(leaveRepo, &testutil.MockEmployeeRepo{})

	body := jsonBody(dto.ReviewLeaveRequest{Status: "invalid"})
	req := httptest.NewRequest(http.MethodPut, "/leaves/"+lr.ID.String()+"/review", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "leaveId": lr.ID.String()})
	rr := httptest.NewRecorder()
	h.Review(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ─── GetImpact ────────────────────────────────────────────────────────────────

// newLeaveHandlerFull constructs a handler whose underlying service has access
// to all four repos (needed by GetImpact which calls assignRepo and shiftRepo).
func newLeaveHandlerFull(
	leaveRepo  *testutil.MockLeaveRequestRepo,
	empRepo    *testutil.MockEmployeeRepo,
	assignRepo *testutil.MockShiftAssignmentRepo,
	shiftRepo  *testutil.MockShiftInstanceRepo,
) *handler.LeaveHandler {
	svc := service.NewLeaveService(leaveRepo, empRepo, assignRepo, shiftRepo, newTestEmitter(), newTestLogger())
	return handler.NewLeaveHandler(svc)
}

func TestLeaveHandler_GetImpact_OK(t *testing.T) {
	storeID    := uuid.New()
	employeeID := uuid.New()
	shiftID    := uuid.New()
	lr         := testutil.NewLeaveRequest(storeID, employeeID)
	assignment := testutil.NewShiftAssignment(storeID, shiftID, employeeID)
	assignment.ShiftStartTime = "09:00"
	assignment.ShiftEndTime   = "17:00"
	shift := testutil.NewShiftInstance(storeID)
	shift.ID = shiftID

	leaveRepo := &testutil.MockLeaveRequestRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.LeaveRequest, error) { return lr, nil },
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByEmployeeFn: func(_ context.Context, _, _ uuid.UUID, _, _ time.Time) ([]*model.ShiftAssignment, error) {
			return []*model.ShiftAssignment{assignment}, nil
		},
		ListByShiftFn: func(_ context.Context, _, _ uuid.UUID) ([]*model.ShiftAssignment, error) {
			return []*model.ShiftAssignment{assignment}, nil // sole assignee
		},
	}
	shiftRepo := &testutil.MockShiftInstanceRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.ShiftInstance, error) { return shift, nil },
	}

	h := newLeaveHandlerFull(leaveRepo, &testutil.MockEmployeeRepo{}, assignRepo, shiftRepo)

	req := httptest.NewRequest(http.MethodGet, "/leave-requests/"+lr.ID.String()+"/impact", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "leaveId": lr.ID.String()})
	rr := httptest.NewRecorder()
	h.GetImpact(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp dto.LeaveImpactResponse
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Equal(t, 1, resp.TotalCancellations)
	assert.Equal(t, 1, resp.UncoveredShifts)
}

func TestLeaveHandler_GetImpact_LeaveNotFound(t *testing.T) {
	storeID := uuid.New()

	leaveRepo := &testutil.MockLeaveRequestRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.LeaveRequest, error) { return nil, nil },
	}
	h := newLeaveHandlerFull(leaveRepo, &testutil.MockEmployeeRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockShiftInstanceRepo{})

	req := httptest.NewRequest(http.MethodGet, "/leave-requests/"+uuid.New().String()+"/impact", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "leaveId": uuid.New().String()})
	rr := httptest.NewRecorder()
	h.GetImpact(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestLeaveHandler_GetImpact_InvalidLeaveID(t *testing.T) {
	storeID := uuid.New()
	h := newLeaveHandlerFull(&testutil.MockLeaveRequestRepo{}, &testutil.MockEmployeeRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockShiftInstanceRepo{})

	req := httptest.NewRequest(http.MethodGet, "/leave-requests/not-a-uuid/impact", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "leaveId": "not-a-uuid"})
	rr := httptest.NewRecorder()
	h.GetImpact(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestLeaveHandler_GetImpact_NoConflicts(t *testing.T) {
	storeID := uuid.New()
	lr      := testutil.NewLeaveRequest(storeID, uuid.New())

	leaveRepo := &testutil.MockLeaveRequestRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.LeaveRequest, error) { return lr, nil },
	}
	// No assignments during leave period.
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByEmployeeFn: func(_ context.Context, _, _ uuid.UUID, _, _ time.Time) ([]*model.ShiftAssignment, error) {
			return nil, nil
		},
	}
	h := newLeaveHandlerFull(leaveRepo, &testutil.MockEmployeeRepo{}, assignRepo, &testutil.MockShiftInstanceRepo{})

	req := httptest.NewRequest(http.MethodGet, "/leave-requests/"+lr.ID.String()+"/impact", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "leaveId": lr.ID.String()})
	rr := httptest.NewRecorder()
	h.GetImpact(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp dto.LeaveImpactResponse
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Equal(t, 0, resp.TotalCancellations)
	assert.NotNil(t, resp.AffectedShifts, "affected_shifts should never be null in JSON response")
}
