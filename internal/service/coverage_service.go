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

const (
	CoverageOK           = "OK"
	CoverageUnderstaffed = "UNDERSTAFFED"
	CoverageOverstaffed  = "OVERSTAFFED"
	CoverageMissingRole  = "MISSING_ROLE"
)

// CoverageService computes staffing coverage against requirements.
type CoverageService struct {
	coverageReqRepo repo.CoverageRequirementRepository
	shiftRepo       repo.ShiftInstanceRepository
	assignRepo      repo.ShiftAssignmentRepository
	empRepo         repo.EmployeeRepository
	emitter         *event.Emitter
	logger          *logrus.Entry
	holidaySvc      *PublicHolidayService         // optional; when set, public holidays are skipped
	exceptionRepo   repo.StoreExceptionRepository // optional; EXTRA_OPEN exceptions override holiday skips
}

// NewCoverageService creates a new CoverageService.
func NewCoverageService(
	coverageReqRepo repo.CoverageRequirementRepository,
	shiftRepo repo.ShiftInstanceRepository,
	assignRepo repo.ShiftAssignmentRepository,
	empRepo repo.EmployeeRepository,
	emitter *event.Emitter,
	logger *logrus.Entry,
) *CoverageService {
	return &CoverageService{
		coverageReqRepo: coverageReqRepo,
		shiftRepo:       shiftRepo,
		assignRepo:      assignRepo,
		empRepo:         empRepo,
		emitter:         emitter,
		logger:          logger,
	}
}

// WithHolidayService attaches a PublicHolidayService so that public holidays are
// excluded from coverage gap reporting. Follows the same optional-inject pattern
// used by ScheduleService.
func (s *CoverageService) WithHolidayService(svc *PublicHolidayService) *CoverageService {
	s.holidaySvc = svc
	return s
}

// WithStoreExceptionRepo attaches a StoreExceptionRepository so that EXTRA_OPEN
// exceptions can override the public-holiday skip — a store that opens exceptionally
// on a holiday should still have coverage gaps reported.
func (s *CoverageService) WithStoreExceptionRepo(r repo.StoreExceptionRepository) *CoverageService {
	s.exceptionRepo = r
	return s
}

// derefString safely dereferences a *string, returning "" if nil.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ListRequirements retrieves all coverage requirements for a store.
func (s *CoverageService) ListRequirements(ctx context.Context, tenantID uuid.UUID) ([]*model.CoverageRequirement, error) {
	logger := ctxutil.GetLogger(ctx)
	reqs, err := s.coverageReqRepo.List(ctx, tenantID)
	if err != nil {
		logger.WithError(err).Error("failed to list coverage requirements")
		return nil, apierror.Internal("failed to list requirements").WithKey("errors.unknown")
	}
	return reqs, nil
}

