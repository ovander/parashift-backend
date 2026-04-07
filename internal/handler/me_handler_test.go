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

func newMeHandler(
	empRepo *testutil.MockEmployeeRepo,
	shiftRepo *testutil.MockShiftInstanceRepo,
	assignRepo *testutil.MockShiftAssignmentRepo,
) *handler.MeHandler {
	empSvc := service.NewEmployeeService(empRepo, &testutil.MockContractRepo{}, newTestEmitter(), newTestLogger())
	schSvc := service.NewScheduleService(
		shiftRepo, assignRepo, &testutil.MockWeekTemplateRepo{},
		empRepo,
		&testutil.MockLeaveRequestRepo{},
		&testutil.MockAvailabilityRepo{},
		&testutil.MockStoreRepo{},
		newTestEmitter(), newTestLogger(),
	)
	return handler.NewMeHandler(empSvc, schSvc)
}

// meCtx is a context that represents an authenticated employee user.
func meCtx(storeID, userID uuid.UUID, sub string) context.Context {
	return employeeCtx(storeID, userID, sub)
}

// ─── GetProfile ───────────────────────────────────────────────────────────────

func TestMeHandler_GetProfile_OK(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	sub := "sub-alice-123"
	emp := testutil.NewEmployee(storeID)
	emp.AuthID = sub

	empRepo := &testutil.MockEmployeeRepo{
		GetByAuthIDFn: func(_ context.Context, authID string) (*model.Employee, error) {
			assert.Equal(t, sub, authID)
			return emp, nil
		},
	}
	h := newMeHandler(empRepo, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{})

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req = req.WithContext(meCtx(storeID, userID, sub))
	rr := httptest.NewRecorder()
	h.GetProfile(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp dto.EmployeeResponse
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Equal(t, emp.ID, resp.ID)
	assert.Equal(t, emp.Name, resp.Name)
}

func TestMeHandler_GetProfile_NotFound(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	sub := "sub-unknown"

	empRepo := &testutil.MockEmployeeRepo{
		// GetByAuthIDFn returns nil — employee not found
		GetByAuthIDFn: func(_ context.Context, _ string) (*model.Employee, error) {
			return nil, nil
		},
	}
	h := newMeHandler(empRepo, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{})

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req = req.WithContext(meCtx(storeID, userID, sub))
	rr := httptest.NewRecorder()
	h.GetProfile(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ─── GetMySchedule ────────────────────────────────────────────────────────────

func TestMeHandler_GetMySchedule_OK(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	sub := "sub-alice-123"
	emp := testutil.NewEmployee(storeID)
	emp.AuthID = sub

	empRepo := &testutil.MockEmployeeRepo{
		GetByAuthIDFn: func(_ context.Context, _ string) (*model.Employee, error) {
			return emp, nil
		},
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByEmployeeFn: func(_ context.Context, _, _ uuid.UUID, _, _ time.Time) ([]*model.ShiftAssignment, error) {
			return nil, nil
		},
	}
	h := newMeHandler(empRepo, &testutil.MockShiftInstanceRepo{}, assignRepo)

	req := httptest.NewRequest(http.MethodGet, "/me/schedule?from=2026-03-01&to=2026-03-31", nil)
	req = req.WithContext(meCtx(storeID, userID, sub))
	rr := httptest.NewRecorder()
	h.GetMySchedule(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestMeHandler_GetMySchedule_MissingParams(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	sub := "sub-alice-123"
	emp := testutil.NewEmployee(storeID)
	emp.AuthID = sub

	// Must have GetByAuthIDFn so the handler gets past the employee lookup
	empRepo := &testutil.MockEmployeeRepo{
		GetByAuthIDFn: func(_ context.Context, _ string) (*model.Employee, error) {
			return emp, nil
		},
	}
	h := newMeHandler(empRepo, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{})

	req := httptest.NewRequest(http.MethodGet, "/me/schedule", nil) // no from/to
	req = req.WithContext(meCtx(storeID, userID, sub))
	rr := httptest.NewRecorder()
	h.GetMySchedule(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestMeHandler_GetMySchedule_EmployeeNotFound(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	sub := "sub-unknown"

	empRepo := &testutil.MockEmployeeRepo{
		GetByAuthIDFn: func(_ context.Context, _ string) (*model.Employee, error) {
			return nil, nil
		},
	}
	h := newMeHandler(empRepo, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{})

	req := httptest.NewRequest(http.MethodGet, "/me/schedule?from=2026-03-01&to=2026-03-31", nil)
	req = req.WithContext(meCtx(storeID, userID, sub))
	rr := httptest.NewRecorder()
	h.GetMySchedule(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ─── ExportICS ────────────────────────────────────────────────────────────────

func TestMeHandler_ExportICS_OK(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	sub := "sub-alice-123"
	emp := testutil.NewEmployee(storeID)
	emp.AuthID = sub

	empRepo := &testutil.MockEmployeeRepo{
		GetByAuthIDFn: func(_ context.Context, _ string) (*model.Employee, error) {
			return emp, nil
		},
		// GenerateICS also calls GetByID internally
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Employee, error) {
			return emp, nil
		},
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByEmployeeFn: func(_ context.Context, _, _ uuid.UUID, _, _ time.Time) ([]*model.ShiftAssignment, error) {
			return nil, nil // no shifts → empty ICS still valid
		},
	}
	h := newMeHandler(empRepo, &testutil.MockShiftInstanceRepo{}, assignRepo)

	req := httptest.NewRequest(http.MethodGet, "/me/schedule/ics?from=2026-03-01&to=2026-03-31", nil)
	req = req.WithContext(meCtx(storeID, userID, sub))
	rr := httptest.NewRecorder()
	h.ExportICS(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "text/calendar", rr.Header().Get("Content-Type"))
}

func TestMeHandler_ExportICS_MissingParams(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	sub := "sub-alice-123"
	emp := testutil.NewEmployee(storeID)
	emp.AuthID = sub

	// Must have GetByAuthIDFn so the handler gets past the employee lookup
	empRepo := &testutil.MockEmployeeRepo{
		GetByAuthIDFn: func(_ context.Context, _ string) (*model.Employee, error) {
			return emp, nil
		},
	}
	h := newMeHandler(empRepo, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{})

	req := httptest.NewRequest(http.MethodGet, "/me/schedule/ics", nil) // no from/to
	req = req.WithContext(meCtx(storeID, userID, sub))
	rr := httptest.NewRecorder()
	h.ExportICS(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
