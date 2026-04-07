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
)

// TestE2E_Leave_Create verifies that an employee can submit a leave request.
// POST /stores/{storeId}/leave-requests → 201 (no RBAC restriction on create).
func TestE2E_Leave_Create(t *testing.T) {
	storeID := uuid.New()
	empID := uuid.New()
	sub := "sub-emp-leave"
	mocks := emptyMocks()

	emp := testutil.NewEmployee(storeID)
	emp.ID = empID
	emp.AuthID = sub
	mocks.emp.GetByAuthIDFn = func(_ context.Context, authID string) (*model.Employee, error) {
		if authID == sub {
			return emp, nil
		}
		return nil, nil
	}

	// CreateLeaveRequest calls empRepo.GetByID to validate employee
	mocks.emp.GetByIDFn = func(_ context.Context, _, _ uuid.UUID) (*model.Employee, error) {
		return emp, nil
	}
	mocks.leave.HasActiveLeaveFn = func(_ context.Context, _, _ uuid.UUID, _, _ time.Time) (bool, error) {
		return false, nil
	}
	mocks.leave.CreateFn = func(_ context.Context, lr *model.LeaveRequest) error {
		return nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, empID, sub, "employee")

	body := dto.CreateLeaveRequest{
		StartDate: time.Now().AddDate(0, 0, 7).Format("2006-01-02"),
		EndDate:   time.Now().AddDate(0, 0, 14).Format("2006-01-02"),
		Type:      "vacation",
	}

	resp := client.do(t, http.MethodPost, "/stores/"+storeID.String()+"/leave-requests", body)
	drain(resp)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

// TestE2E_Leave_Get verifies that an employee can retrieve their own leave request.
// GET /stores/{storeId}/leave-requests/{leaveId}
func TestE2E_Leave_Get(t *testing.T) {
	storeID := uuid.New()
	empID := uuid.New()
	sub := "sub-emp-leave-get"
	mocks := emptyMocks()

	emp := testutil.NewEmployee(storeID)
	emp.ID = empID
	emp.AuthID = sub
	mocks.emp.GetByAuthIDFn = func(_ context.Context, authID string) (*model.Employee, error) {
		if authID == sub {
			return emp, nil
		}
		return nil, nil
	}

	leave := testutil.NewLeaveRequest(storeID, empID)
	mocks.leave.GetByIDFn = func(_ context.Context, _, id uuid.UUID) (*model.LeaveRequest, error) {
		if id == leave.ID {
			return leave, nil
		}
		return nil, nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, empID, sub, "employee")

	resp := client.do(t, http.MethodGet, "/stores/"+storeID.String()+"/leave-requests/"+leave.ID.String(), nil)
	defer drain(resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result dto.LeaveRequestResponse
	decode(t, resp, &result)
	assert.Equal(t, leave.ID, result.ID)
}

// TestE2E_Leave_List verifies that a manager can list all leave requests for a store.
// GET /stores/{storeId}/leave-requests (requires PermViewLeave; managers see all via ListByStore).
func TestE2E_Leave_List(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-leave-list"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	leave := testutil.NewLeaveRequest(storeID, uuid.New())
	mocks.leave.ListByStoreFn = func(_ context.Context, tenantID uuid.UUID, status string, page, pageSize int) ([]*model.LeaveRequest, int64, error) {
		return []*model.LeaveRequest{leave}, 1, nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := client.do(t, http.MethodGet, "/stores/"+storeID.String()+"/leave-requests", nil)
	drain(resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestE2E_Leave_ListOwn verifies that an employee can list their own leave requests.
// GET /stores/{storeId}/leave-requests (requires PermViewLeave; employees see only their own via ListByEmployee).
func TestE2E_Leave_ListOwn(t *testing.T) {
	storeID := uuid.New()
	empID := uuid.New()
	sub := "sub-emp-leave-list-own"
	mocks := emptyMocks()

	emp := testutil.NewEmployee(storeID)
	emp.ID = empID
	emp.AuthID = sub
	mocks.emp.GetByAuthIDFn = func(_ context.Context, authID string) (*model.Employee, error) {
		if authID == sub {
			return emp, nil
		}
		return nil, nil
	}

	leave := testutil.NewLeaveRequest(storeID, empID)
	mocks.leave.ListByEmployeeFn = func(_ context.Context, tenantID, employeeID uuid.UUID, page, pageSize int) ([]*model.LeaveRequest, int64, error) {
		return []*model.LeaveRequest{leave}, 1, nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, empID, sub, "employee")

	resp := client.do(t, http.MethodGet, "/stores/"+storeID.String()+"/leave-requests", nil)
	drain(resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestE2E_Leave_Review verifies that a manager can approve or reject a leave request.
// PUT /stores/{storeId}/leave-requests/{leaveId}/review (requires PermManageLeave).
func TestE2E_Leave_Review(t *testing.T) {
	storeID := uuid.New()
	mgrID := uuid.New()
	mgrSub := "sub-mgr-leave-review"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	leave := testutil.NewLeaveRequest(storeID, uuid.New())
	leave.Status = model.LeaveStatusPending

	mocks.leave.GetByIDFn = func(_ context.Context, _, id uuid.UUID) (*model.LeaveRequest, error) {
		if id == leave.ID {
			return leave, nil
		}
		return nil, nil
	}
	mocks.leave.UpdateFn = func(_ context.Context, lr *model.LeaveRequest) error {
		return nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, mgrID, mgrSub, "manager")

	body := dto.ReviewLeaveRequest{Status: "approved"}

	resp := client.do(t, http.MethodPut, "/stores/"+storeID.String()+"/leave-requests/"+leave.ID.String()+"/review", body)
	drain(resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestE2E_Leave_Cancel verifies that an employee can cancel (delete) a pending leave request.
// DELETE /stores/{storeId}/leave-requests/{leaveId} → 204.
func TestE2E_Leave_Cancel(t *testing.T) {
	storeID := uuid.New()
	empID := uuid.New()
	sub := "sub-emp-leave-cancel"
	mocks := emptyMocks()

	emp := testutil.NewEmployee(storeID)
	emp.ID = empID
	emp.AuthID = sub
	mocks.emp.GetByAuthIDFn = func(_ context.Context, authID string) (*model.Employee, error) {
		if authID == sub {
			return emp, nil
		}
		return nil, nil
	}

	leave := testutil.NewLeaveRequest(storeID, empID)
	leave.Status = model.LeaveStatusPending

	mocks.leave.GetByIDFn = func(_ context.Context, _, id uuid.UUID) (*model.LeaveRequest, error) {
		if id == leave.ID {
			return leave, nil
		}
		return nil, nil
	}
	mocks.leave.DeleteFn = func(_ context.Context, _, _ uuid.UUID) error {
		return nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, empID, sub, "employee")

	resp := client.do(t, http.MethodDelete, "/stores/"+storeID.String()+"/leave-requests/"+leave.ID.String(), nil)
	drain(resp)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// TestE2E_Leave_Cancel_NotPending verifies that cancelling an already-approved leave request
// is rejected with 400.
func TestE2E_Leave_Cancel_NotPending(t *testing.T) {
	storeID := uuid.New()
	empID := uuid.New()
	sub := "sub-emp-leave-cancel-approved"
	mocks := emptyMocks()

	emp := testutil.NewEmployee(storeID)
	emp.ID = empID
	emp.AuthID = sub
	mocks.emp.GetByAuthIDFn = func(_ context.Context, authID string) (*model.Employee, error) {
		if authID == sub {
			return emp, nil
		}
		return nil, nil
	}

	leave := testutil.NewLeaveRequest(storeID, empID)
	leave.Status = model.LeaveStatusApproved // already approved — cannot cancel

	mocks.leave.GetByIDFn = func(_ context.Context, _, id uuid.UUID) (*model.LeaveRequest, error) {
		if id == leave.ID {
			return leave, nil
		}
		return nil, nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, empID, sub, "employee")

	resp := client.do(t, http.MethodDelete, "/stores/"+storeID.String()+"/leave-requests/"+leave.ID.String(), nil)
	drain(resp)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestE2E_Leave_Cancel_NotFound verifies that cancelling a nonexistent leave request returns 404.
func TestE2E_Leave_Cancel_NotFound(t *testing.T) {
	storeID := uuid.New()
	sub := "sub-emp-leave-cancel-404"
	mocks := emptyMocks()

	emp := testutil.NewEmployee(storeID)
	emp.AuthID = sub
	mocks.emp.GetByAuthIDFn = func(_ context.Context, authID string) (*model.Employee, error) {
		if authID == sub {
			return emp, nil
		}
		return nil, nil
	}
	// GetByIDFn nil → nil,nil → service returns NotFound

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), sub, "employee")

	resp := client.do(t, http.MethodDelete, "/stores/"+storeID.String()+"/leave-requests/"+uuid.New().String(), nil)
	drain(resp)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestE2E_Leave_Review_NotFound verifies that reviewing a nonexistent leave request
// returns 404.
func TestE2E_Leave_Review_NotFound(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-leave-404"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)
	// GetByIDFn nil → nil,nil → service returns NotFound

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	body := dto.ReviewLeaveRequest{Status: "approved"}
	resp := client.do(t, http.MethodPut, "/stores/"+storeID.String()+"/leave-requests/"+uuid.New().String()+"/review", body)
	drain(resp)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
