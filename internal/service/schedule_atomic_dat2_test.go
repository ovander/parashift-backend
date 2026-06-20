package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/repo"
	"github.com/ovander/parashift/internal/service"
	"github.com/ovander/parashift/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTxRunner routes tx-bound repo access to the supplied mocks and counts how
// many transactions were opened. fn's error propagates (simulating rollback).
func fakeTxRunner(calls *int, shift *testutil.MockShiftInstanceRepo, assign *testutil.MockShiftAssignmentRepo) service.TxRunner {
	return func(ctx context.Context, fn func(tx *repo.RepoBundle) error) error {
		*calls++
		return fn(&repo.RepoBundle{ShiftInstance: shift, ShiftAssignment: assign})
	}
}

// mondayFor returns emp pinned to a Monday StartDate plus that Monday.
func mondayFor(tenantID uuid.UUID) (*model.Employee, time.Time) {
	emp := testutil.NewEmployee(tenantID)
	monday := emp.StartDate
	for monday.Weekday() != time.Monday {
		monday = monday.AddDate(0, 0, 1)
	}
	emp.StartDate = monday
	return emp, monday
}

// DAT-2: when assignment creation fails, the compensating delete targets ONLY the
// created shift IDs (DeleteByIDs) — never the range-wide DeleteByDateRange.
func TestProjectAB_Atomic_ScopedCompensatingDelete(t *testing.T) {
	tenantID := uuid.New()
	emp, monday := mondayFor(tenantID)
	tmpl := newWeekTemplate(tenantID, emp.ID, "A", int(time.Monday), "09:00", "17:00")

	var createdShiftIDs []uuid.UUID
	var deleteByIDsArg []uuid.UUID
	rangeDeleteCalled := false

	shiftRepo := &testutil.MockShiftInstanceRepo{
		DeleteBySourceTemplateFn: func(_ context.Context, _ uuid.UUID, _, _ time.Time) error { return nil },
		CreateBatchFn: func(_ context.Context, shifts []*model.ShiftInstance) error {
			for _, s := range shifts {
				createdShiftIDs = append(createdShiftIDs, s.ID)
			}
			return nil
		},
		DeleteByIDsFn:       func(_ context.Context, _ uuid.UUID, ids []uuid.UUID) error { deleteByIDsArg = ids; return nil },
		DeleteByDateRangeFn: func(_ context.Context, _ uuid.UUID, _, _ time.Time) error { rangeDeleteCalled = true; return nil },
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		CreateBatchFn: func(_ context.Context, _ []*model.ShiftAssignment) error { return errors.New("assign boom") },
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

	calls := 0
	svc := newScheduleService(shiftRepo, assignRepo, tmplRepo, empRepo, &testutil.MockLeaveRequestRepo{}, &testutil.MockAvailabilityRepo{}).
		WithTxRunner(fakeTxRunner(&calls, shiftRepo, assignRepo))

	_, err := svc.ProjectABSchedule(authCtx(tenantID), tenantID, dto.GenerateScheduleRequest{DateFrom: monday, DateTo: monday})

	require.Error(t, err)
	assert.Equal(t, 1, calls, "generation must run inside a single transaction")
	assert.False(t, rangeDeleteCalled, "must NOT range-delete (the old over-deleting compensation)")
	assert.ElementsMatch(t, createdShiftIDs, deleteByIDsArg, "compensating delete must target only the created shift IDs")
}

// DAT-2: successful generation runs in one transaction with no compensating delete.
func TestProjectAB_Atomic_Success(t *testing.T) {
	tenantID := uuid.New()
	emp, monday := mondayFor(tenantID)
	tmpl := newWeekTemplate(tenantID, emp.ID, "A", int(time.Monday), "09:00", "17:00")

	compensated := false
	shiftRepo := &testutil.MockShiftInstanceRepo{
		DeleteBySourceTemplateFn: func(_ context.Context, _ uuid.UUID, _, _ time.Time) error { return nil },
		CreateBatchFn:            func(_ context.Context, _ []*model.ShiftInstance) error { return nil },
		DeleteByIDsFn:            func(_ context.Context, _ uuid.UUID, _ []uuid.UUID) error { compensated = true; return nil },
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		CreateBatchFn: func(_ context.Context, _ []*model.ShiftAssignment) error { return nil },
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
	calls := 0
	svc := newScheduleService(shiftRepo, assignRepo, tmplRepo, empRepo, &testutil.MockLeaveRequestRepo{}, &testutil.MockAvailabilityRepo{}).
		WithTxRunner(fakeTxRunner(&calls, shiftRepo, assignRepo))

	count, err := svc.ProjectABSchedule(authCtx(tenantID), tenantID, dto.GenerateScheduleRequest{DateFrom: monday, DateTo: monday})
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Equal(t, 1, calls, "exactly one transaction")
	assert.False(t, compensated, "no compensating delete on success")
}

// DAT-2: RegenerateWeek runs delete + delete + reproject inside ONE transaction,
// so a projection failure rolls the deletes back (week left intact).
func TestRegenerateWeek_SingleTransaction_RollsBackOnProjectionFailure(t *testing.T) {
	tenantID := uuid.New()
	emp, monday := mondayFor(tenantID)
	tmpl := newWeekTemplate(tenantID, emp.ID, "A", int(time.Monday), "09:00", "17:00")

	assignDeleted, shiftDeleted := false, false
	shiftRepo := &testutil.MockShiftInstanceRepo{
		DeleteByDateRangeFn:      func(_ context.Context, _ uuid.UUID, _, _ time.Time) error { shiftDeleted = true; return nil },
		DeleteBySourceTemplateFn: func(_ context.Context, _ uuid.UUID, _, _ time.Time) error { return nil },
		CreateBatchFn:            func(_ context.Context, _ []*model.ShiftInstance) error { return errors.New("reproject boom") },
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		DeleteByDateRangeFn: func(_ context.Context, _ uuid.UUID, _, _ time.Time) (int64, error) { assignDeleted = true; return 3, nil },
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
	calls := 0
	svc := newScheduleService(shiftRepo, assignRepo, tmplRepo, empRepo, &testutil.MockLeaveRequestRepo{}, &testutil.MockAvailabilityRepo{}).
		WithTxRunner(fakeTxRunner(&calls, shiftRepo, assignRepo))

	_, err := svc.RegenerateWeek(authCtx(tenantID), tenantID, monday)

	require.Error(t, err, "projection failure must propagate so the tx rolls back")
	assert.Equal(t, 1, calls, "delete + delete + reproject must run in a SINGLE transaction")
	assert.True(t, assignDeleted && shiftDeleted, "deletes run inside the same transaction as the reprojection")
}
