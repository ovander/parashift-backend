package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/repo/mocks"
	"github.com/ovander/parashift/internal/service"
	"github.com/ovander/parashift/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// newExceptionRepoMock builds a generated (mockery) StoreExceptionRepository whose
// ListByDateRange returns fn(tenantID). The expectation is optional (.Maybe) so
// tests that never reach the exception check don't fail (DX-3).
func newExceptionRepoMock(t *testing.T, fn func(tID uuid.UUID) []*model.StoreException) *mocks.StoreExceptionRepository {
	t.Helper()
	m := mocks.NewStoreExceptionRepository(t)
	m.EXPECT().
		ListByDateRange(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, tID uuid.UUID, _, _ time.Time) ([]*model.StoreException, error) {
			return fn(tID), nil
		}).Maybe()
	return m
}

func newCoverageService(
	covRepo *testutil.MockCoverageRequirementRepo,
	shiftRepo *testutil.MockShiftInstanceRepo,
	assignRepo *testutil.MockShiftAssignmentRepo,
	empRepo *testutil.MockEmployeeRepo,
) *service.CoverageService {
	return service.NewCoverageService(covRepo, shiftRepo, assignRepo, empRepo, newTestEmitter(), newTestLogger())
}

// ─── ListRequirements ─────────────────────────────────────────────────────────

func TestCoverageService_ListRequirements(t *testing.T) {
	tenantID := uuid.New()
	reqs := []*model.CoverageRequirement{
		testutil.NewCoverageRequirement(tenantID),
		testutil.NewCoverageRequirement(tenantID),
	}

	covRepo := &testutil.MockCoverageRequirementRepo{
		ListFn: func(_ context.Context, tID uuid.UUID) ([]*model.CoverageRequirement, error) {
			assert.Equal(t, tenantID, tID)
			return reqs, nil
		},
	}
	svc := newCoverageService(covRepo, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockEmployeeRepo{})

	got, err := svc.ListRequirements(context.Background(), tenantID)
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestCoverageService_ListRequirements_RepoError(t *testing.T) {
	covRepo := &testutil.MockCoverageRequirementRepo{
		ListFn: func(_ context.Context, _ uuid.UUID) ([]*model.CoverageRequirement, error) {
			return nil, errors.New("db error")
		},
	}
	svc := newCoverageService(covRepo, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockEmployeeRepo{})

	_, err := svc.ListRequirements(context.Background(), uuid.New())
	require.Error(t, err)
}

// ─── CreateRequirement ────────────────────────────────────────────────────────

func TestCoverageService_CreateRequirement(t *testing.T) {
	tenantID := uuid.New()
	created := false

	covRepo := &testutil.MockCoverageRequirementRepo{
		CreateFn: func(_ context.Context, cr *model.CoverageRequirement) error {
			created = true
			assert.Equal(t, tenantID, cr.TenantID)
			assert.Equal(t, 1, cr.DayOfWeek)
			assert.Equal(t, "09:00", cr.StartTime)
			assert.Equal(t, "17:00", cr.EndTime)
			assert.Equal(t, 2, cr.MinStaff)
			return nil
		},
	}
	svc := newCoverageService(covRepo, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockEmployeeRepo{})

	role := "pharmacist"
	req := dto.CoverageRequirementRequest{
		DayOfWeek:    1,
		StartTime:    "09:00",
		EndTime:      "17:00",
		MinStaff:     2,
		RequiredRole: &role,
	}
	got, err := svc.CreateRequirement(context.Background(), tenantID, req)
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, tenantID, got.TenantID)
	assert.Equal(t, 1, got.DayOfWeek)
}

func TestCoverageService_CreateRequirement_RepoError(t *testing.T) {
	covRepo := &testutil.MockCoverageRequirementRepo{
		CreateFn: func(_ context.Context, _ *model.CoverageRequirement) error {
			return errors.New("db error")
		},
	}
	svc := newCoverageService(covRepo, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockEmployeeRepo{})

	_, err := svc.CreateRequirement(context.Background(), uuid.New(), dto.CoverageRequirementRequest{
		DayOfWeek: 1, StartTime: "09:00", EndTime: "17:00", MinStaff: 1,
	})
	require.Error(t, err)
}

// ─── UpdateRequirement ────────────────────────────────────────────────────────

func TestCoverageService_UpdateRequirement(t *testing.T) {
	tenantID := uuid.New()
	existing := testutil.NewCoverageRequirement(tenantID)
	updated := false

	covRepo := &testutil.MockCoverageRequirementRepo{
		ListFn: func(_ context.Context, _ uuid.UUID) ([]*model.CoverageRequirement, error) {
			return []*model.CoverageRequirement{existing}, nil
		},
		UpdateFn: func(_ context.Context, cr *model.CoverageRequirement) error {
			updated = true
			assert.Equal(t, 3, cr.MinStaff)
			return nil
		},
	}
	svc := newCoverageService(covRepo, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockEmployeeRepo{})

	req := dto.CoverageRequirementRequest{
		DayOfWeek: existing.DayOfWeek,
		StartTime: existing.StartTime,
		EndTime:   existing.EndTime,
		MinStaff:  3,
	}
	got, err := svc.UpdateRequirement(context.Background(), tenantID, existing.ID, req)
	require.NoError(t, err)
	assert.True(t, updated)
	assert.Equal(t, 3, got.MinStaff)
}

