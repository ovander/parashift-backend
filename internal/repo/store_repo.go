package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
)

// storeRepository implements StoreRepository.
type storeRepository struct {
	db *gorm.DB
}

// NewStoreRepository creates a new store repository.
func NewStoreRepository(db *gorm.DB) StoreRepository {
	return &storeRepository{db: db}
}

// GetByID retrieves a store by its ID.
func (r *storeRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Store, error) {
	var store model.Store
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&store).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &store, nil
}

// List retrieves all stores with pagination.
func (r *storeRepository) List(ctx context.Context, page, pageSize int) ([]*model.Store, int64, error) {
	var stores []*model.Store
	var total int64

	if err := r.db.WithContext(ctx).Model(&model.Store{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := r.db.WithContext(ctx).Offset(offset).Limit(pageSize).Find(&stores).Error; err != nil {
		return nil, 0, err
	}

	return stores, total, nil
}

// Create creates a new store.
func (r *storeRepository) Create(ctx context.Context, s *model.Store) error {
	return r.db.WithContext(ctx).Create(s).Error
}

// Update updates an existing store.
func (r *storeRepository) Update(ctx context.Context, s *model.Store) error {
	return r.db.WithContext(ctx).Save(s).Error
}

// Delete deletes a store by ID.
func (r *storeRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Store{}, "id = ?", id).Error
}
