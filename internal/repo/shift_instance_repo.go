package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
)

// SetStatusByDateRange bulk-updates the status for all shifts in the date range.
func (r *shiftInstanceRepository) SetStatusByDateRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time, status string) (int64, error) {
	result := r.db.WithContext(ctx).
		Model(&model.ShiftInstance{}).
		Where("tenant_id = ? AND date >= ? AND date <= ?", tenantID, from, to).
		Updates(map[string]interface{}{"status": status, "updated_at": time.Now()})
	return result.RowsAffected, result.Error
}

// shiftInstanceRepository implements ShiftInstanceRepository.
type shiftInstanceRepository struct {
	db *gorm.DB
}

// NewShiftInstanceRepository creates a new shift instance repository.
func NewShiftInstanceRepository(db *gorm.DB) ShiftInstanceRepository {
	return &shiftInstanceRepository{db: db}
}

// GetByID retrieves a shift by tenant and ID.
func (r *shiftInstanceRepository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.ShiftInstance, error) {
	var shift model.ShiftInstance
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&shift).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &shift, nil
}

// ListByDateRange retrieves shifts within a date range with pagination.
func (r *shiftInstanceRepository) ListByDateRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time, page, pageSize int) ([]*model.ShiftInstance, int64, error) {
	var shifts []*model.ShiftInstance
	var total int64

	if err := r.db.WithContext(ctx).
		Model(&model.ShiftInstance{}).
		Where("tenant_id = ? AND date >= ? AND date <= ?", tenantID, from, to).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND date >= ? AND date <= ?", tenantID, from, to).
		Offset(offset).
		Limit(pageSize).
		Find(&shifts).Error; err != nil {
		return nil, 0, err
	}

	return shifts, total, nil
}

// ListByDate retrieves all shifts for a specific date.
func (r *shiftInstanceRepository) ListByDate(ctx context.Context, tenantID uuid.UUID, date time.Time) ([]*model.ShiftInstance, error) {
	var shifts []*model.ShiftInstance
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND date = ?", tenantID, date).
		Find(&shifts).Error; err != nil {
		return nil, err
	}
	return shifts, nil
}

// Create creates a new shift.
func (r *shiftInstanceRepository) Create(ctx context.Context, s *model.ShiftInstance) error {
	return r.db.WithContext(ctx).Create(s).Error
}

// CreateBatch creates multiple shifts in a batch.
func (r *shiftInstanceRepository) CreateBatch(ctx context.Context, shifts []*model.ShiftInstance) error {
	if len(shifts) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(shifts, 100).Error
}

// Update updates an existing shift.
func (r *shiftInstanceRepository) Update(ctx context.Context, s *model.ShiftInstance) error {
	return r.db.WithContext(ctx).Save(s).Error
}

// Delete deletes a shift by tenant and ID.
func (r *shiftInstanceRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&model.ShiftInstance{}).Error
}

// DeleteByIDs hard-deletes the given shifts (scoped to tenant). No-op for empty ids.
// Used as a scoped compensating delete so only just-created shifts are removed.
func (r *shiftInstanceRepository) DeleteByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Unscoped().
		Where("tenant_id = ? AND id IN ?", tenantID, ids).
		Delete(&model.ShiftInstance{}).Error
}

// DeleteBySourceTemplate hard-deletes all template-sourced shifts within a date range.
// Unscoped() bypasses GORM's soft-delete so rows are physically removed and the
// shift_instance_id FK on shift_assignments cannot be left dangling.
func (r *shiftInstanceRepository) DeleteBySourceTemplate(ctx context.Context, tenantID uuid.UUID, from, to time.Time) error {
	return r.db.WithContext(ctx).Unscoped().
		Where("tenant_id = ? AND source = ? AND date >= ? AND date <= ?", tenantID, model.SourceTemplate, from, to).
		Delete(&model.ShiftInstance{}).Error
}

// DeleteBySlotIDs hard-deletes slot-sourced shifts whose SourceTemplateID is in slotIDs.
func (r *shiftInstanceRepository) DeleteBySlotIDs(ctx context.Context, tenantID uuid.UUID, slotIDs []uuid.UUID, from, to time.Time) error {
	if len(slotIDs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Unscoped().
		Where("tenant_id = ? AND source = ? AND source_template_id IN ? AND date >= ? AND date <= ?",
			tenantID, model.SourceSlot, slotIDs, from, to).
		Delete(&model.ShiftInstance{}).Error
}

// DeleteByDateRange hard-deletes ALL shifts (any source) within a date range.
// Unscoped() ensures physical removal — used by the force-regenerate action.
func (r *shiftInstanceRepository) DeleteByDateRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time) error {
	return r.db.WithContext(ctx).Unscoped().
		Where("tenant_id = ? AND date >= ? AND date <= ?", tenantID, from, to).
		Delete(&model.ShiftInstance{}).Error
}

// ListByIDs retrieves multiple shifts by their IDs in a single query.
func (r *shiftInstanceRepository) ListByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) ([]*model.ShiftInstance, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var shifts []*model.ShiftInstance
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id IN ?", tenantID, ids).
		Find(&shifts).Error; err != nil {
		return nil, err
	}
	return shifts, nil
}