func TestCoverageService_UpdateRequirement_NotFound(t *testing.T) {
	covRepo := &testutil.MockCoverageRequirementRepo{
		ListFn: func(_ context.Context, _ uuid.UUID) ([]*model.CoverageRequirement, error) {
			return []*model.CoverageRequirement{}, nil
		},
	}
	svc := newCoverageService(covRepo, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockEmployeeRepo{})

	_, err := svc.UpdateRequirement(context.Background(), uuid.New(), uuid.New(), dto.CoverageRequirementRequest{
		DayOfWeek: 1, StartTime: "09:00", EndTime: "17:00", MinStaff: 2,
	})
	require.Error(t, err)
}

// ─── DeleteRequirement ────────────────────────────────────────────────────────

func TestCoverageService_DeleteRequirement(t *testing.T) {
	tenantID := uuid.New()
	reqID := uuid.New()
	deleted := false

	covRepo := &testutil.MockCoverageRequirementRepo{
		DeleteFn: func(_ context.Context, tID, id uuid.UUID) error {
			deleted = true
			assert.Equal(t, tenantID, tID)
			assert.Equal(t, reqID, id)
			return nil
		},
	}
	svc := newCoverageService(covRepo, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockEmployeeRepo{})

	err := svc.DeleteRequirement(context.Background(), tenantID, reqID)
	require.NoError(t, err)
	assert.True(t, deleted)
}

