package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
)

// shiftAssignmentRepository implements ShiftAssignmentRepository.
type shiftAssignmentRepository struct {
	db *gorm.DB
}

// NewShiftAssignmentRepository creates a new shift assignment repository.
func NewShiftAssignmentRepository(db *gorm.DB) ShiftAssignmentRepository {
	return &shiftAssignmentRepository{db: db}
}

// GetByID retrieves an assignment by tenant and ID.
func (r *shiftAssignmentRepository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.ShiftAssignment, error) {
	var assignment model.ShiftAssignment
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&assignment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &assignment, nil
}

// ListByShift retrieves all assignments for a shift.
func (r *shiftAssignmentRepository) ListByShift(ctx context.Context, tenantID, shiftID uuid.UUID) ([]*model.ShiftAssignment, error) {
	var assignments []*model.ShiftAssignment
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND shift_instance_id = ?", tenantID, shiftID).
		Find(&assignments).Error; err != nil {
		return nil, err
	}
	return assignments, nil
}

// ListByDateRange retrieves active (non-cancelled) assignments for a tenant within a date range (inclusive).
// Cancelled assignments are excluded — they represent removed coverage and should not appear in schedules.
// Uses the denormalized ShiftDate field to avoid a join.
func (r *shiftAssignmentRepository) ListByDateRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]*model.ShiftAssignment, error) {
	var assignments []*model.ShiftAssignment
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND shift_date >= ? AND shift_date <= ? AND status != ?", tenantID, from, to, model.AssignmentStatusCancelled).
		Find(&assignments).Error; err != nil {
		return nil, err
	}
	return assignments, nil
}

// ListByEmployee retrieves all assignments for an employee within a date range.
func (r *shiftAssignmentRepository) ListByEmployee(ctx context.Context, tenantID, employeeID uuid.UUID, from, to time.Time) ([]*model.ShiftAssignment, error) {
	var assignments []*model.ShiftAssignment
	if err := r.db.WithContext(ctx).
		Joins("JOIN shift_instances si ON si.id = shift_assignments.shift_instance_id").
		Where("shift_assignments.tenant_id = ? AND shift_assignments.employee_id = ? AND si.date >= ? AND si.date <= ?", tenantID, employeeID, from, to).
		Find(&assignments).Error; err != nil {
		return nil, err
	}
	return assignments, nil
}

// CountByShift counts confirmed assignments for a shift.
func (r *shiftAssignmentRepository) CountByShift(ctx context.Context, tenantID, shiftID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.ShiftAssignment{}).
		Where("tenant_id = ? AND shift_instance_id = ? AND status = ?", tenantID, shiftID, model.AssignmentStatusConfirmed).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// Create creates a new assignment.
func (r *shiftAssignmentRepository) Create(ctx context.Context, a *model.ShiftAssignment) error {
	return r.db.WithContext(ctx).Create(a).Error
}

// CreateBatch inserts all assignments in a single SQL statement.
func (r *shiftAssignmentRepository) CreateBatch(ctx context.Context, assignments []*model.ShiftAssignment) error {
	if len(assignments) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(assignments).Error
}

// Update updates an existing assignment with optimistic-lock guarding (ARC-3).
func (r *shiftAssignmentRepository) Update(ctx context.Context, a *model.ShiftAssignment) error {
	return updateOptimistic(r.db, ctx, a)
}

// Delete deletes an assignment by tenant and ID.
func (r *shiftAssignmentRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&model.ShiftAssignment{}).Error
}

// DeleteByDateRange hard-deletes all assignments for a tenant whose shift_date falls
// within [from, to] inclusive. Unscoped() ensures physical removal so the unique
// constraint on (shift_instance_id, employee_id, deleted_at) cannot be confused by
// leftover soft-deleted rows. Returns the number of rows deleted.
func (r *shiftAssignmentRepository) DeleteByDateRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Unscoped().
		Where("tenant_id = ? AND shift_date >= ? AND shift_date <= ?", tenantID, from, to).
		Delete(&model.ShiftAssignment{})
	return result.RowsAffected, result.Error
}

// ExistsConflict checks if an employee has conflicting assignments on the same date with overlapping times.
func (r *shiftAssignmentRepository) ExistsConflict(ctx context.Context, tenantID, employeeID uuid.UUID, date time.Time, startTime, endTime string, excludeAssignmentID *uuid.UUID) (bool, error) {
	var count int64

	query := r.db.WithContext(ctx).
		Joins("JOIN shift_instances si ON si.id = shift_assignments.shift_instance_id").
		Where("shift_assignments.tenant_id = ? AND shift_assignments.employee_id = ? AND si.date = ? AND shift_assignments.status != ? AND NOT (si.end_time <= ? OR si.start_time >= ?)",
			tenantID, employeeID, date, model.AssignmentStatusCancelled, startTime, endTime)

	if excludeAssignmentID != nil {
		query = query.Where("shift_assignments.id != ?", *excludeAssignmentID)
	}

	if err := query.Model(&model.ShiftAssignment{}).Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}
