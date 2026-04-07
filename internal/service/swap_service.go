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
	"gorm.io/gorm"
)

// SwapService handles shift swap requests between employees.
type SwapService struct {
	repo        repo.SwapRequestRepository
	shiftRepo   repo.ShiftInstanceRepository
	assignRepo  repo.ShiftAssignmentRepository
	empRepo     repo.EmployeeRepository
	db          *gorm.DB // used for transactional performSwap
	ruleEngine  *RuleEngine // optional — nil disables rule evaluation
	emitter     *event.Emitter
	logger      *logrus.Entry
}

// NewSwapService creates a new SwapService.
func NewSwapService(repo repo.SwapRequestRepository, shiftRepo repo.ShiftInstanceRepository, assignRepo repo.ShiftAssignmentRepository, empRepo repo.EmployeeRepository, db *gorm.DB, emitter *event.Emitter, logger *logrus.Entry) *SwapService {
	return &SwapService{
		repo:       repo,
		shiftRepo:  shiftRepo,
		assignRepo: assignRepo,
		empRepo:    empRepo,
		db:         db,
		emitter:    emitter,
		logger:     logger,
	}
}

// WithRuleEngine attaches a RuleEngine to the service (called during wiring in service_bundle).
func (s *SwapService) WithRuleEngine(re *RuleEngine) *SwapService {
	s.ruleEngine = re
	return s
}

// CreateSwapRequest creates a new swap request.
func (s *SwapService) CreateSwapRequest(ctx context.Context, tenantID, requesterID uuid.UUID, req dto.CreateSwapRequest) (*model.SwapRequest, error) {
	logger := ctxutil.GetLogger(ctx)

	// req.ShiftInstanceID is already uuid.UUID from DTO
	shiftID := req.ShiftInstanceID

	// Validate shift exists in tenant
	shift, err := s.shiftRepo.GetByID(ctx, tenantID, shiftID)
	if err != nil {
		logger.WithError(err).Error("failed to get shift")
		return nil, apierror.Internal("failed to get shift")
	}
	if shift == nil {
		return nil, apierror.NotFound("shift", shiftID.String())
	}

	// Check requester has an assignment for this shift
	assignments, err := s.assignRepo.ListByShift(ctx, tenantID, shiftID)
	if err != nil {
		logger.WithError(err).Error("failed to get assignments")
		return nil, apierror.Internal("failed to get assignments")
	}

	var requesterAssignment *model.ShiftAssignment
	for _, a := range assignments {
		if a.EmployeeID == requesterID && a.Status == model.AssignmentStatusConfirmed {
			requesterAssignment = a
			break
		}
	}

	if requesterAssignment == nil {
		return nil, apierror.Conflict("requester does not have an active assignment for this shift")
	}

	// req.TargetEmployeeID and req.TargetShiftID are already *uuid.UUID from DTO — use directly
	swapRequest := &model.SwapRequest{
		TenantScoped: model.TenantScoped{
			ID:        uuid.New(),
			TenantID:  tenantID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		RequesterID:      requesterID,
		ShiftInstanceID:  shiftID,
		TargetEmployeeID: req.TargetEmployeeID, // *uuid.UUID
		TargetShiftID:    req.TargetShiftID,    // *uuid.UUID
		Status:           model.SwapStatusPending,
		Note:             derefString(req.Note), // *string → string
	}

	if err := s.repo.Create(ctx, swapRequest); err != nil {
		logger.WithError(err).Error("failed to create swap request")
		return nil, apierror.Internal("failed to create swap request")
	}

	// Publish event
	s.emitter.Publish(event.Event{
		Type:     event.TypeSwapCreated,
		TenantID: tenantID,
		UserID:   ctxutil.GetUserID(ctx),
		Payload:  swapRequest,
	})

	return swapRequest, nil
}

// ReviewSwapRequest reviews a swap request (accepts or rejects).
func (s *SwapService) ReviewSwapRequest(ctx context.Context, tenantID, id uuid.UUID, req dto.ReviewSwapRequest) (*model.SwapRequest, error) {
	logger := ctxutil.GetLogger(ctx)

	swapRequest, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		logger.WithError(err).Error("failed to get swap request")
		return nil, apierror.Internal("failed to get swap request")
	}
	if swapRequest == nil {
		return nil, apierror.NotFound("swap request", id.String())
	}

	// Validate status
	if req.Status != model.SwapStatusAccepted && req.Status != model.SwapStatusRejected && req.Status != model.SwapStatusCancelled {
		return nil, apierror.BadRequest("status must be 'accepted', 'rejected', or 'cancelled'")
	}

	now := time.Now()
	reviewedBy := ctxutil.GetUserID(ctx)

	swapRequest.Status = req.Status
	swapRequest.ReviewedBy = &reviewedBy
	swapRequest.UpdatedAt = now

	if err := s.repo.Update(ctx, swapRequest); err != nil {
		logger.WithError(err).Error("failed to update swap request")
		return nil, apierror.Internal("failed to update swap request")
	}

	// If accepted, run rule engine validation before performing the swap.
	if req.Status == model.SwapStatusAccepted {
		if s.ruleEngine != nil {
			if err := s.validateSwapRules(ctx, tenantID, swapRequest); err != nil {
				return nil, err
			}
		}
		if err := s.performSwap(ctx, tenantID, swapRequest); err != nil {
			logger.WithError(err).Error("failed to perform swap")
			return nil, apierror.Internal("failed to perform swap")
		}
	}

	// Publish event
	s.emitter.Publish(event.Event{
		Type:     event.TypeSwapUpdated,
		TenantID: tenantID,
		UserID:   ctxutil.GetUserID(ctx),
		Payload:  swapRequest,
	})

	return swapRequest, nil
}

