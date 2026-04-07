package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
)

type shiftSlotRepository struct {
	db *gorm.DB
}

// NewShiftSlotRepository creates a new ShiftSlotRepository.
func NewShiftSlotRepository(db *gorm.DB) ShiftSlotRepository {
	return &shiftSlotRepository{db: db}
}

func (r *shiftSlotRepository) List(ctx context.Context, tenantID uuid.UUID) ([]*model.ShiftSlot, error) {
	var slots []*model.ShiftSlot
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("scheme, day_of_week, start_time").
		Find(&slots).Error; err != nil {
		return nil, err
	}
	return slots, nil
}

func (r *shiftSlotRepository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.ShiftSlot, error) {
	var slot model.ShiftSlot
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&slot).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &slot, nil
}

func (r *shiftSlotRepository) ListByScheme(ctx context.Context, tenantID uuid.UUID, scheme string) ([]*model.ShiftSlot, error) {
	var slots []*model.ShiftSlot
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND scheme = ?", tenantID, scheme).
		Order("day_of_week, start_time").
		Find(&slots).Error; err != nil {
		return nil, err
	}
	return slots, nil
}

func (r *shiftSlotRepository) Create(ctx context.Context, slot *model.ShiftSlot) error {
	return r.db.WithContext(ctx).Create(slot).Error
}

func (r *shiftSlotRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&model.ShiftSlot{}).Error
}
