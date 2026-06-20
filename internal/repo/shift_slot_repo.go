package repo

import (
	"context"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
)

// shiftSlotRepository embeds the generic tenant repository for
// Create/GetByID/Delete (ARC-2) and adds list queries.
type shiftSlotRepository struct {
	TenantRepository[model.ShiftSlot]
}

// NewShiftSlotRepository creates a new ShiftSlotRepository.
func NewShiftSlotRepository(db *gorm.DB) ShiftSlotRepository {
	return &shiftSlotRepository{NewTenantRepository[model.ShiftSlot](db)}
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
