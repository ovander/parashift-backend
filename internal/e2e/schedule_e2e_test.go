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

// TestE2E_Schedule_GetSchedule verifies that a manager can retrieve the store
// schedule for a date range via GET /stores/{storeId}/schedule?from=...&to=...
func TestE2E_Schedule_GetSchedule(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-sched"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	shift := testutil.NewShiftInstance(storeID)
	mocks.shift.ListByDateRangeFn = func(_ context.Context, tenantID uuid.UUID, from, to time.Time, page, pageSize int) ([]*model.ShiftInstance, int64, error) {
		return []*model.ShiftInstance{shift}, 1, nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := client.do(t, http.MethodGet, "/stores/"+storeID.String()+"/schedule?from=2026-04-01&to=2026-04-30", nil)
	drain(resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestE2E_Schedule_GetSchedule_MissingParams verifies that missing date query params
// returns 400.
func TestE2E_Schedule_GetSchedule_MissingParams(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-noparam"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := client.do(t, http.MethodGet, "/stores/"+storeID.String()+"/schedule", nil)
	drain(resp)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestE2E_Schedule_CreateShift verifies that a manager can create a new shift.
// POST /stores/{storeId}/shifts → 201.
func TestE2E_Schedule_CreateShift(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-create-shift"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	mocks.shift.CreateFn = func(_ context.Context, s *model.ShiftInstance) error {
		return nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	body := dto.CreateShiftInstanceRequest{
		Date:      time.Now(),
		StartTime: "09:00",
		EndTime:   "17:00",
	}

	resp := client.do(t, http.MethodPost, "/stores/"+storeID.String()+"/shifts", body)
	drain(resp)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

// TestE2E_Schedule_GetShift verifies that a manager can retrieve a shift by ID.
func TestE2E_Schedule_GetShift(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-get-shift"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	shift := testutil.NewShiftInstance(storeID)
	mocks.shift.GetByIDFn = func(_ context.Context, tenantID, id uuid.UUID) (*model.ShiftInstance, error) {
		if id == shift.ID {
			return shift, nil
		}
		return nil, nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := client.do(t, http.MethodGet, "/stores/"+storeID.String()+"/shifts/"+shift.ID.String(), nil)
	defer drain(resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result dto.ShiftInstanceResponse
	decode(t, resp, &result)
	assert.Equal(t, shift.ID, result.ID)
}

// TestE2E_Schedule_DeleteShift verifies that a manager can delete a shift.
func TestE2E_Schedule_DeleteShift(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-del-shift"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	shift := testutil.NewShiftInstance(storeID)
	mocks.shift.GetByIDFn = func(_ context.Context, _, id uuid.UUID) (*model.ShiftInstance, error) {
		if id == shift.ID {
			return shift, nil
		}
		return nil, nil
	}
	mocks.shift.DeleteFn = func(_ context.Context, _, _ uuid.UUID) error {
		return nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := client.do(t, http.MethodDelete, "/stores/"+storeID.String()+"/shifts/"+shift.ID.String(), nil)
	drain(resp)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// TestE2E_Schedule_CreateAssignment verifies that a manager can assign an employee
// to a shift. POST /stores/{storeId}/assignments → 201.
func TestE2E_Schedule_CreateAssignment(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-assign"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	shift := testutil.NewShiftInstance(storeID)
	emp := testutil.NewEmployee(storeID)

	// CreateAssignment calls empRepo.GetByID to validate employee exists
	mocks.emp.GetByIDFn = func(_ context.Context, _, id uuid.UUID) (*model.Employee, error) {
		if id == emp.ID {
			return emp, nil
		}
		return nil, nil
	}
	mocks.shift.GetByIDFn = func(_ context.Context, _, id uuid.UUID) (*model.ShiftInstance, error) {
		if id == shift.ID {
			return shift, nil
		}
		return nil, nil
	}
	mocks.assign.ExistsConflictFn = func(_ context.Context, _, _ uuid.UUID, _ time.Time, _, _ string, _ *uuid.UUID) (bool, error) {
		return false, nil
	}
	mocks.assign.CreateFn = func(_ context.Context, a *model.ShiftAssignment) error {
		return nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	body := dto.CreateAssignmentRequest{
		ShiftID: shift.ID,
		EmployeeID:      emp.ID,
	}

	resp := client.do(t, http.MethodPost, "/stores/"+storeID.String()+"/assignments", body)
	drain(resp)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

// TestE2E_Schedule_GetAssignments verifies that a manager can list all assignments
// for a given shift.
func TestE2E_Schedule_GetAssignments(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-listassign"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	shift := testutil.NewShiftInstance(storeID)
	assign := testutil.NewShiftAssignment(storeID, shift.ID, uuid.New())

	mocks.assign.ListByShiftFn = func(_ context.Context, _, shiftID uuid.UUID) ([]*model.ShiftAssignment, error) {
		if shiftID == shift.ID {
			return []*model.ShiftAssignment{assign}, nil
		}
		return nil, nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := client.do(t, http.MethodGet, "/stores/"+storeID.String()+"/shifts/"+shift.ID.String()+"/assignments", nil)
	drain(resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// ─── ListShifts (GET /shifts?week_of=) ───────────────────────────────────────

// TestE2E_Schedule_ListShifts verifies that a manager can fetch all shifts for a
// week via GET /stores/{storeId}/shifts?week_of=YYYY-MM-DD.
func TestE2E_Schedule_ListShifts(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-listshifts"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	shift := testutil.NewShiftInstance(storeID)
	mocks.shift.ListByDateRangeFn = func(_ context.Context, _ uuid.UUID, _, _ time.Time, _, _ int) ([]*model.ShiftInstance, int64, error) {
		return []*model.ShiftInstance{shift}, 1, nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	// week_of=2026-03-30 is a Monday; handler snaps to that week's Monday→Sunday.
	resp := client.do(t, http.MethodGet, "/stores/"+storeID.String()+"/shifts?week_of=2026-03-30", nil)
	defer drain(resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result []dto.ShiftInstanceResponse
	decode(t, resp, &result)
	assert.Len(t, result, 1)
	assert.Equal(t, shift.ID, result[0].ID)
}

// TestE2E_Schedule_ListShifts_MissingWeekOf verifies that omitting week_of returns 400.
func TestE2E_Schedule_ListShifts_MissingWeekOf(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-listshifts-noparam"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := client.do(t, http.MethodGet, "/stores/"+storeID.String()+"/shifts", nil)
	drain(resp)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestE2E_Schedule_ListShifts_Employee verifies that employees (role="pharmacist"
// in DB) can view shifts because pharmacist maps to PermViewSchedule in the RBAC map.
func TestE2E_Schedule_ListShifts_Employee(t *testing.T) {
	storeID := uuid.New()
	empID := uuid.New()
	sub := "sub-emp-listshifts"
	mocks := emptyMocks()

	emp := testutil.NewEmployee(storeID) // Role defaults to "pharmacist"
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

	resp := client.do(t, http.MethodGet, "/stores/"+storeID.String()+"/shifts?week_of=2026-03-30", nil)
	drain(resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// ─── ListAssignments (GET /assignments?week_of=) ──────────────────────────────

// TestE2E_Schedule_ListAssignments verifies that a manager can fetch all
// assignments for a week via GET /stores/{storeId}/assignments?week_of=YYYY-MM-DD.
func TestE2E_Schedule_ListAssignments(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-listassignments"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	shift := testutil.NewShiftInstance(storeID)
	empID := uuid.New()
	assign := testutil.NewShiftAssignment(storeID, shift.ID, empID)
	mocks.assign.ListByDateRangeFn = func(_ context.Context, _ uuid.UUID, _, _ time.Time) ([]*model.ShiftAssignment, error) {
		return []*model.ShiftAssignment{assign}, nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := client.do(t, http.MethodGet, "/stores/"+storeID.String()+"/assignments?week_of=2026-03-30", nil)
	defer drain(resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result []dto.AssignmentResponse
	decode(t, resp, &result)
	assert.Len(t, result, 1)
	assert.Equal(t, assign.ID, result[0].ID)
	assert.Equal(t, empID, result[0].EmployeeID)
}

// TestE2E_Schedule_ListAssignments_MissingWeekOf verifies that omitting week_of returns 400.
func TestE2E_Schedule_ListAssignments_MissingWeekOf(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-listassign-noparam"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := client.do(t, http.MethodGet, "/stores/"+storeID.String()+"/assignments", nil)
	drain(resp)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ─── ResetWeekAssignments (DELETE /assignments?week_of=) ─────────────────────

// TestE2E_Schedule_ResetWeek verifies that a manager can delete all assignments
// for a week via DELETE /stores/{storeId}/assignments?week_of=YYYY-MM-DD.
// The handler must return 200 with a JSON body containing the deleted count.
func TestE2E_Schedule_ResetWeek(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-resetweek"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	var deletedFrom, deletedTo time.Time
	mocks.assign.DeleteByDateRangeFn = func(_ context.Context, tenantID uuid.UUID, from, to time.Time) (int64, error) {
		assert.Equal(t, storeID, tenantID)
		deletedFrom = from
		deletedTo = to
		return 3, nil // pretend 3 rows were wiped
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := client.do(t, http.MethodDelete, "/stores/"+storeID.String()+"/assignments?week_of=2026-03-30", nil)
	defer drain(resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	decode(t, resp, &body)
	assert.Equal(t, float64(3), body["deleted"])
	assert.Equal(t, "2026-03-30", body["week_start"])

	// Verify the service derived Monday→Sunday date window from week_of=2026-03-30
	// (which is a Monday). from should be 2026-03-30 00:00 UTC and to 2026-04-05 00:00 UTC.
	assert.Equal(t, "2026-03-30", deletedFrom.UTC().Format("2006-01-02"))
	assert.Equal(t, "2026-04-05", deletedTo.UTC().Format("2006-01-02"))
}

// TestE2E_Schedule_ResetWeek_MissingWeekOf verifies that omitting the week_of
// query parameter returns 400.
func TestE2E_Schedule_ResetWeek_MissingWeekOf(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-resetweek-noparam"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := client.do(t, http.MethodDelete, "/stores/"+storeID.String()+"/assignments", nil)
	drain(resp)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestE2E_Schedule_ResetWeek_InvalidWeekOf verifies that a malformed week_of
// value returns 400 without calling the repo.
func TestE2E_Schedule_ResetWeek_InvalidWeekOf(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-resetweek-badparam"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	called := false
	mocks.assign.DeleteByDateRangeFn = func(_ context.Context, _ uuid.UUID, _, _ time.Time) (int64, error) {
		called = true
		return 0, nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := client.do(t, http.MethodDelete, "/stores/"+storeID.String()+"/assignments?week_of=not-a-date", nil)
	drain(resp)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.False(t, called, "repo must not be called on a bad date")
}

// TestE2E_Schedule_ResetWeek_EmployeeForbidden verifies that a plain employee
// cannot reset the week (requires PermManageSchedule).
func TestE2E_Schedule_ResetWeek_EmployeeForbidden(t *testing.T) {
	storeID := uuid.New()
	empID := uuid.New()
	sub := "sub-emp-resetweek"
	mocks := emptyMocks()

	emp := testutil.NewEmployee(storeID) // role defaults to "pharmacist" → employee RBAC
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

	resp := client.do(t, http.MethodDelete, "/stores/"+storeID.String()+"/assignments?week_of=2026-03-30", nil)
	drain(resp)

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}
