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

func newWeekTemplateHandler(tmplRepo *testutil.MockWeekTemplateRepo) *handler.WeekTemplateHandler {
	svc := service.NewScheduleService(
		&testutil.MockShiftInstanceRepo{},
		&testutil.MockShiftAssignmentRepo{},
		tmplRepo,
		&testutil.MockEmployeeRepo{},
		&testutil.MockLeaveRequestRepo{},
		&testutil.MockAvailabilityRepo{},
		&testutil.MockStoreRepo{},
		newTestEmitter(), newTestLogger(),
	)
	return handler.NewWeekTemplateHandler(svc)
}

// ─── GetTemplates ─────────────────────────────────────────────────────────────

func TestWeekTemplateHandler_GetTemplates_OK(t *testing.T) {
	storeID := uuid.New()
	employeeID := uuid.New()

	tmplRepo := &testutil.MockWeekTemplateRepo{
		GetByEmployeeFn: func(_ context.Context, _, _ uuid.UUID) ([]*model.WeekTemplate, error) {
			return []*model.WeekTemplate{
				{
					TenantScoped: model.TenantScoped{
						ID:       uuid.New(),
						TenantID: storeID,
					},
					EmployeeID: employeeID,
					WeekType:   "A",
					DayOfWeek:  1,
					StartTime:  "09:00",
					EndTime:    "17:00",
				},
			}, nil
		},
	}
	h := newWeekTemplateHandler(tmplRepo)

	req := httptest.NewRequest(http.MethodGet, "/week-templates", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "employeeId": employeeID.String()})
	rr := httptest.NewRecorder()
	h.GetTemplates(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp dto.WeekTemplateResponse
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Equal(t, employeeID, resp.EmployeeID)
}

func TestWeekTemplateHandler_GetTemplates_InvalidEmployeeID(t *testing.T) {
	storeID := uuid.New()
	h := newWeekTemplateHandler(&testutil.MockWeekTemplateRepo{})

	req := httptest.NewRequest(http.MethodGet, "/week-templates", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "employeeId": "bad"})
	rr := httptest.NewRecorder()
	h.GetTemplates(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestWeekTemplateHandler_GetTemplates_Forbidden(t *testing.T) {
	storeID := uuid.New()
	otherStore := uuid.New()
	h := newWeekTemplateHandler(&testutil.MockWeekTemplateRepo{})

	req := httptest.NewRequest(http.MethodGet, "/week-templates", nil)
	req = req.WithContext(managerCtx(otherStore)) // tenantID != storeID
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "employeeId": uuid.New().String()})
	rr := httptest.NewRecorder()
	h.GetTemplates(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

// ─── UpsertTemplates ──────────────────────────────────────────────────────────

func TestWeekTemplateHandler_UpsertTemplates_OK(t *testing.T) {
	storeID := uuid.New()
	employeeID := uuid.New()
	upserted := false

	tmplRepo := &testutil.MockWeekTemplateRepo{
		UpsertForEmployeeFn: func(_ context.Context, _, _ uuid.UUID, _ []*model.WeekTemplate) error {
			upserted = true
			return nil
		},
		GetByEmployeeFn: func(_ context.Context, _, eID uuid.UUID) ([]*model.WeekTemplate, error) {
			return []*model.WeekTemplate{
				{
					TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: storeID},
					EmployeeID:   eID,
					WeekType:     "A",
					DayOfWeek:    1,
					StartTime:    "09:00",
					EndTime:      "17:00",
				},
			}, nil
		},
	}
	empRepo := &testutil.MockEmployeeRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Employee, error) {
			return testutil.NewEmployee(storeID), nil
		},
	}
	svc := service.NewScheduleService(
		&testutil.MockShiftInstanceRepo{},
		&testutil.MockShiftAssignmentRepo{},
		tmplRepo,
		empRepo,
		&testutil.MockLeaveRequestRepo{},
		&testutil.MockAvailabilityRepo{},
		&testutil.MockStoreRepo{},
		newTestEmitter(), newTestLogger(),
	)
	h := handler.NewWeekTemplateHandler(svc)

	body := jsonBody(dto.UpsertWeekTemplateRequest{
		EmployeeID: employeeID,
		Templates: []dto.WeekTemplateEntry{
			{WeekType: "A", DayOfWeek: 1, StartTime: "09:00", EndTime: "17:00"},
		},
	})
	req := httptest.NewRequest(http.MethodPut, "/week-templates", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "employeeId": employeeID.String()})
	rr := httptest.NewRecorder()
	h.UpsertTemplates(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, upserted)
}

func TestWeekTemplateHandler_UpsertTemplates_EmptyTemplates(t *testing.T) {
	storeID := uuid.New()
	employeeID := uuid.New()
	h := newWeekTemplateHandler(&testutil.MockWeekTemplateRepo{})

	body := jsonBody(dto.UpsertWeekTemplateRequest{
		EmployeeID: employeeID,
		Templates:  []dto.WeekTemplateEntry{}, // empty — rejected
	})
	req := httptest.NewRequest(http.MethodPut, "/week-templates", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "employeeId": employeeID.String()})
	rr := httptest.NewRecorder()
	h.UpsertTemplates(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestWeekTemplateHandler_UpsertTemplates_BadJSON(t *testing.T) {
	storeID := uuid.New()
	h := newWeekTemplateHandler(&testutil.MockWeekTemplateRepo{})

	req := httptest.NewRequest(http.MethodPut, "/week-templates", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "employeeId": uuid.New().String()})
	rr := httptest.NewRecorder()
	h.UpsertTemplates(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
