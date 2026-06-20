package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/event"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/repo"
	"github.com/sirupsen/logrus"
)

// assignmentService owns shift-assignment operations (create with rule/holiday/
// conflict checks, queries, delete, week reset). Extracted from ScheduleService
// (ARC-1); ScheduleService builds one per call from its current deps and delegates.
type assignmentService struct {
	shiftRepo  repo.ShiftInstanceRepository
	assignRepo repo.ShiftAssignmentRepository
	empRepo    repo.EmployeeRepository
	leaveRepo  repo.LeaveRequestRepository
	ruleEngine *RuleEngine
	holidaySvc *PublicHolidayService
	emitter    *event.Emitter
	logger     *logrus.Entry
}

// Create assigns an employee to a shift and runs configured scheduling rules.
// Returned violations (WARNING/INFO) are advisory; BLOCKING violations error.
func (a *assignmentService) Create(ctx context.Context, tenantID uuid.UUID, req dto.CreateAssignmentRequest) (*model.ShiftAssignment, []model.RuleViolation, error) {
	logger := ctxutil.GetLogger(ctx)

	shiftID := req.ShiftID
	empID := req.EmployeeID

	// Check shift exists in tenant
	shift, err := a.shiftRepo.GetByID(ctx, tenantID, shiftID)
	if err != nil {
		logger.WithError(err).Error("failed to get shift")
		return nil, nil, apierror.Internal("failed to get shift").WithKey("errors.unknown")
	}
	if shift == nil {
		return nil, nil, apierror.NotFound("shift", shiftID.String()).WithKey("errors.unknown")
	}

	// Block assignment on French public holidays.
	if err := blockIfHoliday(ctx, a.holidaySvc, a.logger, shift.Date); err != nil {
		return nil, nil, err
	}

	// Check employee exists in tenant
	emp, err := a.empRepo.GetByID(ctx, tenantID, empID)
	if err != nil {
		logger.WithError(err).Error("failed to get employee")
		return nil, nil, apierror.Internal("failed to get employee").WithKey("errors.unknown")
	}
	if emp == nil {
		return nil, nil, apierror.NotFound("employee", empID.String()).WithKey("errors.unknown")
	}

	// Check for assignment conflicts
	hasConflict, err := a.assignRepo.ExistsConflict(ctx, tenantID, empID, shift.Date, shift.StartTime, shift.EndTime, nil)
	if err != nil {
		logger.WithError(err).Error("failed to check assignment conflicts")
		return nil, nil, apierror.Internal("failed to check conflicts").WithKey("errors.unknown")
	}
	if hasConflict {
		return nil, nil, apierror.Conflict("employee already has an assignment during this time").WithKey("errors.conflict")
	}

	// Check employee doesn't have active leave on shift date
	hasLeave, err := a.leaveRepo.HasActiveLeave(ctx, tenantID, empID, shift.Date, shift.Date)
	if err != nil {
		logger.WithError(err).Error("failed to check active leave")
		return nil, nil, apierror.Internal("failed to check leave").WithKey("errors.unknown")
	}
	if hasLeave {
		return nil, nil, apierror.Conflict("employee has active leave during this time").WithKey("errors.conflict")
	}

	// ── Rule Engine evaluation ──────────────────────────────────────────────
	var violations []model.RuleViolation
	if a.ruleEngine != nil {
		violations, err = a.ruleEngine.EvaluateAssignment(ctx, tenantID, shift, emp)
		if err != nil {
			logger.WithError(err).Warn("rule engine evaluation failed, proceeding without checks")
		} else {
			// Block on BLOCKING severity violations.
			for _, v := range violations {
				if v.Severity == model.RuleSeverityBlocking {
					key := v.Key
					if key == "" {
						key = "errors.ruleViolation"
					}
					return nil, violations, apierror.Conflict(v.Message).WithKey(key)
				}
			}
		}
	}

	// Create assignment — denormalize shift time fields so the rule engine can
	// calculate real hours without a JOIN on future evaluations.
	assignment := &model.ShiftAssignment{
		TenantScoped: model.TenantScoped{
			ID:        uuid.New(),
			TenantID:  tenantID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		ShiftInstanceID: shiftID,
		EmployeeID:      empID,
		Status:          model.AssignmentStatusConfirmed,
		AssignedBy:      ctxutil.GetUserID(ctx),
		AssignedAt:      time.Now(),
		ShiftDate:       shift.Date,
		ShiftStartTime:  shift.StartTime,
		ShiftEndTime:    shift.EndTime,
	}

	if err := a.assignRepo.Create(ctx, assignment); err != nil {
		logger.WithError(err).Error("failed to create assignment")
		return nil, nil, apierror.Internal("failed to create assignment").WithKey("errors.unknown")
	}

	a.emitter.Publish(event.Event{
		Type:     event.TypeAssignmentCreated,
		TenantID: tenantID,
		UserID:   ctxutil.GetUserID(ctx),
		Payload:  assignment,
	})

	return assignment, violations, nil
}

// ListByDateRange retrieves all assignments for a tenant within a date range.
func (a *assignmentService) ListByDateRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]*model.ShiftAssignment, error) {
	logger := ctxutil.GetLogger(ctx)
	assignments, err := a.assignRepo.ListByDateRange(ctx, tenantID, from, to)
	if err != nil {
		logger.WithError(err).Error("failed to list assignments by date range")
		return nil, apierror.Internal("failed to list assignments").WithKey("errors.unknown")
	}
	return assignments, nil
}

