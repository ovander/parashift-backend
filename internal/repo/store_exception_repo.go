package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
)

type storeExceptionRepository struct {
	db *gorm.DB
}

// NewStoreExceptionRepository creates a new StoreExceptionRepository.
func NewStoreExceptionRepository(db *gorm.DB) StoreExceptionRepository {
	return &storeExceptionRepository{db: db}
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

func (r *storeExceptionRepository) Create(ctx context.Context, e *model.StoreException) error {
	return r.db.WithContext(ctx).Create(e).Error
}

func (r *storeExceptionRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&model.StoreException{}).Error
}
