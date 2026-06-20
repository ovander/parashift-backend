package service

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
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

// scheduleLockKey derives a stable advisory-lock key from the tenant ID so that
// concurrent generate/regenerate operations for the same tenant are serialized
// (DAT-3 idempotency).
func scheduleLockKey(tenantID uuid.UUID) int64 {
	h := fnv.New64a()
	_, _ = h.Write(tenantID[:])
	return int64(h.Sum64())
}

// projectionEngine projects A/B week templates into concrete ShiftInstances and
// their confirmed assignments. Extracted from ScheduleService (ARC-1) so the
// projection algorithm — the largest and most intricate part of scheduling —
// lives and is tested on its own. ScheduleService builds one per call from its
// current dependencies and delegates ProjectABSchedule / RegenerateWeek to it.
type projectionEngine struct {
	shiftRepo     repo.ShiftInstanceRepository
	assignRepo    repo.ShiftAssignmentRepository
	tmplRepo      repo.WeekTemplateRepository
	empRepo       repo.EmployeeRepository
	leaveRepo     repo.LeaveRequestRepository
	availRepo     repo.AvailabilityRepository
	storeRepo     repo.StoreRepository
	exceptionRepo repo.StoreExceptionRepository
	holidaySvc    *PublicHolidayService
	txRunner      TxRunner
	emitter       *event.Emitter
	logger        *logrus.Entry
}

// Project generates ShiftInstances from WeekTemplates for all employees in the
// store. When a transaction runner is configured the template cleanup and the
// shift+assignment inserts commit or roll back together (DAT-2), serialized per
// tenant via an advisory lock (DAT-3).
func (p *projectionEngine) Project(ctx context.Context, tenantID uuid.UUID, req dto.GenerateScheduleRequest) (int, error) {
	if p.txRunner != nil {
		var count int
		err := p.txRunner(ctx, func(tx *repo.RepoBundle) error {
			if err := tx.AdvisoryXactLock(ctx, scheduleLockKey(tenantID)); err != nil {
				return err
			}
			c, e := p.projectWith(ctx, tenantID, req, tx.ShiftInstance, tx.ShiftAssignment)
			count = c
			return e
		})
		return count, err
	}
	return p.projectWith(ctx, tenantID, req, p.shiftRepo, p.assignRepo)
}

