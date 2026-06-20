package repo

import (
	"context"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
)

// ruleRepository embeds the generic tenant repository for Create/GetByID/Delete
// (ARC-2) and adds rule-specific queries.
type ruleRepository struct {
	TenantRepository[model.Rule]
}

// NewRuleRepository creates a new RuleRepository backed by Postgres.
func NewRuleRepository(db *gorm.DB) RuleRepository {
	return &ruleRepository{NewTenantRepository[model.Rule](db)}
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

func (r *ruleRepository) Update(ctx context.Context, rule *model.Rule) error {
	return r.db.WithContext(ctx).Save(rule).Error
}