// CreateRequirement creates a new coverage requirement.
func (s *CoverageService) CreateRequirement(ctx context.Context, tenantID uuid.UUID, req dto.CoverageRequirementRequest) (*model.CoverageRequirement, error) {
	logger := ctxutil.GetLogger(ctx)

	if req.DayOfWeek < 0 || req.DayOfWeek > 6 {
		return nil, apierror.BadRequest("day_of_week must be 0-6").WithKey("errors.invalidInput")
	}
	if req.StartTime == "" || req.EndTime == "" {
		return nil, apierror.BadRequest("start_time and end_time are required").WithKey("errors.missingParams")
	}
	if req.MinStaff < 1 {
		return nil, apierror.BadRequest("min_staff must be at least 1").WithKey("errors.invalidInput")
	}

	cr := &model.CoverageRequirement{
		TenantScoped: model.TenantScoped{
			ID:        uuid.New(),
			TenantID:  tenantID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		DayOfWeek:    req.DayOfWeek,
		StartTime:    req.StartTime,
		EndTime:      req.EndTime,
		MinStaff:     req.MinStaff,
		RequiredRole: derefString(req.RequiredRole), // *string → string
	}

	if err := s.coverageReqRepo.Create(ctx, cr); err != nil {
		logger.WithError(err).Error("failed to create coverage requirement")
		return nil, apierror.Internal("failed to create requirement").WithKey("errors.unknown")
	}

	return cr, nil
}

// UpdateRequirement updates an existing coverage requirement.
func (s *CoverageService) UpdateRequirement(ctx context.Context, tenantID, id uuid.UUID, req dto.CoverageRequirementRequest) (*model.CoverageRequirement, error) {
	logger := ctxutil.GetLogger(ctx)

	if req.DayOfWeek < 0 || req.DayOfWeek > 6 {
		return nil, apierror.BadRequest("day_of_week must be 0-6").WithKey("errors.invalidInput")
	}
	if req.StartTime == "" || req.EndTime == "" {
		return nil, apierror.BadRequest("start_time and end_time are required").WithKey("errors.missingParams")
	}
	if req.MinStaff < 1 {
		return nil, apierror.BadRequest("min_staff must be at least 1").WithKey("errors.invalidInput")
	}

	// List and find by ID (no dedicated GetByID in repo interface)
	reqs, err := s.coverageReqRepo.List(ctx, tenantID)
	if err != nil {
		logger.WithError(err).Error("failed to list coverage requirements")
		return nil, apierror.Internal("failed to get requirement").WithKey("errors.unknown")
	}

	var cr *model.CoverageRequirement
	for _, r := range reqs {
		if r.ID == id {
			cr = r
			break
		}
	}

	if cr == nil {
		return nil, apierror.NotFound("coverage requirement", id.String()).WithKey("errors.unknown")
	}

	cr.DayOfWeek = req.DayOfWeek
	cr.StartTime = req.StartTime
	cr.EndTime = req.EndTime
	cr.MinStaff = req.MinStaff
	cr.RequiredRole = derefString(req.RequiredRole) // *string → string
	cr.UpdatedAt = time.Now()

	if err := s.coverageReqRepo.Update(ctx, cr); err != nil {
		logger.WithError(err).Error("failed to update coverage requirement")
		return nil, apierror.Internal("failed to update requirement").WithKey("errors.unknown")
	}

	return cr, nil
}

// DeleteRequirement deletes a coverage requirement.
func (s *CoverageService) DeleteRequirement(ctx context.Context, tenantID, id uuid.UUID) error {
	logger := ctxutil.GetLogger(ctx)

	if err := s.coverageReqRepo.Delete(ctx, tenantID, id); err != nil {
		logger.WithError(err).Error("failed to delete coverage requirement")
		return apierror.Internal("failed to delete requirement").WithKey("errors.unknown")
	}

	return nil
}

// ComputeForDateRange computes coverage report for a date range. Implements CoverageChecker interface.
// Employees are pre-loaded once per call to avoid N+1 queries when checking RequiredRole.
func (s *CoverageService) ComputeForDateRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time) (dto.CoverageReport, error) {
	logger := ctxutil.GetLogger(ctx)

	requirements, err := s.coverageReqRepo.List(ctx, tenantID)
	if err != nil {
		logger.WithError(err).Error("failed to list coverage requirements")
		return dto.CoverageReport{}, apierror.Internal("failed to compute coverage").WithKey("errors.unknown")
	}

	// Check whether any requirement uses RequiredRole so we can skip the employee
	// pre-load when it is not needed.
	needsRoleCheck := false
	for _, req := range requirements {
		if req.RequiredRole != "" {
			needsRoleCheck = true
			break
		}
	}

	// Pre-load employees into a map (id → employee) to avoid N+1 in the inner loop.
	employeeByID := make(map[uuid.UUID]*model.Employee)
	if needsRoleCheck {
		page, pageSize := 1, 1000
		for {
			batch, _, err := s.empRepo.List(ctx, tenantID, page, pageSize)
			if err != nil {
				logger.WithError(err).Error("failed to pre-load employees for coverage")
				return dto.CoverageReport{}, apierror.Internal("failed to compute coverage").WithKey("errors.unknown")
			}
			for _, e := range batch {
				employeeByID[e.ID] = e
			}
			if len(batch) < pageSize {
				break
			}
			page++
		}
		for id, e := range employeeByID {
			logger.Infof("coverage: loaded employee id=%s name=%q job_role=%q", id, e.Name, e.JobRole)
		}
	}

	// Pre-load EXTRA_OPEN exceptions for the whole range so that a single date
	// lookup is O(1) inside the loop rather than one DB query per day.
	extraOpenDates := make(map[string]bool)
	if s.exceptionRepo != nil && s.holidaySvc != nil {
		exceptions, exErr := s.exceptionRepo.ListByDateRange(ctx, tenantID, from, to)
		if exErr != nil {
			logger.WithError(exErr).Warn("coverage: failed to pre-load store exceptions — holiday override disabled for this run")
		} else {
			for _, ex := range exceptions {
				if ex.Type == model.ExceptionExtraOpen {
					extraOpenDates[ex.Date.Format("2006-01-02")] = true
				}
			}
		}
	}

	var slots []dto.CoverageSlot
	gapCount := 0

	currentDate := from
	for !currentDate.After(to) {
		// Skip public holidays — no coverage gap should be reported on a holiday —
		// UNLESS an EXTRA_OPEN store exception overrides it for that date.
		if s.holidaySvc != nil {
			isHoliday, holidayName, hErr := s.holidaySvc.IsHoliday(ctx, currentDate, DefaultZone)
			if hErr != nil {
				logger.WithError(hErr).Warn("holiday check failed during coverage computation — proceeding for this day")
			} else if isHoliday && !extraOpenDates[currentDate.Format("2006-01-02")] {
				logger.Debugf("coverage: skipping public holiday %s (%s)", currentDate.Format("2006-01-02"), holidayName)
				currentDate = currentDate.AddDate(0, 0, 1)
				continue
			} else if isHoliday {
				logger.Debugf("coverage: public holiday %s (%s) overridden by EXTRA_OPEN exception — computing coverage", currentDate.Format("2006-01-02"), holidayName)
			}
		}

		dayOfWeek := int(currentDate.Weekday())

		dayShifts, err := s.shiftRepo.ListByDate(ctx, tenantID, currentDate)
		if err != nil {
			logger.WithError(err).Error("failed to list shifts by date")
			return dto.CoverageReport{}, apierror.Internal("failed to compute coverage").WithKey("errors.unknown")
		}

		for _, req := range requirements {
			if req.DayOfWeek != dayOfWeek {
				continue
			}

			assignedCount := 0
			rolePresent := false
			// seenEmployees prevents counting the same person twice when they hold
			// multiple shifts that all overlap this coverage window (e.g. a split shift).
			seenEmployees := make(map[uuid.UUID]bool)

			for _, shift := range dayShifts {
				if !timeOverlaps(shift.StartTime, shift.EndTime, req.StartTime, req.EndTime) {
					continue
				}

				assignments, err := s.assignRepo.ListByShift(ctx, tenantID, shift.ID)
				if err != nil {
					continue
				}

				for _, assign := range assignments {
					if assign.Status != model.AssignmentStatusConfirmed {
						logger.Debugf("coverage: skip assignment %s — status=%s (not confirmed)", assign.ID, assign.Status)
						continue
					}
					if seenEmployees[assign.EmployeeID] {
						continue // already counted this person for this coverage slot
					}
					seenEmployees[assign.EmployeeID] = true
					assignedCount++

					// Track required job role presence using pre-loaded employee map.
					if req.RequiredRole != "" {
						emp, ok := employeeByID[assign.EmployeeID]
						if !ok {
							logger.Warnf("coverage: employee %s NOT FOUND in pre-loaded map (req.RequiredRole=%q) — employee may belong to different tenant", assign.EmployeeID, req.RequiredRole)
						} else {
							logger.Infof("coverage: role check — emp=%s job_role=%q req.RequiredRole=%q match=%v",
							assign.EmployeeID, emp.JobRole, req.RequiredRole, emp.JobRole == req.RequiredRole)
							if emp.JobRole == req.RequiredRole {
								rolePresent = true
							}
						}
					}
				}
			}

			logger.Infof("coverage: slot date=%s dow=%d req=[%s-%s role=%q minStaff=%d] assigned=%d rolePresent=%v",
				currentDate.Format("2006-01-02"), req.DayOfWeek, req.StartTime, req.EndTime, req.RequiredRole, req.MinStaff, assignedCount, rolePresent)

			// Determine coverage status
			missingRole := req.RequiredRole != "" && !rolePresent
			var status string
			switch {
			case missingRole:
				status = CoverageMissingRole
				gapCount++
			case assignedCount < req.MinStaff:
				status = CoverageUnderstaffed
				gapCount++
			case assignedCount > req.MinStaff:
				status = CoverageOverstaffed
			default:
				status = CoverageOK
			}

			slots = append(slots, dto.CoverageSlot{
				Date:          currentDate.Format("2006-01-02"),
				StartTime:     req.StartTime,
				EndTime:       req.EndTime,
				Status:        status,
				AssignedCount: assignedCount,
				RequiredCount: req.MinStaff,
				MissingRole:   missingRole,
				RequiredRole:  req.RequiredRole,
			})
		}

		currentDate = currentDate.AddDate(0, 0, 1)
	}

	return dto.CoverageReport{
		Items:      slots,
		TotalSlots: len(slots),
		GapCount:   gapCount,
	}, nil
}

