package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/service"
	"github.com/ovander/parashift/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newScheduleService(
	shiftRepo *testutil.MockShiftInstanceRepo,
	assignRepo *testutil.MockShiftAssignmentRepo,
	tmplRepo *testutil.MockWeekTemplateRepo,
	empRepo *testutil.MockEmployeeRepo,
	leaveRepo *testutil.MockLeaveRequestRepo,
	availRepo *testutil.MockAvailabilityRepo,
) *service.ScheduleService {
	return service.NewScheduleService(shiftRepo, assignRepo, tmplRepo, empRepo, leaveRepo, availRepo, &testutil.MockStoreRepo{}, newTestEmitter(), newTestLogger())
}

func authCtx(tenantID uuid.UUID) context.Context {
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)
	ctx = ctxutil.WithUserID(ctx, uuid.New())
	return ctx
}

// ─── GetByID ──────────────────────────────────────────────────────────────────

func TestScheduleService_GetByID_Found(t *testing.T) {
	tenantID := uuid.New()
	shift := testutil.NewShiftInstance(tenantID)

	shiftRepo := &testutil.MockShiftInstanceRepo{
		GetByIDFn: func(_ context.Context, tID, id uuid.UUID) (*model.ShiftInstance, error) {
			assert.Equal(t, tenantID, tID)
			assert.Equal(t, shift.ID, id)
			return shift, nil
		},
	}
	svc := newScheduleService(shiftRepo, &testutil.MockShiftAssignmentRepo{}, &testutil.MockWeekTemplateRepo{}, &testutil.MockEmployeeRepo{}, &testutil.MockLeaveRequestRepo{}, &testutil.MockAvailabilityRepo{})

	got, err := svc.GetByID(context.Background(), tenantID, shift.ID)
	require.NoError(t, err)
	assert.Equal(t, shift.ID, got.ID)
}

func TestScheduleService_GetByID_NotFound(t *testing.T) {
	shiftRepo := &testutil.MockShiftInstanceRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.ShiftInstance, error) {
			return nil, nil
		},
	}
	svc := newScheduleService(shiftRepo, &testutil.MockShiftAssignmentRepo{}, &testutil.MockWeekTemplateRepo{}, &testutil.MockEmployeeRepo{}, &testutil.MockLeaveRequestRepo{}, &testutil.MockAvailabilityRepo{})

	_, err := svc.GetByID(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
}

// ─── GetSchedule ──────────────────────────────────────────────────────────────

func TestScheduleService_GetSchedule(t *testing.T) {
	tenantID := uuid.New()
	shifts := []*model.ShiftInstance{
		testutil.NewShiftInstance(tenantID),
		testutil.NewShiftInstance(tenantID),
	}

	shiftRepo := &testutil.MockShiftInstanceRepo{
		ListByDateRangeFn: func(_ context.Context, tID uuid.UUID, from, to time.Time, page, pageSize int) ([]*model.ShiftInstance, int64, error) {
			assert.Equal(t, tenantID, tID)
			return shifts, int64(len(shifts)), nil
		},
	}
	svc := newScheduleService(shiftRepo, &testutil.MockShiftAssignmentRepo{}, &testutil.MockWeekTemplateRepo{}, &testutil.MockEmployeeRepo{}, &testutil.MockLeaveRequestRepo{}, &testutil.MockAvailabilityRepo{})

	from := time.Now()
	to := from.AddDate(0, 0, 7)
	got, total, err := svc.GetSchedule(context.Background(), tenantID, from, to, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, got, 2)
}

// ─── CreateShift ──────────────────────────────────────────────────────────────

