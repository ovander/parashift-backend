package e2e_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/testutil"
	"github.com/stretchr/testify/assert"
)

// TestE2E_Swap_Create verifies that an employee can create a shift swap request.
// POST /stores/{storeId}/swap-requests → 201.
func TestE2E_Swap_Create(t *testing.T) {
	storeID := uuid.New()
	empID := uuid.New()
	sub := "sub-emp-swap-create"
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

	shift := testutil.NewShiftInstance(storeID)
	mocks.shift.GetByIDFn = func(_ context.Context, _, id uuid.UUID) (*model.ShiftInstance, error) {
		if id == shift.ID {
			return shift, nil
		}
		return nil, nil
	}

	// CreateSwapRequest calls assignRepo.ListByShift to verify the requester has
	// a confirmed assignment for the shift before creating the swap request.
	assign := testutil.NewShiftAssignment(storeID, shift.ID, empID)
	mocks.assign.ListByShiftFn = func(_ context.Context, _, shiftID uuid.UUID) ([]*model.ShiftAssignment, error) {
		if shiftID == shift.ID {
			return []*model.ShiftAssignment{assign}, nil
		}
		return nil, nil
	}

	mocks.swap.CreateFn = func(_ context.Context, sr *model.SwapRequest) error {
		return nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, empID, sub, "employee")

	note := "Please swap this shift"
	body := dto.CreateSwapRequest{
		ShiftInstanceID: shift.ID,
		Note:            &note,
	}

	resp := client.do(t, http.MethodPost, "/stores/"+storeID.String()+"/swap-requests", body)
	drain(resp)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

// TestE2E_Swap_Get verifies that any authenticated user can retrieve a swap request by ID.
// GET /stores/{storeId}/swap-requests/{swapId}
func TestE2E_Swap_Get(t *testing.T) {
	storeID := uuid.New()
	empID := uuid.New()
	sub := "sub-emp-swap-get"
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

	swap := testutil.NewSwapRequest(storeID, empID, uuid.New())
	mocks.swap.GetByIDFn = func(_ context.Context, _, id uuid.UUID) (*model.SwapRequest, error) {
		if id == swap.ID {
			return swap, nil
		}
		return nil, nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, empID, sub, "employee")

	resp := client.do(t, http.MethodGet, "/stores/"+storeID.String()+"/swap-requests/"+swap.ID.String(), nil)
	defer drain(resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result dto.SwapRequestResponse
	decode(t, resp, &result)
	assert.Equal(t, swap.ID, result.ID)
}

// TestE2E_Swap_List verifies that an employee can list swap requests for a store.
// GET /stores/{storeId}/swap-requests
func TestE2E_Swap_List(t *testing.T) {
	storeID := uuid.New()
	empID := uuid.New()
	sub := "sub-emp-swap-list"
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

	swap := testutil.NewSwapRequest(storeID, empID, uuid.New())
	mocks.swap.ListByStoreFn = func(_ context.Context, tenantID uuid.UUID, status string, page, pageSize int) ([]*model.SwapRequest, int64, error) {
		return []*model.SwapRequest{swap}, 1, nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, empID, sub, "employee")

	resp := client.do(t, http.MethodGet, "/stores/"+storeID.String()+"/swap-requests", nil)
	drain(resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestE2E_Swap_Review verifies that a manager can reject a swap request.
// PUT /stores/{storeId}/swap-requests/{swapId}/review (requires PermManageSwap).
// We use "rejected" status to avoid triggering performSwap logic, which requires
// additional assignment setup.
func TestE2E_Swap_Review(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-swap-review"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	swap := testutil.NewSwapRequest(storeID, uuid.New(), uuid.New())
	swap.Status = model.SwapStatusPending

	mocks.swap.GetByIDFn = func(_ context.Context, _, id uuid.UUID) (*model.SwapRequest, error) {
		if id == swap.ID {
			return swap, nil
		}
		return nil, nil
	}
	mocks.swap.UpdateFn = func(_ context.Context, sr *model.SwapRequest) error {
		return nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	body := dto.ReviewSwapRequest{Status: "rejected"}

	resp := client.do(t, http.MethodPut, "/stores/"+storeID.String()+"/swap-requests/"+swap.ID.String()+"/review", body)
	drain(resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestE2E_Swap_Get_NotFound verifies that fetching a nonexistent swap returns 404.
func TestE2E_Swap_Get_NotFound(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-swap-404"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	// GetByIDFn nil → nil,nil → NotFound

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := client.do(t, http.MethodGet, "/stores/"+storeID.String()+"/swap-requests/"+uuid.New().String(), nil)
	drain(resp)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