// GetGaps returns only slots with status != OK.
func (s *CoverageService) GetGaps(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]dto.CoverageSlot, error) {
	report, err := s.ComputeForDateRange(ctx, tenantID, from, to)
	if err != nil {
		return nil, err
	}

	var gaps []dto.CoverageSlot
	for _, slot := range report.Items { // was report.Slots (int) — now report.Items ([]CoverageSlot)
		if slot.Status != CoverageOK {
			gaps = append(gaps, slot)
		}
	}

	return gaps, nil
}

// timeToMinutes converts an HH:MM string to minutes since midnight.
// Returns -1 if the string is malformed.
func timeToMinutes(hhmm string) int {
	if len(hhmm) != 5 || hhmm[2] != ':' {
		return -1
	}
	h := int(hhmm[0]-'0')*10 + int(hhmm[1]-'0')
	m := int(hhmm[3]-'0')*10 + int(hhmm[4]-'0')
	return h*60 + m
}

// timeOverlaps checks if two time ranges overlap.
// It correctly handles overnight shifts where endTime < startTime (crosses midnight).
func timeOverlaps(startA, endA, startB, endB string) bool {
	sA, eA := timeToMinutes(startA), timeToMinutes(endA)
	sB, eB := timeToMinutes(startB), timeToMinutes(endB)
	if sA < 0 || eA < 0 || sB < 0 || eB < 0 {
		// Fallback to string comparison if times are malformed.
		return startA < endB && endA > startB
	}

	// Normalise overnight shifts: if end < start, add 1440 (24h) to end.
	if eA <= sA {
		eA += 1440
	}
	if eB <= sB {
		eB += 1440
	}

	return sA < eB && eA > sB
}

