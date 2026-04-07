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

// ─── Employee role restrictions ───────────────────────────────────────────────

// TestE2E_RBAC_EmployeeCanListEmployees verifies that an employee (position="employee")
// can read the employee list (PermViewEmployees is granted to both employee and manager).
// Employees need to see their colleagues for scheduling and contact purposes.
func TestE2E_RBAC_EmployeeCanListEmployees(t *testing.T) {
	storeID := uuid.New()
	empID := uuid.New()
	sub := "sub-rbac-emp-nolist"
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
	mocks.emp.ListFn = func(_ context.Context, _ uuid.UUID, _, _ int) ([]*model.Employee, int64, error) {
		return []*model.Employee{emp}, 1, nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, empID, sub, "employee")

	resp := client.do(t, http.MethodGet, "/stores/"+storeID.String()+"/employees", nil)
	drain(resp)

	// Employees have PermViewEmployees → read-only access to the list is allowed.
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestE2E_RBAC_EmployeeCannotCreateShift verifies that an employee cannot create shifts
// (requires PermManageSchedule → manager).
func TestE2E_RBAC_EmployeeCannotCreateShift(t *testing.T) {
	storeID := uuid.New()
	empID := uuid.New()
	sub := "sub-rbac-emp-noshift"
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

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, empID, sub, "employee")

	body := dto.CreateShiftInstanceRequest{
		Date:      time.Now(),
		StartTime: "09:00",
		EndTime:   "17:00",
	}
	resp := client.do(t, http.MethodPost, "/stores/"+storeID.String()+"/shifts", body)
	drain(resp)

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// TestE2E_RBAC_EmployeeCanViewSchedule verifies that an employee CAN read the store
// schedule (requires PermViewSchedule, which employees have).
func TestE2E_RBAC_EmployeeCanViewSchedule(t *testing.T) {
	storeID := uuid.New()
	empID := uuid.New()
	sub := "sub-rbac-emp-view"
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
	mocks.shift.ListByDateRangeFn = func(_ context.Context, _ uuid.UUID, _, _ time.Time, _, _ int) ([]*model.ShiftInstance, int64, error) {
		return nil, 0, nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, empID, sub, "employee")

	resp := client.do(t, http.MethodGet, "/stores/"+storeID.String()+"/schedule?from=2026-04-01&to=2026-04-30", nil)
	drain(resp)

	// Employee has PermViewSchedule → should reach the handler, returning 200
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestE2E_RBAC_EmployeeCannotReviewLeave verifies that an employee cannot review
// leave requests (requires PermManageLeave → manager).
func TestE2E_RBAC_EmployeeCannotReviewLeave(t *testing.T) {
	storeID := uuid.New()
	empID := uuid.New()
	sub := "sub-rbac-emp-noreview"
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

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, empID, sub, "employee")

	body := dto.ReviewLeaveRequest{Status: "approved"}
	resp := client.do(t, http.MethodPut,
		"/stores/"+storeID.String()+"/leave-requests/"+uuid.New().String()+"/review", body)
	drain(resp)

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// ─── Manager role access ──────────────────────────────────────────────────────

// TestE2E_RBAC_ManagerCanManageEmployees verifies that a manager can create employees.
func TestE2E_RBAC_ManagerCanManageEmployees(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-rbac-mgr-emp"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	mocks.emp.CreateFn = func(_ context.Context, e *model.Employee) error {
		return nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	body := dto.CreateEmployeeRequest{
		Name:      "Bob Martin",
		Position:  "employee",
		JobRole:   "pharmacist",
		StartDate: time.Now(),
		AuthID:    "auth-bob-002",
	}
	resp := client.do(t, http.MethodPost, "/stores/"+storeID.String()+"/employees", body)
	drain(resp)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

// TestE2E_RBAC_ManagerCanReviewSwap verifies that a manager can review swap requests.
func TestE2E_RBAC_ManagerCanReviewSwap(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-rbac-mgr-swap"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	swap := testutil.NewSwapRequest(storeID, uuid.New(), uuid.New())
	mocks.swap.GetByIDFn = func(_ context.Context, _, id uuid.UUID) (*model.SwapRequest, error) {
		if id == swap.ID {
			return swap, nil
		}
		return nil, nil
	}
	mocks.swap.UpdateFn = func(_ context.Context, _ *model.SwapRequest) error {
		return nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	body := dto.ReviewSwapRequest{Status: "rejected"}
	resp := client.do(t, http.MethodPut,
		"/stores/"+storeID.String()+"/swap-requests/"+swap.ID.String()+"/review", body)
	drain(resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// ─── Tenant isolation ─────────────────────────────────────────────────────────

// TestE2E_RBAC_TenantIsolation verifies that TenantMiddleware ignores any tenant
// claim in the JWT and always uses the employee's DB record as the source of truth.
// Even if a user sends X-Test-TenantID for store B, they get store A's data because
// the middleware overwrites the context with emp.TenantID (store A).
// Cross-tenant URL access is separately tested in TestE2E_RBAC_TenantIsolation_StoreRoute.
func TestE2E_RBAC_TenantIsolation(t *testing.T) {
	storeA := uuid.New()
	storeB := uuid.New()
	sub := "sub-rbac-wrong-tenant"
	mocks := emptyMocks()

	// emp belongs to store A
	emp := testutil.NewEmployee(storeA)
	emp.AuthID = sub
	mocks.emp.GetByAuthIDFn = func(_ context.Context, authID string) (*model.Employee, error) {
		if authID == sub {
			return emp, nil
		}
		return nil, nil
	}

	ts := newTestServer(t, mocks)
	// Client sends X-Test-TenantID = store B (attempted tenant injection), but
	// TenantMiddleware will override with emp.TenantID = store A.
	// The request still succeeds and returns the employee's own (store A) profile.
	_ = storeB // only store A matters; B is present to document the injection attempt
	client := newClient(ts, storeB, uuid.New(), sub, "employee")

	resp := client.do(t, http.MethodGet, "/me", nil)
	drain(resp)

	// TenantMiddleware corrects the tenant to storeA; /me returns 200 with correct profile.
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestE2E_RBAC_TenantIsolation_StoreRoute verifies that a manager from store A
// cannot manipulate schedules of store B via the URL path.
// The handler's validateStoreTenant check rejects mismatched storeId vs tenantID.
func TestE2E_RBAC_TenantIsolation_StoreRoute(t *testing.T) {
	storeA := uuid.New()
	storeB := uuid.New()
	mgrSub := "sub-rbac-storepath"
	mocks := emptyMocks()
	// Manager belongs to store A
	withManagerEmp(mocks, storeA, mgrSub)

	ts := newTestServer(t, mocks)
	// Client's JWT claims tenantID = store A
	client := newClient(ts, storeA, uuid.New(), mgrSub, "manager")

	// But URL contains store B's ID — handler's validateStoreTenant will reject
	resp := client.do(t, http.MethodGet,
		"/stores/"+storeB.String()+"/schedule?from=2026-04-01&to=2026-04-30", nil)
	drain(resp)

	// validateStoreTenant: storeB != storeA and role != "admin" → 403
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// ─── TenantMiddleware role injection ──────────────────────────────────────────

// TestE2E_RBAC_TenantMiddleware_InjectsDBRole verifies that TenantMiddleware
// overwrites the JWT role claim with the employee's DB role.
// A manager employee (emp.Role="manager") sends a JWT with role="employee".
// TenantMiddleware injects "manager" from the DB; the manager-only endpoint succeeds.
func TestE2E_RBAC_TenantMiddleware_InjectsDBRole(t *testing.T) {
	storeID := uuid.New()
	sub := "sub-role-inject"
	mocks := emptyMocks()

	// Employee record has role="manager" in the DB.
	emp := testutil.NewEmployee(storeID)
	emp.AuthID = sub
	emp.Position = "manager"
	mocks.emp.GetByAuthIDFn = func(_ context.Context, authID string) (*model.Employee, error) {
		if authID == sub {
			return emp, nil
		}
		return nil, nil
	}
	mocks.emp.CreateFn = func(_ context.Context, _ *model.Employee) error {
		return nil
	}

	ts := newTestServer(t, mocks)
	// JWT claims role="employee" — Socrate only knows "user"; TenantMiddleware must override.
	client := newClient(ts, storeID, uuid.New(), sub, "employee")

	// POST /employees requires PermManageEmployees (manager only).
	// If TenantMiddleware correctly injects "manager" from DB, this returns 201.
	// If the JWT role "employee" were used instead, RBAC would return 403.
	body := dto.CreateEmployeeRequest{
		Name:      "Test Staff",
		Position:  "employee",
		JobRole:   "pharmacist",
		StartDate: time.Now(),
		AuthID:    "auth-test-staff",
	}
	resp := client.do(t, http.MethodPost, "/stores/"+storeID.String()+"/employees", body)
	drain(resp)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

// ─── Unauthenticated access ───────────────────────────────────────────────────

// TestE2E_RBAC_NoAuth verifies that authenticated routes reject requests
// that have no auth headers (empty sub, no tenant, no role).
func TestE2E_RBAC_NoAuth(t *testing.T) {
	mocks := emptyMocks()
	ts := newTestServer(t, mocks)

	// Raw HTTP request with no X-Test-* headers — TenantMiddleware gets empty sub
	resp, err := http.Get(ts.URL + "/api/v1/me")
	if err != nil {
		t.Fatalf("get /me: %v", err)
	}
	drain(resp)

	// TenantMiddleware returns 401 when sub is empty
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
