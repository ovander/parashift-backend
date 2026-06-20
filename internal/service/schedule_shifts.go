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

// shiftService owns shift-instance CRUD, schedule reads and publishing. Extracted
// from ScheduleService (ARC-1); ScheduleService builds one per call from its
// current deps and delegates.
type shiftService struct {
	shiftRepo  repo.ShiftInstanceRepository
	assignRepo repo.ShiftAssignmentRepository
	holidaySvc *PublicHolidayService
	emitter    *event.Emitter
	logger     *logrus.Entry
}

// GetByID retrieves a shift by tenant and ID.
func (s *shiftService) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.ShiftInstance, error) {
	logger := ctxutil.GetLogger(ctx)
	shift, err := s.shiftRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		logger.WithError(err).Error("failed to get shift")
		return nil, apierror.Internal("failed to get shift").WithKey("errors.unknown")
	}
	if shift == nil {
		return nil, apierror.NotFound("shift", id.String()).WithKey("errors.unknown")
	}
	return shift, nil
}

// ListByDate retrieves all shifts for a specific date.
func (s *shiftService) ListByDate(ctx context.Context, tenantID uuid.UUID, date time.Time) ([]*model.ShiftInstance, error) {
	logger := ctxutil.GetLogger(ctx)
	shifts, err := s.shiftRepo.ListByDate(ctx, tenantID, date)
	if err != nil {
		logger.WithError(err).Error("failed to list shifts by date")
		return nil, apierror.Internal("failed to list shifts").WithKey("errors.unknown")
	}
	return shifts, nil
}

// GetSchedule retrieves shifts for a store within a date range with pagination.
func (s *shiftService) GetSchedule(ctx context.Context, tenantID uuid.UUID, dateFrom, dateTo time.Time, page, pageSize int) ([]*model.ShiftInstance, int64, error) {
	logger := ctxutil.GetLogger(ctx)
	shifts, total, err := s.shiftRepo.ListByDateRange(ctx, tenantID, dateFrom, dateTo, page, pageSize)
	if err != nil {
		logger.WithError(err).Error("failed to get schedule")
		return nil, 0, apierror.Internal("failed to get schedule").WithKey("errors.unknown")
	}
	return shifts, total, nil
}

// Create creates a new shift instance.
func (s *shiftService) Create(ctx context.Context, tenantID uuid.UUID, req dto.CreateShiftInstanceRequest) (*model.ShiftInstance, error) {
	logger := ctxutil.GetLogger(ctx)

	if req.StartTime == "" || req.EndTime == "" {
		return nil, apierror.BadRequest("start_time and end_time are required").WithKey("errors.missingParams")
	}

	// Block shift creation on French public holidays.
	if err := blockIfHoliday(ctx, s.holidaySvc, s.logger, req.Date); err != nil {
		return nil, err
	}

	// Reject invalid time ranges (overnight shifts not supported at creation time).
	if req.StartTime >= req.EndTime {
		return nil, apierror.BadRequest("start_time must be earlier than end_time").WithKey("errors.invalidInput")
	}

	source := model.SourceManual
	if req.Source != nil {
		source = *req.Source
	}
	if source != model.SourceTemplate && source != model.SourceOverride && source != model.SourceManual {
		return nil, apierror.BadRequest("invalid source, must be TEMPLATE, OVERRIDE, or MANUAL").WithKey("errors.invalidInput")
	}

	shift := &model.ShiftInstance{
		TenantScoped: model.TenantScoped{
			ID:        uuid.New(),
			TenantID:  tenantID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Date:                  req.Date,
		StartTime:             req.StartTime,
		EndTime:               req.EndTime,
		Role:                  derefString(req.Role),
		RequiredQualification: derefString(req.RequiredQualification),
		Source:                source,
		SourceTemplateID:      req.SourceTemplateID,
	}

	if err := s.shiftRepo.Create(ctx, shift); err != nil {
		logger.WithError(err).Error("failed to create shift")
		return nil, apierror.Internal("failed to create shift").WithKey("errors.unknown")
	}

	s.emitter.Publish(event.Event{
		Type:     event.TypeShiftCreated,
		TenantID: tenantID,
		UserID:   ctxutil.GetUserID(ctx),
		Payload:  shift,
	})

	return shift, nil
}

// Update updates an existing shift.
func (s *shiftService) Update(ctx context.Context, tenantID, id uuid.UUID, req dto.UpdateShiftInstanceRequest) (*model.ShiftInstance, error) {
	logger := ctxutil.GetLogger(ctx)

	shift, err := s.shiftRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		logger.WithError(err).Error("failed to get shift for update")
		return nil, apierror.Internal("failed to get shift").WithKey("errors.unknown")
	}
	if shift == nil {
		return nil, apierror.NotFound("shift", id.String()).WithKey("errors.unknown")
	}

	if req.Date != nil {
		shift.Date = *req.Date
	}
	if req.StartTime != nil {
		shift.StartTime = *req.StartTime
	}
	if req.EndTime != nil {
		shift.EndTime = *req.EndTime
	}
	if req.Role != nil {
		shift.Role = *req.Role
	}
	if req.RequiredQualification != nil {
		shift.RequiredQualification = *req.RequiredQualification
	}
	if req.Source != nil {
		shift.Source = *req.Source
	}
	if req.SourceTemplateID != nil {
		shift.SourceTemplateID = req.SourceTemplateID
	}

	shift.UpdatedAt = time.Now()
	if err := s.shiftRepo.Update(ctx, shift); err != nil {
		logger.WithError(err).Error("failed to update shift")
		return nil, optimisticErr(err, apierror.Internal("failed to update shift").WithKey("errors.unknown"))
	}

	s.emitter.Publish(event.Event{
		Type:     event.TypeShiftUpdated,
		TenantID: tenantID,
		UserID:   ctxutil.GetUserID(ctx),
		Payload:  shift,
	})

	return shift, nil
}

