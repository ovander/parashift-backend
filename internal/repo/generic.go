package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TenantRepository is a generic base for tenant-scoped repositories (ARC-2). It
// provides the create/read-by-id/delete boilerplate that was previously copied
// across ~20 repositories; concrete repositories embed it and add their own
// bespoke queries. Behaviour matches the hand-written versions exactly:
//   - GetByID scopes by tenant_id + id and returns (nil, nil) when not found.
//   - Delete scopes by tenant_id + id (soft delete via the model's DeletedAt).
//
// Update is intentionally not provided here: write semantics differ per model
// (plain save vs. optimistic locking, see ARC-3), so each repository keeps its
// own Update.
type TenantRepository[T any] struct {
	db *gorm.DB
}

// NewTenantRepository builds a generic tenant-scoped repository over T.
func NewTenantRepository[T any](db *gorm.DB) TenantRepository[T] {
	return TenantRepository[T]{db: db}
}

// Create persists a new entity.
func (r TenantRepository[T]) Create(ctx context.Context, e *T) error {
	return r.db.WithContext(ctx).Create(e).Error
}

// GetByID returns the entity with the given id within the tenant, or (nil, nil)
// when no such row exists.
func (r TenantRepository[T]) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*T, error) {
	var out T
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&out).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

// Delete removes the entity with the given id within the tenant.
func (r TenantRepository[T]) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	var zero T
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&zero).Error
}
