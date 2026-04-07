package repo

import (
	"context"
	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
)

// coverageRequirementRepository implements CoverageRequirementRepository.
type coverageRequirementRepository struct {
	db *gorm.DB
}

// NewCoverageRequirementRepository creates a new coverage requirement repository.
func NewCoverageRequirementRepository(db *gorm.DB) CoverageRequirementRepository {
	return &coverageRequirementRepository{db: db}
}

// List retrieves all coverage requirements for a tenant.
func (r *coverageRequirementRepository) List(ctx context.Context, tenantID uuid.UUID) ([]*model.CoverageRequirement, error) {
	var requirements []*model.CoverageRequirement
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Find(&requirements).Error; err != nil {
		return nil, err
	}
	return requirements, nil
}

// Create creates a new coverage requirement.
func (r *coverageRequirementRepository) Create(ctx context.Context, cr *model.CoverageRequirement) error {
	return r.db.WithContext(ctx).Create(cr).Error
}

// Update updates an existing coverage requirement.
func (r *coverageRequirementRepository) Update(ctx context.Context, cr *model.CoverageRequirement) error {
	return r.db.WithContext(ctx).Save(cr).Error
}

// Delete deletes a coverage requirement by tenant and ID.
func (r *coverageRequirementRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&model.CoverageRequirement{}).Error
}
