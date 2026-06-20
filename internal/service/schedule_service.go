package service

import (
	"context"
	"fmt"
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

// TxRunner executes fn inside a single DB transaction, passing tx-bound repos.
// *repo.RepoBundle.WithTx satisfies this; tests can supply a fake.
type TxRunner func(ctx context.Context, fn func(tx *repo.RepoBundle) error) error

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
	txRunner       TxRunner                      // optional — nil falls back to sequential writes
	emitter        *event.Emitter
	logger         *logrus.Entry
}

// WithTxRunner attaches a transaction runner so multi-step schedule operations
// (generate / regenerate / reset) commit or roll back atomically (DAT-2).
func (s *ScheduleService) WithTxRunner(r TxRunner) *ScheduleService {
	s.txRunner = r
	return s
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
	return blockIfHoliday(ctx, s.holidaySvc, s.logger, date)
}

// blockIfHoliday returns a BadRequest error when date is a public holiday. It is
// a package helper so both ScheduleService (shift create/update) and the
// assignment service can gate on holidays without duplicating the logic. A nil
// holidaySvc or a lookup error is non-fatal (the action proceeds).
func blockIfHoliday(ctx context.Context, holidaySvc *PublicHolidayService, logger *logrus.Entry, date time.Time) error {
	if holidaySvc == nil {
		return nil
	}
	isHoliday, name, err := holidaySvc.IsHoliday(ctx, date, DefaultZone)
	if err != nil {
		logger.WithError(err).Warn("holiday check failed — proceeding without block")
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

// newProjectionEngine builds a projection engine from the service's current
// dependencies (ARC-1). Built per call so it always reflects optional deps wired
// via the With* setters after construction.
func (s *ScheduleService) newProjectionEngine() *projectionEngine {
	return &projectionEngine{
		shiftRepo:     s.shiftRepo,
		assignRepo:    s.assignRepo,
		tmplRepo:      s.tmplRepo,
		empRepo:       s.empRepo,
		leaveRepo:     s.leaveRepo,
		availRepo:     s.availRepo,
		storeRepo:     s.storeRepo,
		exceptionRepo: s.exceptionRepo,
		holidaySvc:    s.holidaySvc,
		txRunner:      s.txRunner,
		emitter:       s.emitter,
		logger:        s.logger,
	}
}

// ProjectABSchedule generates ShiftInstances from WeekTemplates for all employees
// in the store. Delegated to the extracted projection engine (ARC-1).
func (s *ScheduleService) ProjectABSchedule(ctx context.Context, tenantID uuid.UUID, req dto.GenerateScheduleRequest) (int, error) {
	return s.newProjectionEngine().Project(ctx, tenantID, req)
}

// RegenerateWeek hard-resets a week: deletes ALL shifts and assignments for the
// week (regardless of source), then re-projects from A/B templates. Delegated to
// the extracted projection engine (ARC-1).
func (s *ScheduleService) RegenerateWeek(ctx context.Context, tenantID uuid.UUID, weekStart time.Time) (int, error) {
	return s.newProjectionEngine().RegenerateWeek(ctx, tenantID, weekStart)
}

// GetSchedule retrieves shifts for a store within a date range with pagination.
// If no shifts exist for the requested period, it automatically projects the
// employees' A/B week templates so the planner always shows a populated week
// without requiring a manual "generate" step.
// GetSchedule is a READ-ONLY query: it returns the shifts for the range and
// never mutates the database (DAT-3). Generating a schedule for an empty week is
// an explicit action via POST .../shifts/generate (ProjectABSchedule). Removing
// the previous auto-projection eliminates write-on-GET and the concurrent-GET
// race that produced duplicate shifts.
func (s *ScheduleService) GetSchedule(ctx context.Context, tenantID uuid.UUID, dateFrom, dateTo time.Time, page, pageSize int) ([]*model.ShiftInstance, int64, error) {
	logger := ctxutil.GetLogger(ctx)

	shifts, total, err := s.shiftRepo.ListByDateRange(ctx, tenantID, dateFrom, dateTo, page, pageSize)
	if err != nil {
		logger.WithError(err).Error("failed to get schedule")
		return nil, 0, apierror.Internal("failed to get schedule").WithKey("errors.unknown")
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
		return nil, optimisticErr(err, apierror.Internal("failed to update shift").WithKey("errors.unknown"))
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
// newAssignmentService builds an assignment service from the service's current
// dependencies (ARC-1), so it reflects optional deps wired via the With* setters.
func (s *ScheduleService) newAssignmentService() *assignmentService {
	return &assignmentService{
		shiftRepo:  s.shiftRepo,
		assignRepo: s.assignRepo,
		empRepo:    s.empRepo,
		leaveRepo:  s.leaveRepo,
		ruleEngine: s.ruleEngine,
		holidaySvc: s.holidaySvc,
		emitter:    s.emitter,
		logger:     s.logger,
	}
}

// CreateAssignment assigns an employee to a shift and runs configured scheduling
// rules. Delegated to the extracted assignment service (ARC-1).
func (s *ScheduleService) CreateAssignment(ctx context.Context, tenantID uuid.UUID, req dto.CreateAssignmentRequest) (*model.ShiftAssignment, []model.RuleViolation, error) {
	return s.newAssignmentService().Create(ctx, tenantID, req)
}

// GetAssignmentsByDateRange retrieves all assignments for a tenant within a date range.
func (s *ScheduleService) GetAssignmentsByDateRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]*model.ShiftAssignment, error) {
	return s.newAssignmentService().ListByDateRange(ctx, tenantID, from, to)
}

// GetAssignments retrieves all assignments for a shift.
func (s *ScheduleService) GetAssignments(ctx context.Context, tenantID, shiftID uuid.UUID) ([]*model.ShiftAssignment, error) {
	return s.newAssignmentService().ListByShift(ctx, tenantID, shiftID)
}

// DeleteAssignment cancels and removes a shift assignment.
func (s *ScheduleService) DeleteAssignment(ctx context.Context, tenantID, assignmentID uuid.UUID) error {
	return s.newAssignmentService().Delete(ctx, tenantID, assignmentID)
}

// ResetWeekAssignments deletes all assignments for the week starting on weekStart.
func (s *ScheduleService) ResetWeekAssignments(ctx context.Context, tenantID uuid.UUID, weekStart time.Time) (int64, error) {
	return s.newAssignmentService().ResetWeek(ctx, tenantID, weekStart)
}

// GenerateICS generates an iCalendar string for employee shifts.
func (s *ScheduleService) GenerateICS(ctx context.Context, tenantID, employeeID uuid.UUID, from, to time.Time) (string, error) {
	// Delegated to the extracted ICS exporter component (ARC-1).
	return newICSExporter(s.empRepo, s.assignRepo, s.shiftRepo).
		Generate(ctx, tenantID, employeeID, from, to)
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
