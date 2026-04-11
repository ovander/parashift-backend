package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/event"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/repo"
	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
)

// AvailabilityService handles employee availability declarations.
type AvailabilityService struct {
	repo    repo.AvailabilityRepository
	empRepo repo.EmployeeRepository
	emitter *event.Emitter
	logger  *logrus.Entry
}

// NewAvailabilityService creates a new AvailabilityService.
func NewAvailabilityService(repo repo.AvailabilityRepository, empRepo repo.EmployeeRepository, emitter *event.Emitter, logger *logrus.Entry) *AvailabilityService {
	return &AvailabilityService{
		repo:    repo,
		empRepo: empRepo,
		emitter: emitter,
		logger:  logger,
	}
}

// SetAvailability upserts availability for an employee on a specific date.
func (s *AvailabilityService) SetAvailability(ctx context.Context, tenantID, employeeID uuid.UUID, req dto.SetAvailabilityRequest) (*model.Availability, error) {
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

	// Validate time ranges
	var timeRanges []model.TimeRange
	for _, tr := range req.TimeRanges {
		if tr.Start == "" || tr.End == "" {
			return nil, apierror.BadRequest("time range start and end are required").WithKey("errors.invalidInput")
		}
		timeRanges = append(timeRanges, model.TimeRange{
			Start: tr.Start,
			End:   tr.End,
		})
	}

	// Serialize time ranges to JSON
	timeRangesJSON, err := json.Marshal(timeRanges)
	if err != nil {
		logger.WithError(err).Error("failed to marshal time ranges")
		return nil, apierror.Internal("failed to set availability").WithKey("errors.unknown")
	}

	// Dereference optional Note pointer
	note := ""
	if req.Note != nil {
		note = *req.Note
	}

	availability := &model.Availability{
		TenantScoped: model.TenantScoped{
			ID:        uuid.New(),
			TenantID:  tenantID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		EmployeeID: employeeID,
		Date:       req.Date, // already time.Time from DTO
		TimeRanges: datatypes.JSON(timeRangesJSON),
		Note:       note,
	}

	if err := s.repo.Upsert(ctx, availability); err != nil {
		logger.WithError(err).Error("failed to upsert availability")
		return nil, apierror.Internal("failed to set availability").WithKey("errors.unknown")
	}

	return availability, nil
}

// GetAvailability retrieves availability for an employee on a specific date.
func (s *AvailabilityService) GetAvailability(ctx context.Context, tenantID, employeeID uuid.UUID, date time.Time) (*model.Availability, error) {
	logger := ctxutil.GetLogger(ctx)

	availability, err := s.repo.GetByEmployeeDate(ctx, tenantID, employeeID, date)
	if err != nil {
		logger.WithError(err).Error("failed to get availability")
		return nil, apierror.Internal("failed to get availability").WithKey("errors.unknown")
	}

	if availability == nil {
		return nil, apierror.NotFound("availability", date.Format("2006-01-02")).WithKey("errors.unknown")
	}

	return availability, nil
}

// ListAvailability retrieves availability records for an employee within a date range.
func (s *AvailabilityService) ListAvailability(ctx context.Context, tenantID, employeeID uuid.UUID, from, to time.Time) ([]*model.Availability, error) {
	logger := ctxutil.GetLogger(ctx)

	availabilities, err := s.repo.ListByEmployee(ctx, tenantID, employeeID, from, to)
	if err != nil {
		logger.WithError(err).Error("failed to list availability")
		return nil, apierror.Internal("failed to list availability").WithKey("errors.unknown")
	}

	return availabilities, nil
}