func TestCoverageService_DeleteRequirement_RepoError(t *testing.T) {
	covRepo := &testutil.MockCoverageRequirementRepo{
		DeleteFn: func(_ context.Context, _, _ uuid.UUID) error {
			return errors.New("db error")
		},
	}
	svc := newCoverageService(covRepo, &testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, &testutil.MockEmployeeRepo{})

	err := svc.DeleteRequirement(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
}

// ─── ComputeForDateRange ──────────────────────────────────────────────────────

func TestCoverageService_ComputeForDateRange_NoRequirements(t *testing.T) {
	// When there are no requirements, the report should be empty / no gaps
	covRepo := &testutil.MockCoverageRequirementRepo{
		ListFn: func(_ context.Context, _ uuid.UUID) ([]*model.CoverageRequirement, error) {
			return []*model.CoverageRequirement{}, nil
		},
	}
	shiftRepo := &testutil.MockShiftInstanceRepo{
		ListByDateFn: func(_ context.Context, _ uuid.UUID, _ time.Time) ([]*model.ShiftInstance, error) {
			return []*model.ShiftInstance{}, nil
		},
	}
	svc := newCoverageService(covRepo, shiftRepo, &testutil.MockShiftAssignmentRepo{}, &testutil.MockEmployeeRepo{})

	from := time.Now()
	to := from.AddDate(0, 0, 1)
	report, err := svc.ComputeForDateRange(context.Background(), uuid.New(), from, to)
	require.NoError(t, err)
	assert.Equal(t, 0, report.GapCount)
}

func TestCoverageService_ComputeForDateRange_Understaffed(t *testing.T) {
	tenantID := uuid.New()
	now := time.Now()
	// A Monday
	monday := now.AddDate(0, 0, int(time.Monday-now.Weekday()))
	if monday.Before(now) {
		monday = monday.AddDate(0, 0, 7)
	}

	req := testutil.NewCoverageRequirement(tenantID)
	req.DayOfWeek = int(monday.Weekday())
	req.MinStaff = 3 // need 3 but 0 assigned

	shift := testutil.NewShiftInstance(tenantID)
	shift.Date = monday
	shift.StartTime = req.StartTime
	shift.EndTime = req.EndTime

	covRepo := &testutil.MockCoverageRequirementRepo{
		ListFn: func(_ context.Context, _ uuid.UUID) ([]*model.CoverageRequirement, error) {
			return []*model.CoverageRequirement{req}, nil
		},
	}
	shiftRepo := &testutil.MockShiftInstanceRepo{
		ListByDateFn: func(_ context.Context, _ uuid.UUID, date time.Time) ([]*model.ShiftInstance, error) {
			if date.Day() == monday.Day() {
				return []*model.ShiftInstance{shift}, nil
			}
			return nil, nil
		},
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByShiftFn: func(_ context.Context, _, _ uuid.UUID) ([]*model.ShiftAssignment, error) {
			return []*model.ShiftAssignment{}, nil // 0 confirmed
		},
		CountByShiftFn: func(_ context.Context, _, _ uuid.UUID) (int64, error) {
			return 0, nil
		},
	}
	svc := newCoverageService(covRepo, shiftRepo, assignRepo, &testutil.MockEmployeeRepo{})

	report, err := svc.ComputeForDateRange(context.Background(), tenantID, monday, monday.AddDate(0, 0, 1))
	require.NoError(t, err)
	assert.Greater(t, report.GapCount, 0)
}

// ─── ComputeForDateRange — assigned_count and status scenarios ────────────────

// monday returns the Monday of the current ISO week (UTC, midnight).
func monday() time.Time {
	now := time.Now().UTC()
	offset := int(time.Monday - now.Weekday())
	if offset > 0 {
		offset -= 7
	}
	d := now.AddDate(0, 0, offset)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
}

// coverageScenario wires up the three repos for a single-day, single-slot test.
type coverageScenario struct {
	tenantID  uuid.UUID
	day       time.Time          // exact date of the scenario
	req       *model.CoverageRequirement
	shift     *model.ShiftInstance
	assigns   []*model.ShiftAssignment
	employees []*model.Employee
}

func newCoverageScenario(t *testing.T) *coverageScenario {
	t.Helper()
	tenantID := uuid.New()
	day := monday()

	req := testutil.NewCoverageRequirement(tenantID)
	req.DayOfWeek = int(day.Weekday()) // match the test day
	req.StartTime = "09:00"
	req.EndTime = "17:00"
	req.MinStaff = 1
	req.RequiredRole = "" // default: no role requirement

	shift := testutil.NewShiftInstance(tenantID)
	shift.Date = day
	shift.StartTime = "09:00"
	shift.EndTime = "17:00"

	return &coverageScenario{tenantID: tenantID, day: day, req: req, shift: shift}
}

func (s *coverageScenario) buildService() *service.CoverageService {
	covRepo := &testutil.MockCoverageRequirementRepo{
		ListFn: func(_ context.Context, _ uuid.UUID) ([]*model.CoverageRequirement, error) {
			return []*model.CoverageRequirement{s.req}, nil
		},
	}
	shiftRepo := &testutil.MockShiftInstanceRepo{
		ListByDateFn: func(_ context.Context, _ uuid.UUID, date time.Time) ([]*model.ShiftInstance, error) {
			if date.Year() == s.day.Year() && date.Month() == s.day.Month() && date.Day() == s.day.Day() {
				return []*model.ShiftInstance{s.shift}, nil
			}
			return nil, nil
		},
	}
	empMap := make(map[uuid.UUID]*model.Employee, len(s.employees))
	for _, e := range s.employees {
		empMap[e.ID] = e
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByShiftFn: func(_ context.Context, _, _ uuid.UUID) ([]*model.ShiftAssignment, error) {
			return s.assigns, nil
		},
	}
	empRepo := &testutil.MockEmployeeRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, _, _ int) ([]*model.Employee, int64, error) {
			return s.employees, int64(len(s.employees)), nil
		},
	}
	return newCoverageService(covRepo, shiftRepo, assignRepo, empRepo)
}

// TestCoverageService_ComputeForDateRange_OK verifies that a confirmed assignment
// matching the requirement results in status OK and assigned_count=1.
func TestCoverageService_ComputeForDateRange_OK(t *testing.T) {
	sc := newCoverageScenario(t)
	emp := testutil.NewEmployee(sc.tenantID)
	assign := testutil.NewShiftAssignment(sc.tenantID, sc.shift.ID, emp.ID)
	sc.assigns = []*model.ShiftAssignment{assign}
	sc.employees = []*model.Employee{emp}
	sc.req.MinStaff = 1

	svc := sc.buildService()
	report, err := svc.ComputeForDateRange(context.Background(), sc.tenantID, sc.day, sc.day)
	require.NoError(t, err)

	require.Len(t, report.Items, 1)
	slot := report.Items[0]
	assert.Equal(t, service.CoverageOK, slot.Status)
	assert.Equal(t, 1, slot.AssignedCount)
	assert.Equal(t, 0, report.GapCount)
}

// TestCoverageService_ComputeForDateRange_Overstaffed checks OVERSTAFFED is returned
// when more employees are assigned than the minimum required.
func TestCoverageService_ComputeForDateRange_Overstaffed(t *testing.T) {
	sc := newCoverageScenario(t)
	emp1 := testutil.NewEmployee(sc.tenantID)
	emp2 := testutil.NewEmployee(sc.tenantID)
	sc.assigns = []*model.ShiftAssignment{
		testutil.NewShiftAssignment(sc.tenantID, sc.shift.ID, emp1.ID),
		testutil.NewShiftAssignment(sc.tenantID, sc.shift.ID, emp2.ID),
	}
	sc.employees = []*model.Employee{emp1, emp2}
	sc.req.MinStaff = 1

	svc := sc.buildService()
	report, err := svc.ComputeForDateRange(context.Background(), sc.tenantID, sc.day, sc.day)
	require.NoError(t, err)

	require.Len(t, report.Items, 1)
	assert.Equal(t, service.CoverageOverstaffed, report.Items[0].Status)
	assert.Equal(t, 2, report.Items[0].AssignedCount)
	assert.Equal(t, 0, report.GapCount) // overstaffed is NOT a gap
}

