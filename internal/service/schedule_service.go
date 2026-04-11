package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

// ScheduleService is the core scheduling domain service.
type ScheduleService struct {
	shiftRepo      repo.ShiftInstanceRepository
	assignRepo     repo.ShiftAssignmentRepository
	tmplRepo       repo.WeekTemplateRepository
	empRepo        repo.EmployeeRepository
	leaveRepo      repo.LeaveRequestRepository
	availRepo      repo.AvailabilityRepository
	storeRepo      repo.StoreRepository
	exceptionRepo  repo.StoreExceptionRepository // optional — nil disables exception checks
	ruleEngine     *RuleEngine                   // optional — nil disables rule evaluation
	holidaySvc     *PublicHolidayService         // optional — nil disables holiday blocking
	emitter        *event.Emitter
	logger         *logrus.Entry
}

// NewScheduleService creates a new ScheduleService.
func NewScheduleService(
	shiftRepo repo.ShiftInstanceRepository,
	assignRepo repo.ShiftAssignmentRepository,
	tmplRepo repo.WeekTemplateRepository,
	empRepo repo.EmployeeRepository,
	leaveRepo repo.LeaveRequestRepository,
	availRepo repo.AvailabilityRepository,
	storeRepo repo.StoreRepository,
	emitter *event.Emitter,
	logger *logrus.Entry,
) *ScheduleService {
	return &ScheduleService{
		shiftRepo:  shiftRepo,
		assignRepo: assignRepo,
		tmplRepo:   tmplRepo,
		empRepo:    empRepo,
		leaveRepo:  leaveRepo,
		availRepo:  availRepo,
		storeRepo:  storeRepo,
		emitter:    emitter,
		logger:     logger,
	}
}

// WithRuleEngine attaches a RuleEngine to the service (called during wiring in service_bundle).
func (s *ScheduleService) WithRuleEngine(re *RuleEngine) *ScheduleService {
	s.ruleEngine = re
	return s
}

// WithPublicHolidayService attaches a PublicHolidayService to enable holiday blocking.
func (s *ScheduleService) WithPublicHolidayService(svc *PublicHolidayService) *ScheduleService {
	s.holidaySvc = svc
	return s
}

// WithStoreExceptionRepo attaches a StoreExceptionRepository so auto-projection
// can skip FORCED_CLOSED days.
func (s *ScheduleService) WithStoreExceptionRepo(r repo.StoreExceptionRepository) *ScheduleService {
	s.exceptionRepo = r
	return s
}

// checkHoliday returns a non-nil error if date falls on a French public holiday.
func (s *ScheduleService) checkHoliday(ctx context.Context, date time.Time) error {
	if s.holidaySvc == nil {
		return nil
	}
	isHoliday, name, err := s.holidaySvc.IsHoliday(ctx, date, DefaultZone)
	if err != nil {
		// Non-fatal: log and let the action proceed.
		s.logger.WithError(err).Warn("holiday check failed — proceeding without block")
		return nil
	}
	if isHoliday {
		return apierror.BadRequest(fmt.Sprintf("'%s' est un jour férié (%s) — aucune affectation autorisée", date.Format("2006-01-02"), name)).WithKey("errors.publicHoliday")
	}
	return nil
}

