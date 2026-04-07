package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
)

type ruleRepository struct{ db *gorm.DB }

// NewRuleRepository creates a new RuleRepository backed by Postgres.
func NewRuleRepository(db *gorm.DB) RuleRepository {
	return &ruleRepository{db: db}
}

func (r *ruleRepository) List(ctx context.Context, tenantID uuid.UUID) ([]*model.Rule, error) {
	var rules []*model.Rule
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at ASC").
		Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

func (r *ruleRepository) ListEnabled(ctx context.Context, tenantID uuid.UUID) ([]*model.Rule, error) {
	var rules []*model.Rule
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND enabled = true", tenantID).
		Order("created_at ASC").
		Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

func (r *ruleRepository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.Rule, error) {
	var rule model.Rule
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&rule).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &rule, nil
}

func (r *ruleRepository) Create(ctx context.Context, rule *model.Rule) error {
	return r.db.WithContext(ctx).Create(rule).Error
}

func (r *ruleRepository) Update(ctx context.Context, rule *model.Rule) error {
	return r.db.WithContext(ctx).Save(rule).Error
}

func (r *ruleRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&model.Rule{}).Error
}
