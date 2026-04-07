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
	"github.com/stretchr/testify/require"
)

// TestE2E_ShiftSlot_List verifies that a manager can list all shift slots for a store.
// GET /stores/{storeId}/templates (requires PermViewSchedule → manager has it).
func TestE2E_ShiftSlot_List(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-slot-list"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	slotA := testutil.NewShiftSlot(storeID, "A", 1) // Monday, Scheme A
	slotB := testutil.NewShiftSlot(storeID, "B", 3) // Wednesday, Scheme B

	mocks.slot.ListFn = func(_ context.Context, tenantID uuid.UUID) ([]*model.ShiftSlot, error) {
		if tenantID == storeID {
			return []*model.ShiftSlot{slotA, slotB}, nil
		}
		return nil, nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := client.do(t, http.MethodGet, "/stores/"+storeID.String()+"/templates", nil)
	defer drain(resp)

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result []dto.ShiftSlotResponse
	decode(t, resp, &result)
	assert.Len(t, result, 2)
	assert.Equal(t, slotA.ID, result[0].ID)
	assert.Equal(t, "A", result[0].Scheme)
	assert.Equal(t, 1, result[0].DayOfWeek)
}

// TestE2E_ShiftSlot_List_Employee verifies that a regular employee can also read the
// template list (PermViewSchedule is granted to employees too).
func TestE2E_ShiftSlot_List_Employee(t *testing.T) {
	storeID := uuid.New()
	sub := "sub-emp-slot-list"
	mocks := emptyMocks()

	emp := testutil.NewEmployee(storeID)
	emp.AuthID = sub
	mocks.emp.GetByAuthIDFn = func(_ context.Context, authID string) (*model.Employee, error) {
		if authID == sub {
			return emp, nil
		}
		return nil, nil
	}

	mocks.slot.ListFn = func(_ context.Context, _ uuid.UUID) ([]*model.ShiftSlot, error) {
		return []*model.ShiftSlot{testutil.NewShiftSlot(storeID, "A", 2)}, nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, emp.ID, sub, "employee")

	resp := client.do(t, http.MethodGet, "/stores/"+storeID.String()+"/templates", nil)
	drain(resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestE2E_ShiftSlot_Create verifies that a manager can create a new shift slot.
// POST /stores/{storeId}/templates (requires PermManageSchedule).
func TestE2E_ShiftSlot_Create(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-slot-create"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	mocks.slot.CreateFn = func(_ context.Context, slot *model.ShiftSlot) error {
		return nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	body := dto.CreateShiftSlotRequest{
		Scheme:       "A",
		DayOfWeek:    1, // Monday
		StartTime:    "08:00",
		EndTime:      "16:00",
		RequiredRole: "pharmacist",
	}

	resp := client.do(t, http.MethodPost, "/stores/"+storeID.String()+"/templates", body)
	defer drain(resp)

	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var result dto.ShiftSlotResponse
	decode(t, resp, &result)
	assert.Equal(t, "A", result.Scheme)
	assert.Equal(t, 1, result.DayOfWeek)
	assert.Equal(t, "08:00", result.StartTime)
	assert.Equal(t, "pharmacist", result.RequiredRole)
	assert.Equal(t, storeID, result.StoreID)
}

// TestE2E_ShiftSlot_Create_InvalidScheme verifies that an invalid scheme is rejected with 400.
func TestE2E_ShiftSlot_Create_InvalidScheme(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-slot-bad-scheme"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	body := dto.CreateShiftSlotRequest{
		Scheme:    "C", // invalid
		DayOfWeek: 1,
		StartTime: "08:00",
		EndTime:   "16:00",
	}

	resp := client.do(t, http.MethodPost, "/stores/"+storeID.String()+"/templates", body)
	drain(resp)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestE2E_ShiftSlot_Create_InvalidDayOfWeek verifies that day_of_week=0 (out of 1-7 range) is rejected.
func TestE2E_ShiftSlot_Create_InvalidDayOfWeek(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-slot-bad-dow"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	body := dto.CreateShiftSlotRequest{
		Scheme:    "A",
		DayOfWeek: 0, // invalid — frontend convention is 1–7
		StartTime: "08:00",
		EndTime:   "16:00",
	}

	resp := client.do(t, http.MethodPost, "/stores/"+storeID.String()+"/templates", body)
	drain(resp)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestE2E_ShiftSlot_Create_EmployeeForbidden verifies that a plain employee cannot create slots
// (requires PermManageSchedule, which employees lack).
func TestE2E_ShiftSlot_Create_EmployeeForbidden(t *testing.T) {
	storeID := uuid.New()
	sub := "sub-emp-slot-forbidden"
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
	client := newClient(ts, storeID, emp.ID, sub, "employee")

	body := dto.CreateShiftSlotRequest{
		Scheme: "A", DayOfWeek: 1, StartTime: "08:00", EndTime: "16:00",
	}
	resp := client.do(t, http.MethodPost, "/stores/"+storeID.String()+"/templates", body)
	drain(resp)

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// TestE2E_ShiftSlot_Delete verifies that a manager can delete an existing slot.
// DELETE /stores/{storeId}/templates/{templateId} → 204.
func TestE2E_ShiftSlot_Delete(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-slot-delete"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	slot := testutil.NewShiftSlot(storeID, "A", 1)

	mocks.slot.GetByIDFn = func(_ context.Context, _, id uuid.UUID) (*model.ShiftSlot, error) {
		if id == slot.ID {
			return slot, nil
		}
		return nil, nil
	}
	mocks.slot.DeleteFn = func(_ context.Context, _, _ uuid.UUID) error {
		return nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := client.do(t, http.MethodDelete, "/stores/"+storeID.String()+"/templates/"+slot.ID.String(), nil)
	drain(resp)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// TestE2E_ShiftSlot_Delete_NotFound verifies that deleting a nonexistent slot returns 404.
func TestE2E_ShiftSlot_Delete_NotFound(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-slot-delete-404"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)
	// GetByIDFn nil → nil,nil → service returns NotFound

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := client.do(t, http.MethodDelete, "/stores/"+storeID.String()+"/templates/"+uuid.New().String(), nil)
	drain(resp)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestE2E_ShiftSlot_Publish verifies that a manager can publish a scheme,
// generating shift instances for the next 4 weeks.
// POST /stores/{storeId}/templates/{templateId}/publish → 200.
func TestE2E_ShiftSlot_Publish(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-slot-publish"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)

	// Slot on Monday (day 1), Scheme A
	slot := testutil.NewShiftSlot(storeID, "A", 1)

	mocks.slot.GetByIDFn = func(_ context.Context, _, id uuid.UUID) (*model.ShiftSlot, error) {
		if id == slot.ID {
			return slot, nil
		}
		return nil, nil
	}
	mocks.slot.ListBySchemeFn = func(_ context.Context, _ uuid.UUID, scheme string) ([]*model.ShiftSlot, error) {
		if scheme == "A" {
			return []*model.ShiftSlot{slot}, nil
		}
		return nil, nil
	}
	mocks.store.GetByIDFn = func(_ context.Context, _ uuid.UUID) (*model.Store, error) {
		return testutil.NewStore(), nil // ABWeekAnchor is nil → uses computed anchor
	}

	var createdCount int
	mocks.shift.CreateFn = func(_ context.Context, s *model.ShiftInstance) error {
		createdCount++
		return nil
	}

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := client.do(t, http.MethodPost, "/stores/"+storeID.String()+"/templates/"+slot.ID.String()+"/publish", nil)
	defer drain(resp)

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]int
	decode(t, resp, &result)
	// Over 4 weeks, ~2 Mondays will be Scheme A weeks → at least 1 shift created
	assert.GreaterOrEqual(t, result["shifts_created"], 1)
	assert.Equal(t, result["shifts_created"], createdCount)
}

// TestE2E_ShiftSlot_Publish_NotFound verifies that publishing a nonexistent slot returns 404.
func TestE2E_ShiftSlot_Publish_NotFound(t *testing.T) {
	storeID := uuid.New()
	mgrSub := "sub-mgr-slot-publish-404"
	mocks := emptyMocks()
	withManagerEmp(mocks, storeID, mgrSub)
	// slot.GetByIDFn nil → nil,nil → service returns NotFound

	ts := newTestServer(t, mocks)
	client := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := client.do(t, http.MethodPost, "/stores/"+storeID.String()+"/templates/"+uuid.New().String()+"/publish", nil)
	drain(resp)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestE2E_ShiftSlot_TenantIsolation verifies that a manager from a different tenant
// cannot access another store's templates.
func TestE2E_ShiftSlot_TenantIsolation(t *testing.T) {
	storeA := uuid.New()
	storeB := uuid.New()
	mgrSub := "sub-mgr-slot-isolation"
	mocks := emptyMocks()

	// Manager belongs to storeA.
	emp := testutil.NewEmployee(storeA)
	emp.AuthID = mgrSub
	emp.Position = "manager"
	mocks.emp.GetByAuthIDFn = func(_ context.Context, authID string) (*model.Employee, error) {
		if authID == mgrSub {
			return emp, nil
		}
		return nil, nil
	}

	ts := newTestServer(t, mocks)
	// Client sends requests scoped to storeB while JWT identifies them as storeA manager.
	client := newClient(ts, storeB, uuid.New(), mgrSub, "manager")

	resp := client.do(t, http.MethodGet, "/stores/"+storeB.String()+"/templates", nil)
	drain(resp)

	// TenantMiddleware overrides the request tenant with the employee's TenantID (storeA),
	// which does not match storeB → validateStoreTenant returns false → 403.
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}
