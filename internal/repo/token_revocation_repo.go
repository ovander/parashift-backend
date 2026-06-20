package repo

import (
	"context"
	"errors"
	"time"

	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type tokenRevocationRepository struct {
	db *gorm.DB
}

// NewTokenRevocationRepository creates a new token revocation repository.
func NewTokenRevocationRepository(db *gorm.DB) TokenRevocationRepository {
	return &tokenRevocationRepository{db: db}
}

func (r *tokenRevocationRepository) GetBySub(ctx context.Context, sub string) (*model.TokenRevocation, error) {
	var tr model.TokenRevocation
	if err := r.db.WithContext(ctx).Where("sub = ?", sub).First(&tr).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &tr, nil
}

// Upsert raises the floor for sub. On conflict it only moves revoked_after
// forward (GREATEST) so a stale/older revoke can never lower an existing floor.
func (r *tokenRevocationRepository) Upsert(ctx context.Context, sub string, revokedAfter time.Time) error {
	rec := &model.TokenRevocation{Sub: sub, RevokedAfter: revokedAfter, UpdatedAt: revokedAfter}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "sub"}},
			DoUpdates: clause.Assignments(map[string]any{
				"revoked_after": gorm.Expr("GREATEST(token_revocations.revoked_after, EXCLUDED.revoked_after)"),
				"updated_at":    revokedAfter,
			}),
		}).
		Create(rec).Error
}