func TestScheduleService_CreateShift(t *testing.T) {
	tenantID := uuid.New()
	created := false

	shiftRepo := &testutil.MockShiftInstanceRepo{
		CreateFn: func(_ context.Context, s *model.ShiftInstance) error {
			created = true
			assert.Equal(t, tenantID, s.TenantID)
			assert.Equal(t, "09:00", s.StartTime)
			assert.Equal(t, "17:00", s.EndTime)
			return nil
		},
	}
	svc := newScheduleService(shiftRepo, &testutil.MockShiftAssignmentRepo{}, &testutil.MockWeekTemplateRepo{}, &testutil.MockEmployeeRepo{}, &testutil.MockLeaveRequestRepo{}, &testutil.MockAvailabilityRepo{})

	req := dto.CreateShiftInstanceRequest{
		Date:      time.Now(),
		StartTime: "09:00",
		EndTime:   "17:00",
		Role:      strPtr("pharmacist"),
	}
	got, err := svc.CreateShift(authCtx(tenantID), tenantID, req)
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, "09:00", got.StartTime)
}

func TestScheduleService_CreateShift_RepoError(t *testing.T) {
	shiftRepo := &testutil.MockShiftInstanceRepo{
		CreateFn: func(_ context.Context, _ *model.ShiftInstance) error {
			return errors.New("db error")
		},
	}
	svc := newScheduleService(shiftRepo, &testutil.MockShiftAssignmentRepo{}, &testutil.MockWeekTemplateRepo{}, &testutil.MockEmployeeRepo{}, &testutil.MockLeaveRequestRepo{}, &testutil.MockAvailabilityRepo{})

	_, err := svc.CreateShift(authCtx(uuid.New()), uuid.New(), dto.CreateShiftInstanceRequest{
		Date: time.Now(), StartTime: "09:00", EndTime: "17:00",
	})
	require.Error(t, err)
}

// ─── UpdateShift ──────────────────────────────────────────────────────────────

func TestScheduleService_UpdateShift(t *testing.T) {
	tenantID := uuid.New()
	shift := testutil.NewShiftInstance(tenantID)
	newRole := "manager"

	shiftRepo := &testutil.MockShiftInstanceRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.ShiftInstance, error) {
			return shift, nil
		},
		UpdateFn: func(_ context.Context, s *model.ShiftInstance) error {
			assert.Equal(t, newRole, s.Role)
			return nil
		},
	}
	svc := newScheduleService(shiftRepo, &testutil.MockShiftAssignmentRepo{}, &testutil.MockWeekTemplateRepo{}, &testutil.MockEmployeeRepo{}, &testutil.MockLeaveRequestRepo{}, &testutil.MockAvailabilityRepo{})

	got, err := svc.UpdateShift(authCtx(tenantID), tenantID, shift.ID, dto.UpdateShiftInstanceRequest{Role: &newRole})
	require.NoError(t, err)
	assert.Equal(t, newRole, got.Role)
}

func TestScheduleService_UpdateShift_NotFound(t *testing.T) {
	shiftRepo := &testutil.MockShiftInstanceRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.ShiftInstance, error) {
			return nil, nil
		},
	}
	svc := newScheduleService(shiftRepo, &testutil.MockShiftAssignmentRepo{}, &testutil.MockWeekTemplateRepo{}, &testutil.MockEmployeeRepo{}, &testutil.MockLeaveRequestRepo{}, &testutil.MockAvailabilityRepo{})

	role := "x"
	_, err := svc.UpdateShift(authCtx(uuid.New()), uuid.New(), uuid.New(), dto.UpdateShiftInstanceRequest{Role: &role})
	require.Error(t, err)
}

// ─── DeleteShift ──────────────────────────────────────────────────────────────

func TestScheduleService_DeleteShift(t *testing.T) {
	tenantID := uuid.New()
	shiftID := uuid.New()
	deleted := false

	shiftRepo := &testutil.MockShiftInstanceRepo{
		DeleteFn: func(_ context.Context, tID, id uuid.UUID) error {
			deleted = true
			assert.Equal(t, tenantID, tID)
			assert.Equal(t, shiftID, id)
			return nil
		},
	}
	svc := newScheduleService(shiftRepo, &testutil.MockShiftAssignmentRepo{}, &testutil.MockWeekTemplateRepo{}, &testutil.MockEmployeeRepo{}, &testutil.MockLeaveRequestRepo{}, &testutil.MockAvailabilityRepo{})

	err := svc.DeleteShift(authCtx(tenantID), tenantID, shiftID)
	require.NoError(t, err)
	assert.True(t, deleted)
}