// GetByID retrieves a shift by tenant and ID. Implements ShiftLookup interface.
func (s *ScheduleService) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.ShiftInstance, error) {
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

// ListByDate retrieves all shifts for a specific date. Implements ShiftLookup interface.
func (s *ScheduleService) ListByDate(ctx context.Context, tenantID uuid.UUID, date time.Time) ([]*model.ShiftInstance, error) {
	logger := ctxutil.GetLogger(ctx)
	shifts, err := s.shiftRepo.ListByDate(ctx, tenantID, date)
	if err != nil {
		logger.WithError(err).Error("failed to list shifts by date")
		return nil, apierror.Internal("failed to list shifts").WithKey("errors.unknown")
	}
	return shifts, nil
}

// GetWeekTemplates retrieves week templates for an employee.
func (s *ScheduleService) GetWeekTemplates(ctx context.Context, tenantID, employeeID uuid.UUID) (dto.WeekTemplateResponse, error) {
	logger := ctxutil.GetLogger(ctx)

	templates, err := s.tmplRepo.GetByEmployee(ctx, tenantID, employeeID)
	if err != nil {
		logger.WithError(err).Error("failed to get week templates")
		return dto.WeekTemplateResponse{}, apierror.Internal("failed to get week templates").WithKey("errors.unknown")
	}

	var entries []dto.WeekTemplateEntry
	for _, t := range templates {
		entries = append(entries, dto.WeekTemplateEntry{
			WeekType:  t.WeekType,
			DayOfWeek: t.DayOfWeek,
			StartTime: t.StartTime,
			EndTime:   t.EndTime,
			Role:      t.Role,
		})
	}

	return dto.WeekTemplateResponse{
		EmployeeID: employeeID, // uuid.UUID
		Entries:    entries,
	}, nil
}

// UpsertWeekTemplates upserts week templates for an employee.
func (s *ScheduleService) UpsertWeekTemplates(ctx context.Context, tenantID, employeeID uuid.UUID, req dto.UpsertWeekTemplateRequest) error {
	logger := ctxutil.GetLogger(ctx)

	// Verify employee exists
	emp, err := s.empRepo.GetByID(ctx, tenantID, employeeID)
	if err != nil {
		logger.WithError(err).Error("failed to get employee")
		return apierror.Internal("failed to get employee").WithKey("errors.unknown")
	}
	if emp == nil {
		return apierror.NotFound("employee", employeeID.String()).WithKey("errors.unknown")
	}

	// Build templates from entries (req.Templates, each entry carries its own WeekType)
	var templates []*model.WeekTemplate
	for _, entry := range req.Templates {
		if entry.WeekType != "A" && entry.WeekType != "B" {
			return apierror.BadRequest("week_type must be 'A' or 'B'").WithKey("errors.invalidInput")
		}
		if entry.DayOfWeek < 0 || entry.DayOfWeek > 6 {
			return apierror.BadRequest("day_of_week must be 0-6").WithKey("errors.invalidInput")
		}
		if entry.StartTime == "" || entry.EndTime == "" {
			return apierror.BadRequest("start_time and end_time are required").WithKey("errors.missingParams")
		}

		template := &model.WeekTemplate{
			TenantScoped: model.TenantScoped{
				ID:        uuid.New(),
				TenantID:  tenantID,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			EmployeeID: employeeID,
			WeekType:   entry.WeekType, // from each entry
			DayOfWeek:  entry.DayOfWeek,
			StartTime:  entry.StartTime,
			EndTime:    entry.EndTime,
			Role:       entry.Role,
		}
		templates = append(templates, template)
	}

	// Upsert templates (this replaces all for the employee)
	if err := s.tmplRepo.UpsertForEmployee(ctx, tenantID, employeeID, templates); err != nil {
		logger.WithError(err).Error("failed to upsert week templates")
		return apierror.Internal("failed to upsert week templates").WithKey("errors.unknown")
	}

	return nil
}

// ProjectABSchedule generates ShiftInstances from WeekTemplates for all employees in the store.
func (s *ScheduleService) ProjectABSchedule(ctx context.Context, tenantID uuid.UUID, req dto.GenerateScheduleRequest) (int, error) {
	logger := ctxutil.GetLogger(ctx)

	// req.DateFrom and req.DateTo are already time.Time from DTO JSON binding
	dateFrom := req.DateFrom
	dateTo := req.DateTo

	// Enforce a sensible date range cap (max 730 days / ~2 years) to prevent runaway generation.
	const maxRangeDays = 730
	if dateTo.Sub(dateFrom).Hours()/24 > float64(maxRangeDays) {
		return 0, apierror.BadRequest("date range must not exceed 730 days").WithKey("errors.invalidInput")
	}

	// Delete existing TEMPLATE-sourced shifts for the date range first (idempotent)
	if err := s.shiftRepo.DeleteBySourceTemplate(ctx, tenantID, dateFrom, dateTo); err != nil {
		logger.WithError(err).Error("failed to delete existing template shifts")
		return 0, apierror.Internal("failed to delete existing shifts").WithKey("errors.unknown")
	}

	// Load all employees in the store (using pagination to get all)
	var employees []*model.Employee
	page := 0
	pageSize := 1000
	for {
		batch, _, err := s.empRepo.List(ctx, tenantID, page, pageSize)
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
	// templateMap[employeeID] → []WeekTemplate
	templateMap := make(map[uuid.UUID][]*model.WeekTemplate, len(employees))
	for _, emp := range employees {
		tmpls, err := s.tmplRepo.GetByEmployee(ctx, tenantID, emp.ID)
		if err != nil {
			logger.WithError(err).Error("failed to get templates for employee")
			return 0, apierror.Internal("failed to get templates").WithKey("errors.unknown")
		}
		templateMap[emp.ID] = tmpls
	}

	// Load the store's ABWeekAnchor once; fall back to per-employee StartDate if absent.
	var storeAnchor *time.Time
	if s.storeRepo != nil {
		store, storeErr := s.storeRepo.GetByID(ctx, tenantID)
		if storeErr == nil && store != nil && store.ABWeekAnchor != nil {
			storeAnchor = store.ABWeekAnchor
		}
	}

	// type empShiftKey ties a shift to the employee who should be assigned to it.
	type pendingShift struct {
		shift *model.ShiftInstance
		empID uuid.UUID
	}

	var pending []pendingShift

	// Pre-load FORCED_CLOSED exceptions for the range so we can skip those days.
	forcedClosed := make(map[string]bool)
	if s.exceptionRepo != nil {
		excs, excErr := s.exceptionRepo.ListByDateRange(ctx, tenantID, dateFrom, dateTo)
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
		if s.holidaySvc != nil {
			isHoliday, _, hErr := s.holidaySvc.IsHoliday(ctx, currentDate, DefaultZone)
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
			hasLeave, err := s.leaveRepo.HasActiveLeave(ctx, tenantID, emp.ID, currentDate, currentDate)
			if err != nil {
				logger.WithError(err).Error("failed to check active leave")
				return 0, apierror.Internal("failed to check leave").WithKey("errors.unknown")
			}
			if hasLeave {
				continue
			}

			// Load availability once per employee+day (shared across all split shifts).
			avail, availErr := s.availRepo.GetByEmployeeDate(ctx, tenantID, emp.ID, currentDate)
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

	// Batch-create all shifts.
	shifts := make([]*model.ShiftInstance, len(pending))
	for i, p := range pending {
		shifts[i] = p.shift
	}
	if err := s.shiftRepo.CreateBatch(ctx, shifts); err != nil {
		logger.WithError(err).Error("failed to create shifts batch")
		return 0, apierror.Internal("failed to create shifts").WithKey("errors.unknown")
	}

	// Batch-create all assignments in a single SQL statement — include denormalized
	// shift time fields so the rule engine can calculate real hours without a JOIN.
	assignedBy := ctxutil.GetUserID(ctx)
	now := time.Now()
	assignments := make([]*model.ShiftAssignment, len(pending))
	for i, p := range pending {
		assignments[i] = &model.ShiftAssignment{
			TenantScoped: model.TenantScoped{
				ID:        uuid.New(),
				TenantID:  tenantID,
				CreatedAt: now,
				UpdatedAt: now,
			},
			ShiftInstanceID: p.shift.ID,
			EmployeeID:      p.empID,
			Status:          model.AssignmentStatusConfirmed,
			AssignedBy:      assignedBy,
			AssignedAt:      now,
			ShiftDate:       p.shift.Date,
			ShiftStartTime:  p.shift.StartTime,
			ShiftEndTime:    p.shift.EndTime,
		}
	}
	if err := s.assignRepo.CreateBatch(ctx, assignments); err != nil {
		logger.WithError(err).Errorf("failed to batch-create %d assignments — rolling back shifts", len(assignments))
		// Compensating delete: hard-remove the shifts we just created so the week
		// stays at total=0 and auto-projection can retry on the next load.
		if delErr := s.shiftRepo.DeleteByDateRange(ctx, tenantID, dateFrom, dateTo); delErr != nil {
			logger.WithError(delErr).Error("compensating shift delete also failed — week may be stuck; use Regenerate")
		}
		return 0, apierror.Internal(fmt.Sprintf("failed to create assignments: %s", err.Error())).WithKey("errors.unknown")
	}

	count := len(pending)

	// Publish a single summary event for the bulk generation (not per-shift).
	s.emitter.Publish(event.Event{
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
// week (regardless of source), then re-projects from A/B templates.
// Used by the "Force regenerate" toolbar button when the week is stuck in a
// broken state (shifts without assignments, stale manual shifts, etc.).
func (s *ScheduleService) RegenerateWeek(ctx context.Context, tenantID uuid.UUID, weekStart time.Time) (int, error) {
	logger := ctxutil.GetLogger(ctx)

	// Snap to Monday 00:00 → Sunday end-of-day UTC
	from := weekStart
	to := weekStart.AddDate(0, 0, 6)

	// 1. Delete all assignments for the week first (FK child before parent).
	if _, err := s.assignRepo.DeleteByDateRange(ctx, tenantID, from, to); err != nil {
		logger.WithError(err).Error("regenerate: failed to delete assignments")
		return 0, apierror.Internal("failed to reset week assignments").WithKey("errors.unknown")
	}

	// 2. Delete all shifts for the week (any source).
	if err := s.shiftRepo.DeleteByDateRange(ctx, tenantID, from, to); err != nil {
		logger.WithError(err).Error("regenerate: failed to delete shifts")
		return 0, apierror.Internal("failed to reset week shifts").WithKey("errors.unknown")
	}

	// 3. Re-project from templates.
	count, err := s.ProjectABSchedule(ctx, tenantID, dto.GenerateScheduleRequest{
		DateFrom: from,
		DateTo:   to,
	})
	if err != nil {
		logger.WithError(err).Error("regenerate: projection failed")
		return 0, err
	}

	logger.WithField("count", count).Info("week regenerated from templates")
	return count, nil
}

// GetSchedule retrieves shifts for a store within a date range with pagination.
// If no shifts exist for the requested period, it automatically projects the
// employees' A/B week templates so the planner always shows a populated week
// without requiring a manual "generate" step.
func (s *ScheduleService) GetSchedule(ctx context.Context, tenantID uuid.UUID, dateFrom, dateTo time.Time, page, pageSize int) ([]*model.ShiftInstance, int64, error) {
	logger := ctxutil.GetLogger(ctx)

	shifts, total, err := s.shiftRepo.ListByDateRange(ctx, tenantID, dateFrom, dateTo, page, pageSize)
	if err != nil {
		logger.WithError(err).Error("failed to get schedule")
		return nil, 0, apierror.Internal("failed to get schedule").WithKey("errors.unknown")
	}

	if total == 0 {
		// No shifts for this week — auto-project from A/B templates.
		// Errors are non-fatal: a store with no templates simply gets an empty week.
		count, projErr := s.ProjectABSchedule(ctx, tenantID, dto.GenerateScheduleRequest{
			DateFrom: dateFrom,
			DateTo:   dateTo,
		})
		if projErr != nil {
			logger.WithError(projErr).Warn("auto-projection failed — returning empty week")
			return nil, 0, nil
		}
		if count > 0 {
			shifts, total, err = s.shiftRepo.ListByDateRange(ctx, tenantID, dateFrom, dateTo, page, pageSize)
			if err != nil {
				logger.WithError(err).Error("failed to re-fetch schedule after auto-projection")
				return nil, 0, apierror.Internal("failed to get schedule").WithKey("errors.unknown")
			}
		}
	}

	return shifts, total, nil
}

// CreateShift creates a new shift instance.
func (s *ScheduleService) CreateShift(ctx context.Context, tenantID uuid.UUID, req dto.CreateShiftInstanceRequest) (*model.ShiftInstance, error) {
	logger := ctxutil.GetLogger(ctx)

	// req.Date is already time.Time from DTO JSON binding
	if req.StartTime == "" || req.EndTime == "" {
		return nil, apierror.BadRequest("start_time and end_time are required").WithKey("errors.missingParams")
	}

	// Block shift creation on French public holidays.
	if err := s.checkHoliday(ctx, req.Date); err != nil {
		return nil, err
	}

	// Reject invalid time ranges (overnight shifts not supported at creation time).
	if req.StartTime >= req.EndTime {
		return nil, apierror.BadRequest("start_time must be earlier than end_time").WithKey("errors.invalidInput")
	}

	// Resolve source (req.Source is *string, defaults to MANUAL)
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
		Date:                  req.Date, // already time.Time
		StartTime:             req.StartTime,
		EndTime:               req.EndTime,
		Role:                  derefString(req.Role),                  // *string → string
		RequiredQualification: derefString(req.RequiredQualification), // *string → string
		Source:                source,
		SourceTemplateID:      req.SourceTemplateID,
	}

	if err := s.shiftRepo.Create(ctx, shift); err != nil {
		logger.WithError(err).Error("failed to create shift")
		return nil, apierror.Internal("failed to create shift").WithKey("errors.unknown")
	}

	// Publish event
	s.emitter.Publish(event.Event{
		Type:     event.TypeShiftCreated,
		TenantID: tenantID,
		UserID:   ctxutil.GetUserID(ctx),
		Payload:  shift,
	})

	return shift, nil
}

// UpdateShift updates an existing shift.
func (s *ScheduleService) UpdateShift(ctx context.Context, tenantID, id uuid.UUID, req dto.UpdateShiftInstanceRequest) (*model.ShiftInstance, error) {
	logger := ctxutil.GetLogger(ctx)

	shift, err := s.shiftRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		logger.WithError(err).Error("failed to get shift for update")
		return nil, apierror.Internal("failed to get shift").WithKey("errors.unknown")
	}
	if shift == nil {
		return nil, apierror.NotFound("shift", id.String()).WithKey("errors.unknown")
	}

	// All UpdateShiftInstanceRequest fields are optional pointers
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
		return nil, apierror.Internal("failed to update shift").WithKey("errors.unknown")
	}

	// Publish event
	s.emitter.Publish(event.Event{
		Type:     event.TypeShiftUpdated,
		TenantID: tenantID,
		UserID:   ctxutil.GetUserID(ctx),
		Payload:  shift,
	})

	return shift, nil
}

// DeleteShift deletes a shift instance.
func (s *ScheduleService) DeleteShift(ctx context.Context, tenantID, id uuid.UUID) error {
	logger := ctxutil.GetLogger(ctx)

	if err := s.shiftRepo.Delete(ctx, tenantID, id); err != nil {
		logger.WithError(err).Error("failed to delete shift")
		return apierror.Internal("failed to delete shift").WithKey("errors.unknown")
	}

	// Publish event
	s.emitter.Publish(event.Event{
		Type:     event.TypeShiftDeleted,
		TenantID: tenantID,
		UserID:   ctxutil.GetUserID(ctx),
		Payload:  map[string]interface{}{"id": id},
	})

	return nil
}

// CreateAssignment creates a new shift assignment.
// CreateAssignment assigns an employee to a shift and runs configured scheduling rules.
// Returned violations (WARNING/INFO) are advisory; BLOCKING violations cause an error.
func (s *ScheduleService) CreateAssignment(ctx context.Context, tenantID uuid.UUID, req dto.CreateAssignmentRequest) (*model.ShiftAssignment, []model.RuleViolation, error) {
	logger := ctxutil.GetLogger(ctx)

	// req.ShiftID and req.EmployeeID are already uuid.UUID from DTO
	shiftID := req.ShiftID
	empID := req.EmployeeID

	// Check shift exists in tenant
	shift, err := s.shiftRepo.GetByID(ctx, tenantID, shiftID)
	if err != nil {
		logger.WithError(err).Error("failed to get shift")
		return nil, nil, apierror.Internal("failed to get shift").WithKey("errors.unknown")
	}
	if shift == nil {
		return nil, nil, apierror.NotFound("shift", shiftID.String()).WithKey("errors.unknown")
	}

	// Block assignment on French public holidays.
	if err := s.checkHoliday(ctx, shift.Date); err != nil {
		return nil, nil, err
	}

	// Check employee exists in tenant
	emp, err := s.empRepo.GetByID(ctx, tenantID, empID)
	if err != nil {
		logger.WithError(err).Error("failed to get employee")
		return nil, nil, apierror.Internal("failed to get employee").WithKey("errors.unknown")
	}
	if emp == nil {
		return nil, nil, apierror.NotFound("employee", empID.String()).WithKey("errors.unknown")
	}

	// Check for assignment conflicts
	hasConflict, err := s.assignRepo.ExistsConflict(ctx, tenantID, empID, shift.Date, shift.StartTime, shift.EndTime, nil)
	if err != nil {
		logger.WithError(err).Error("failed to check assignment conflicts")
		return nil, nil, apierror.Internal("failed to check conflicts").WithKey("errors.unknown")
	}
	if hasConflict {
		return nil, nil, apierror.Conflict("employee already has an assignment during this time").WithKey("errors.conflict")
	}

	// Check employee doesn't have active leave on shift date
	hasLeave, err := s.leaveRepo.HasActiveLeave(ctx, tenantID, empID, shift.Date, shift.Date)
	if err != nil {
		logger.WithError(err).Error("failed to check active leave")
		return nil, nil, apierror.Internal("failed to check leave").WithKey("errors.unknown")
	}
	if hasLeave {
		return nil, nil, apierror.Conflict("employee has active leave during this time").WithKey("errors.conflict")
	}

	// ── Rule Engine evaluation ──────────────────────────────────────────────
	var violations []model.RuleViolation
	if s.ruleEngine != nil {
		violations, err = s.ruleEngine.EvaluateAssignment(ctx, tenantID, shift, emp)
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

	if err := s.assignRepo.Create(ctx, assignment); err != nil {
		logger.WithError(err).Error("failed to create assignment")
		return nil, nil, apierror.Internal("failed to create assignment").WithKey("errors.unknown")
	}

	// Publish event
	s.emitter.Publish(event.Event{
		Type:     event.TypeAssignmentCreated,
		TenantID: tenantID,
		UserID:   ctxutil.GetUserID(ctx),
		Payload:  assignment,
	})

	return assignment, violations, nil
}

// GetAssignmentsByDateRange retrieves all assignments for a tenant within a date range.
func (s *ScheduleService) GetAssignmentsByDateRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]*model.ShiftAssignment, error) {
	logger := ctxutil.GetLogger(ctx)
	assignments, err := s.assignRepo.ListByDateRange(ctx, tenantID, from, to)
	if err != nil {
		logger.WithError(err).Error("failed to list assignments by date range")
		return nil, apierror.Internal("failed to list assignments").WithKey("errors.unknown")
	}
	return assignments, nil
}

// GetAssignments retrieves all assignments for a shift.
func (s *ScheduleService) GetAssignments(ctx context.Context, tenantID, shiftID uuid.UUID) ([]*model.ShiftAssignment, error) {
	logger := ctxutil.GetLogger(ctx)
	assignments, err := s.assignRepo.ListByShift(ctx, tenantID, shiftID)
	if err != nil {
		logger.WithError(err).Error("failed to get assignments")
		return nil, apierror.Internal("failed to get assignments").WithKey("errors.unknown")
	}
	return assignments, nil
}

// DeleteAssignment cancels and removes a shift assignment.
func (s *ScheduleService) DeleteAssignment(ctx context.Context, tenantID, assignmentID uuid.UUID) error {
	logger := ctxutil.GetLogger(ctx)

	assignment, err := s.assignRepo.GetByID(ctx, tenantID, assignmentID)
	if err != nil {
		logger.WithError(err).Error("failed to get assignment")
		return apierror.Internal("failed to get assignment").WithKey("errors.unknown")
	}
	if assignment == nil {
		return apierror.NotFound("assignment", assignmentID.String()).WithKey("errors.unknown")
	}

	if err := s.assignRepo.Delete(ctx, tenantID, assignmentID); err != nil {
		logger.WithError(err).Error("failed to delete assignment")
		return apierror.Internal("failed to delete assignment").WithKey("errors.unknown")
	}

	s.emitter.Publish(event.Event{
		Type:     event.TypeAssignmentUpdated,
		TenantID: tenantID,
		UserID:   ctxutil.GetUserID(ctx),
		Payload:  map[string]interface{}{"id": assignmentID, "action": "deleted"},
	})

	return nil
}

// ResetWeekAssignments deletes all assignments for the week that starts on weekStart.
// weekStart must be a Monday; the function deletes assignments from weekStart to weekStart+6 days inclusive.
// Returns the number of deleted rows.
func (s *ScheduleService) ResetWeekAssignments(ctx context.Context, tenantID uuid.UUID, weekStart time.Time) (int64, error) {
	logger := ctxutil.GetLogger(ctx)

	// Normalise to midnight UTC and derive end of week (Sunday).
	from := weekStart.UTC().Truncate(24 * time.Hour)
	to := from.AddDate(0, 0, 6)

	deleted, err := s.assignRepo.DeleteByDateRange(ctx, tenantID, from, to)
	if err != nil {
		logger.WithError(err).Error("failed to reset week assignments")
		return 0, apierror.Internal("failed to reset week assignments").WithKey("errors.unknown")
	}

	s.emitter.Publish(event.Event{
		Type:     event.TypeAssignmentUpdated,
		TenantID: tenantID,
		UserID:   ctxutil.GetUserID(ctx),
		Payload:  map[string]interface{}{"action": "reset_week", "week_start": from.Format("2006-01-02"), "deleted": deleted},
	})

	return deleted, nil
}

// GenerateICS generates an iCalendar string for employee shifts.
func (s *ScheduleService) GenerateICS(ctx context.Context, tenantID, employeeID uuid.UUID, from, to time.Time) (string, error) {
	logger := ctxutil.GetLogger(ctx)

	// Get employee to find store/name
	emp, err := s.empRepo.GetByID(ctx, tenantID, employeeID)
	if err != nil {
		logger.WithError(err).Error("failed to get employee")
		return "", apierror.Internal("failed to get employee").WithKey("errors.unknown")
	}
	if emp == nil {
		return "", apierror.NotFound("employee", employeeID.String()).WithKey("errors.unknown")
	}

	// Get assignments for the employee in the date range
	assignments, err := s.assignRepo.ListByEmployee(ctx, tenantID, employeeID, from, to)
	if err != nil {
		logger.WithError(err).Error("failed to get assignments")
		return "", apierror.Internal("failed to get assignments").WithKey("errors.unknown")
	}

	// Batch-load all required shifts in one query.
	var shiftByID map[uuid.UUID]*model.ShiftInstance
	if len(assignments) > 0 {
		ids := make([]uuid.UUID, 0, len(assignments))
		seen := make(map[uuid.UUID]bool, len(assignments))
		for _, a := range assignments {
			if !seen[a.ShiftInstanceID] {
				ids = append(ids, a.ShiftInstanceID)
				seen[a.ShiftInstanceID] = true
			}
		}
		batchShifts, err := s.shiftRepo.ListByIDs(ctx, tenantID, ids)
		if err != nil {
			logger.WithError(err).Error("failed to batch-load shifts for ICS")
			return "", apierror.Internal("failed to load shifts").WithKey("errors.unknown")
		}
		shiftByID = make(map[uuid.UUID]*model.ShiftInstance, len(batchShifts))
		for _, sh := range batchShifts {
			shiftByID[sh.ID] = sh
		}
	}

	// Build ICS string
	var icsBuilder strings.Builder
	icsBuilder.WriteString("BEGIN:VCALENDAR\r\n")
	icsBuilder.WriteString("VERSION:2.0\r\n")
	icsBuilder.WriteString("PRODID:-//ParaShift//WFM//EN\r\n")
	icsBuilder.WriteString("CALSCALE:GREGORIAN\r\n")
	icsBuilder.WriteString("METHOD:PUBLISH\r\n")

	for _, assignment := range assignments {
		shift, ok := shiftByID[assignment.ShiftInstanceID]
		if !ok {
			continue
		}

		// Format dates/times for iCalendar: YYYYMMDDTHHMMSS
		dateStr := shift.Date.Format("20060102")
		startTimeStr := strings.ReplaceAll(shift.StartTime, ":", "")
		endTimeStr := strings.ReplaceAll(shift.EndTime, ":", "")

		dtStart := fmt.Sprintf("%sT%s00", dateStr, startTimeStr)
		dtEnd := fmt.Sprintf("%sT%s00", dateStr, endTimeStr)

		icsBuilder.WriteString("BEGIN:VEVENT\r\n")
		icsBuilder.WriteString(fmt.Sprintf("UID:%s@parashift\r\n", assignment.ID))
		icsBuilder.WriteString(fmt.Sprintf("DTSTART:%s\r\n", dtStart))
		icsBuilder.WriteString(fmt.Sprintf("DTEND:%s\r\n", dtEnd))
		icsBuilder.WriteString("SUMMARY:Work Shift\r\n")
		icsBuilder.WriteString("END:VEVENT\r\n")
	}

	icsBuilder.WriteString("END:VCALENDAR\r\n")

	return icsBuilder.String(), nil
}

// ShiftWithAssignment is a combined view of a shift and its assignment for schedule display.
type ShiftWithAssignment struct {
	ShiftInstanceID uuid.UUID `json:"shift_instance_id"`
	Date            time.Time `json:"date"`
	StartTime       string    `json:"start_time"`
	EndTime         string    `json:"end_time"`
	Role            string    `json:"role"`
	Status          string    `json:"status"`
	AssignedAt      time.Time `json:"assigned_at"`
}

// PublishSchedule marks all shifts in the given date range as PUBLISHED, making
// them visible to employees. Returns the number of shifts transitioned.
func (s *ScheduleService) PublishSchedule(ctx context.Context, tenantID uuid.UUID, from, to time.Time) (int64, error) {
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

// ListMyShifts returns enriched shift+assignment data for an employee in a date range.
// Shift instances are batch-loaded to avoid N+1 queries.
func (s *ScheduleService) ListMyShifts(ctx context.Context, tenantID, employeeID uuid.UUID, from, to time.Time) ([]*ShiftWithAssignment, error) {
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

	// Index shifts by ID for O(1) lookup.
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
