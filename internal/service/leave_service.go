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

// LeaveService handles leave requests.
type LeaveService struct {
	repo       repo.LeaveRequestRepository
	empRepo    repo.EmployeeRepository
	assignRepo repo.ShiftAssignmentRepository
	shiftRepo  repo.ShiftInstanceRepository
	emitter    *event.Emitter
	logger     *logrus.Entry
}

// NewLeaveService creates a new LeaveService.
func NewLeaveService(repo repo.LeaveRequestRepository, empRepo repo.EmployeeRepository, assignRepo repo.ShiftAssignmentRepository, shiftRepo repo.ShiftInstanceRepository, emitter *event.Emitter, logger *logrus.Entry) *LeaveService {
	return &LeaveService{
		repo:       repo,
		empRepo:    empRepo,
		assignRepo: assignRepo,
		shiftRepo:  shiftRepo,
		emitter:    emitter,
		logger:     logger,
	}
}

// CreateLeaveRequest creates a new leave request.
func (s *LeaveService) CreateLeaveRequest(ctx context.Context, tenantID, employeeID uuid.UUID, req dto.CreateLeaveRequest) (*model.LeaveRequest, error) {
	logger := ctxutil.GetLogger(ctx)

	// Verify employee exists
	emp, err := s.empRepo.GetByID(ctx, tenantID, employeeID)
	if err != nil {
		logger.WithError(err).Error("failed to get employee")
		return nil, apierror.Internal("failed to get employee").WithKey("errors.unknown")
	}
	if emp == nil {
		return nil, apierror.NotFound("employee", employeeID.String()).WithKey("errors.unknown")
	}

	// Parse YYYY-MM-DD string dates from the DTO.
	startDate, endDate, err := req.ParsedDates()
	if err != nil {
		return nil, apierror.BadRequest(err.Error()).WithKey("errors.invalidInput")
	}

	if startDate.After(endDate) {
		return nil, apierror.BadRequest("start_date must be before or equal to end_date").WithKey("errors.invalidInput")
	}

	// Validate leave type
	if req.Type != model.LeaveTypeVacation && req.Type != model.LeaveTypeSick && req.Type != model.LeaveTypeOther {
		return nil, apierror.BadRequest("invalid leave type").WithKey("errors.invalidInput")
	}

	// Check for overlapping pending/approved leave
	hasOverlap, err := s.repo.HasActiveLeave(ctx, tenantID, employeeID, startDate, endDate)
	if err != nil {
		logger.WithError(err).Error("failed to check for overlapping leave")
		return nil, apierror.Internal("failed to check overlapping leave").WithKey("errors.unknown")
	}
	if hasOverlap {
		return nil, apierror.Conflict("employee already has leave during this period").WithKey("errors.conflict")
	}

	leave := &model.LeaveRequest{
		TenantScoped: model.TenantScoped{
			ID:        uuid.New(),
			TenantID:  tenantID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		EmployeeID: employeeID,
		StartDate:  startDate,
		EndDate:    endDate,
		Type:       req.Type,
		Status:     model.LeaveStatusPending,
		Reason:     derefString(req.Reason), // *string → string
	}

	if err := s.repo.Create(ctx, leave); err != nil {
		logger.WithError(err).Error("failed to create leave request")
		return nil, apierror.Internal("failed to create leave request").WithKey("errors.unknown")
	}

	// Publish event
	s.emitter.Publish(event.Event{
		Type:     event.TypeLeaveCreated,
		TenantID: tenantID,
		UserID:   ctxutil.GetUserID(ctx),
		Payload:  leave,
	})

	return leave, nil
}

// ReviewLeaveRequest approves or rejects a leave request.
func (s *LeaveService) ReviewLeaveRequest(ctx context.Context, tenantID, id uuid.UUID, req dto.ReviewLeaveRequest) (*model.LeaveRequest, error) {
	logger := ctxutil.GetLogger(ctx)

	leave, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		logger.WithError(err).Error("failed to get leave request")
		return nil, apierror.Internal("failed to get leave request").WithKey("errors.unknown")
	}
	if leave == nil {
		return nil, apierror.NotFound("leave request", id.String()).WithKey("errors.unknown")
	}

	// Validate status
	if req.Status != model.LeaveStatusApproved && req.Status != model.LeaveStatusRejected {
		return nil, apierror.BadRequest("status must be 'approved' or 'rejected'").WithKey("errors.invalidInput")
	}

	now := time.Now()
	reviewedBy := ctxutil.GetUserID(ctx)

	leave.Status = req.Status
	leave.ReviewedBy = &reviewedBy
	leave.ReviewedAt = &now
	leave.UpdatedAt = now

	if err := s.repo.Update(ctx, leave); err != nil {
		logger.WithError(err).Error("failed to update leave request")
		return nil, optimisticErr(err, apierror.Internal("failed to update leave request").WithKey("errors.unknown"))
	}

	// If approved, cancel assignments in the leave period
	if req.Status == model.LeaveStatusApproved {
		if err := s.cancelAssignmentsInLeave(ctx, tenantID, leave); err != nil {
			logger.WithError(err).Error("failed to cancel assignments during leave")
			// Log but don't fail the operation
		}
	}

	// Publish event
	s.emitter.Publish(event.Event{
		Type:     event.TypeLeaveUpdated,
		TenantID: tenantID,
		UserID:   ctxutil.GetUserID(ctx),
		Payload:  leave,
	})

	return leave, nil
}