// ─── CreateAssignment ─────────────────────────────────────────────────────────

func TestScheduleService_CreateAssignment(t *testing.T) {
	tenantID := uuid.New()
	shift := testutil.NewShiftInstance(tenantID)
	employee := testutil.NewEmployee(tenantID)
	assigned := false

	shiftRepo := &testutil.MockShiftInstanceRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.ShiftInstance, error) {
			return shift, nil
		},
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ExistsConflictFn: func(_ context.Context, _, _ uuid.UUID, _ time.Time, _, _ string, _ *uuid.UUID) (bool, error) {
			return false, nil
		},
		CountByShiftFn: func(_ context.Context, _, _ uuid.UUID) (int64, error) {
			return 0, nil
		},
		CreateFn: func(_ context.Context, a *model.ShiftAssignment) error {
			assigned = true
			assert.Equal(t, shift.ID, a.ShiftInstanceID)
			assert.Equal(t, employee.ID, a.EmployeeID)
			return nil
		},
	}
	empRepo := &testutil.MockEmployeeRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Employee, error) {
			return employee, nil
		},
	}
	svc := newScheduleService(shiftRepo, assignRepo, &testutil.MockWeekTemplateRepo{}, empRepo, &testutil.MockLeaveRequestRepo{}, &testutil.MockAvailabilityRepo{})

	req := dto.CreateAssignmentRequest{
		ShiftID: shift.ID,
		EmployeeID:      employee.ID,
	}
	got, violations, err := svc.CreateAssignment(authCtx(tenantID), tenantID, req)
	require.NoError(t, err)
	assert.Empty(t, violations)
	assert.True(t, assigned)
	assert.Equal(t, shift.ID, got.ShiftInstanceID)
}

func TestScheduleService_CreateAssignment_ConflictExists(t *testing.T) {
	tenantID := uuid.New()
	shift := testutil.NewShiftInstance(tenantID)
	employee := testutil.NewEmployee(tenantID)

	shiftRepo := &testutil.MockShiftInstanceRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.ShiftInstance, error) {
			return shift, nil
		},
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ExistsConflictFn: func(_ context.Context, _, _ uuid.UUID, _ time.Time, _, _ string, _ *uuid.UUID) (bool, error) {
			return true, nil // conflict!
		},
	}
	empRepo := &testutil.MockEmployeeRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Employee, error) {
			return employee, nil
		},
	}
	svc := newScheduleService(shiftRepo, assignRepo, &testutil.MockWeekTemplateRepo{}, empRepo, &testutil.MockLeaveRequestRepo{}, &testutil.MockAvailabilityRepo{})

	_, _, err := svc.CreateAssignment(authCtx(tenantID), tenantID, dto.CreateAssignmentRequest{
		ShiftID: shift.ID,
		EmployeeID:      employee.ID,
	})
	require.Error(t, err)
}

// ─── GetAssignments ───────────────────────────────────────────────────────────

func TestScheduleService_GetAssignments(t *testing.T) {
	tenantID := uuid.New()
	shiftID := uuid.New()
	assignments := []*model.ShiftAssignment{
		testutil.NewShiftAssignment(tenantID, shiftID, uuid.New()),
	}

	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByShiftFn: func(_ context.Context, tID, sID uuid.UUID) ([]*model.ShiftAssignment, error) {
			assert.Equal(t, tenantID, tID)
			assert.Equal(t, shiftID, sID)
			return assignments, nil
		},
	}
	svc := newScheduleService(&testutil.MockShiftInstanceRepo{}, assignRepo, &testutil.MockWeekTemplateRepo{}, &testutil.MockEmployeeRepo{}, &testutil.MockLeaveRequestRepo{}, &testutil.MockAvailabilityRepo{})

	got, err := svc.GetAssignments(context.Background(), tenantID, shiftID)
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