// ListByShift retrieves all assignments for a shift.
func (a *assignmentService) ListByShift(ctx context.Context, tenantID, shiftID uuid.UUID) ([]*model.ShiftAssignment, error) {
	logger := ctxutil.GetLogger(ctx)
	assignments, err := a.assignRepo.ListByShift(ctx, tenantID, shiftID)
	if err != nil {
		logger.WithError(err).Error("failed to get assignments")
		return nil, apierror.Internal("failed to get assignments").WithKey("errors.unknown")
	}
	return assignments, nil
}

// Delete cancels and removes a shift assignment.
func (a *assignmentService) Delete(ctx context.Context, tenantID, assignmentID uuid.UUID) error {
	logger := ctxutil.GetLogger(ctx)

	assignment, err := a.assignRepo.GetByID(ctx, tenantID, assignmentID)
	if err != nil {
		logger.WithError(err).Error("failed to get assignment")
		return apierror.Internal("failed to get assignment").WithKey("errors.unknown")
	}
	if assignment == nil {
		return apierror.NotFound("assignment", assignmentID.String()).WithKey("errors.unknown")
	}

	if err := a.assignRepo.Delete(ctx, tenantID, assignmentID); err != nil {
		logger.WithError(err).Error("failed to delete assignment")
		return apierror.Internal("failed to delete assignment").WithKey("errors.unknown")
	}

	a.emitter.Publish(event.Event{
		Type:     event.TypeAssignmentUpdated,
		TenantID: tenantID,
		UserID:   ctxutil.GetUserID(ctx),
		Payload:  map[string]interface{}{"id": assignmentID, "action": "deleted"},
	})

	return nil
}

// ResetWeek deletes all assignments for the week starting on weekStart (Monday),
// inclusive of weekStart..weekStart+6. Returns the number of deleted rows.
func (a *assignmentService) ResetWeek(ctx context.Context, tenantID uuid.UUID, weekStart time.Time) (int64, error) {
	logger := ctxutil.GetLogger(ctx)

	from := weekStart.UTC().Truncate(24 * time.Hour)
	to := from.AddDate(0, 0, 6)

	deleted, err := a.assignRepo.DeleteByDateRange(ctx, tenantID, from, to)
	if err != nil {
		logger.WithError(err).Error("failed to reset week assignments")
		return 0, apierror.Internal("failed to reset week assignments").WithKey("errors.unknown")
	}

	a.emitter.Publish(event.Event{
		Type:     event.TypeAssignmentUpdated,
		TenantID: tenantID,
		UserID:   ctxutil.GetUserID(ctx),
		Payload:  map[string]interface{}{"action": "reset_week", "week_start": from.Format("2006-01-02"), "deleted": deleted},
	})

	return deleted, nil
}
