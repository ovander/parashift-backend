package e2e_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_Employee_List verifies that a manager can list employees for their store.
// The route is GET /stores/{storeId}/employees (requires PermManageEmployees).
func TestE2E_Employee_List(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-list"
	mocks := emptyMocks()
	mgr := withManagerEmp(mocks, storeID, mgrSub)
	_ = mgr

	emp1 := testutil.NewEmployee(storeID)
	mocks.emp.ListFn = func(_ context.Context, tenantID uuid.UUID, page, pageSize int) ([]*model.Employee, int64, error) {
		return []*model.Employee{emp1}, 1, nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := client.do(t, http.MethodGet, "/stores/"+storeID.String()+"/employees", nil)
	defer drain(resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestE2E_Employee_Create verifies that a manager can create a new employee.
// The response contains the created employee ID.
func TestE2E_Employee_Create(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-create"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	mocks.emp.CreateFn = func(_ context.Context, e *model.Employee) error {
		return nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	body := dto.CreateEmployeeRequest{
		Name:      "Alice Dumont",
		Position:  "employee",
		JobRole:   "pharmacist",
		StartDate: time.Now(),
		AuthID:    "auth-alice-001",
	}

	resp := client.do(t, http.MethodPost, "/stores/"+storeID.String()+"/employees", body)
	drain(resp)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

// TestE2E_Employee_Get verifies that a manager can retrieve an employee by ID.
func TestE2E_Employee_Get(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-get"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	emp := testutil.NewEmployee(storeID)
	mocks.emp.GetByIDFn = func(_ context.Context, tenantID, id uuid.UUID) (*model.Employee, error) {
		return emp, nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := client.do(t, http.MethodGet, "/stores/"+storeID.String()+"/employees/"+emp.ID.String(), nil)
	defer drain(resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result dto.EmployeeResponse
	decode(t, resp, &result)
	assert.Equal(t, emp.ID, result.ID)
	assert.Equal(t, "Alice Dumont", result.Name)
}

// TestE2E_Employee_Update verifies that a manager can update an employee's details.
func TestE2E_Employee_Update(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-update"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	emp := testutil.NewEmployee(storeID)
	mocks.emp.GetByIDFn = func(_ context.Context, tenantID, id uuid.UUID) (*model.Employee, error) {
		return emp, nil
	}
	mocks.emp.UpdateFn = func(_ context.Context, e *model.Employee) error {
		return nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	newName := "Alice Smith"
	body := dto.UpdateEmployeeRequest{Name: &newName}

	resp := client.do(t, http.MethodPut, "/stores/"+storeID.String()+"/employees/"+emp.ID.String(), body)
	drain(resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestE2E_Employee_Delete verifies that a manager can delete an employee.
func TestE2E_Employee_Delete(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-delete"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	emp := testutil.NewEmployee(storeID)
	mocks.emp.GetByIDFn = func(_ context.Context, tenantID, id uuid.UUID) (*model.Employee, error) {
		return emp, nil
	}
	mocks.emp.DeleteFn = func(_ context.Context, tenantID, id uuid.UUID) error {
		return nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := client.do(t, http.MethodDelete, "/stores/"+storeID.String()+"/employees/"+emp.ID.String(), nil)
	drain(resp)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// TestE2E_Employee_Get_NotFound verifies that requesting a nonexistent employee returns 404.
func TestE2E_Employee_Get_NotFound(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-notfound"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	// GetByIDFn is nil → mock returns nil,nil → service returns NotFound
	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := client.do(t, http.MethodGet, "/stores/"+storeID.String()+"/employees/"+uuid.New().String(), nil)
	drain(resp)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestE2E_Employee_InvalidID verifies that a malformed employee ID returns 400.
func TestE2E_Employee_InvalidID(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-invalid"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := client.do(t, http.MethodGet, "/stores/"+storeID.String()+"/employees/not-a-uuid", nil)
	drain(resp)

	// Handler parses UUID from URL param; invalid UUID → 400
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
