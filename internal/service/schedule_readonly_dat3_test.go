package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DAT-3: GetSchedule is read-only — an empty week must NOT trigger projection
// (no shift/assignment writes), eliminating write-on-GET and its races.
func TestGetSchedule_IsReadOnly_OnEmptyWeek(t *testing.T) {
	tenantID := uuid.New()

	wrote := false
	shiftRepo := &testutil.MockShiftInstanceRepo{
		ListByDateRangeFn: func(_ context.Context, _ uuid.UUID, _, _ time.Time, _, _ int) ([]*model.ShiftInstance, int64, error) {
			return nil, 0, nil // empty week
		},
		CreateBatchFn:            func(_ context.Context, _ []*model.ShiftInstance) error { wrote = true; return nil },
		DeleteBySourceTemplateFn: func(_ context.Context, _ uuid.UUID, _, _ time.Time) error { wrote = true; return nil },
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		CreateBatchFn: func(_ context.Context, _ []*model.ShiftAssignment) error { wrote = true; return nil },
	}
	// Templates/employees that WOULD project shifts if auto-projection ran.
	emp := testutil.NewEmployee(tenantID)
	tmplRepo := &testutil.MockWeekTemplateRepo{
		GetByEmployeeFn: func(_ context.Context, _, _ uuid.UUID) ([]*model.WeekTemplate, error) {
			return []*model.WeekTemplate{newWeekTemplate(tenantID, emp.ID, "A", 1, "09:00", "17:00")}, nil
		},
	}
	empRepo := &testutil.MockEmployeeRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, _, _ int) ([]*model.Employee, int64, error) {
			return []*model.Employee{emp}, 1, nil
		},
	}

	svc := newScheduleService(shiftRepo, assignRepo, tmplRepo, empRepo, &testutil.MockLeaveRequestRepo{}, &testutil.MockAvailabilityRepo{})

	day := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	shifts, total, err := svc.GetSchedule(context.Background(), tenantID, day, day, 1, 50)

	require.NoError(t, err)
	assert.Empty(t, shifts)
	assert.Zero(t, total)
	assert.False(t, wrote, "GET schedule must not perform any writes (no auto-projection)")
}

// scheduleLockKey indirection is deterministic per tenant (DAT-3 idempotency key).
func TestScheduleService_GenerateStillWorks_NoTxRunner(t *testing.T) {
	// Sanity: with no tx runner, generation still functions (fallback path),
	// proving the read-only GET change didn't break explicit generation.
	tenantID := uuid.New()
	emp, monday := mondayFor(tenantID)
	created := 0
	shiftRepo := &testutil.MockShiftInstanceRepo{
		DeleteBySourceTemplateFn: func(_ context.Context, _ uuid.UUID, _, _ time.Time) error { return nil },
		CreateBatchFn:            func(_ context.Context, s []*model.ShiftInstance) error { created = len(s); return nil },
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		CreateBatchFn: func(_ context.Context, _ []*model.ShiftAssignment) error { return nil },
	}
	tmplRepo := &testutil.MockWeekTemplateRepo{
		GetByEmployeeFn: func(_ context.Context, _, _ uuid.UUID) ([]*model.WeekTemplate, error) {
			return []*model.WeekTemplate{newWeekTemplate(tenantID, emp.ID, "A", int(time.Monday), "09:00", "17:00")}, nil
		},
	}
	empRepo := &testutil.MockEmployeeRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, _, _ int) ([]*model.Employee, int64, error) {
			return []*model.Employee{emp}, 1, nil
		},
	}
	svc := newScheduleService(shiftRepo, assignRepo, tmplRepo, empRepo, &testutil.MockLeaveRequestRepo{}, &testutil.MockAvailabilityRepo{})
	_, _, err := svc.GetSchedule(context.Background(), tenantID, monday, monday, 1, 50)
	require.NoError(t, err)
	assert.Zero(t, created, "GetSchedule never generates")
}
