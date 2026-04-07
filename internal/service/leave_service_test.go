package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/service"
	"github.com/ovander/parashift/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newLeaveService(
	leaveRepo *testutil.MockLeaveRequestRepo,
	empRepo *testutil.MockEmployeeRepo,
	assignRepo *testutil.MockShiftAssignmentRepo,
	shiftRepo *testutil.MockShiftInstanceRepo,
) *service.LeaveService {
	return service.NewLeaveService(leaveRepo, empRepo, assignRepo, shiftRepo, newTestEmitter(), newTestLogger())
}

// ─── CreateLeaveRequest ───────────────────────────────────────────────────────

func TestLeaveService_CreateLeaveRequest(t *testing.T) {
	tenantID := uuid.New()
	employeeID := uuid.New()
	reason := "Family vacation"
	created := false

	leaveRepo := &testutil.MockLeaveRequestRepo{
		HasActiveLeaveFn: func(_ context.Context, _, _ uuid.UUID, _, _ time.Time) (bool, error) {
			return false, nil
		},
		CreateFn: func(_ context.Context, lr *model.LeaveRequest) error {
			created = true
			assert.Equal(t, tenantID, lr.TenantID)
			assert.Equal(t, employeeID, lr.EmployeeID)
			assert.Equal(t, model.LeaveTypeVacation, lr.Type)
			assert.Equal(t, model.LeaveStatusPending, lr.Status)
			return nil
		},
	}
	empRepo := &testutil.MockEmployeeRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Employee, error) {
			return testutil.NewEmployee(tenantID), nil
		},
	}
	svc := newLeaveService(leaveRepo, empRepo, &testutil.MockShiftAssignmentRepo{}, &testutil.MockShiftInstanceRepo{})

	req := dto.CreateLeaveRequest{
		StartDate: time.Now().Format("2006-01-02"),
		EndDate:   time.Now().AddDate(0, 0, 5).Format("2006-01-02"),
		Type:      model.LeaveTypeVacation,
		Reason:    &reason,
	}
	got, err := svc.CreateLeaveRequest(context.Background(), tenantID, employeeID, req)
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, model.LeaveStatusPending, got.Status)
}

func TestLeaveService_CreateLeaveRequest_EmployeeNotFound(t *testing.T) {
	leaveRepo := &testutil.MockLeaveRequestRepo{}
	empRepo := &testutil.MockEmployeeRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Employee, error) {
			return nil, nil
		},
	}
	svc := newLeaveService(leaveRepo, empRepo, &testutil.MockShiftAssignmentRepo{}, &testutil.MockShiftInstanceRepo{})

	_, err := svc.CreateLeaveRequest(context.Background(), uuid.New(), uuid.New(), dto.CreateLeaveRequest{
		StartDate: time.Now().Format("2006-01-02"), EndDate: time.Now().AddDate(0, 0, 3).Format("2006-01-02"), Type: "vacation",
	})
	require.Error(t, err)
}

func TestLeaveService_CreateLeaveRequest_OverlapConflict(t *testing.T) {
	tenantID := uuid.New()

	leaveRepo := &testutil.MockLeaveRequestRepo{
		HasActiveLeaveFn: func(_ context.Context, _, _ uuid.UUID, _, _ time.Time) (bool, error) {
			return true, nil // overlap exists
		},
	}
	empRepo := &testutil.MockEmployeeRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Employee, error) {
			return testutil.NewEmployee(tenantID), nil
		},
	}
	svc := newLeaveService(leaveRepo, empRepo, &testutil.MockShiftAssignmentRepo{}, &testutil.MockShiftInstanceRepo{})

	_, err := svc.CreateLeaveRequest(context.Background(), tenantID, uuid.New(), dto.CreateLeaveRequest{
		StartDate: time.Now().Format("2006-01-02"), EndDate: time.Now().AddDate(0, 0, 3).Format("2006-01-02"), Type: "vacation",
	})
	require.Error(t, err)
}

// ─── GetLeaveRequest ──────────────────────────────────────────────────────────

