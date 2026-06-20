package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
)

// swapRequestRepository implements SwapRequestRepository.
type swapRequestRepository struct {
	db *gorm.DB
}

// NewSwapRequestRepository creates a new swap request repository.
func NewSwapRequestRepository(db *gorm.DB) SwapRequestRepository {
	return &swapRequestRepository{db: db}
}

// GetByID retrieves a swap request by tenant and ID.
func (r *swapRequestRepository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.SwapRequest, error) {
	var swapRequest model.SwapRequest
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&swapRequest).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &swapRequest, nil
}

// ListByEmployee retrieves swap requests for an employee with pagination.
func (r *swapRequestRepository) ListByEmployee(ctx context.Context, tenantID, employeeID uuid.UUID, page, pageSize int) ([]*model.SwapRequest, int64, error) {
	var swapRequests []*model.SwapRequest
	var total int64

	if err := r.db.WithContext(ctx).
		Model(&model.SwapRequest{}).
		Where("tenant_id = ? AND requester_id = ?", tenantID, employeeID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND requester_id = ?", tenantID, employeeID).
		Offset(offset).
		Limit(pageSize).
		Find(&swapRequests).Error; err != nil {
		return nil, 0, err
	}

	return swapRequests, total, nil
}

// ListByStore retrieves swap requests for a tenant with pagination, optionally filtered by status.
func (r *swapRequestRepository) ListByStore(ctx context.Context, tenantID uuid.UUID, status string, page, pageSize int) ([]*model.SwapRequest, int64, error) {
	var swapRequests []*model.SwapRequest
	var total int64

	query := r.db.WithContext(ctx).
		Model(&model.SwapRequest{}).
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
		Find(&swapRequests).Error; err != nil {
		return nil, 0, err
	}

	return swapRequests, total, nil
}

// Create creates a new swap request.
func (r *swapRequestRepository) Create(ctx context.Context, sr *model.SwapRequest) error {
	return r.db.WithContext(ctx).Create(sr).Error
}

// Update updates an existing swap request with optimistic-lock guarding (ARC-3).
func (r *swapRequestRepository) Update(ctx context.Context, sr *model.SwapRequest) error {
	return updateOptimistic(r.db, ctx, sr)
}