// TestCoverageService_ComputeForDateRange_PendingNotCounted verifies that a
// pending (non-confirmed) assignment does NOT increment assigned_count.
func TestCoverageService_ComputeForDateRange_PendingNotCounted(t *testing.T) {
	sc := newCoverageScenario(t)
	emp := testutil.NewEmployee(sc.tenantID)
	assign := testutil.NewShiftAssignment(sc.tenantID, sc.shift.ID, emp.ID)
	assign.Status = model.AssignmentStatusPending // pending → must not count
	sc.assigns = []*model.ShiftAssignment{assign}
	sc.employees = []*model.Employee{emp}
	sc.req.MinStaff = 1

	svc := sc.buildService()
	report, err := svc.ComputeForDateRange(context.Background(), sc.tenantID, sc.day, sc.day)
	require.NoError(t, err)

	require.Len(t, report.Items, 1)
	slot := report.Items[0]
	assert.Equal(t, 0, slot.AssignedCount)
	assert.Equal(t, service.CoverageUnderstaffed, slot.Status)
	assert.Equal(t, 1, report.GapCount)
}

// TestCoverageService_ComputeForDateRange_RequiredRoleMatch verifies that when
// required_role is set and the assigned employee's Role matches, status is OK.
func TestCoverageService_ComputeForDateRange_RequiredRoleMatch(t *testing.T) {
	sc := newCoverageScenario(t)
	emp := testutil.NewEmployee(sc.tenantID) // Role = "pharmacist" from helper
	assign := testutil.NewShiftAssignment(sc.tenantID, sc.shift.ID, emp.ID)
	sc.assigns = []*model.ShiftAssignment{assign}
	sc.employees = []*model.Employee{emp}
	sc.req.MinStaff = 1
	sc.req.RequiredRole = "pharmacist"

	svc := sc.buildService()
	report, err := svc.ComputeForDateRange(context.Background(), sc.tenantID, sc.day, sc.day)
	require.NoError(t, err)

	require.Len(t, report.Items, 1)
	slot := report.Items[0]
	assert.Equal(t, service.CoverageOK, slot.Status)
	assert.Equal(t, 1, slot.AssignedCount)
	assert.False(t, slot.MissingRole)
}

// TestCoverageService_ComputeForDateRange_RequiredRoleMismatch is a regression
// test for the bug where MISSING_ROLE incorrectly reset assigned_count to 0.
// Even when the role doesn't match, confirmed assignments must still be counted.
func TestCoverageService_ComputeForDateRange_RequiredRoleMismatch(t *testing.T) {
	sc := newCoverageScenario(t)
	emp := testutil.NewEmployee(sc.tenantID)
	emp.JobRole = "animator" // does NOT match requirement's required_role
	assign := testutil.NewShiftAssignment(sc.tenantID, sc.shift.ID, emp.ID)
	sc.assigns = []*model.ShiftAssignment{assign}
	sc.employees = []*model.Employee{emp}
	sc.req.MinStaff = 1
	sc.req.RequiredRole = "pharmacist" // mismatch

	svc := sc.buildService()
	report, err := svc.ComputeForDateRange(context.Background(), sc.tenantID, sc.day, sc.day)
	require.NoError(t, err)

	require.Len(t, report.Items, 1)
	slot := report.Items[0]
	// assigned_count must reflect the real number of confirmed assignments,
	// NOT be zeroed out by the role mismatch.
	assert.Equal(t, 1, slot.AssignedCount, "assigned_count must not be 0 even when role mismatches")
	assert.Equal(t, service.CoverageMissingRole, slot.Status)
	assert.True(t, slot.MissingRole)
	assert.Equal(t, 1, report.GapCount)
}

// TestCoverageService_ComputeForDateRange_RequiredRoleEmpty verifies that when
// required_role is empty the role check is skipped entirely.
func TestCoverageService_ComputeForDateRange_RequiredRoleEmpty(t *testing.T) {
	sc := newCoverageScenario(t)
	emp := testutil.NewEmployee(sc.tenantID)
	emp.JobRole = "pharmacist" // any job_role should be fine when required_role is ""
	assign := testutil.NewShiftAssignment(sc.tenantID, sc.shift.ID, emp.ID)
	sc.assigns = []*model.ShiftAssignment{assign}
	sc.employees = []*model.Employee{emp}
	sc.req.MinStaff = 1
	sc.req.RequiredRole = "" // empty → no role enforcement

	svc := sc.buildService()
	report, err := svc.ComputeForDateRange(context.Background(), sc.tenantID, sc.day, sc.day)
	require.NoError(t, err)

	require.Len(t, report.Items, 1)
	slot := report.Items[0]
	assert.Equal(t, service.CoverageOK, slot.Status)
	assert.Equal(t, 1, slot.AssignedCount)
	assert.False(t, slot.MissingRole)
	assert.Equal(t, 0, report.GapCount)
}