// GetLeaveImpact returns a preview of what will be cancelled if a pending leave is approved.
// The manager calls this before clicking Approve to understand scheduling consequences.
func (s *LeaveService) GetLeaveImpact(ctx context.Context, tenantID, id uuid.UUID) (*dto.LeaveImpactResponse, error) {
	logger := ctxutil.GetLogger(ctx)

	leave, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		logger.WithError(err).Error("failed to get leave request")
		return nil, apierror.Internal("failed to get leave request").WithKey("errors.unknown")
	}
	if leave == nil {
		return nil, apierror.NotFound("leave request", id.String()).WithKey("errors.unknown")
	}

	// Fetch all non-cancelled assignments for this employee during the leave period.
	assignments, err := s.assignRepo.ListByEmployee(ctx, tenantID, leave.EmployeeID, leave.StartDate, leave.EndDate)
	if err != nil {
		logger.WithError(err).Error("failed to list employee assignments")
		return nil, apierror.Internal("failed to compute leave impact").WithKey("errors.unknown")
	}

	var (
		affectedShifts     []dto.LeaveImpactShift
		totalHoursLost     float64
		uncoveredShifts    int
	)

	for _, a := range assignments {
		if a.Status == model.AssignmentStatusCancelled {
			continue // already cancelled, not part of the impact
		}

		// Count how many OTHER active assignments exist on the same shift.
		allOnShift, err := s.assignRepo.ListByShift(ctx, tenantID, a.ShiftInstanceID)
		if err != nil {
			logger.WithError(err).Warn("could not count assignments for shift — skipping")
			continue
		}
		otherActive := 0
		for _, other := range allOnShift {
			if other.ID != a.ID && other.Status != model.AssignmentStatusCancelled {
				otherActive++
			}
		}

		willNeedCover := otherActive == 0

		// Hours lost for this assignment.
		sh, sm := parseHHMM(a.ShiftStartTime)
		eh, em := parseHHMM(a.ShiftEndTime)
		hours := float64((eh*60+em)-(sh*60+sm)) / 60.0

		// Load shift for role and date string.
		shift, err := s.shiftRepo.GetByID(ctx, tenantID, a.ShiftInstanceID)
		if err != nil || shift == nil {
			logger.WithError(err).Warn("could not load shift — skipping")
			continue
		}

		affectedShifts = append(affectedShifts, dto.LeaveImpactShift{
			Date:          shift.Date.Format("2006-01-02"),
			StartTime:     a.ShiftStartTime,
			EndTime:       a.ShiftEndTime,
			Role:          shift.Role,
			OtherAssigned: otherActive,
			WillNeedCover: willNeedCover,
		})

		totalHoursLost += hours
		if willNeedCover {
			uncoveredShifts++
		}
	}

	if affectedShifts == nil {
		affectedShifts = []dto.LeaveImpactShift{} // never return null
	}

	return &dto.LeaveImpactResponse{
		AffectedShifts:     affectedShifts,
		TotalCancellations: len(affectedShifts),
		TotalHoursLost:     totalHoursLost,
		UncoveredShifts:    uncoveredShifts,
	}, nil
}

