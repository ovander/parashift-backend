package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
)

// schedulePlanRepository implements SchedulePlanRepository.
type schedulePlanRepository struct {
	db *gorm.DB
}

// NewSchedulePlanRepository creates a new schedule plan repository.
func NewSchedulePlanRepository(db *gorm.DB) SchedulePlanRepository {
	return &schedulePlanRepository{db: db}
}

// GetByWeekStart retrieves a plan by store and week start date.
func (r *schedulePlanRepository) GetByWeekStart(ctx context.Context, tenantID uuid.UUID, weekStart time.Time) (*model.SchedulePlan, error) {
	var plan model.SchedulePlan
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND week_start = ?", tenantID, weekStart).
		First(&plan).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &plan, nil
}

// GetByID retrieves a plan by tenant and ID.
func (r *schedulePlanRepository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.SchedulePlan, error) {
	var plan model.SchedulePlan
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&plan).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &plan, nil
}

// Create creates a new plan.
func (r *schedulePlanRepository) Create(ctx context.Context, p *model.SchedulePlan) error {
	return r.db.WithContext(ctx).Create(p).Error
}

// Update updates an existing plan.
func (r *schedulePlanRepository) Update(ctx context.Context, p *model.SchedulePlan) error {
	return r.db.WithContext(ctx).Save(p).Error
}