// TestCoverageService_ComputeForDateRange_NonOverlappingShift verifies that a
// shift whose time window does NOT overlap with the requirement is ignored.
func TestCoverageService_ComputeForDateRange_NonOverlappingShift(t *testing.T) {
	sc := newCoverageScenario(t)
	// Requirement: 09:00-12:00, shift: 13:00-18:00 → no overlap
	sc.req.StartTime = "09:00"
	sc.req.EndTime = "12:00"
	sc.shift.StartTime = "13:00"
	sc.shift.EndTime = "18:00"
	sc.req.MinStaff = 1

	emp := testutil.NewEmployee(sc.tenantID)
	assign := testutil.NewShiftAssignment(sc.tenantID, sc.shift.ID, emp.ID)
	sc.assigns = []*model.ShiftAssignment{assign}
	sc.employees = []*model.Employee{emp}

	svc := sc.buildService()
	report, err := svc.ComputeForDateRange(context.Background(), sc.tenantID, sc.day, sc.day)
	require.NoError(t, err)

	require.Len(t, report.Items, 1)
	slot := report.Items[0]
	// Shift doesn't cover the requirement window → not counted.
	assert.Equal(t, 0, slot.AssignedCount)
	assert.Equal(t, service.CoverageUnderstaffed, slot.Status)
}

// TestCoverageService_ComputeForDateRange_MultiDayRange checks that the service
// correctly processes multiple days and accumulates gap_count.
func TestCoverageService_ComputeForDateRange_MultiDayRange(t *testing.T) {
	tenantID := uuid.New()
	day := monday()

	// Requirement for Monday only.
	req := testutil.NewCoverageRequirement(tenantID)
	req.DayOfWeek = int(day.Weekday())
	req.RequiredRole = ""
	req.MinStaff = 1

	shiftOnMonday := testutil.NewShiftInstance(tenantID)
	shiftOnMonday.Date = day
	shiftOnMonday.StartTime = req.StartTime
	shiftOnMonday.EndTime = req.EndTime

	emp := testutil.NewEmployee(tenantID)
	assignOnMonday := testutil.NewShiftAssignment(tenantID, shiftOnMonday.ID, emp.ID)

	covRepo := &testutil.MockCoverageRequirementRepo{
		ListFn: func(_ context.Context, _ uuid.UUID) ([]*model.CoverageRequirement, error) {
			return []*model.CoverageRequirement{req}, nil
		},
	}
	shiftRepo := &testutil.MockShiftInstanceRepo{
		ListByDateFn: func(_ context.Context, _ uuid.UUID, date time.Time) ([]*model.ShiftInstance, error) {
			if date.Year() == day.Year() && date.Month() == day.Month() && date.Day() == day.Day() {
				return []*model.ShiftInstance{shiftOnMonday}, nil
			}
			return nil, nil
		},
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByShiftFn: func(_ context.Context, _, shiftID uuid.UUID) ([]*model.ShiftAssignment, error) {
			if shiftID == shiftOnMonday.ID {
				return []*model.ShiftAssignment{assignOnMonday}, nil
			}
			return nil, nil
		},
	}
	empRepo := &testutil.MockEmployeeRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, _, _ int) ([]*model.Employee, int64, error) {
			return []*model.Employee{emp}, 1, nil
		},
	}
	svc := newCoverageService(covRepo, shiftRepo, assignRepo, empRepo)

	// Query Monday through Friday (5 days). Req only matches Monday.
	friday := day.AddDate(0, 0, 4)
	report, err := svc.ComputeForDateRange(context.Background(), tenantID, day, friday)
	require.NoError(t, err)

	// Only one slot (Monday) — other days have no matching requirement day.
	assert.Equal(t, 1, report.TotalSlots)
	assert.Equal(t, 0, report.GapCount)
	require.Len(t, report.Items, 1)
	assert.Equal(t, service.CoverageOK, report.Items[0].Status)
	assert.Equal(t, 1, report.Items[0].AssignedCount)
}

// ─── timeOverlaps (via ComputeForDateRange) ───────────────────────────────────

