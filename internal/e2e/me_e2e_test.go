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

// TestE2E_Me_GetProfile verifies that GET /me returns the authenticated user's profile.
// The handler resolves the employee by the auth sub claim (X-Test-Sub).
func TestE2E_Me_GetProfile(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	sub := "sub-alice-me"

	mocks := emptyMocks()

	emp := testutil.NewEmployee(storeID)
	emp.AuthID = sub
	// TenantMiddleware uses GetByAuthID; MeHandler also uses GetByAuthID
	mocks.emp.GetByAuthIDFn = func(_ context.Context, authID string) (*model.Employee, error) {
		if authID == sub {
			return emp, nil
		}
		return nil, nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, userID, sub, "employee")

	resp := client.do(t, http.MethodGet, "/me", nil)
	defer drain(resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result dto.EmployeeResponse
	decode(t, resp, &result)
	assert.Equal(t, emp.ID, result.ID)
}

// TestE2E_Me_GetProfile_NotFound verifies that GET /me returns 404 when the sub
// does not match any employee in the database.
// TenantMiddleware handles the not-found case (returns 404) before the handler is reached.
func TestE2E_Me_GetProfile_NotFound(t *testing.T) {
	storeID := uuid.New()
	sub := "sub-unknown"

	mocks := emptyMocks()
	// GetByAuthIDFn returns nil, nil → TenantMiddleware sees emp==nil and returns 404.
	mocks.emp.GetByAuthIDFn = func(_ context.Context, _ string) (*model.Employee, error) {
		return nil, nil
	}

	ts := newTestServer(t, mocks)
	// Use employee role so TenantMiddleware runs the DB lookup and catches the nil emp.
	client := newClient(ts, storeID, uuid.New(), sub, "employee")

	resp := client.do(t, http.MethodGet, "/me", nil)
	drain(resp)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestE2E_Me_GetSchedule verifies that an authenticated employee can retrieve their
// personal schedule for a date range.
func TestE2E_Me_GetSchedule(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New()
	sub := "sub-alice-schedule"

	mocks := emptyMocks()

	emp := testutil.NewEmployee(storeID)
	emp.ID = userID
	emp.AuthID = sub
	mocks.emp.GetByAuthIDFn = func(_ context.Context, authID string) (*model.Employee, error) {
		if authID == sub {
			return emp, nil
		}
		return nil, nil
	}

	shift := testutil.NewShiftInstance(storeID)
	assign := testutil.NewShiftAssignment(storeID, shift.ID, emp.ID)

	// ListMyShifts calls assignRepo.ListByEmployee then shiftRepo.GetByID for each
	mocks.assign.ListByEmployeeFn = func(_ context.Context, _, _ uuid.UUID, _, _ time.Time) ([]*model.ShiftAssignment, error) {
		return []*model.ShiftAssignment{assign}, nil
	}
	mocks.shift.GetByIDFn = func(_ context.Context, _, id uuid.UUID) (*model.ShiftInstance, error) {
		if id == shift.ID {
			return shift, nil
		}
		return nil, nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, userID, sub, "employee")

	resp := client.do(t, http.MethodGet, "/me/schedule?from=2026-04-01&to=2026-04-30", nil)
	drain(resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestE2E_Me_GetSchedule_MissingParams verifies that GET /me/schedule without
// query parameters returns 400.
func TestE2E_Me_GetSchedule_MissingParams(t *testing.T) {
	storeID := uuid.New()
	sub := "sub-alice-mparams"

	mocks := emptyMocks()
	emp := testutil.NewEmployee(storeID)
	emp.AuthID = sub
	mocks.emp.GetByAuthIDFn = func(_ context.Context, authID string) (*model.Employee, error) {
		if authID == sub {
			return emp, nil
		}
		return nil, nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), sub, "employee")

	resp := client.do(t, http.MethodGet, "/me/schedule", nil)
	drain(resp)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
