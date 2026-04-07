package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
)

type aiInsightRepository struct{ db *gorm.DB }

// NewAIInsightRepository creates a new AIInsightRepository backed by Postgres.
func NewAIInsightRepository(db *gorm.DB) AIInsightRepository {
	return &aiInsightRepository{db: db}
}

func (r *aiInsightRepository) List(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]*model.AIInsight, int64, error) {
	var insights []*model.AIInsight
	var total int64

	q := r.db.WithContext(ctx).Model(&model.AIInsight{}).
		Where("tenant_id = ? AND dismissed = false", tenantID)

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&insights).Error; err != nil {
		return nil, 0, err
	}
	return insights, total, nil
}

func (r *aiInsightRepository) Create(ctx context.Context, i *model.AIInsight) error {
	return r.db.WithContext(ctx).Create(i).Error
}

func (r *aiInsightRepository) Dismiss(ctx context.Context, tenantID, id uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&model.AIInsight{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Update("dismissed", true)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("insight not found")
	}
	return nil
}