func TestLeaveService_GetLeaveRequest_Found(t *testing.T) {
	tenantID := uuid.New()
	lr := testutil.NewLeaveRequest(tenantID, uuid.New())

	leaveRepo := &testutil.MockLeaveRequestRepo{
		GetByIDFn: func(_ context.Context, tID, id uuid.UUID) (*model.LeaveRequest, error) {
			assert.Equal(t, tenantID, tID)
			assert.Equal(t, lr.ID, id)
			return lr, nil
		},
	}
	svc := newLeaveService(leaveRepo, &testutil.MockEmployeeRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockShiftInstanceRepo{})

	got, err := svc.GetLeaveRequest(context.Background(), tenantID, lr.ID)
	require.NoError(t, err)
	assert.Equal(t, lr.ID, got.ID)
}

func TestLeaveService_GetLeaveRequest_NotFound(t *testing.T) {
	leaveRepo := &testutil.MockLeaveRequestRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.LeaveRequest, error) {
			return nil, nil
		},
	}
	svc := newLeaveService(leaveRepo, &testutil.MockEmployeeRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockShiftInstanceRepo{})

	_, err := svc.GetLeaveRequest(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
}

// ─── ReviewLeaveRequest ───────────────────────────────────────────────────────

func TestLeaveService_ReviewLeaveRequest_Approve(t *testing.T) {
	tenantID := uuid.New()
	lr := testutil.NewLeaveRequest(tenantID, uuid.New())
	updated := false

	leaveRepo := &testutil.MockLeaveRequestRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.LeaveRequest, error) {
			return lr, nil
		},
		UpdateFn: func(_ context.Context, l *model.LeaveRequest) error {
			updated = true
			assert.Equal(t, model.LeaveStatusApproved, l.Status)
			return nil
		},
	}
	svc := newLeaveService(leaveRepo, &testutil.MockEmployeeRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockShiftInstanceRepo{})

	got, err := svc.ReviewLeaveRequest(context.Background(), tenantID, lr.ID, dto.ReviewLeaveRequest{Status: model.LeaveStatusApproved})
	require.NoError(t, err)
	assert.True(t, updated)
	assert.Equal(t, model.LeaveStatusApproved, got.Status)
}

// TestLeaveService_ReviewLeaveRequest_CancelsConfirmedAssignments verifies that
// a "confirmed" assignment that overlaps with the approved leave is cancelled and
// the parent shift is flagged NeedsCover=true.
func TestLeaveService_ReviewLeaveRequest_CancelsConfirmedAssignments(t *testing.T) {
	tenantID    := uuid.New()
	employeeID  := uuid.New()
	shiftID     := uuid.New()
	lr          := testutil.NewLeaveRequest(tenantID, employeeID)
	assignment  := testutil.NewShiftAssignment(tenantID, shiftID, employeeID)
	assignment.Status = model.AssignmentStatusConfirmed // default, but explicit for clarity
	shift       := testutil.NewShiftInstance(tenantID)
	shift.ID    = shiftID

	cancelledStatus := ""
	shiftNeedsCover := false

	leaveRepo := &testutil.MockLeaveRequestRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.LeaveRequest, error) { return lr, nil },
		UpdateFn:  func(_ context.Context, _ *model.LeaveRequest) error { return nil },
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByEmployeeFn: func(_ context.Context, _, _ uuid.UUID, _, _ time.Time) ([]*model.ShiftAssignment, error) {
			return []*model.ShiftAssignment{assignment}, nil
		},
		UpdateFn: func(_ context.Context, a *model.ShiftAssignment) error {
			cancelledStatus = a.Status
			return nil
		},
	}
	shiftRepo := &testutil.MockShiftInstanceRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.ShiftInstance, error) { return shift, nil },
		UpdateFn:  func(_ context.Context, s *model.ShiftInstance) error {
			shiftNeedsCover = s.NeedsCover
			return nil
		},
	}

	svc := newLeaveService(leaveRepo, &testutil.MockEmployeeRepo{}, assignRepo, shiftRepo)
	_, err := svc.ReviewLeaveRequest(context.Background(), tenantID, lr.ID, dto.ReviewLeaveRequest{Status: model.LeaveStatusApproved})
	require.NoError(t, err)
	assert.Equal(t, model.AssignmentStatusCancelled, cancelledStatus, "confirmed assignment should be cancelled")
	assert.True(t, shiftNeedsCover, "parent shift should be flagged NeedsCover=true")
}