// GetSwapRequest retrieves a swap request.
func (s *SwapService) GetSwapRequest(ctx context.Context, tenantID, id uuid.UUID) (*model.SwapRequest, error) {
	logger := ctxutil.GetLogger(ctx)

	swapRequest, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		logger.WithError(err).Error("failed to get swap request")
		return nil, apierror.Internal("failed to get swap request")
	}
	if swapRequest == nil {
		return nil, apierror.NotFound("swap request", id.String())
	}

	return swapRequest, nil
}

// ListByEmployee retrieves swap requests for an employee.
func (s *SwapService) ListByEmployee(ctx context.Context, tenantID, employeeID uuid.UUID, page, pageSize int) ([]*model.SwapRequest, int64, error) {
	logger := ctxutil.GetLogger(ctx)

	swaps, total, err := s.repo.ListByEmployee(ctx, tenantID, employeeID, page, pageSize)
	if err != nil {
		logger.WithError(err).Error("failed to list swap requests")
		return nil, 0, apierror.Internal("failed to list swap requests")
	}

	return swaps, total, nil
}

// ListByStore retrieves swap requests for a store.
func (s *SwapService) ListByStore(ctx context.Context, tenantID uuid.UUID, status string, page, pageSize int) ([]*model.SwapRequest, int64, error) {
	logger := ctxutil.GetLogger(ctx)

	swaps, total, err := s.repo.ListByStore(ctx, tenantID, status, page, pageSize)
	if err != nil {
		logger.WithError(err).Error("failed to list swap requests")
		return nil, 0, apierror.Internal("failed to list swap requests")
	}

	return swaps, total, nil
}

// validateSwapRules runs the rule engine for each side of the proposed swap.
// Returns a non-nil error if any BLOCKING violation is found.
func (s *SwapService) validateSwapRules(ctx context.Context, tenantID uuid.UUID, swap *model.SwapRequest) error {
	logger := ctxutil.GetLogger(ctx)

	// Load the requester's shift
	shift, err := s.shiftRepo.GetByID(ctx, tenantID, swap.ShiftInstanceID)
	if err != nil || shift == nil {
		return nil // gracefully skip if shift not found
	}

	// Load the requester employee
	requester, err := s.empRepo.GetByID(ctx, tenantID, swap.RequesterID)
	if err != nil || requester == nil {
		return nil
	}

	// Evaluate rules for requester ↔ shift (they're keeping their shift, so this is mainly advisory)
	violations, err := s.ruleEngine.EvaluateAssignment(ctx, tenantID, shift, requester)
	if err != nil {
		logger.WithError(err).Warn("swap rule engine evaluation failed, skipping")
		return nil
	}
	for _, v := range violations {
		if v.Severity == model.RuleSeverityBlocking {
			return apierror.Conflict(v.Message)
		}
	}

	// If there's a target employee and target shift, validate their side too
	if swap.TargetEmployeeID != nil && swap.TargetShiftID != nil {
		targetShift, err := s.shiftRepo.GetByID(ctx, tenantID, *swap.TargetShiftID)
		if err != nil || targetShift == nil {
			return nil
		}
		targetEmp, err := s.empRepo.GetByID(ctx, tenantID, *swap.TargetEmployeeID)
		if err != nil || targetEmp == nil {
			return nil
		}
		violations, err = s.ruleEngine.EvaluateAssignment(ctx, tenantID, targetShift, targetEmp)
		if err != nil {
			logger.WithError(err).Warn("swap rule engine target evaluation failed, skipping")
			return nil
		}
		for _, v := range violations {
			if v.Severity == model.RuleSeverityBlocking {
				return apierror.Conflict(v.Message)
			}
		}
	}

	return nil
}