// ─── GetWeekTemplates ─────────────────────────────────────────────────────────

func TestScheduleService_GetWeekTemplates(t *testing.T) {
	tenantID := uuid.New()
	employeeID := uuid.New()
	templates := []*model.WeekTemplate{
		{TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID}, EmployeeID: employeeID, WeekType: "A", DayOfWeek: 1, StartTime: "09:00", EndTime: "17:00"},
	}

	tmplRepo := &testutil.MockWeekTemplateRepo{
		GetByEmployeeFn: func(_ context.Context, tID, eID uuid.UUID) ([]*model.WeekTemplate, error) {
			assert.Equal(t, tenantID, tID)
			assert.Equal(t, employeeID, eID)
			return templates, nil
		},
	}
	svc := newScheduleService(&testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, tmplRepo, &testutil.MockEmployeeRepo{}, &testutil.MockLeaveRequestRepo{}, &testutil.MockAvailabilityRepo{})

	got, err := svc.GetWeekTemplates(context.Background(), tenantID, employeeID)
	require.NoError(t, err)
	assert.Equal(t, employeeID, got.EmployeeID)
	assert.Len(t, got.Entries, 1)
}

// ─── UpsertWeekTemplates ──────────────────────────────────────────────────────

func TestScheduleService_UpsertWeekTemplates(t *testing.T) {
	tenantID := uuid.New()
	employeeID := uuid.New()
	upserted := false

	tmplRepo := &testutil.MockWeekTemplateRepo{
		UpsertForEmployeeFn: func(_ context.Context, tID, eID uuid.UUID, templates []*model.WeekTemplate) error {
			upserted = true
			assert.Equal(t, tenantID, tID)
			assert.Equal(t, employeeID, eID)
			assert.Len(t, templates, 1)
			return nil
		},
	}
	// UpsertWeekTemplates calls empRepo.GetByID to validate the employee exists first
	empRepo := &testutil.MockEmployeeRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Employee, error) {
			return testutil.NewEmployee(tenantID), nil
		},
	}
	svc := newScheduleService(&testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, tmplRepo, empRepo, &testutil.MockLeaveRequestRepo{}, &testutil.MockAvailabilityRepo{})

	req := dto.UpsertWeekTemplateRequest{
		Templates: []dto.WeekTemplateEntry{
			{WeekType: "A", DayOfWeek: 1, StartTime: "09:00", EndTime: "17:00"},
		},
	}
	err := svc.UpsertWeekTemplates(context.Background(), tenantID, employeeID, req)
	require.NoError(t, err)
	assert.True(t, upserted)
}

// ─── ListMyShifts ─────────────────────────────────────────────────────────────

func TestScheduleService_ListMyShifts(t *testing.T) {
	tenantID := uuid.New()
	employeeID := uuid.New()
	shiftID := uuid.New()
	shift := testutil.NewShiftInstance(tenantID)
	shift.ID = shiftID
	assignment := testutil.NewShiftAssignment(tenantID, shiftID, employeeID)

	shiftRepo := &testutil.MockShiftInstanceRepo{
		// ListMyShifts batch-loads shifts via ListByIDs, not ListByDateRange/GetByID
		ListByIDsFn: func(_ context.Context, _ uuid.UUID, ids []uuid.UUID) ([]*model.ShiftInstance, error) {
			return []*model.ShiftInstance{shift}, nil
		},
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByEmployeeFn: func(_ context.Context, _, _ uuid.UUID, _, _ time.Time) ([]*model.ShiftAssignment, error) {
			return []*model.ShiftAssignment{assignment}, nil
		},
	}
	svc := newScheduleService(shiftRepo, assignRepo, &testutil.MockWeekTemplateRepo{}, &testutil.MockEmployeeRepo{}, &testutil.MockLeaveRequestRepo{}, &testutil.MockAvailabilityRepo{})

	from := time.Now()
	to := from.AddDate(0, 0, 7)
	got, err := svc.ListMyShifts(context.Background(), tenantID, employeeID, from, to)
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, shiftID, got[0].ShiftInstanceID)
}