// TestCoverageService_TimeOverlaps_PartialOverlap checks that a shift overlapping
// only partially with the requirement is still counted.
func TestCoverageService_TimeOverlaps_PartialOverlap(t *testing.T) {
	sc := newCoverageScenario(t)
	// Requirement 09:00-12:00, shift 11:00-14:00 → overlap 11:00-12:00
	sc.req.StartTime = "09:00"
	sc.req.EndTime = "12:00"
	sc.shift.StartTime = "11:00"
	sc.shift.EndTime = "14:00"
	sc.req.MinStaff = 1

	emp := testutil.NewEmployee(sc.tenantID)
	sc.assigns = []*model.ShiftAssignment{testutil.NewShiftAssignment(sc.tenantID, sc.shift.ID, emp.ID)}
	sc.employees = []*model.Employee{emp}

	svc := sc.buildService()
	report, err := svc.ComputeForDateRange(context.Background(), sc.tenantID, sc.day, sc.day)
	require.NoError(t, err)

	require.Len(t, report.Items, 1)
	assert.Equal(t, 1, report.Items[0].AssignedCount, "partially overlapping shift should be counted")
	assert.Equal(t, service.CoverageOK, report.Items[0].Status)
}

// TestCoverageService_TimeOverlaps_AdjacentNoOverlap checks that exactly-adjacent
// windows (shift ends exactly when requirement starts) do NOT count as overlap.
func TestCoverageService_TimeOverlaps_AdjacentNoOverlap(t *testing.T) {
	sc := newCoverageScenario(t)
	// Shift: 07:00-09:00, Requirement: 09:00-17:00 → touch at 09:00 only, no real overlap
	sc.req.StartTime = "09:00"
	sc.req.EndTime = "17:00"
	sc.shift.StartTime = "07:00"
	sc.shift.EndTime = "09:00"
	sc.req.MinStaff = 1

	emp := testutil.NewEmployee(sc.tenantID)
	sc.assigns = []*model.ShiftAssignment{testutil.NewShiftAssignment(sc.tenantID, sc.shift.ID, emp.ID)}
	sc.employees = []*model.Employee{emp}

	svc := sc.buildService()
	report, err := svc.ComputeForDateRange(context.Background(), sc.tenantID, sc.day, sc.day)
	require.NoError(t, err)

	require.Len(t, report.Items, 1)
	assert.Equal(t, 0, report.Items[0].AssignedCount, "adjacent but non-overlapping shift must not be counted")
}

// ─── EXTRA_OPEN exception — holiday override ──────────────────────────────────
//
// newHolidaySvcStub returns a PublicHolidayService whose repo mock makes
// IsHoliday(ctx, date, zone) return (true, name, nil) for every date present in
// the holidayDates map, and (false, "", nil) for all other dates.
// ListByYearFn returns a non-empty slice so EnsureYear skips the real API call.
func newHolidaySvcStub(holidayDates map[string]string) *service.PublicHolidayService {
	stub := &testutil.MockPublicHolidayRepo{
		// Return one placeholder so EnsureYear thinks the year is already cached.
		ListByYearFn: func(_ context.Context, _ int, _ string) ([]*model.PublicHoliday, error) {
			return []*model.PublicHoliday{
				{Date: time.Now(), Name: "placeholder", Zone: service.DefaultZone},
			}, nil
		},
		GetByDateFn: func(_ context.Context, date time.Time, _ string) (*model.PublicHoliday, error) {
			key := date.Format("2006-01-02")
			if name, ok := holidayDates[key]; ok {
				return &model.PublicHoliday{Date: date, Name: name, Zone: service.DefaultZone}, nil
			}
			return nil, nil
		},
	}
	return service.NewPublicHolidayService(stub, newTestLogger())
}

// TestCoverageService_ComputeForDateRange_HolidaySkipped verifies that a public
// holiday without an EXTRA_OPEN exception is excluded from the coverage report.
func TestCoverageService_ComputeForDateRange_HolidaySkipped(t *testing.T) {
	tenantID := uuid.New()
	// Use a fixed Monday so we control the day-of-week requirement.
	day := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC) // Monday (day 1)

	req := testutil.NewCoverageRequirement(tenantID)
	req.DayOfWeek = int(day.Weekday()) // Monday
	req.RequiredRole = ""
	req.MinStaff = 1

	covRepo := &testutil.MockCoverageRequirementRepo{
		ListFn: func(_ context.Context, _ uuid.UUID) ([]*model.CoverageRequirement, error) {
			return []*model.CoverageRequirement{req}, nil
		},
	}
	shiftRepo := &testutil.MockShiftInstanceRepo{
		ListByDateFn: func(_ context.Context, _ uuid.UUID, _ time.Time) ([]*model.ShiftInstance, error) {
			return nil, nil
		},
	}

	holidaySvc := newHolidaySvcStub(map[string]string{
		"2026-04-06": "Lundi de Pâques",
	})
	// No EXTRA_OPEN exception for this date.
	exceptionRepo := newExceptionRepoMock(t, func(uuid.UUID) []*model.StoreException { return nil })

	svc := service.NewCoverageService(covRepo, shiftRepo, &testutil.MockShiftAssignmentRepo{}, &testutil.MockEmployeeRepo{}, newTestEmitter(), newTestLogger()).
		WithHolidayService(holidaySvc).
		WithStoreExceptionRepo(exceptionRepo)

	report, err := svc.ComputeForDateRange(context.Background(), tenantID, day, day)
	require.NoError(t, err)
	// Holiday without EXTRA_OPEN → day is skipped → no items.
	assert.Empty(t, report.Items, "public holiday without EXTRA_OPEN must produce no coverage slot")
	assert.Equal(t, 0, report.GapCount)
}

