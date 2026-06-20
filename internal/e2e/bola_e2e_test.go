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

// SEC-5 / BOLA suite: per-employee resources must enforce object-level
// authorization. A plain employee may act only on their own records; a manager
// may act across the tenant. These tests prove the deny path (403) for the
// "victim" employee and the allow path (not 403) for self/manager.

// bolaActor wires a tenant-resolved caller (employee or manager) backed by a
// mock employee, and returns the client plus the store and the caller's own id.
func bolaActor(t *testing.T, role string) (*testClient, uuid.UUID, uuid.UUID) {
	t.Helper()
	storeID := uuid.New()
	callerID := uuid.New()
	sub := "sub-bola-" + role + "-" + callerID.String()

	mocks := emptyMocks()
	caller := testutil.NewEmployee(storeID)
	caller.ID = callerID
	caller.AuthID = sub
	caller.Position = role
	mocks.emp.GetByAuthIDFn = func(_ context.Context, authID string) (*model.Employee, error) {
		if authID == sub {
			return caller, nil
		}
		return nil, nil
	}
	// Leave/swap lookups return records owned by a DIFFERENT employee (the victim),
	// so ownership checks must reject a plain employee.
	victim := uuid.New()
	mocks.leave.GetByIDFn = func(_ context.Context, _ uuid.UUID, id uuid.UUID) (*model.LeaveRequest, error) {
		lr := &model.LeaveRequest{EmployeeID: victim}
		lr.ID = id
		return lr, nil
	}
	mocks.swap.GetByIDFn = func(_ context.Context, _ uuid.UUID, id uuid.UUID) (*model.SwapRequest, error) {
		sr := &model.SwapRequest{RequesterID: victim} // caller is neither requester nor target
		sr.ID = id
		return sr, nil
	}

	ts := newTestServer(t, mocks)
	return newClient(ts, storeID, callerID, sub, role), storeID, callerID
}

func TestE2E_BOLA_EmployeeCannotTouchAnotherAvailability(t *testing.T) {
	client, storeID, _ := bolaActor(t, "employee")
	victim := uuid.New().String()
	base := "/stores/" + storeID.String() + "/employees/" + victim

	t.Run("set", func(t *testing.T) {
		resp := client.do(t, http.MethodPost, base+"/availability",
			dto.SetAvailabilityRequest{Date: time.Now()})
		drain(resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})
	t.Run("get", func(t *testing.T) {
		resp := client.do(t, http.MethodGet, base+"/availability?date=2026-06-20", nil)
		drain(resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})
	t.Run("range", func(t *testing.T) {
		resp := client.do(t, http.MethodGet, base+"/availability/range?from=2026-06-01&to=2026-06-30", nil)
		drain(resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})
}

func TestE2E_BOLA_EmployeeCanTouchOwnAvailability(t *testing.T) {
	client, storeID, callerID := bolaActor(t, "employee")
	// Acting on self must pass the ownership gate (not 403).
	resp := client.do(t, http.MethodGet,
		"/stores/"+storeID.String()+"/employees/"+callerID.String()+"/availability?date=2026-06-20", nil)
	drain(resp)
	assert.NotEqual(t, http.StatusForbidden, resp.StatusCode,
		"an employee must be allowed to read their own availability")
}

func TestE2E_BOLA_ManagerCanTouchAnotherAvailability(t *testing.T) {
	client, storeID, _ := bolaActor(t, "manager")
	victim := uuid.New().String()
	resp := client.do(t, http.MethodGet,
		"/stores/"+storeID.String()+"/employees/"+victim+"/availability?date=2026-06-20", nil)
	drain(resp)
	assert.NotEqual(t, http.StatusForbidden, resp.StatusCode,
		"a manager may read any employee's availability in the tenant")
}

func TestE2E_BOLA_EmployeeCannotForgeLeaveForAnother(t *testing.T) {
	client, storeID, _ := bolaActor(t, "employee")
	other := uuid.New()
	body := struct {
		EmployeeID *uuid.UUID `json:"employee_id"`
		dto.CreateLeaveRequest
	}{EmployeeID: &other}
	resp := client.do(t, http.MethodPost, "/stores/"+storeID.String()+"/leave-requests", body)
	drain(resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode,
		"an employee must not file leave on behalf of another employee")
}

func TestE2E_BOLA_EmployeeCannotReadOrDeleteAnotherLeave(t *testing.T) {
	client, storeID, _ := bolaActor(t, "employee")
	leaveID := uuid.New().String()
	base := "/stores/" + storeID.String() + "/leave-requests/" + leaveID

	getResp := client.do(t, http.MethodGet, base, nil)
	drain(getResp)
	assert.Equal(t, http.StatusForbidden, getResp.StatusCode, "cannot read a colleague's leave")

	delResp := client.do(t, http.MethodDelete, base, nil)
	drain(delResp)
	assert.Equal(t, http.StatusForbidden, delResp.StatusCode, "cannot cancel a colleague's leave")
}

func TestE2E_BOLA_EmployeeCannotReadAnotherSwap(t *testing.T) {
	client, storeID, _ := bolaActor(t, "employee")
	resp := client.do(t, http.MethodGet,
		"/stores/"+storeID.String()+"/swap-requests/"+uuid.New().String(), nil)
	drain(resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode,
		"an employee who is neither requester nor target cannot read a swap")
}

func TestE2E_BOLA_EmployeeCannotReadAnotherTemplatesOrQualifications(t *testing.T) {
	client, storeID, _ := bolaActor(t, "employee")
	victim := uuid.New().String()
	base := "/stores/" + storeID.String() + "/employees/" + victim

	tmplResp := client.do(t, http.MethodGet, base+"/week-templates", nil)
	drain(tmplResp)
	assert.Equal(t, http.StatusForbidden, tmplResp.StatusCode, "cannot read a colleague's week templates")

	qualResp := client.do(t, http.MethodGet, base+"/qualifications", nil)
	drain(qualResp)
	assert.Equal(t, http.StatusForbidden, qualResp.StatusCode, "cannot read a colleague's qualifications")
}