// projectWith performs the projection, routing every mutation (template cleanup +
// shift/assignment inserts + any compensating delete) through the supplied writer
// repos so the caller can run the whole thing inside one transaction. Reads use
// the engine's base repos (reference data not mutated here).
func (p *projectionEngine) projectWith(ctx context.Context, tenantID uuid.UUID, req dto.GenerateScheduleRequest, shiftW repo.ShiftInstanceRepository, assignW repo.ShiftAssignmentRepository) (int, error) {
	logger := ctxutil.GetLogger(ctx)

	dateFrom := req.DateFrom
	dateTo := req.DateTo

	// Enforce a sensible date range cap (max 730 days / ~2 years) to prevent runaway generation.
	const maxRangeDays = 730
	if dateTo.Sub(dateFrom).Hours()/24 > float64(maxRangeDays) {
		return 0, apierror.BadRequest("date range must not exceed 730 days").WithKey("errors.invalidInput")
	}

	// Delete existing TEMPLATE-sourced shifts for the date range first (idempotent)
	if err := shiftW.DeleteBySourceTemplate(ctx, tenantID, dateFrom, dateTo); err != nil {
		logger.WithError(err).Error("failed to delete existing template shifts")
		return 0, apierror.Internal("failed to delete existing shifts").WithKey("errors.unknown")
	}

	// Load all employees in the store (using pagination to get all)
	var employees []*model.Employee
	page := 0
	pageSize := 1000
	for {
		batch, _, err := p.empRepo.List(ctx, tenantID, page, pageSize)
		if err != nil {
			logger.WithError(err).Error("failed to list employees")
			return 0, apierror.Internal("failed to list employees").WithKey("errors.unknown")
		}
		employees = append(employees, batch...)
		if len(batch) < pageSize {
			break
		}
		page++
	}

	// Pre-load all templates for all employees into a map to avoid N+1 queries.
	templateMap := make(map[uuid.UUID][]*model.WeekTemplate, len(employees))
	for _, emp := range employees {
		tmpls, err := p.tmplRepo.GetByEmployee(ctx, tenantID, emp.ID)
		if err != nil {
			logger.WithError(err).Error("failed to get templates for employee")
			return 0, apierror.Internal("failed to get templates").WithKey("errors.unknown")
		}
		templateMap[emp.ID] = tmpls
	}

	// Load the store's ABWeekAnchor once; fall back to per-employee StartDate if absent.
	var storeAnchor *time.Time
	if p.storeRepo != nil {
		store, storeErr := p.storeRepo.GetByID(ctx, tenantID)
		if storeErr == nil && store != nil && store.ABWeekAnchor != nil {
			storeAnchor = store.ABWeekAnchor
		}
	}

	type pendingShift struct {
		shift *model.ShiftInstance
		empID uuid.UUID
	}

	var pending []pendingShift

	// Pre-load FORCED_CLOSED exceptions for the range so we can skip those days.
	forcedClosed := make(map[string]bool)
	if p.exceptionRepo != nil {
		excs, excErr := p.exceptionRepo.ListByDateRange(ctx, tenantID, dateFrom, dateTo)
		if excErr != nil {
			logger.WithError(excErr).Warn("failed to load store exceptions — proceeding without skip")
		} else {
			for _, ex := range excs {
				if ex.Type == model.ExceptionForcedClosed {
					forcedClosed[ex.Date.Format("2006-01-02")] = true
				}
			}
		}
	}

	// Iterate through each day in the range
	currentDate := dateFrom
	for !currentDate.After(dateTo) {
		// Skip days the store is explicitly closed.
		if forcedClosed[currentDate.Format("2006-01-02")] {
			currentDate = currentDate.AddDate(0, 0, 1)
			continue
		}
		// Skip public holidays.
		if p.holidaySvc != nil {
			isHoliday, _, hErr := p.holidaySvc.IsHoliday(ctx, currentDate, DefaultZone)
			if hErr != nil {
				logger.WithError(hErr).Warn("holiday check failed during generation — proceeding for this day")
			} else if isHoliday {
				currentDate = currentDate.AddDate(0, 0, 1)
				continue
			}
		}
		dayOfWeek := int(currentDate.Weekday())

		for _, emp := range employees {
			// Determine the A/B anchor: prefer store-level anchor over employee StartDate.
			anchor := emp.StartDate
			if storeAnchor != nil {
				anchor = *storeAnchor
			}
			weekType := model.WeekType(currentDate, anchor)
			tmpls := templateMap[emp.ID]

			// Collect ALL matching templates for this employee+day (supports split shifts).
			var dayTemplates []*model.WeekTemplate
			for _, t := range tmpls {
				if t.WeekType == weekType && t.DayOfWeek == dayOfWeek {
					dayTemplates = append(dayTemplates, t)
				}
			}
			if len(dayTemplates) == 0 {
				continue
			}

			// Skip if employee is on approved leave this day.
			hasLeave, err := p.leaveRepo.HasActiveLeave(ctx, tenantID, emp.ID, currentDate, currentDate)
			if err != nil {
				logger.WithError(err).Error("failed to check active leave")
				return 0, apierror.Internal("failed to check leave").WithKey("errors.unknown")
			}
			if hasLeave {
				continue
			}

			// Load availability once per employee+day (shared across all split shifts).
			avail, availErr := p.availRepo.GetByEmployeeDate(ctx, tenantID, emp.ID, currentDate)
			if availErr != nil {
				logger.WithError(availErr).Error("failed to check availability")
				return 0, apierror.Internal("failed to check availability").WithKey("errors.unknown")
			}

			var availTimeRanges []model.TimeRange
			employeeUnavailable := false
			if avail != nil {
				if len(avail.TimeRanges) > 0 {
					if jsonErr := json.Unmarshal(avail.TimeRanges, &availTimeRanges); jsonErr != nil || len(availTimeRanges) == 0 {
						employeeUnavailable = true // malformed or empty JSON → treat as unavailable
					}
				} else {
					employeeUnavailable = true // record exists with no windows → fully unavailable
				}
			}
			if employeeUnavailable {
				continue
			}

			// Generate one shift per matching template entry (handles split shifts).
			for _, templateEntry := range dayTemplates {
				// If employee has declared availability windows, skip shifts that don't overlap.
				if len(availTimeRanges) > 0 {
					shiftCovered := false
					for _, tr := range availTimeRanges {
						if timeOverlaps(templateEntry.StartTime, templateEntry.EndTime, tr.Start, tr.End) {
							shiftCovered = true
							break
						}
					}
					if !shiftCovered {
						continue
					}
				}

				shift := &model.ShiftInstance{
					TenantScoped: model.TenantScoped{
						ID:        uuid.New(),
						TenantID:  tenantID,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					Date:             currentDate,
					StartTime:        templateEntry.StartTime,
					EndTime:          templateEntry.EndTime,
					Role:             templateEntry.Role,
					Source:           model.SourceTemplate,
					SourceTemplateID: &templateEntry.ID,
				}
				pending = append(pending, pendingShift{shift: shift, empID: emp.ID})
			}
		}

		currentDate = currentDate.AddDate(0, 0, 1)
	}

	if len(pending) == 0 {
		return 0, nil
	}

	// Build shifts and their confirmed assignments up front (shift IDs are
	// pre-assigned), then persist both atomically.
	shifts := make([]*model.ShiftInstance, len(pending))
	for i, pnd := range pending {
		shifts[i] = pnd.shift
	}

	assignedBy := ctxutil.GetUserID(ctx)
	now := time.Now()
	assignments := make([]*model.ShiftAssignment, len(pending))
	for i, pnd := range pending {
		assignments[i] = &model.ShiftAssignment{
			TenantScoped: model.TenantScoped{
				ID:        uuid.New(),
				TenantID:  tenantID,
				CreatedAt: now,
				UpdatedAt: now,
			},
			ShiftInstanceID: pnd.shift.ID,
			EmployeeID:      pnd.empID,
			Status:          model.AssignmentStatusConfirmed,
			AssignedBy:      assignedBy,
			AssignedAt:      now,
			ShiftDate:       pnd.shift.Date,
			ShiftStartTime:  pnd.shift.StartTime,
			ShiftEndTime:    pnd.shift.EndTime,
		}
	}

	// Persist shifts then their confirmed assignments via the writer repos. When
	// these are tx-bound (the default in production) a mid-sequence failure rolls
	// both back. As defense-in-depth for the non-tx path, an assignment failure
	// triggers a SCOPED compensating delete of ONLY the just-created shift IDs —
	// never other shifts in the range (DAT-2).
	if err := shiftW.CreateBatch(ctx, shifts); err != nil {
		logger.WithError(err).Error("failed to create shifts batch")
		return 0, apierror.Internal("failed to create shifts").WithKey("errors.unknown")
	}
	if err := assignW.CreateBatch(ctx, assignments); err != nil {
		logger.WithError(err).Errorf("failed to batch-create %d assignments — rolling back just-created shifts", len(assignments))
		ids := make([]uuid.UUID, len(shifts))
		for i, sh := range shifts {
			ids[i] = sh.ID
		}
		if delErr := shiftW.DeleteByIDs(ctx, tenantID, ids); delErr != nil {
			logger.WithError(delErr).Error("compensating shift delete failed — week may be inconsistent; use Regenerate")
		}
		return 0, apierror.Internal(fmt.Sprintf("failed to create assignments: %s", err.Error())).WithKey("errors.unknown")
	}

	count := len(pending)

	// Publish a single summary event for the bulk generation (not per-shift).
	p.emitter.Publish(event.Event{
		Type:     event.TypeScheduleGenerated,
		TenantID: tenantID,
		UserID:   ctxutil.GetUserID(ctx),
		Payload: map[string]interface{}{
			"from":  dateFrom.Format("2006-01-02"),
			"to":    dateTo.Format("2006-01-02"),
			"count": count,
		},
	})

	return count, nil
}

// RegenerateWeek hard-resets a week: deletes ALL shifts and assignments for the
// week (regardless of source), then re-projects from A/B templates. Used by the
// "Force regenerate" toolbar button when the week is stuck in a broken state.
func (p *projectionEngine) RegenerateWeek(ctx context.Context, tenantID uuid.UUID, weekStart time.Time) (int, error) {
	logger := ctxutil.GetLogger(ctx)

	// Snap to Monday 00:00 → Sunday end-of-day UTC
	from := weekStart
	to := weekStart.AddDate(0, 0, 6)
	req := dto.GenerateScheduleRequest{DateFrom: from, DateTo: to}

	// Run delete-assignments → delete-shifts → re-project as a SINGLE transaction
	// when a runner is configured: if projection fails, the deletes roll back and
	// the prior week is left intact (DAT-2). Falls back to the sequential flow when
	// no runner is set (tests).
	regen := func(shiftW repo.ShiftInstanceRepository, assignW repo.ShiftAssignmentRepository) (int, error) {
		if _, err := assignW.DeleteByDateRange(ctx, tenantID, from, to); err != nil {
			logger.WithError(err).Error("regenerate: failed to delete assignments")
			return 0, apierror.Internal("failed to reset week assignments").WithKey("errors.unknown")
		}
		if err := shiftW.DeleteByDateRange(ctx, tenantID, from, to); err != nil {
			logger.WithError(err).Error("regenerate: failed to delete shifts")
			return 0, apierror.Internal("failed to reset week shifts").WithKey("errors.unknown")
		}
		return p.projectWith(ctx, tenantID, req, shiftW, assignW)
	}

	var count int
	var err error
	if p.txRunner != nil {
		err = p.txRunner(ctx, func(tx *repo.RepoBundle) error {
			if lockErr := tx.AdvisoryXactLock(ctx, scheduleLockKey(tenantID)); lockErr != nil {
				return lockErr
			}
			count, err = regen(tx.ShiftInstance, tx.ShiftAssignment)
			return err
		})
	} else {
		count, err = regen(p.shiftRepo, p.assignRepo)
	}
	if err != nil {
		logger.WithError(err).Error("regenerate: failed (rolled back if transactional)")
		return 0, err
	}

	logger.WithField("count", count).Info("week regenerated from templates")
	return count, nil
}
