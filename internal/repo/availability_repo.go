package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
)

// availabilityRepository implements AvailabilityRepository.
type availabilityRepository struct {
	db *gorm.DB
}

// NewAvailabilityRepository creates a new availability repository.
func NewAvailabilityRepository(db *gorm.DB) AvailabilityRepository {
	return &availabilityRepository{db: db}
}

// GetByEmployeeDate retrieves availability for an employee on a specific date.
func (r *availabilityRepository) GetByEmployeeDate(ctx context.Context, tenantID, employeeID uuid.UUID, date time.Time) (*model.Availability, error) {
	var availability model.Availability
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND employee_id = ? AND date = ?", tenantID, employeeID, date).
		First(&availability).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &availability, nil
}

// ListByEmployee retrieves all availability records for an employee within a date range.
func (r *availabilityRepository) ListByEmployee(ctx context.Context, tenantID, employeeID uuid.UUID, from, to time.Time) ([]*model.Availability, error) {
	var availabilities []*model.Availability
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND employee_id = ? AND date >= ? AND date <= ?", tenantID, employeeID, from, to).
		Find(&availabilities).Error; err != nil {
		return nil, err
	}
	return availabilities, nil
}

// Upsert inserts or updates an availability record using GORM's Save.
func (r *availabilityRepository) Upsert(ctx context.Context, a *model.Availability) error {
	return r.db.WithContext(ctx).Save(a).Error
}
