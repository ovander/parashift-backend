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

func newAvailabilityHandler(
	avRepo *testutil.MockAvailabilityRepo,
	empRepo *testutil.MockEmployeeRepo,
) *handler.AvailabilityHandler {
	svc := service.NewAvailabilityService(avRepo, empRepo, newTestEmitter(), newTestLogger())
	return handler.NewAvailabilityHandler(svc)
}

// ─── SetAvailability ──────────────────────────────────────────────────────────

func TestAvailabilityHandler_SetAvailability_OK(t *testing.T) {
	storeID := uuid.New()
	employeeID := uuid.New()

	empRepo := &testutil.MockEmployeeRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Employee, error) {
			return testutil.NewEmployee(storeID), nil
		},
	}
	avRepo := &testutil.MockAvailabilityRepo{
		UpsertFn: func(_ context.Context, _ *model.Availability) error { return nil },
	}
	h := newAvailabilityHandler(avRepo, empRepo)

	body := jsonBody(dto.SetAvailabilityRequest{
		Date:       time.Now(),
		TimeRanges: []dto.TimeSlot{{Start: "09:00", End: "17:00"}},
		Note:       func() *string { s := "OK"; return &s }(),
	})
	req := httptest.NewRequest(http.MethodPost, "/availability", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "employeeId": employeeID.String()})
	rr := httptest.NewRecorder()
	h.SetAvailability(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp dto.AvailabilityResponse
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Equal(t, employeeID, resp.EmployeeID)
}

func TestAvailabilityHandler_SetAvailability_BadJSON(t *testing.T) {
	storeID := uuid.New()
	h := newAvailabilityHandler(&testutil.MockAvailabilityRepo{}, &testutil.MockEmployeeRepo{})

	req := httptest.NewRequest(http.MethodPost, "/availability", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "employeeId": uuid.New().String()})
	rr := httptest.NewRecorder()
	h.SetAvailability(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAvailabilityHandler_SetAvailability_InvalidStoreID(t *testing.T) {
	h := newAvailabilityHandler(&testutil.MockAvailabilityRepo{}, &testutil.MockEmployeeRepo{})

	req := httptest.NewRequest(http.MethodPost, "/availability", nil)
	req = req.WithContext(managerCtx(uuid.New()))
	req = withChiURLParams(req, map[string]string{"storeId": "bad", "employeeId": uuid.New().String()})
	rr := httptest.NewRecorder()
	h.SetAvailability(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ─── GetAvailability ──────────────────────────────────────────────────────────

func TestAvailabilityHandler_GetAvailability_OK(t *testing.T) {
	storeID := uuid.New()
	employeeID := uuid.New()
	av := testutil.NewAvailability(storeID, employeeID)

	avRepo := &testutil.MockAvailabilityRepo{
		GetByEmployeeDateFn: func(_ context.Context, _, _ uuid.UUID, _ time.Time) (*model.Availability, error) {
			return av, nil
		},
	}
	h := newAvailabilityHandler(avRepo, &testutil.MockEmployeeRepo{})

	req := httptest.NewRequest(http.MethodGet, "/availability?date=2026-03-30", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "employeeId": employeeID.String()})
	rr := httptest.NewRecorder()
	h.GetAvailability(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp dto.AvailabilityResponse
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Equal(t, av.ID, resp.ID)
}

func TestAvailabilityHandler_GetAvailability_MissingDate(t *testing.T) {
	storeID := uuid.New()
	h := newAvailabilityHandler(&testutil.MockAvailabilityRepo{}, &testutil.MockEmployeeRepo{})

	req := httptest.NewRequest(http.MethodGet, "/availability", nil) // no date param
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "employeeId": uuid.New().String()})
	rr := httptest.NewRecorder()
	h.GetAvailability(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAvailabilityHandler_GetAvailability_InvalidDate(t *testing.T) {
	storeID := uuid.New()
	h := newAvailabilityHandler(&testutil.MockAvailabilityRepo{}, &testutil.MockEmployeeRepo{})

	req := httptest.NewRequest(http.MethodGet, "/availability?date=not-a-date", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "employeeId": uuid.New().String()})
	rr := httptest.NewRecorder()
	h.GetAvailability(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ─── ListAvailability ─────────────────────────────────────────────────────────

func TestAvailabilityHandler_ListAvailability_OK(t *testing.T) {
	storeID := uuid.New()
	employeeID := uuid.New()
	avs := []*model.Availability{testutil.NewAvailability(storeID, employeeID)}

	avRepo := &testutil.MockAvailabilityRepo{
		ListByEmployeeFn: func(_ context.Context, _, _ uuid.UUID, _, _ time.Time) ([]*model.Availability, error) {
			return avs, nil
		},
	}
	h := newAvailabilityHandler(avRepo, &testutil.MockEmployeeRepo{})

	req := httptest.NewRequest(http.MethodGet, "/availability?from=2026-03-01&to=2026-03-31", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "employeeId": employeeID.String()})
	rr := httptest.NewRecorder()
	h.ListAvailability(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAvailabilityHandler_ListAvailability_MissingParams(t *testing.T) {
	storeID := uuid.New()
	h := newAvailabilityHandler(&testutil.MockAvailabilityRepo{}, &testutil.MockEmployeeRepo{})

	req := httptest.NewRequest(http.MethodGet, "/availability", nil) // no from/to
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "employeeId": uuid.New().String()})
	rr := httptest.NewRecorder()
	h.ListAvailability(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