// TestLeaveService_ReviewLeaveRequest_CancelsPendingAssignments verifies that
// a "pending" assignment is also cancelled on leave approval — previously only
// "confirmed" assignments were cancelled, leaving pending ones as scheduling conflicts.
func TestLeaveService_ReviewLeaveRequest_CancelsPendingAssignments(t *testing.T) {
	tenantID   := uuid.New()
	employeeID := uuid.New()
	shiftID    := uuid.New()
	lr         := testutil.NewLeaveRequest(tenantID, employeeID)
	assignment := testutil.NewShiftAssignment(tenantID, shiftID, employeeID)
	assignment.Status = model.AssignmentStatusPending // <-- the previously unhandled case
	shift      := testutil.NewShiftInstance(tenantID)
	shift.ID   = shiftID

	cancelledStatus := ""

	leaveRepo := &testutil.MockLeaveRequestRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.LeaveRequest, error) { return lr, nil },
		UpdateFn:  func(_ context.Context, _ *model.LeaveRequest) error { return nil },
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByEmployeeFn: func(_ context.Context, _, _ uuid.UUID, _, _ time.Time) ([]*model.ShiftAssignment, error) {
			return []*model.ShiftAssignment{assignment}, nil
		},
		UpdateFn: func(_ context.Context, a *model.ShiftAssignment) error {
			cancelledStatus = a.Status
			return nil
		},
	}
	shiftRepo := &testutil.MockShiftInstanceRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.ShiftInstance, error) { return shift, nil },
		UpdateFn:  func(_ context.Context, _ *model.ShiftInstance) error { return nil },
	}

	svc := newLeaveService(leaveRepo, &testutil.MockEmployeeRepo{}, assignRepo, shiftRepo)
	_, err := svc.ReviewLeaveRequest(context.Background(), tenantID, lr.ID, dto.ReviewLeaveRequest{Status: model.LeaveStatusApproved})
	require.NoError(t, err)
	assert.Equal(t, model.AssignmentStatusCancelled, cancelledStatus, "pending assignment should also be cancelled")
}

// TestLeaveService_ReviewLeaveRequest_SkipsAlreadyCancelledAssignments verifies that
// already-cancelled assignments are not redundantly updated.
func TestLeaveService_ReviewLeaveRequest_SkipsAlreadyCancelledAssignments(t *testing.T) {
	tenantID   := uuid.New()
	employeeID := uuid.New()
	lr         := testutil.NewLeaveRequest(tenantID, employeeID)
	assignment := testutil.NewShiftAssignment(tenantID, uuid.New(), employeeID)
	assignment.Status = model.AssignmentStatusCancelled // already cancelled

	updateCalled := false

	leaveRepo := &testutil.MockLeaveRequestRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.LeaveRequest, error) { return lr, nil },
		UpdateFn:  func(_ context.Context, _ *model.LeaveRequest) error { return nil },
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByEmployeeFn: func(_ context.Context, _, _ uuid.UUID, _, _ time.Time) ([]*model.ShiftAssignment, error) {
			return []*model.ShiftAssignment{assignment}, nil
		},
		UpdateFn: func(_ context.Context, _ *model.ShiftAssignment) error {
			updateCalled = true
			return nil
		},
	}

	svc := newLeaveService(leaveRepo, &testutil.MockEmployeeRepo{}, assignRepo, &testutil.MockShiftInstanceRepo{})
	_, err := svc.ReviewLeaveRequest(context.Background(), tenantID, lr.ID, dto.ReviewLeaveRequest{Status: model.LeaveStatusApproved})
	require.NoError(t, err)
	assert.False(t, updateCalled, "already-cancelled assignment should not trigger an Update call")
}

