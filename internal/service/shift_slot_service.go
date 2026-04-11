package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/repo"
	"github.com/sirupsen/logrus"
)

const publishWeeks = 4

// ShiftSlotService manages store-level shift slot templates and publishes them as ShiftInstances.
type ShiftSlotService struct {
	slotRepo  repo.ShiftSlotRepository
	shiftRepo repo.ShiftInstanceRepository
	storeRepo repo.StoreRepository
	logger    *logrus.Entry
}

// NewShiftSlotService creates a new ShiftSlotService.
func NewShiftSlotService(
	slotRepo repo.ShiftSlotRepository,
	shiftRepo repo.ShiftInstanceRepository,
	storeRepo repo.StoreRepository,
	logger *logrus.Entry,
) *ShiftSlotService {
	return &ShiftSlotService{
		slotRepo:  slotRepo,
		shiftRepo: shiftRepo,
		storeRepo: storeRepo,
		logger:    logger,
	}
}

// List returns all shift slots for a store.
func (s *ShiftSlotService) List(ctx context.Context, tenantID uuid.UUID) ([]*model.ShiftSlot, error) {
	logger := ctxutil.GetLogger(ctx)
	slots, err := s.slotRepo.List(ctx, tenantID)
	if err != nil {
		logger.WithError(err).Error("failed to list shift slots")
		return nil, apierror.Internal("failed to list shift slots").WithKey("errors.unknown")
	}
	return slots, nil
}

// Create creates a new shift slot.
func (s *ShiftSlotService) Create(ctx context.Context, tenantID uuid.UUID, req dto.CreateShiftSlotRequest) (*model.ShiftSlot, error) {
	logger := ctxutil.GetLogger(ctx)

	if req.Scheme != "A" && req.Scheme != "B" {
		return nil, apierror.BadRequest("scheme must be 'A' or 'B'").WithKey("errors.invalidInput")
	}
	if req.DayOfWeek < 1 || req.DayOfWeek > 7 {
		return nil, apierror.BadRequest("day_of_week must be 1–7 (1=Monday)").WithKey("errors.invalidInput")
	}
	if req.StartTime == "" || req.EndTime == "" {
		return nil, apierror.BadRequest("start_time and end_time are required").WithKey("errors.missingParams")
	}

	slot := &model.ShiftSlot{
		TenantScoped: model.TenantScoped{
			ID:        uuid.New(),
			TenantID:  tenantID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Scheme:       req.Scheme,
		DayOfWeek:    req.DayOfWeek,
		StartTime:    req.StartTime,
		EndTime:      req.EndTime,
		RequiredRole: req.RequiredRole,
	}

	if err := s.slotRepo.Create(ctx, slot); err != nil {
		logger.WithError(err).Error("failed to create shift slot")
		return nil, apierror.Internal("failed to create shift slot").WithKey("errors.unknown")
	}
	return slot, nil
}

// Delete removes a shift slot.
func (s *ShiftSlotService) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	logger := ctxutil.GetLogger(ctx)

	slot, err := s.slotRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		logger.WithError(err).Error("failed to get shift slot")
		return apierror.Internal("failed to get shift slot").WithKey("errors.unknown")
	}
	if slot == nil {
		return apierror.NotFound("shift slot", id.String()).WithKey("errors.unknown")
	}

	if err := s.slotRepo.Delete(ctx, tenantID, id); err != nil {
		logger.WithError(err).Error("failed to delete shift slot")
		return apierror.Internal("failed to delete shift slot").WithKey("errors.unknown")
	}
	return nil
}