// ─── ProjectABSchedule ────────────────────────────────────────────────────────

// newWeekTemplate builds a WeekTemplate stub for a given tenant/employee.
func newWeekTemplate(tenantID, employeeID uuid.UUID, weekType string, dow int, start, end string) *model.WeekTemplate {
	return &model.WeekTemplate{
		TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
		EmployeeID:   employeeID,
		WeekType:     weekType,
		DayOfWeek:    dow,
		StartTime:    start,
		EndTime:      end,
	}
}

// TestProjectABSchedule_SingleShift verifies that a single-shift day creates exactly
// one shift + one confirmed assignment for the matching employee.
func TestProjectABSchedule_SingleShift(t *testing.T) {
	tenantID  := uuid.New()
	emp       := testutil.NewEmployee(tenantID)

	// Advance to the first Monday on or after StartDate.
	monday := emp.StartDate
	for monday.Weekday() != time.Monday {
		monday = monday.AddDate(0, 0, 1)
	}
	// Pin StartDate to this Monday so that StartDate and monday share the same
	// ISO week number → monday is always "A" under the ISO-week-parity algorithm.
	emp.StartDate = monday

	tmpl := newWeekTemplate(tenantID, emp.ID, "A", int(time.Monday), "09:00", "17:00")

	shiftsCreated    := 0
	assignmentsCreated := 0

	shiftRepo := &testutil.MockShiftInstanceRepo{
		DeleteBySourceTemplateFn: func(_ context.Context, _ uuid.UUID, _, _ time.Time) error { return nil },
		CreateBatchFn: func(_ context.Context, shifts []*model.ShiftInstance) error {
			shiftsCreated = len(shifts)
			return nil
		},
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		CreateBatchFn: func(_ context.Context, assignments []*model.ShiftAssignment) error {
			assignmentsCreated += len(assignments)
			return nil
		},
	}
	tmplRepo := &testutil.MockWeekTemplateRepo{
		GetByEmployeeFn: func(_ context.Context, _, _ uuid.UUID) ([]*model.WeekTemplate, error) {
			return []*model.WeekTemplate{tmpl}, nil
		},
	}
	empRepo := &testutil.MockEmployeeRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, _, _ int) ([]*model.Employee, int64, error) {
			return []*model.Employee{emp}, 1, nil
		},
	}

	svc := newScheduleService(shiftRepo, assignRepo, tmplRepo, empRepo, &testutil.MockLeaveRequestRepo{}, &testutil.MockAvailabilityRepo{})

	req := dto.GenerateScheduleRequest{DateFrom: monday, DateTo: monday}
	count, err := svc.ProjectABSchedule(authCtx(tenantID), tenantID, req)

	require.NoError(t, err)
	assert.Equal(t, 1, count, "expected 1 shift for a single-template Monday")
	assert.Equal(t, 1, shiftsCreated)
	assert.Equal(t, 1, assignmentsCreated)
}