func TestLeaveService_ReviewLeaveRequest_InvalidStatus(t *testing.T) {
	tenantID := uuid.New()
	lr := testutil.NewLeaveRequest(tenantID, uuid.New())

	leaveRepo := &testutil.MockLeaveRequestRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.LeaveRequest, error) {
			return lr, nil
		},
	}
	svc := newLeaveService(leaveRepo, &testutil.MockEmployeeRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockShiftInstanceRepo{})

	_, err := svc.ReviewLeaveRequest(context.Background(), tenantID, lr.ID, dto.ReviewLeaveRequest{Status: "invalid-status"})
	require.Error(t, err)
}

// ─── ListByEmployee ───────────────────────────────────────────────────────────

func TestLeaveService_ListByEmployee(t *testing.T) {
	tenantID := uuid.New()
	employeeID := uuid.New()
	lrs := []*model.LeaveRequest{
		testutil.NewLeaveRequest(tenantID, employeeID),
		testutil.NewLeaveRequest(tenantID, employeeID),
	}

	leaveRepo := &testutil.MockLeaveRequestRepo{
		ListByEmployeeFn: func(_ context.Context, tID, eID uuid.UUID, page, pageSize int) ([]*model.LeaveRequest, int64, error) {
			assert.Equal(t, tenantID, tID)
			assert.Equal(t, employeeID, eID)
			return lrs, int64(len(lrs)), nil
		},
	}
	svc := newLeaveService(leaveRepo, &testutil.MockEmployeeRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockShiftInstanceRepo{})

	got, total, err := svc.ListByEmployee(context.Background(), tenantID, employeeID, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, got, 2)
}

func TestLeaveService_ListByStore(t *testing.T) {
	tenantID := uuid.New()
	lrs := []*model.LeaveRequest{testutil.NewLeaveRequest(tenantID, uuid.New())}

	leaveRepo := &testutil.MockLeaveRequestRepo{
		ListByStoreFn: func(_ context.Context, tID uuid.UUID, status string, _, _ int) ([]*model.LeaveRequest, int64, error) {
			assert.Equal(t, tenantID, tID)
			assert.Equal(t, "pending", status)
			return lrs, 1, nil
		},
	}
	svc := newLeaveService(leaveRepo, &testutil.MockEmployeeRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockShiftInstanceRepo{})

	got, total, err := svc.ListByStore(context.Background(), tenantID, "pending", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, got, 1)
}

func TestLeaveService_ListByStore_RepoError(t *testing.T) {
	leaveRepo := &testutil.MockLeaveRequestRepo{
		ListByStoreFn: func(_ context.Context, _ uuid.UUID, _ string, _, _ int) ([]*model.LeaveRequest, int64, error) {
			return nil, 0, errors.New("db error")
		},
	}
	svc := newLeaveService(leaveRepo, &testutil.MockEmployeeRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockShiftInstanceRepo{})

	_, _, err := svc.ListByStore(context.Background(), uuid.New(), "", 1, 20)
	require.Error(t, err)
}

// ─── GetLeaveImpact ───────────────────────────────────────────────────────────

// TestLeaveService_GetLeaveImpact_LeaveNotFound verifies a 404 when the leave
// request does not exist.
func TestLeaveService_GetLeaveImpact_LeaveNotFound(t *testing.T) {
	leaveRepo := &testutil.MockLeaveRequestRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.LeaveRequest, error) {
			return nil, nil
		},
	}
	svc := newLeaveService(leaveRepo, &testutil.MockEmployeeRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockShiftInstanceRepo{})

	_, err := svc.GetLeaveImpact(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
}

