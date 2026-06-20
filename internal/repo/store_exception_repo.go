package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
)

// storeExceptionRepository embeds the generic tenant repository for
// Create/Delete (ARC-2) and adds a date-range query.
type storeExceptionRepository struct {
	TenantRepository[model.StoreException]
}

// NewStoreExceptionRepository creates a new StoreExceptionRepository.
func NewStoreExceptionRepository(db *gorm.DB) StoreExceptionRepository {
	return &storeExceptionRepository{NewTenantRepository[model.StoreException](db)}
}

func (r *storeExceptionRepository) ListByDateRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]*model.StoreException, error) {
	var exceptions []*model.StoreException
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND date >= ? AND date <= ?", tenantID, from, to).
		Order("date ASC").
		Find(&exceptions).Error; err != nil {
		return nil, err
	}
	return exceptions, nil
}