// Delete deletes a shift instance.
func (s *shiftService) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	logger := ctxutil.GetLogger(ctx)

	if err := s.shiftRepo.Delete(ctx, tenantID, id); err != nil {
		logger.WithError(err).Error("failed to delete shift")
		return apierror.Internal("failed to delete shift").WithKey("errors.unknown")
	}

	s.emitter.Publish(event.Event{
		Type:     event.TypeShiftDeleted,
		TenantID: tenantID,
		UserID:   ctxutil.GetUserID(ctx),
		Payload:  map[string]interface{}{"id": id},
	})

	return nil
}

// Publish marks all shifts in the given date range as PUBLISHED. Returns the
// number of shifts transitioned.
func (s *shiftService) Publish(ctx context.Context, tenantID uuid.UUID, from, to time.Time) (int64, error) {
	logger := ctxutil.GetLogger(ctx)

	count, err := s.shiftRepo.SetStatusByDateRange(ctx, tenantID, from, to, model.ShiftStatusPublished)
	if err != nil {
		logger.WithError(err).Error("failed to publish schedule")
		return 0, apierror.Internal("failed to publish schedule").WithKey("errors.unknown")
	}

	s.emitter.Publish(event.Event{
		Type:     event.TypeShiftUpdated,
		TenantID: tenantID,
		UserID:   ctxutil.GetUserID(ctx),
		Payload: map[string]interface{}{
			"action": "published",
			"from":   from.Format("2006-01-02"),
			"to":     to.Format("2006-01-02"),
			"count":  count,
		},
	})

	return count, nil
}

// ListMyShifts returns enriched shift+assignment data for an employee in a date
// range. Shift instances are batch-loaded to avoid N+1 queries.
func (s *shiftService) ListMyShifts(ctx context.Context, tenantID, employeeID uuid.UUID, from, to time.Time) ([]*ShiftWithAssignment, error) {
	logger := ctxutil.GetLogger(ctx)

	assignments, err := s.assignRepo.ListByEmployee(ctx, tenantID, employeeID, from, to)
	if err != nil {
		logger.WithError(err).Error("failed to list employee assignments")
		return nil, apierror.Internal("failed to list assignments").WithKey("errors.unknown")
	}

	if len(assignments) == 0 {
		return nil, nil
	}

	// Collect unique shift IDs for a single batch query.
	shiftIDs := make([]uuid.UUID, 0, len(assignments))
	seen := make(map[uuid.UUID]bool, len(assignments))
	for _, a := range assignments {
		if !seen[a.ShiftInstanceID] {
			shiftIDs = append(shiftIDs, a.ShiftInstanceID)
			seen[a.ShiftInstanceID] = true
		}
	}

	shifts, err := s.shiftRepo.ListByIDs(ctx, tenantID, shiftIDs)
	if err != nil {
		logger.WithError(err).Error("failed to batch-load shifts")
		return nil, apierror.Internal("failed to load shifts").WithKey("errors.unknown")
	}

	shiftByID := make(map[uuid.UUID]*model.ShiftInstance, len(shifts))
	for _, sh := range shifts {
		shiftByID[sh.ID] = sh
	}

	var items []*ShiftWithAssignment
	for _, a := range assignments {
		sh, ok := shiftByID[a.ShiftInstanceID]
		if !ok {
			continue
		}
		items = append(items, &ShiftWithAssignment{
			ShiftInstanceID: a.ShiftInstanceID,
			Date:            sh.Date,
			StartTime:       sh.StartTime,
			EndTime:         sh.EndTime,
			Role:            sh.Role,
			Status:          a.Status,
			AssignedAt:      a.AssignedAt,
		})
	}
	return items, nil
}