// TestProjectABSchedule_SplitShift verifies that two templates on the same day
// (a split shift, e.g. 09:00–12:00 and 13:00–19:00) generate two separate shifts
// and two confirmed assignments — not just the first one.
func TestProjectABSchedule_SplitShift(t *testing.T) {
	tenantID := uuid.New()
	emp      := testutil.NewEmployee(tenantID)

	monday := emp.StartDate
	for monday.Weekday() != time.Monday {
		monday = monday.AddDate(0, 0, 1)
	}
	// Pin StartDate to this Monday so monday is always "A" (same ISO week).
	emp.StartDate = monday

	// Two entries for the same Monday week-A: morning + afternoon.
	tmplMorning   := newWeekTemplate(tenantID, emp.ID, "A", int(time.Monday), "09:00", "12:00")
	tmplAfternoon := newWeekTemplate(tenantID, emp.ID, "A", int(time.Monday), "13:00", "19:00")

	var capturedShifts []*model.ShiftInstance
	assignmentsCreated := 0

	shiftRepo := &testutil.MockShiftInstanceRepo{
		DeleteBySourceTemplateFn: func(_ context.Context, _ uuid.UUID, _, _ time.Time) error { return nil },
		CreateBatchFn: func(_ context.Context, shifts []*model.ShiftInstance) error {
			capturedShifts = shifts
			return nil
		},
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		CreateBatchFn: func(_ context.Context, assignments []*model.ShiftAssignment) error {
			assignmentsCreated += len(assignments)
			return nil
		},
	}
	tmplRepo := &testutil.MockWeekTemplateRepo{
		GetByEmployeeFn: func(_ context.Context, _, _ uuid.UUID) ([]*model.WeekTemplate, error) {
			return []*model.WeekTemplate{tmplMorning, tmplAfternoon}, nil
		},
	}
	empRepo := &testutil.MockEmployeeRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, _, _ int) ([]*model.Employee, int64, error) {
			return []*model.Employee{emp}, 1, nil
		},
	}

	svc := newScheduleService(shiftRepo, assignRepo, tmplRepo, empRepo, &testutil.MockLeaveRequestRepo{}, &testutil.MockAvailabilityRepo{})

	req := dto.GenerateScheduleRequest{DateFrom: monday, DateTo: monday}
	count, err := svc.ProjectABSchedule(authCtx(tenantID), tenantID, req)

	require.NoError(t, err)
	assert.Equal(t, 2, count, "split shift: expected 2 shifts for one day with two templates")
	require.Len(t, capturedShifts, 2)
	assert.Equal(t, 2, assignmentsCreated)

	// Verify both time ranges are present.
	times := map[string]bool{}
	for _, s := range capturedShifts {
		times[s.StartTime] = true
	}
	assert.True(t, times["09:00"], "morning shift start missing")
	assert.True(t, times["13:00"], "afternoon shift start missing")
}

// TestProjectABSchedule_WrongWeekType verifies that a week-B template does NOT
// generate shifts on a week-A date (and vice-versa).
func TestProjectABSchedule_WrongWeekType(t *testing.T) {
	tenantID := uuid.New()
	emp      := testutil.NewEmployee(tenantID)

	monday := emp.StartDate
	for monday.Weekday() != time.Monday {
		monday = monday.AddDate(0, 0, 1)
	}
	// Pin StartDate to this Monday so monday is "A" (same ISO week parity).
	// A "B" template must therefore not fire on this date.
	emp.StartDate = monday
	tmpl := newWeekTemplate(tenantID, emp.ID, "B", int(time.Monday), "09:00", "17:00")

	shiftsCreated := 0
	shiftRepo := &testutil.MockShiftInstanceRepo{
		DeleteBySourceTemplateFn: func(_ context.Context, _ uuid.UUID, _, _ time.Time) error { return nil },
		CreateBatchFn: func(_ context.Context, shifts []*model.ShiftInstance) error {
			shiftsCreated = len(shifts)
			return nil
		},
	}
	tmplRepo := &testutil.MockWeekTemplateRepo{
		GetByEmployeeFn: func(_ context.Context, _, _ uuid.UUID) ([]*model.WeekTemplate, error) {
			return []*model.WeekTemplate{tmpl}, nil
		},
	}
	empRepo := &testutil.MockEmployeeRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, _, _ int) ([]*model.Employee, int64, error) {
			return []*model.Employee{emp}, 1, nil
		},
	}

	svc := newScheduleService(shiftRepo, &testutil.MockShiftAssignmentRepo{}, tmplRepo, empRepo, &testutil.MockLeaveRequestRepo{}, &testutil.MockAvailabilityRepo{})

	req := dto.GenerateScheduleRequest{DateFrom: monday, DateTo: monday}
	count, err := svc.ProjectABSchedule(authCtx(tenantID), tenantID, req)

	require.NoError(t, err)
	assert.Equal(t, 0, count, "week-B template must not fire on a week-A date")
	assert.Equal(t, 0, shiftsCreated)
}

