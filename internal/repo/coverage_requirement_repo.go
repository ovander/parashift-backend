package repo

import (
	"context"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
)

// coverageRequirementRepository embeds the generic tenant repository for
// Create/GetByID/Delete (ARC-2) and adds list + update.
type coverageRequirementRepository struct {
	TenantRepository[model.CoverageRequirement]
}

// NewCoverageRequirementRepository creates a new coverage requirement repository.
func NewCoverageRequirementRepository(db *gorm.DB) CoverageRequirementRepository {
	return &coverageRequirementRepository{NewTenantRepository[model.CoverageRequirement](db)}
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

// Update updates an existing coverage requirement.
func (r *coverageRequirementRepository) Update(ctx context.Context, cr *model.CoverageRequirement) error {
	return r.db.WithContext(ctx).Save(cr).Error
}