// DeleteLeaveRequest removes a pending leave request. Only the employee who owns
// the request (or a manager) may cancel it, and only while it is still pending.
func (s *LeaveService) DeleteLeaveRequest(ctx context.Context, tenantID, id uuid.UUID) error {
	logger := ctxutil.GetLogger(ctx)

	leave, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		logger.WithError(err).Error("failed to get leave request")
		return apierror.Internal("failed to get leave request").WithKey("errors.unknown")
	}
	if leave == nil {
		return apierror.NotFound("leave request", id.String()).WithKey("errors.unknown")
	}
	if leave.Status != model.LeaveStatusPending {
		return apierror.BadRequest("only pending leave requests can be cancelled").WithKey("errors.invalidInput")
	}

	if err := s.repo.Delete(ctx, tenantID, id); err != nil {
		logger.WithError(err).Error("failed to delete leave request")
		return apierror.Internal("failed to delete leave request").WithKey("errors.unknown")
	}

	// Publish event
	s.emitter.Publish(event.Event{
		Type:     event.TypeLeaveDeleted,
		TenantID: tenantID,
		UserID:   ctxutil.GetUserID(ctx),
		Payload:  map[string]interface{}{"id": id, "employee_id": leave.EmployeeID},
	})

	return nil
}

// GetLeaveRequest retrieves a leave request.
func (s *LeaveService) GetLeaveRequest(ctx context.Context, tenantID, id uuid.UUID) (*model.LeaveRequest, error) {
	logger := ctxutil.GetLogger(ctx)

	leave, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		logger.WithError(err).Error("failed to get leave request")
		return nil, apierror.Internal("failed to get leave request").WithKey("errors.unknown")
	}
	if leave == nil {
		return nil, apierror.NotFound("leave request", id.String()).WithKey("errors.unknown")
	}

	return leave, nil
}

// ListByEmployee retrieves leave requests for an employee.
func (s *LeaveService) ListByEmployee(ctx context.Context, tenantID, employeeID uuid.UUID, page, pageSize int) ([]*model.LeaveRequest, int64, error) {
	logger := ctxutil.GetLogger(ctx)

	leaves, total, err := s.repo.ListByEmployee(ctx, tenantID, employeeID, page, pageSize)
	if err != nil {
		logger.WithError(err).Error("failed to list leave requests")
		return nil, 0, apierror.Internal("failed to list leave requests").WithKey("errors.unknown")
	}

	return leaves, total, nil
}

// ListByStore retrieves leave requests for a store.
func (s *LeaveService) ListByStore(ctx context.Context, tenantID uuid.UUID, status string, page, pageSize int) ([]*model.LeaveRequest, int64, error) {
	logger := ctxutil.GetLogger(ctx)

	leaves, total, err := s.repo.ListByStore(ctx, tenantID, status, page, pageSize)
	if err != nil {
		logger.WithError(err).Error("failed to list leave requests")
		return nil, 0, apierror.Internal("failed to list leave requests").WithKey("errors.unknown")
	}

	return leaves, total, nil
}

// cancelAssignmentsInLeave cancels all shift assignments for the employee during the leave period
// and marks the affected shifts as NeedsCover=true so the planner can see open gaps.
func (s *LeaveService) cancelAssignmentsInLeave(ctx context.Context, tenantID uuid.UUID, leave *model.LeaveRequest) error {
	logger := ctxutil.GetLogger(ctx)

	// Get all assignments for the employee during the leave period
	assignments, err := s.assignRepo.ListByEmployee(ctx, tenantID, leave.EmployeeID, leave.StartDate, leave.EndDate)
	if err != nil {
		return err
	}

	// Cancel each assignment and flag the parent shift as needing cover.
	// We cancel both "confirmed" and "pending" assignments — any active assignment
	// should be removed when the employee is on approved leave. Already-cancelled
	// assignments are skipped to avoid redundant writes.
	for _, assignment := range assignments {
		if assignment.Status != model.AssignmentStatusCancelled {
			assignment.Status = model.AssignmentStatusCancelled
			assignment.UpdatedAt = time.Now()

			if err := s.assignRepo.Update(ctx, assignment); err != nil {
				logger.WithError(err).Error("failed to cancel assignment during leave")
				// Continue to next assignment
				continue
			}

			// Publish event for each cancelled assignment
			s.emitter.Publish(event.Event{
				Type:     event.TypeAssignmentUpdated,
				TenantID: tenantID,
				UserID:   ctxutil.GetUserID(ctx),
				Payload:  assignment,
			})

			// Mark the parent shift as needing cover so the planner can identify the gap.
			shift, err := s.shiftRepo.GetByID(ctx, tenantID, assignment.ShiftInstanceID)
			if err != nil || shift == nil {
				logger.WithError(err).Warn("could not load shift to set NeedsCover — skipping")
				continue
			}
			if !shift.NeedsCover {
				shift.NeedsCover = true
				shift.UpdatedAt = time.Now()
				if err := s.shiftRepo.Update(ctx, shift); err != nil {
					logger.WithError(err).Warn("failed to set NeedsCover on shift")
				}
			}
		}
	}

	return nil
}