// TestProjectABSchedule_LeaveSkipped verifies that an employee on approved leave
// that day produces no shifts.
func TestProjectABSchedule_LeaveSkipped(t *testing.T) {
	tenantID := uuid.New()
	emp      := testutil.NewEmployee(tenantID)

	monday := emp.StartDate
	for monday.Weekday() != time.Monday {
		monday = monday.AddDate(0, 0, 1)
	}

	tmpl := newWeekTemplate(tenantID, emp.ID, "A", int(time.Monday), "09:00", "17:00")

	shiftsCreated := 0
	shiftRepo := &testutil.MockShiftInstanceRepo{
		DeleteBySourceTemplateFn: func(_ context.Context, _ uuid.UUID, _, _ time.Time) error { return nil },
		CreateBatchFn: func(_ context.Context, shifts []*model.ShiftInstance) error {
			shiftsCreated = len(shifts)
			return nil
		},
	}
	tmplRepo := &testutil.MockWeekTemplateRepo{
		GetByEmployeeFn: func(_ context.Context, _, _ uuid.UUID) ([]*model.WeekTemplate, error) {
			return []*model.WeekTemplate{tmpl}, nil
		},
	}
	empRepo := &testutil.MockEmployeeRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, _, _ int) ([]*model.Employee, int64, error) {
			return []*model.Employee{emp}, 1, nil
		},
	}
	leaveRepo := &testutil.MockLeaveRequestRepo{
		HasActiveLeaveFn: func(_ context.Context, _, _ uuid.UUID, _, _ time.Time) (bool, error) {
			return true, nil // employee is on leave
		},
	}

	svc := newScheduleService(shiftRepo, &testutil.MockShiftAssignmentRepo{}, tmplRepo, empRepo, leaveRepo, &testutil.MockAvailabilityRepo{})

	req := dto.GenerateScheduleRequest{DateFrom: monday, DateTo: monday}
	count, err := svc.ProjectABSchedule(authCtx(tenantID), tenantID, req)

	require.NoError(t, err)
	assert.Equal(t, 0, count, "employee on leave must generate no shifts")
	assert.Equal(t, 0, shiftsCreated)
}

// helper
func strPtr(s string) *string { return &s }

// ─── Holiday blocking ─────────────────────────────────────────────────────────

// newScheduleServiceWithHolidays builds a ScheduleService with a real
// PublicHolidayService wired in, using mock repos.
func newScheduleServiceWithHolidays(
	shiftRepo *testutil.MockShiftInstanceRepo,
	assignRepo *testutil.MockShiftAssignmentRepo,
	holidayRepo *testutil.MockPublicHolidayRepo,
) *service.ScheduleService {
	svc := newScheduleService(shiftRepo, assignRepo, &testutil.MockWeekTemplateRepo{}, &testutil.MockEmployeeRepo{}, &testutil.MockLeaveRequestRepo{}, &testutil.MockAvailabilityRepo{})
	holidaySvc := service.NewPublicHolidayService(holidayRepo, newTestLogger())
	return svc.WithPublicHolidayService(holidaySvc)
}

