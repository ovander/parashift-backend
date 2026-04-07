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

func newScheduleHandler(
	shiftRepo *testutil.MockShiftInstanceRepo,
	assignRepo *testutil.MockShiftAssignmentRepo,
	tmplRepo *testutil.MockWeekTemplateRepo,
) *handler.ScheduleHandler {
	svc := service.NewScheduleService(
		shiftRepo, assignRepo, tmplRepo,
		&testutil.MockEmployeeRepo{},
		&testutil.MockLeaveRequestRepo{},
		&testutil.MockAvailabilityRepo{},
		&testutil.MockStoreRepo{},
		newTestEmitter(), newTestLogger(),
	)
	return handler.NewScheduleHandler(svc)
}

// ─── GetSchedule ──────────────────────────────────────────────────────────────

func TestScheduleHandler_GetSchedule_OK(t *testing.T) {
	storeID := uuid.New()
	shift := testutil.NewShiftInstance(storeID)

	shiftRepo := &testutil.MockShiftInstanceRepo{
		ListByDateRangeFn: func(_ context.Context, _ uuid.UUID, _, _ time.Time, _, _ int) ([]*model.ShiftInstance, int64, error) {
			return []*model.ShiftInstance{shift}, 1, nil
		},
	}
	h := newScheduleHandler(shiftRepo, &testutil.MockShiftAssignmentRepo{}, &testutil.MockWeekTemplateRepo{})

	req := httptest.NewRequest(http.MethodGet, "/schedule?from=2026-03-01&to=2026-03-31", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.GetSchedule(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestScheduleHandler_GetSchedule_MissingParams(t *testing.T) {
	storeID := uuid.New()
	h := newScheduleHandler(&testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockWeekTemplateRepo{})

	req := httptest.NewRequest(http.MethodGet, "/schedule", nil) // no from/to
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.GetSchedule(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestScheduleHandler_GetSchedule_InvalidFrom(t *testing.T) {
	storeID := uuid.New()
	h := newScheduleHandler(&testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockWeekTemplateRepo{})

	req := httptest.NewRequest(http.MethodGet, "/schedule?from=not-a-date&to=2026-03-31", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.GetSchedule(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestScheduleHandler_GetSchedule_Forbidden(t *testing.T) {
	storeID := uuid.New()
	otherStore := uuid.New()
	h := newScheduleHandler(&testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockWeekTemplateRepo{})

	req := httptest.NewRequest(http.MethodGet, "/schedule?from=2026-03-01&to=2026-03-31", nil)
	req = req.WithContext(managerCtx(otherStore)) // tenantID != storeID
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.GetSchedule(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

// ─── CreateShift ──────────────────────────────────────────────────────────────

func TestScheduleHandler_CreateShift_OK(t *testing.T) {
	storeID := uuid.New()
	created := false

	shiftRepo := &testutil.MockShiftInstanceRepo{
		CreateFn: func(_ context.Context, s *model.ShiftInstance) error {
			created = true
			assert.Equal(t, storeID, s.TenantID)
			return nil
		},
	}
	h := newScheduleHandler(shiftRepo, &testutil.MockShiftAssignmentRepo{}, &testutil.MockWeekTemplateRepo{})

	body := jsonBody(dto.CreateShiftInstanceRequest{
		Date:      time.Now(),
		StartTime: "09:00",
		EndTime:   "17:00",
	})
	req := httptest.NewRequest(http.MethodPost, "/schedule", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.CreateShift(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.True(t, created)
}

func TestScheduleHandler_CreateShift_BadJSON(t *testing.T) {
	storeID := uuid.New()
	h := newScheduleHandler(&testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockWeekTemplateRepo{})

	req := httptest.NewRequest(http.MethodPost, "/schedule", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.CreateShift(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ─── GetShift ─────────────────────────────────────────────────────────────────

func TestScheduleHandler_GetShift_OK(t *testing.T) {
	storeID := uuid.New()
	shift := testutil.NewShiftInstance(storeID)

	shiftRepo := &testutil.MockShiftInstanceRepo{
		GetByIDFn: func(_ context.Context, tID, id uuid.UUID) (*model.ShiftInstance, error) {
			assert.Equal(t, storeID, tID)
			assert.Equal(t, shift.ID, id)
			return shift, nil
		},
	}
	h := newScheduleHandler(shiftRepo, &testutil.MockShiftAssignmentRepo{}, &testutil.MockWeekTemplateRepo{})

	req := httptest.NewRequest(http.MethodGet, "/schedule/"+shift.ID.String(), nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "shiftId": shift.ID.String()})
	rr := httptest.NewRecorder()
	h.GetShift(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestScheduleHandler_GetShift_InvalidShiftID(t *testing.T) {
	storeID := uuid.New()
	h := newScheduleHandler(&testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockWeekTemplateRepo{})

	req := httptest.NewRequest(http.MethodGet, "/schedule/bad", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "shiftId": "bad"})
	rr := httptest.NewRecorder()
	h.GetShift(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ─── UpdateShift ──────────────────────────────────────────────────────────────

func TestScheduleHandler_UpdateShift_OK(t *testing.T) {
	storeID := uuid.New()
	shift := testutil.NewShiftInstance(storeID)
	newRole := "technician"

	shiftRepo := &testutil.MockShiftInstanceRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.ShiftInstance, error) {
			return shift, nil
		},
		UpdateFn: func(_ context.Context, s *model.ShiftInstance) error {
			s.Role = newRole
			return nil
		},
	}
	h := newScheduleHandler(shiftRepo, &testutil.MockShiftAssignmentRepo{}, &testutil.MockWeekTemplateRepo{})

	body := jsonBody(dto.UpdateShiftInstanceRequest{Role: &newRole})
	req := httptest.NewRequest(http.MethodPut, "/schedule/"+shift.ID.String(), body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "shiftId": shift.ID.String()})
	rr := httptest.NewRecorder()
	h.UpdateShift(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestScheduleHandler_UpdateShift_BadJSON(t *testing.T) {
	storeID := uuid.New()
	shiftID := uuid.New()
	h := newScheduleHandler(&testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockWeekTemplateRepo{})

	req := httptest.NewRequest(http.MethodPut, "/schedule/"+shiftID.String(), nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "shiftId": shiftID.String()})
	rr := httptest.NewRecorder()
	h.UpdateShift(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ─── DeleteShift ──────────────────────────────────────────────────────────────

func TestScheduleHandler_DeleteShift_OK(t *testing.T) {
	storeID := uuid.New()
	shiftID := uuid.New()
	deleted := false

	shiftRepo := &testutil.MockShiftInstanceRepo{
		DeleteFn: func(_ context.Context, _, _ uuid.UUID) error {
			deleted = true
			return nil
		},
	}
	h := newScheduleHandler(shiftRepo, &testutil.MockShiftAssignmentRepo{}, &testutil.MockWeekTemplateRepo{})

	req := httptest.NewRequest(http.MethodDelete, "/schedule/"+shiftID.String(), nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "shiftId": shiftID.String()})
	rr := httptest.NewRecorder()
	h.DeleteShift(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.True(t, deleted)
}

func TestScheduleHandler_DeleteShift_InvalidShiftID(t *testing.T) {
	storeID := uuid.New()
	h := newScheduleHandler(&testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockWeekTemplateRepo{})

	req := httptest.NewRequest(http.MethodDelete, "/schedule/bad", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "shiftId": "bad"})
	rr := httptest.NewRecorder()
	h.DeleteShift(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ─── CreateAssignment ─────────────────────────────────────────────────────────

func TestScheduleHandler_CreateAssignment_OK(t *testing.T) {
	storeID := uuid.New()
	shift := testutil.NewShiftInstance(storeID)
	employeeID := uuid.New()
	emp := testutil.NewEmployee(storeID)
	created := false

	shiftRepo := &testutil.MockShiftInstanceRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.ShiftInstance, error) {
			return shift, nil
		},
	}
	empRepo := &testutil.MockEmployeeRepo{
		// CreateAssignment calls empRepo.GetByID to validate the employee exists
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Employee, error) {
			return emp, nil
		},
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		CreateFn: func(_ context.Context, _ *model.ShiftAssignment) error {
			created = true
			return nil
		},
		ExistsConflictFn: func(_ context.Context, _, _ uuid.UUID, _ time.Time, _, _ string, _ *uuid.UUID) (bool, error) {
			return false, nil
		},
	}
	svc := service.NewScheduleService(
		shiftRepo, assignRepo, &testutil.MockWeekTemplateRepo{},
		empRepo,
		&testutil.MockLeaveRequestRepo{},
		&testutil.MockAvailabilityRepo{},
		&testutil.MockStoreRepo{},
		newTestEmitter(), newTestLogger(),
	)
	h := handler.NewScheduleHandler(svc)

	body := jsonBody(dto.CreateAssignmentRequest{
		ShiftID: shift.ID,
		EmployeeID:      employeeID,
	})
	req := httptest.NewRequest(http.MethodPost, "/schedule/"+shift.ID.String()+"/assignments", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.CreateAssignment(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.True(t, created)
}

func TestScheduleHandler_CreateAssignment_MissingFields(t *testing.T) {
	storeID := uuid.New()
	h := newScheduleHandler(&testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockWeekTemplateRepo{})

	// Both ShiftID and EmployeeID are zero-value uuid.Nil
	body := jsonBody(dto.CreateAssignmentRequest{})
	req := httptest.NewRequest(http.MethodPost, "/schedule/assignments", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.CreateAssignment(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ─── GetAssignments ───────────────────────────────────────────────────────────

func TestScheduleHandler_GetAssignments_OK(t *testing.T) {
	storeID := uuid.New()
	shiftID := uuid.New()
	employeeID := uuid.New()
	assignment := testutil.NewShiftAssignment(storeID, shiftID, employeeID)

	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByShiftFn: func(_ context.Context, _, _ uuid.UUID) ([]*model.ShiftAssignment, error) {
			return []*model.ShiftAssignment{assignment}, nil
		},
	}
	h := newScheduleHandler(&testutil.MockShiftInstanceRepo{}, assignRepo, &testutil.MockWeekTemplateRepo{})

	req := httptest.NewRequest(http.MethodGet, "/schedule/"+shiftID.String()+"/assignments", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "shiftId": shiftID.String()})
	rr := httptest.NewRecorder()
	h.GetAssignments(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp []map[string]interface{}
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Len(t, resp, 1)
}

func TestScheduleHandler_GetAssignments_InvalidShiftID(t *testing.T) {
	storeID := uuid.New()
	h := newScheduleHandler(&testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockWeekTemplateRepo{})

	req := httptest.NewRequest(http.MethodGet, "/schedule/bad/assignments", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "shiftId": "bad"})
	rr := httptest.NewRecorder()
	h.GetAssignments(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ─── GenerateSchedule ─────────────────────────────────────────────────────────

func TestScheduleHandler_GenerateSchedule_OK(t *testing.T) {
	storeID := uuid.New()

	shiftRepo := &testutil.MockShiftInstanceRepo{
		DeleteBySourceTemplateFn: func(_ context.Context, _ uuid.UUID, _, _ time.Time) error { return nil },
		CreateBatchFn:            func(_ context.Context, _ []*model.ShiftInstance) error { return nil },
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		CreateFn: func(_ context.Context, _ *model.ShiftAssignment) error { return nil },
	}
	h := newScheduleHandler(shiftRepo, assignRepo, &testutil.MockWeekTemplateRepo{})

	body := jsonBody(map[string]string{
		"date_from": "2026-03-30",
		"date_to":   "2026-04-05",
	})
	req := httptest.NewRequest(http.MethodPost, "/schedule/generate", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.GenerateSchedule(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]int
	require.NoError(t, decodeJSON(rr, &resp))
	_, ok := resp["shifts_created"]
	assert.True(t, ok, "response must contain shifts_created")
}

func TestScheduleHandler_GenerateSchedule_MissingDates(t *testing.T) {
	storeID := uuid.New()
	h := newScheduleHandler(&testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockWeekTemplateRepo{})

	body := jsonBody(map[string]string{}) // no date_from / date_to
	req := httptest.NewRequest(http.MethodPost, "/schedule/generate", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.GenerateSchedule(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestScheduleHandler_GenerateSchedule_InvalidDateFormat(t *testing.T) {
	storeID := uuid.New()
	h := newScheduleHandler(&testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockWeekTemplateRepo{})

	body := jsonBody(map[string]string{
		"date_from": "30-03-2026", // wrong format
		"date_to":   "2026-04-05",
	})
	req := httptest.NewRequest(http.MethodPost, "/schedule/generate", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.GenerateSchedule(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestScheduleHandler_GenerateSchedule_InvalidStoreID(t *testing.T) {
	storeID := uuid.New()
	h := newScheduleHandler(&testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockWeekTemplateRepo{})

	body := jsonBody(map[string]string{"date_from": "2026-03-30", "date_to": "2026-04-05"})
	req := httptest.NewRequest(http.MethodPost, "/schedule/generate", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", "not-a-uuid")
	rr := httptest.NewRecorder()
	h.GenerateSchedule(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
