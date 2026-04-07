package repo

import (
	"context"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// planningModelMetricRepository implements PlanningModelMetricRepository.
type planningModelMetricRepository struct {
	db *gorm.DB
}

// NewPlanningModelMetricRepository creates a new planning model metric repository.
func NewPlanningModelMetricRepository(db *gorm.DB) PlanningModelMetricRepository {
	return &planningModelMetricRepository{db: db}
}

// ListByScheme retrieves the most recent N weeks of metrics for a model scheme.
func (r *planningModelMetricRepository) ListByScheme(ctx context.Context, tenantID uuid.UUID, scheme string, limitWeeks int) ([]*model.PlanningModelMetric, error) {
	var metrics []*model.PlanningModelMetric
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND model_scheme = ?", tenantID, scheme).
		Order("week_start DESC").
		Limit(limitWeeks).
		Find(&metrics).Error; err != nil {
		return nil, err
	}
	return metrics, nil
}

// Upsert inserts or updates a metric by (tenant_id, model_scheme, week_start).
func (r *planningModelMetricRepository) Upsert(ctx context.Context, m *model.PlanningModelMetric) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "model_scheme"}, {Name: "week_start"}},
			DoUpdates: clause.AssignmentColumns([]string{"coverage_rate", "overtime_hours", "adjustment_count", "violation_count", "updated_at"}),
		}).
		Create(m).Error
}