// TestLeaveService_GetLeaveImpact_NoAssignments verifies that an employee with
// no shifts during the leave period produces a zero-impact response.
func TestLeaveService_GetLeaveImpact_NoAssignments(t *testing.T) {
	tenantID := uuid.New()
	lr := testutil.NewLeaveRequest(tenantID, uuid.New())

	leaveRepo := &testutil.MockLeaveRequestRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.LeaveRequest, error) { return lr, nil },
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByEmployeeFn: func(_ context.Context, _, _ uuid.UUID, _, _ time.Time) ([]*model.ShiftAssignment, error) {
			return nil, nil // no assignments
		},
	}
	svc := newLeaveService(leaveRepo, &testutil.MockEmployeeRepo{}, assignRepo, &testutil.MockShiftInstanceRepo{})

	impact, err := svc.GetLeaveImpact(context.Background(), tenantID, lr.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, impact.TotalCancellations)
	assert.Equal(t, 0, impact.UncoveredShifts)
	assert.Equal(t, float64(0), impact.TotalHoursLost)
	assert.Empty(t, impact.AffectedShifts, "affected_shifts must be an empty slice, not nil")
}

// TestLeaveService_GetLeaveImpact_SoleAssignee verifies that when the employee
// is the only person assigned to a shift, will_need_cover=true and uncovered_shifts=1.
func TestLeaveService_GetLeaveImpact_SoleAssignee(t *testing.T) {
	tenantID    := uuid.New()
	employeeID  := uuid.New()
	shiftID     := uuid.New()
	lr          := testutil.NewLeaveRequest(tenantID, employeeID)
	assignment  := testutil.NewShiftAssignment(tenantID, shiftID, employeeID)
	assignment.ShiftStartTime = "09:00"
	assignment.ShiftEndTime   = "17:00"
	shift       := testutil.NewShiftInstance(tenantID)
	shift.ID    = shiftID

	leaveRepo := &testutil.MockLeaveRequestRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.LeaveRequest, error) { return lr, nil },
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByEmployeeFn: func(_ context.Context, _, _ uuid.UUID, _, _ time.Time) ([]*model.ShiftAssignment, error) {
			return []*model.ShiftAssignment{assignment}, nil
		},
		// No other active assignments on this shift (returns only this employee's).
		ListByShiftFn: func(_ context.Context, _, _ uuid.UUID) ([]*model.ShiftAssignment, error) {
			return []*model.ShiftAssignment{assignment}, nil
		},
	}
	shiftRepo := &testutil.MockShiftInstanceRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.ShiftInstance, error) { return shift, nil },
	}
	svc := newLeaveService(leaveRepo, &testutil.MockEmployeeRepo{}, assignRepo, shiftRepo)

	impact, err := svc.GetLeaveImpact(context.Background(), tenantID, lr.ID)
	require.NoError(t, err)
	require.Len(t, impact.AffectedShifts, 1)
	assert.True(t, impact.AffectedShifts[0].WillNeedCover, "sole assignee: shift must be flagged will_need_cover")
	assert.Equal(t, 0, impact.AffectedShifts[0].OtherAssigned)
	assert.Equal(t, 1, impact.TotalCancellations)
	assert.Equal(t, 1, impact.UncoveredShifts)
	// 09:00–17:00 = 8 hours
	assert.InDelta(t, 8.0, impact.TotalHoursLost, 0.01)
}

