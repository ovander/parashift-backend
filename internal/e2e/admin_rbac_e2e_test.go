package e2e_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/testutil"
	"github.com/stretchr/testify/assert"
)

// SEC-1 regression suite: the cross-tenant /admin surface must be reachable
// ONLY by the platform "admin" role. A store "manager" (who holds
// PermManageStore for its own tenant) must be rejected with 403, closing the
// tenant-isolation escalation where managers could read/modify other tenants'
// employees, stores, and audit logs.

// managerClient builds a tenant-resolved manager identity backed by a mock employee.
func managerClientFor(t *testing.T, role string) (*httptest.Server, *testClient, uuid.UUID) {
	t.Helper()
	storeID := uuid.New()
	empID := uuid.New()
	sub := "sub-admin-rbac-" + role

	mocks := emptyMocks()
	emp := testutil.NewEmployee(storeID)
	emp.ID = empID
	emp.AuthID = sub
	emp.Position = role // "manager" | "employee"
	mocks.emp.GetByAuthIDFn = func(_ context.Context, authID string) (*model.Employee, error) {
		if authID == sub {
			return emp, nil
		}
		return nil, nil
	}

	ts := newTestServer(t, mocks)
	return ts, newClient(ts, storeID, empID, sub, role), storeID
}

func TestE2E_AdminRBAC_ManagerCannotAccessAdmin(t *testing.T) {
	_, client, _ := managerClientFor(t, "manager")
	resp := client.do(t, http.MethodGet, "/admin/metadata", nil)
	drain(resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode,
		"a store manager must NOT reach the cross-tenant /admin surface")
}

func TestE2E_AdminRBAC_EmployeeCannotAccessAdmin(t *testing.T) {
	_, client, _ := managerClientFor(t, "employee")
	resp := client.do(t, http.MethodGet, "/admin/metadata", nil)
	drain(resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode,
		"an employee must NOT reach the /admin surface")
}

func TestE2E_AdminRBAC_PlatformAdminCanAccessAdmin(t *testing.T) {
	// Platform admins bypass TenantMiddleware (no employee record needed).
	mocks := emptyMocks()
	ts := newTestServer(t, mocks)
	client := newClient(ts, uuid.Nil, uuid.Nil, "sub-platform-admin", "admin")

	resp := client.do(t, http.MethodGet, "/admin/metadata", nil)
	drain(resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"the platform admin role must reach the /admin surface")
}

// TestE2E_AdminRBAC_ManagerCannotCrossTenantEmployeeCRUD proves the concrete
// exploit is closed: a manager cannot read another tenant's employee via the
// global admin endpoint.
func TestE2E_AdminRBAC_ManagerCannotCrossTenantEmployeeCRUD(t *testing.T) {
	_, client, _ := managerClientFor(t, "manager")
	otherTenantEmployee := uuid.New().String()
	resp := client.do(t, http.MethodGet, "/admin/employees/"+otherTenantEmployee, nil)
	drain(resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode,
		"manager must be blocked before reaching GetByIDGlobal")
}
