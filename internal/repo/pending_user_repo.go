package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PendingUserRepository defines operations for pending user management.
type PendingUserRepository interface {
	// Upsert inserts a pending user record, or does nothing if the auth_id already exists.
	Upsert(ctx context.Context, authID string) error
	// List returns all pending users ordered by creation date.
	List(ctx context.Context) ([]*model.PendingUser, error)
	// Delete removes a pending user by ID.
	Delete(ctx context.Context, id uuid.UUID) error
	// Count returns the total number of pending users.
	Count(ctx context.Context) (int64, error)
}

type pendingUserRepository struct {
	db *gorm.DB
}

func NewPendingUserRepository(db *gorm.DB) PendingUserRepository {
	return &pendingUserRepository{db: db}
}

func (r *pendingUserRepository) Upsert(ctx context.Context, authID string) error {
	record := &model.PendingUser{
		ID:        uuid.New(),
		AuthID:    authID,
		CreatedAt: time.Now(),
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "auth_id"}}, DoNothing: true}).
		Create(record).Error
}

func (r *pendingUserRepository) List(ctx context.Context) ([]*model.PendingUser, error) {
	var users []*model.PendingUser
	if err := r.db.WithContext(ctx).Order("created_at asc").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *pendingUserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.PendingUser{}, "id = ?", id).Error
}

func (r *pendingUserRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.PendingUser{}).Count(&count).Error
	return count, err
}