// performSwap exchanges assignments between two employees within a single DB transaction.
func (s *SwapService) performSwap(ctx context.Context, tenantID uuid.UUID, swapRequest *model.SwapRequest) error {
	// Resolve all assignments before the transaction so reads don't hold locks.
	assignments, err := s.assignRepo.ListByShift(ctx, tenantID, swapRequest.ShiftInstanceID)
	if err != nil {
		return err
	}

	var requesterAssignment *model.ShiftAssignment
	for _, a := range assignments {
		if a.EmployeeID == swapRequest.RequesterID && a.Status == model.AssignmentStatusConfirmed {
			requesterAssignment = a
			break
		}
	}

	if requesterAssignment == nil {
		return apierror.Conflict("requester assignment not found")
	}

	// Collect all updates to apply atomically.
	type update struct {
		assignment *model.ShiftAssignment
	}
	var updates []update

	now := time.Now()

	if swapRequest.TargetEmployeeID != nil && swapRequest.TargetShiftID != nil {
		// Full shift-for-shift swap between two employees.
		targetAssignments, err := s.assignRepo.ListByShift(ctx, tenantID, *swapRequest.TargetShiftID)
		if err != nil {
			return err
		}

		var targetAssignment *model.ShiftAssignment
		for _, a := range targetAssignments {
			if a.EmployeeID == *swapRequest.TargetEmployeeID && a.Status == model.AssignmentStatusConfirmed {
				targetAssignment = a
				break
			}
		}

		if targetAssignment == nil {
			return apierror.Conflict("target assignment not found")
		}

		// Load the new shifts to refresh the denormalized time fields.
		targetShift, err := s.shiftRepo.GetByID(ctx, tenantID, *swapRequest.TargetShiftID)
		if err != nil || targetShift == nil {
			return apierror.Conflict("target shift not found")
		}
		originalShift, err := s.shiftRepo.GetByID(ctx, tenantID, swapRequest.ShiftInstanceID)
		if err != nil || originalShift == nil {
			return apierror.Conflict("original shift not found")
		}

		requesterAssignment.ShiftInstanceID = *swapRequest.TargetShiftID
		requesterAssignment.ShiftDate = targetShift.Date
		requesterAssignment.ShiftStartTime = targetShift.StartTime
		requesterAssignment.ShiftEndTime = targetShift.EndTime
		requesterAssignment.UpdatedAt = now

		targetAssignment.ShiftInstanceID = swapRequest.ShiftInstanceID
		targetAssignment.ShiftDate = originalShift.Date
		targetAssignment.ShiftStartTime = originalShift.StartTime
		targetAssignment.ShiftEndTime = originalShift.EndTime
		targetAssignment.UpdatedAt = now

		updates = append(updates, update{requesterAssignment}, update{targetAssignment})

	} else if swapRequest.TargetEmployeeID != nil {
		// Reassign shift to a different employee (open-slot handover).
		// Shift itself doesn't change — denormalized time fields stay valid.
		requesterAssignment.EmployeeID = *swapRequest.TargetEmployeeID
		requesterAssignment.UpdatedAt = now
		updates = append(updates, update{requesterAssignment})

	} else if swapRequest.TargetShiftID != nil {
		// Move requester to a different shift — refresh the denormalized fields.
		newShift, err := s.shiftRepo.GetByID(ctx, tenantID, *swapRequest.TargetShiftID)
		if err != nil || newShift == nil {
			return apierror.Conflict("target shift not found")
		}
		requesterAssignment.ShiftInstanceID = *swapRequest.TargetShiftID
		requesterAssignment.ShiftDate = newShift.Date
		requesterAssignment.ShiftStartTime = newShift.StartTime
		requesterAssignment.ShiftEndTime = newShift.EndTime
		requesterAssignment.UpdatedAt = now
		updates = append(updates, update{requesterAssignment})
	}

	if len(updates) == 0 {
		return nil
	}

	// Execute all assignment updates inside a single DB transaction.
	if s.db != nil {
		if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			for _, u := range updates {
				if err := tx.Save(u.assignment).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	} else {
		// Fallback for tests where db is nil — use repo (no transaction guarantee).
		for _, u := range updates {
			if err := s.assignRepo.Update(ctx, u.assignment); err != nil {
				return err
			}
		}
	}

	// Publish events after the transaction commits.
	for _, u := range updates {
		s.emitter.Publish(event.Event{
			Type:     event.TypeAssignmentUpdated,
			TenantID: tenantID,
			UserID:   ctxutil.GetUserID(ctx),
			Payload:  u.assignment,
		})
	}

	return nil
}