func TestScheduleService_CreateShift_BlockedOnHoliday(t *testing.T) {
	easterMonday := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC)

	holidayRepo := &testutil.MockPublicHolidayRepo{
		ListByYearFn: func(_ context.Context, _ int, _ string) ([]*model.PublicHoliday, error) {
			return []*model.PublicHoliday{{Date: easterMonday, Zone: "metropole", Name: "Lundi de Pâques"}}, nil
		},
		GetByDateFn: func(_ context.Context, d time.Time, _ string) (*model.PublicHoliday, error) {
			if d.Day() == 6 && d.Month() == 4 {
				return &model.PublicHoliday{Date: easterMonday, Zone: "metropole", Name: "Lundi de Pâques"}, nil
			}
			return nil, nil
		},
	}

	svc := newScheduleServiceWithHolidays(
		&testutil.MockShiftInstanceRepo{}, &testutil.MockShiftAssignmentRepo{}, holidayRepo,
	)

	_, err := svc.CreateShift(authCtx(uuid.New()), uuid.New(), dto.CreateShiftInstanceRequest{
		Date: easterMonday, StartTime: "09:00", EndTime: "17:00",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Lundi de Pâques", "error must name the holiday")
}

func TestScheduleService_CreateShift_PassesOnNonHoliday(t *testing.T) {
	regularDay := time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC)
	created := false

	holidayRepo := &testutil.MockPublicHolidayRepo{
		ListByYearFn: func(_ context.Context, _ int, _ string) ([]*model.PublicHoliday, error) {
			return []*model.PublicHoliday{{Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Zone: "metropole", Name: "Jour de l'An"}}, nil
		},
		GetByDateFn: func(_ context.Context, _ time.Time, _ string) (*model.PublicHoliday, error) {
			return nil, nil // not a holiday
		},
	}
	shiftRepo := &testutil.MockShiftInstanceRepo{
		CreateFn: func(_ context.Context, _ *model.ShiftInstance) error { created = true; return nil },
	}

	svc := newScheduleServiceWithHolidays(shiftRepo, &testutil.MockShiftAssignmentRepo{}, holidayRepo)

	_, err := svc.CreateShift(authCtx(uuid.New()), uuid.New(), dto.CreateShiftInstanceRequest{
		Date: regularDay, StartTime: "09:00", EndTime: "17:00",
	})
	require.NoError(t, err)
	assert.True(t, created)
}

func TestScheduleService_CreateAssignment_BlockedOnHoliday(t *testing.T) {
	tenantID := uuid.New()
	shift := testutil.NewShiftInstance(tenantID)
	easterMonday := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC)
	shift.Date = easterMonday

	holidayRepo := &testutil.MockPublicHolidayRepo{
		ListByYearFn: func(_ context.Context, _ int, _ string) ([]*model.PublicHoliday, error) {
			return []*model.PublicHoliday{{Date: easterMonday, Zone: "metropole", Name: "Lundi de Pâques"}}, nil
		},
		GetByDateFn: func(_ context.Context, d time.Time, _ string) (*model.PublicHoliday, error) {
			if d.Day() == 6 && d.Month() == 4 {
				return &model.PublicHoliday{Date: easterMonday, Zone: "metropole", Name: "Lundi de Pâques"}, nil
			}
			return nil, nil
		},
	}
	shiftRepo := &testutil.MockShiftInstanceRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.ShiftInstance, error) { return shift, nil },
	}

	svc := newScheduleServiceWithHolidays(shiftRepo, &testutil.MockShiftAssignmentRepo{}, holidayRepo)

	_, _, err := svc.CreateAssignment(authCtx(tenantID), tenantID, dto.CreateAssignmentRequest{
		ShiftID: shift.ID, EmployeeID: uuid.New(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Lundi de Pâques")
}

func TestScheduleService_CreateShift_HolidayAPIFailureIsNonFatal(t *testing.T) {
	// If the holiday repo returns an error, CreateShift must NOT be blocked —
	// the holiday check is non-fatal by design.
	created := false
	holidayRepo := &testutil.MockPublicHolidayRepo{
		ListByYearFn: func(_ context.Context, _ int, _ string) ([]*model.PublicHoliday, error) {
			return nil, errors.New("db unreachable")
		},
	}
	shiftRepo := &testutil.MockShiftInstanceRepo{
		CreateFn: func(_ context.Context, _ *model.ShiftInstance) error { created = true; return nil },
	}

	svc := newScheduleServiceWithHolidays(shiftRepo, &testutil.MockShiftAssignmentRepo{}, holidayRepo)

	_, err := svc.CreateShift(authCtx(uuid.New()), uuid.New(), dto.CreateShiftInstanceRequest{
		Date: time.Now(), StartTime: "09:00", EndTime: "17:00",
	})
	require.NoError(t, err, "holiday check failure must be non-fatal")
	assert.True(t, created)
}