// TestLeaveService_GetLeaveImpact_OtherAssignmentsExist verifies that when
// another active employee covers the same shift, will_need_cover=false and
// other_assigned reflects the count correctly.
func TestLeaveService_GetLeaveImpact_OtherAssignmentsExist(t *testing.T) {
	tenantID    := uuid.New()
	employeeID  := uuid.New()
	otherID     := uuid.New()
	shiftID     := uuid.New()
	lr          := testutil.NewLeaveRequest(tenantID, employeeID)
	assignment  := testutil.NewShiftAssignment(tenantID, shiftID, employeeID)
	assignment.ShiftStartTime = "08:00"
	assignment.ShiftEndTime   = "12:00"
	otherAssign := testutil.NewShiftAssignment(tenantID, shiftID, otherID)
	otherAssign.Status = model.AssignmentStatusConfirmed
	shift := testutil.NewShiftInstance(tenantID)
	shift.ID = shiftID

	leaveRepo := &testutil.MockLeaveRequestRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.LeaveRequest, error) { return lr, nil },
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByEmployeeFn: func(_ context.Context, _, _ uuid.UUID, _, _ time.Time) ([]*model.ShiftAssignment, error) {
			return []*model.ShiftAssignment{assignment}, nil
		},
		ListByShiftFn: func(_ context.Context, _, _ uuid.UUID) ([]*model.ShiftAssignment, error) {
			// Both the employee's and another active assignment on the same shift.
			return []*model.ShiftAssignment{assignment, otherAssign}, nil
		},
	}
	shiftRepo := &testutil.MockShiftInstanceRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.ShiftInstance, error) { return shift, nil },
	}
	svc := newLeaveService(leaveRepo, &testutil.MockEmployeeRepo{}, assignRepo, shiftRepo)

	impact, err := svc.GetLeaveImpact(context.Background(), tenantID, lr.ID)
	require.NoError(t, err)
	require.Len(t, impact.AffectedShifts, 1)
	assert.False(t, impact.AffectedShifts[0].WillNeedCover, "shift has another assignee: will_need_cover must be false")
	assert.Equal(t, 1, impact.AffectedShifts[0].OtherAssigned)
	assert.Equal(t, 0, impact.UncoveredShifts)
	// 08:00–12:00 = 4 hours
	assert.InDelta(t, 4.0, impact.TotalHoursLost, 0.01)
}

// TestLeaveService_GetLeaveImpact_SkipsCancelledAssignmentsInPeriod verifies that
// assignments that are already cancelled are excluded from the impact calculation —
// they won't be cancelled again and don't count as affected shifts.
func TestLeaveService_GetLeaveImpact_SkipsCancelledAssignmentsInPeriod(t *testing.T) {
	tenantID   := uuid.New()
	employeeID := uuid.New()
	lr         := testutil.NewLeaveRequest(tenantID, employeeID)
	cancelled  := testutil.NewShiftAssignment(tenantID, uuid.New(), employeeID)
	cancelled.Status = model.AssignmentStatusCancelled

	leaveRepo := &testutil.MockLeaveRequestRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.LeaveRequest, error) { return lr, nil },
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByEmployeeFn: func(_ context.Context, _, _ uuid.UUID, _, _ time.Time) ([]*model.ShiftAssignment, error) {
			return []*model.ShiftAssignment{cancelled}, nil
		},
	}
	svc := newLeaveService(leaveRepo, &testutil.MockEmployeeRepo{}, assignRepo, &testutil.MockShiftInstanceRepo{})

	impact, err := svc.GetLeaveImpact(context.Background(), tenantID, lr.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, impact.TotalCancellations, "cancelled assignment must not be counted as an affected shift")
	assert.Empty(t, impact.AffectedShifts)
}

// TestLeaveService_GetLeaveImpact_HoursCalculation checks that partial-hour
// shifts (e.g. 09:00–13:30 = 4.5 h) are computed correctly.
func TestLeaveService_GetLeaveImpact_HoursCalculation(t *testing.T) {
	tenantID   := uuid.New()
	employeeID := uuid.New()
	shiftID    := uuid.New()
	lr         := testutil.NewLeaveRequest(tenantID, employeeID)
	a          := testutil.NewShiftAssignment(tenantID, shiftID, employeeID)
	a.ShiftStartTime = "09:00"
	a.ShiftEndTime   = "13:30" // 4.5 hours
	shift      := testutil.NewShiftInstance(tenantID)
	shift.ID   = shiftID

	leaveRepo := &testutil.MockLeaveRequestRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.LeaveRequest, error) { return lr, nil },
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByEmployeeFn: func(_ context.Context, _, _ uuid.UUID, _, _ time.Time) ([]*model.ShiftAssignment, error) {
			return []*model.ShiftAssignment{a}, nil
		},
		ListByShiftFn: func(_ context.Context, _, _ uuid.UUID) ([]*model.ShiftAssignment, error) {
			return []*model.ShiftAssignment{a}, nil // sole assignee
		},
	}
	shiftRepo := &testutil.MockShiftInstanceRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.ShiftInstance, error) { return shift, nil },
	}
	svc := newLeaveService(leaveRepo, &testutil.MockEmployeeRepo{}, assignRepo, shiftRepo)

	impact, err := svc.GetLeaveImpact(context.Background(), tenantID, lr.ID)
	require.NoError(t, err)
	assert.InDelta(t, 4.5, impact.TotalHoursLost, 0.01)
}

