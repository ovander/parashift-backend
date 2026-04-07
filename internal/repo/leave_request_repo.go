package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
)

// leaveRequestRepository implements LeaveRequestRepository.
type leaveRequestRepository struct {
	db *gorm.DB
}

// NewLeaveRequestRepository creates a new leave request repository.
func NewLeaveRequestRepository(db *gorm.DB) LeaveRequestRepository {
	return &leaveRequestRepository{db: db}
}

// GetByID retrieves a leave request by tenant and ID.
func (r *leaveRequestRepository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.LeaveRequest, error) {
	var leaveRequest model.LeaveRequest
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&leaveRequest).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &leaveRequest, nil
}

// ListByEmployee retrieves leave requests for an employee with pagination.
func (r *leaveRequestRepository) ListByEmployee(ctx context.Context, tenantID, employeeID uuid.UUID, page, pageSize int) ([]*model.LeaveRequest, int64, error) {
	var leaveRequests []*model.LeaveRequest
	var total int64

	if err := r.db.WithContext(ctx).
		Model(&model.LeaveRequest{}).
		Where("tenant_id = ? AND employee_id = ?", tenantID, employeeID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND employee_id = ?", tenantID, employeeID).
		Offset(offset).
		Limit(pageSize).
		Find(&leaveRequests).Error; err != nil {
		return nil, 0, err
	}

	return leaveRequests, total, nil
}

// ListByStore retrieves leave requests for a tenant with pagination, optionally filtered by status.
func (r *leaveRequestRepository) ListByStore(ctx context.Context, tenantID uuid.UUID, status string, page, pageSize int) ([]*model.LeaveRequest, int64, error) {
	var leaveRequests []*model.LeaveRequest
	var total int64

	query := r.db.WithContext(ctx).
		Model(&model.LeaveRequest{}).
		Where("tenant_id = ?", tenantID)

	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).
		Limit(pageSize).
		Find(&leaveRequests).Error; err != nil {
		return nil, 0, err
	}

	return leaveRequests, total, nil
}

// HasActiveLeave checks if an employee has any approved leaves overlapping a date range.
func (r *leaveRequestRepository) HasActiveLeave(ctx context.Context, tenantID, employeeID uuid.UUID, from, to time.Time) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.LeaveRequest{}).
		Where("tenant_id = ? AND employee_id = ? AND status = ? AND start_date <= ? AND end_date >= ?",
			tenantID, employeeID, model.LeaveStatusApproved, to, from).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Create creates a new leave request.
func (r *leaveRequestRepository) Create(ctx context.Context, lr *model.LeaveRequest) error {
	return r.db.WithContext(ctx).Create(lr).Error
}

// Update updates an existing leave request.
func (r *leaveRequestRepository) Update(ctx context.Context, lr *model.LeaveRequest) error {
	return r.db.WithContext(ctx).Save(lr).Error
}

// Delete removes a leave request by tenant and ID.
func (r *leaveRequestRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&model.LeaveRequest{}).Error
}
