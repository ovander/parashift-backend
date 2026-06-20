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

// SEC-7: a store manager must not be able to mint or promote another manager
// (privilege escalation via mass-assignment of the position field), on EITHER
// the store-scoped or the /manager employee routes. Only a platform admin may.

func managerCallerServer(t *testing.T) (*testClient, uuid.UUID) {
	t.Helper()
	storeID := uuid.New()
	callerID := uuid.New()
	sub := "sub-sec7-mgr-" + callerID.String()

	mocks := emptyMocks()
	mgr := testutil.NewEmployee(storeID)
	mgr.ID = callerID
	mgr.AuthID = sub
	mgr.Position = "manager"
	mocks.emp.GetByAuthIDFn = func(_ context.Context, authID string) (*model.Employee, error) {
		if authID == sub {
			return mgr, nil
		}
		return nil, nil
	}
	// Make Create/Update succeed if they are ever reached (so a non-403 is meaningful).
	mocks.emp.CreateFn = func(_ context.Context, _ *model.Employee) error { return nil }
	mocks.emp.UpdateFn = func(_ context.Context, _ *model.Employee) error { return nil }
	mocks.emp.GetByIDFn = func(_ context.Context, _, id uuid.UUID) (*model.Employee, error) {
		e := testutil.NewEmployee(storeID)
		e.ID = id
		e.Position = "employee"
		return e, nil
	}

	ts := newTestServer(t, mocks)
	return newClient(ts, storeID, callerID, sub, "manager"), storeID
}

func TestE2E_SEC7_ManagerCannotCreateManager_StoreRoute(t *testing.T) {
	client, storeID := managerCallerServer(t)
	body := dto.CreateEmployeeRequest{Name: "X", Position: "manager", JobRole: "pharmacist", StartDate: time.Now()}
	resp := client.do(t, http.MethodPost, "/stores/"+storeID.String()+"/employees", body)
	drain(resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode,
		"manager must not create a manager via the store-scoped route")
}

func TestE2E_SEC7_ManagerCannotPromoteToManager_StoreRoute(t *testing.T) {
	client, storeID := managerCallerServer(t)
	mgrPos := "manager"
	body := dto.UpdateEmployeeRequest{Position: &mgrPos}
	resp := client.do(t, http.MethodPut,
		"/stores/"+storeID.String()+"/employees/"+uuid.New().String(), body)
	drain(resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode,
		"manager must not promote an employee to manager via the store-scoped route")
}

func TestE2E_SEC7_ManagerCannotCreateManager_ManagerRoute(t *testing.T) {
	client, _ := managerCallerServer(t)
	body := dto.CreateEmployeeRequest{Name: "X", Position: "manager", JobRole: "pharmacist", StartDate: time.Now()}
	resp := client.do(t, http.MethodPost, "/manager/employees", body)
	drain(resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode,
		"manager must not create a manager via the /manager route")
}

func TestE2E_SEC7_ManagerCanCreateEmployee(t *testing.T) {
	client, storeID := managerCallerServer(t)
	body := dto.CreateEmployeeRequest{Name: "X", Position: "employee", JobRole: "pharmacist", StartDate: time.Now()}
	resp := client.do(t, http.MethodPost, "/stores/"+storeID.String()+"/employees", body)
	drain(resp)
	assert.NotEqual(t, http.StatusForbidden, resp.StatusCode,
		"manager must still be able to create a regular employee")
}

func TestE2E_SEC7_AdminCanCreateManager(t *testing.T) {
	// Platform admin (bypasses tenant middleware) may create managers.
	mocks := emptyMocks()
	mocks.emp.CreateFn = func(_ context.Context, _ *model.Employee) error { return nil }
	ts := newTestServer(t, mocks)
	client := newClient(ts, uuid.Nil, uuid.Nil, "sub-admin", "admin")

	body := dto.CreateEmployeeRequest{Name: "Boss", Position: "manager", JobRole: "pharmacist", StartDate: time.Now()}
	resp := client.do(t, http.MethodPost, "/admin/employees", body)
	drain(resp)
	assert.NotEqual(t, http.StatusForbidden, resp.StatusCode,
		"a platform admin may create a manager")
}