// TestCoverageService_ComputeForDateRange_ExtraOpenOverridesHoliday verifies that
// when an EXTRA_OPEN exception exists for a public holiday, coverage IS computed
// for that day (the holiday-skip is bypassed).
func TestCoverageService_ComputeForDateRange_ExtraOpenOverridesHoliday(t *testing.T) {
	tenantID := uuid.New()
	day := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC) // Lundi de Pâques (Monday)

	req := testutil.NewCoverageRequirement(tenantID)
	req.DayOfWeek = int(day.Weekday())
	req.RequiredRole = ""
	req.MinStaff = 1

	shift := testutil.NewShiftInstance(tenantID)
	shift.Date = day
	shift.StartTime = req.StartTime
	shift.EndTime = req.EndTime

	emp := testutil.NewEmployee(tenantID)
	assign := testutil.NewShiftAssignment(tenantID, shift.ID, emp.ID)

	covRepo := &testutil.MockCoverageRequirementRepo{
		ListFn: func(_ context.Context, _ uuid.UUID) ([]*model.CoverageRequirement, error) {
			return []*model.CoverageRequirement{req}, nil
		},
	}
	shiftRepo := &testutil.MockShiftInstanceRepo{
		ListByDateFn: func(_ context.Context, _ uuid.UUID, date time.Time) ([]*model.ShiftInstance, error) {
			if date.Format("2006-01-02") == "2026-04-06" {
				return []*model.ShiftInstance{shift}, nil
			}
			return nil, nil
		},
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByShiftFn: func(_ context.Context, _, _ uuid.UUID) ([]*model.ShiftAssignment, error) {
			return []*model.ShiftAssignment{assign}, nil
		},
	}
	empRepo := &testutil.MockEmployeeRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, _, _ int) ([]*model.Employee, int64, error) {
			return []*model.Employee{emp}, 1, nil
		},
	}

	holidaySvc := newHolidaySvcStub(map[string]string{"2026-04-06": "Lundi de Pâques"})
	// EXTRA_OPEN exception for the holiday date.
	exceptionRepo := newExceptionRepoMock(t, func(tID uuid.UUID) []*model.StoreException {
		return []*model.StoreException{testutil.NewStoreException(tID, day, model.ExceptionExtraOpen)}
	})

	svc := service.NewCoverageService(covRepo, shiftRepo, assignRepo, empRepo, newTestEmitter(), newTestLogger()).
		WithHolidayService(holidaySvc).
		WithStoreExceptionRepo(exceptionRepo)

	report, err := svc.ComputeForDateRange(context.Background(), tenantID, day, day)
	require.NoError(t, err)
	// EXTRA_OPEN overrides the holiday-skip → coverage IS computed for this day.
	require.Len(t, report.Items, 1, "EXTRA_OPEN exception must cause coverage to be computed on the holiday")
	assert.Equal(t, service.CoverageOK, report.Items[0].Status)
	assert.Equal(t, 1, report.Items[0].AssignedCount)
	assert.Equal(t, 0, report.GapCount)
}

// TestCoverageService_ComputeForDateRange_ForcedClosedDoesNotOverrideHoliday verifies
// that a FORCED_CLOSED exception (not EXTRA_OPEN) does NOT bypass the holiday-skip.
func TestCoverageService_ComputeForDateRange_ForcedClosedDoesNotOverrideHoliday(t *testing.T) {
	tenantID := uuid.New()
	day := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC) // Lundi de Pâques (Monday)

	req := testutil.NewCoverageRequirement(tenantID)
	req.DayOfWeek = int(day.Weekday())
	req.RequiredRole = ""
	req.MinStaff = 1

	covRepo := &testutil.MockCoverageRequirementRepo{
		ListFn: func(_ context.Context, _ uuid.UUID) ([]*model.CoverageRequirement, error) {
			return []*model.CoverageRequirement{req}, nil
		},
	}
	shiftRepo := &testutil.MockShiftInstanceRepo{
		ListByDateFn: func(_ context.Context, _ uuid.UUID, _ time.Time) ([]*model.ShiftInstance, error) {
			return nil, nil
		},
	}

	holidaySvc := newHolidaySvcStub(map[string]string{"2026-04-06": "Lundi de Pâques"})
	// FORCED_CLOSED exception — must NOT override the holiday skip.
	exceptionRepo := newExceptionRepoMock(t, func(tID uuid.UUID) []*model.StoreException {
		return []*model.StoreException{testutil.NewStoreException(tID, day, model.ExceptionForcedClosed)}
	})

	svc := service.NewCoverageService(covRepo, shiftRepo, &testutil.MockShiftAssignmentRepo{}, &testutil.MockEmployeeRepo{}, newTestEmitter(), newTestLogger()).
		WithHolidayService(holidaySvc).
		WithStoreExceptionRepo(exceptionRepo)

	report, err := svc.ComputeForDateRange(context.Background(), tenantID, day, day)
	require.NoError(t, err)
	// FORCED_CLOSED is not EXTRA_OPEN → holiday-skip is still in effect → no items.
	assert.Empty(t, report.Items, "FORCED_CLOSED must not override the public holiday skip")
	assert.Equal(t, 0, report.GapCount)
}