// TestLeaveService_GetLeaveImpact_MultipleShiftsMixed tests a realistic scenario:
// three shifts during the leave period — one with no other cover, one with cover,
// and one already cancelled. Verifies totals aggregate correctly.
func TestLeaveService_GetLeaveImpact_MultipleShiftsMixed(t *testing.T) {
	tenantID   := uuid.New()
	employeeID := uuid.New()
	shift1ID   := uuid.New()
	shift2ID   := uuid.New()

	lr := testutil.NewLeaveRequest(tenantID, employeeID)

	// Shift 1: sole assignee (uncovered), 09:00–17:00 = 8h
	a1 := testutil.NewShiftAssignment(tenantID, shift1ID, employeeID)
	a1.ShiftStartTime = "09:00"
	a1.ShiftEndTime   = "17:00"

	// Shift 2: has another active assignee (covered), 14:00–18:00 = 4h
	a2 := testutil.NewShiftAssignment(tenantID, shift2ID, employeeID)
	a2.ShiftStartTime = "14:00"
	a2.ShiftEndTime   = "18:00"
	otherA2 := testutil.NewShiftAssignment(tenantID, shift2ID, uuid.New())
	otherA2.Status = model.AssignmentStatusConfirmed

	// Shift 3: already cancelled, should be ignored entirely
	cancelledA := testutil.NewShiftAssignment(tenantID, uuid.New(), employeeID)
	cancelledA.Status = model.AssignmentStatusCancelled

	s1 := testutil.NewShiftInstance(tenantID)
	s1.ID = shift1ID
	s2 := testutil.NewShiftInstance(tenantID)
	s2.ID = shift2ID

	leaveRepo := &testutil.MockLeaveRequestRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.LeaveRequest, error) { return lr, nil },
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByEmployeeFn: func(_ context.Context, _, _ uuid.UUID, _, _ time.Time) ([]*model.ShiftAssignment, error) {
			return []*model.ShiftAssignment{a1, a2, cancelledA}, nil
		},
		ListByShiftFn: func(_ context.Context, _, shiftID uuid.UUID) ([]*model.ShiftAssignment, error) {
			if shiftID == shift1ID {
				return []*model.ShiftAssignment{a1}, nil // sole assignee
			}
			if shiftID == shift2ID {
				return []*model.ShiftAssignment{a2, otherA2}, nil // covered
			}
			return nil, nil
		},
	}
	shiftRepo := &testutil.MockShiftInstanceRepo{
		GetByIDFn: func(_ context.Context, _, id uuid.UUID) (*model.ShiftInstance, error) {
			if id == shift1ID {
				return s1, nil
			}
			if id == shift2ID {
				return s2, nil
			}
			return nil, nil
		},
	}
	svc := newLeaveService(leaveRepo, &testutil.MockEmployeeRepo{}, assignRepo, shiftRepo)

	impact, err := svc.GetLeaveImpact(context.Background(), tenantID, lr.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, impact.TotalCancellations, "only non-cancelled assignments count")
	assert.Equal(t, 1, impact.UncoveredShifts, "only shift1 is uncovered")
	assert.InDelta(t, 12.0, impact.TotalHoursLost, 0.01, "8h + 4h = 12h")
	require.Len(t, impact.AffectedShifts, 2)
}
