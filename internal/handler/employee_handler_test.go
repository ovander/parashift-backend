package handler_test

import (
	"context"
	"errors"
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

func newEmployeeHandler(empRepo *testutil.MockEmployeeRepo) *handler.EmployeeHandler {
	svc := service.NewEmployeeService(empRepo, &testutil.MockContractRepo{}, newTestEmitter(), newTestLogger())
	return handler.NewEmployeeHandler(svc)
}

// ─── List ─────────────────────────────────────────────────────────────────────

func TestEmployeeHandler_List_OK(t *testing.T) {
	storeID := uuid.New()
	employees := []*model.Employee{
		testutil.NewEmployee(storeID),
		testutil.NewEmployee(storeID),
	}
	empRepo := &testutil.MockEmployeeRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, _, _ int) ([]*model.Employee, int64, error) {
			return employees, 2, nil
		},
	}
	h := newEmployeeHandler(empRepo)

	req := httptest.NewRequest(http.MethodGet, "/employees", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.List(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestEmployeeHandler_List_InvalidStoreID(t *testing.T) {
	h := newEmployeeHandler(&testutil.MockEmployeeRepo{})

	req := httptest.NewRequest(http.MethodGet, "/employees", nil)
	req = req.WithContext(managerCtx(uuid.New()))
	req = withChiURLParam(req, "storeId", "not-a-uuid")
	rr := httptest.NewRecorder()
	h.List(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ─── Get ──────────────────────────────────────────────────────────────────────

func TestEmployeeHandler_Get_OK(t *testing.T) {
	storeID := uuid.New()
	emp := testutil.NewEmployee(storeID)

	empRepo := &testutil.MockEmployeeRepo{
		GetByIDFn: func(_ context.Context, tID, id uuid.UUID) (*model.Employee, error) {
			assert.Equal(t, storeID, tID)
			assert.Equal(t, emp.ID, id)
			return emp, nil
		},
	}
	h := newEmployeeHandler(empRepo)

	req := httptest.NewRequest(http.MethodGet, "/employees/"+emp.ID.String(), nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "employeeId": emp.ID.String()})
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp dto.EmployeeResponse
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Equal(t, emp.ID, resp.ID)
	assert.Equal(t, emp.Name, resp.Name)
}

func TestEmployeeHandler_Get_NotFound(t *testing.T) {
	storeID := uuid.New()
	empRepo := &testutil.MockEmployeeRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Employee, error) {
			return nil, nil
		},
	}
	h := newEmployeeHandler(empRepo)

	req := httptest.NewRequest(http.MethodGet, "/employees/"+uuid.New().String(), nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "employeeId": uuid.New().String()})
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ─── Create ───────────────────────────────────────────────────────────────────

func TestEmployeeHandler_Create_OK(t *testing.T) {
	storeID := uuid.New()
	empRepo := &testutil.MockEmployeeRepo{
		CreateFn: func(_ context.Context, e *model.Employee) error {
			assert.Equal(t, storeID, e.TenantID)
			assert.Equal(t, "Jane Doe", e.Name)
			return nil
		},
	}
	h := newEmployeeHandler(empRepo)

	body := jsonBody(dto.CreateEmployeeRequest{
		Name:      "Jane Doe",
		Position:  "employee",
		JobRole:   "pharmacist",
		StartDate: time.Now(),
		AuthID:    "sub-jane",
	})
	req := httptest.NewRequest(http.MethodPost, "/employees", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp dto.EmployeeResponse
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Equal(t, "Jane Doe", resp.Name)
}

func TestEmployeeHandler_Create_BadJSON(t *testing.T) {
	storeID := uuid.New()
	h := newEmployeeHandler(&testutil.MockEmployeeRepo{})

	req := httptest.NewRequest(http.MethodPost, "/employees", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestEmployeeHandler_Create_RepoError(t *testing.T) {
	storeID := uuid.New()
	empRepo := &testutil.MockEmployeeRepo{
		CreateFn: func(_ context.Context, _ *model.Employee) error {
			return errors.New("db error")
		},
	}
	h := newEmployeeHandler(empRepo)

	body := jsonBody(dto.CreateEmployeeRequest{
		Name: "X", Position: "employee", JobRole: "cashier", StartDate: time.Now(), AuthID: "sub-x",
	})
	req := httptest.NewRequest(http.MethodPost, "/employees", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// ─── Update ───────────────────────────────────────────────────────────────────

func TestEmployeeHandler_Update_OK(t *testing.T) {
	storeID := uuid.New()
	emp := testutil.NewEmployee(storeID)
	newRole := "manager"

	empRepo := &testutil.MockEmployeeRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Employee, error) { return emp, nil },
		UpdateFn:  func(_ context.Context, e *model.Employee) error { e.Position = newRole; return nil },
	}
	h := newEmployeeHandler(empRepo)

	body := jsonBody(dto.UpdateEmployeeRequest{Position: &newRole})
	req := httptest.NewRequest(http.MethodPut, "/employees/"+emp.ID.String(), body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "employeeId": emp.ID.String()})
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp dto.EmployeeResponse
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Equal(t, newRole, resp.Position)
}

// ─── Delete ───────────────────────────────────────────────────────────────────

func TestEmployeeHandler_Delete_OK(t *testing.T) {
	storeID := uuid.New()
	empID := uuid.New()
	deleted := false

	empRepo := &testutil.MockEmployeeRepo{
		DeleteFn: func(_ context.Context, _, _ uuid.UUID) error { deleted = true; return nil },
	}
	h := newEmployeeHandler(empRepo)

	req := httptest.NewRequest(http.MethodDelete, "/employees/"+empID.String(), nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "employeeId": empID.String()})
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.True(t, deleted)
}

func TestEmployeeHandler_Delete_InvalidEmployeeID(t *testing.T) {
	storeID := uuid.New()
	h := newEmployeeHandler(&testutil.MockEmployeeRepo{})

	req := httptest.NewRequest(http.MethodDelete, "/employees/bad", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "employeeId": "bad"})
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