// TestCoverageService_ComputeForDateRange_ExtraOpenOnNonHoliday verifies that an
// EXTRA_OPEN exception on a regular (non-holiday) day does not break anything:
// coverage is computed normally regardless of the exception.
func TestCoverageService_ComputeForDateRange_ExtraOpenOnNonHoliday(t *testing.T) {
	tenantID := uuid.New()
	// A Sunday that is NOT a public holiday.
	day := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC) // Regular Sunday

	req := testutil.NewCoverageRequirement(tenantID)
	req.DayOfWeek = int(day.Weekday()) // Sunday = 0
	req.RequiredRole = ""
	req.MinStaff = 1

	shift := testutil.NewShiftInstance(tenantID)
	shift.Date = day
	shift.StartTime = req.StartTime
	shift.EndTime = req.EndTime

	emp := testutil.NewEmployee(tenantID)
	assign := testutil.NewShiftAssignment(tenantID, shift.ID, emp.ID)

	covRepo := &testutil.MockCoverageRequirementRepo{
		ListFn: func(_ context.Context, _ uuid.UUID) ([]*model.CoverageRequirement, error) {
			return []*model.CoverageRequirement{req}, nil
		},
	}
	shiftRepo := &testutil.MockShiftInstanceRepo{
		ListByDateFn: func(_ context.Context, _ uuid.UUID, _ time.Time) ([]*model.ShiftInstance, error) {
			return []*model.ShiftInstance{shift}, nil
		},
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByShiftFn: func(_ context.Context, _, _ uuid.UUID) ([]*model.ShiftAssignment, error) {
			return []*model.ShiftAssignment{assign}, nil
		},
	}
	empRepo := &testutil.MockEmployeeRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, _, _ int) ([]*model.Employee, int64, error) {
			return []*model.Employee{emp}, 1, nil
		},
	}

	// No public holiday on this Sunday.
	holidaySvc := newHolidaySvcStub(map[string]string{})
	exceptionRepo := newExceptionRepoMock(t, func(tID uuid.UUID) []*model.StoreException {
		return []*model.StoreException{testutil.NewStoreException(tID, day, model.ExceptionExtraOpen)}
	})

	svc := service.NewCoverageService(covRepo, shiftRepo, assignRepo, empRepo, newTestEmitter(), newTestLogger()).
		WithHolidayService(holidaySvc).
		WithStoreExceptionRepo(exceptionRepo)

	report, err := svc.ComputeForDateRange(context.Background(), tenantID, day, day)
	require.NoError(t, err)
	// Non-holiday: coverage computed normally regardless of the exception type.
	require.Len(t, report.Items, 1, "coverage must be computed for a non-holiday day")
	assert.Equal(t, service.CoverageOK, report.Items[0].Status)
	assert.Equal(t, 0, report.GapCount)
}

// ─── GetGaps ──────────────────────────────────────────────────────────────────

func TestCoverageService_GetGaps_Empty(t *testing.T) {
	covRepo := &testutil.MockCoverageRequirementRepo{
		ListFn: func(_ context.Context, _ uuid.UUID) ([]*model.CoverageRequirement, error) {
			return []*model.CoverageRequirement{}, nil
		},
	}
	shiftRepo := &testutil.MockShiftInstanceRepo{
		ListByDateFn: func(_ context.Context, _ uuid.UUID, _ time.Time) ([]*model.ShiftInstance, error) {
			return nil, nil
		},
	}
	svc := newCoverageService(covRepo, shiftRepo, &testutil.MockShiftAssignmentRepo{}, &testutil.MockEmployeeRepo{})

	from := time.Now()
	gaps, err := svc.GetGaps(context.Background(), uuid.New(), from, from.AddDate(0, 0, 1))
	require.NoError(t, err)
	assert.Empty(t, gaps)
}