// PublishScheme generates ShiftInstances for the next publishWeeks weeks from all
// slots belonging to the same scheme as the referenced slot.
func (s *ShiftSlotService) PublishScheme(ctx context.Context, tenantID, slotID uuid.UUID) (int, error) {
	logger := ctxutil.GetLogger(ctx)

	// 1. Find the slot to determine the scheme.
	slot, err := s.slotRepo.GetByID(ctx, tenantID, slotID)
	if err != nil {
		logger.WithError(err).Error("failed to get shift slot")
		return 0, apierror.Internal("failed to get shift slot").WithKey("errors.unknown")
	}
	if slot == nil {
		return 0, apierror.NotFound("shift slot", slotID.String()).WithKey("errors.unknown")
	}

	// 2. Load all slots for this scheme.
	slots, err := s.slotRepo.ListByScheme(ctx, tenantID, slot.Scheme)
	if err != nil {
		logger.WithError(err).Error("failed to list shift slots by scheme")
		return 0, apierror.Internal("failed to list shift slots").WithKey("errors.unknown")
	}
	if len(slots) == 0 {
		return 0, nil
	}

	// 3. Determine the date range: next publishWeeks weeks from today's Monday.
	today := time.Now().UTC().Truncate(24 * time.Hour)
	weekday := int(today.Weekday()) // 0=Sunday, 1=Monday...
	daysToMonday := (8 - weekday) % 7
	if daysToMonday == 0 {
		daysToMonday = 7 // start from next Monday if today is Monday
	}
	from := today.AddDate(0, 0, daysToMonday)
	to := from.AddDate(0, 0, publishWeeks*7-1)

	// 4. Determine the A/B week anchor for this store.
	store, err := s.storeRepo.GetByID(ctx, tenantID)
	if err != nil || store == nil {
		// Fall back to from as anchor (week 1 = scheme A)
		store = nil
	}
	anchor := from
	if store != nil && store.ABWeekAnchor != nil {
		anchor = *store.ABWeekAnchor
	}

	// 5. Collect slot IDs for deletion.
	slotIDs := make([]uuid.UUID, len(slots))
	for i, sl := range slots {
		slotIDs[i] = sl.ID
	}

	// 6. Delete existing slot-sourced shifts for this scheme in the date range.
	if err := s.shiftRepo.DeleteBySlotIDs(ctx, tenantID, slotIDs, from, to); err != nil {
		logger.WithError(err).Error("failed to delete existing slot shifts")
		return 0, apierror.Internal("failed to clear existing shifts").WithKey("errors.unknown")
	}

	// 7. Build a map: dayOfWeek → slots (for this scheme).
	// Frontend uses 1=Monday...7=Sunday; time.Weekday uses 0=Sunday...6=Saturday.
	slotsByDay := make(map[int][]*model.ShiftSlot, 7)
	for _, sl := range slots {
		slotsByDay[sl.DayOfWeek] = append(slotsByDay[sl.DayOfWeek], sl)
	}

	// 8. Iterate over each date in the range and create ShiftInstances.
	var shifts []*model.ShiftInstance
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		// Determine scheme for this date using the store anchor.
		dateScheme := model.WeekType(d, anchor)
		if dateScheme != slot.Scheme {
			continue
		}

		// Convert time.Weekday to frontend convention (0=Sun→7, 1=Mon→1 … 6=Sat→6).
		goDOW := int(d.Weekday()) // 0=Sunday
		frontendDOW := goDOW
		if goDOW == 0 {
			frontendDOW = 7
		}

		daySlots := slotsByDay[frontendDOW]
		for _, sl := range daySlots {
			slID := sl.ID
			shifts = append(shifts, &model.ShiftInstance{
				TenantScoped: model.TenantScoped{
					ID:        uuid.New(),
					TenantID:  tenantID,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				Date:                  d,
				StartTime:             sl.StartTime,
				EndTime:               sl.EndTime,
				Role:                  sl.RequiredRole,
				RequiredQualification: sl.RequiredRole,
				Source:                model.SourceSlot,
				SourceTemplateID:      &slID,
				Status:                "DRAFT",
			})
		}
	}

	// 9. Persist all generated shifts.
	if len(shifts) == 0 {
		return 0, nil
	}
	for _, sh := range shifts {
		if err := s.shiftRepo.Create(ctx, sh); err != nil {
			logger.WithError(err).Error("failed to create shift instance from slot")
			return 0, apierror.Internal("failed to generate shifts").WithKey("errors.unknown")
		}
	}

	return len(shifts), nil
}
