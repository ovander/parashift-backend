package repo

import (
	"context"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
)

// auditLogRepository implements AuditLogRepository.
type auditLogRepository struct {
	db *gorm.DB
}

// NewAuditLogRepository creates a new audit log repository.
func NewAuditLogRepository(db *gorm.DB) AuditLogRepository {
	return &auditLogRepository{db: db}
}

// List retrieves paginated audit logs for a tenant with optional filters.
func (r *auditLogRepository) List(ctx context.Context, tenantID uuid.UUID, filter AuditLogFilter, page, pageSize int) ([]*model.AuditLog, int64, error) {
	var logs []*model.AuditLog
	var total int64

	q := r.db.WithContext(ctx).Model(&model.AuditLog{}).Where("tenant_id = ?", tenantID)

	if filter.Action != "" {
		q = q.Where("action = ?", filter.Action)
	}
	if filter.ResourceType != "" {
		q = q.Where("resource_type = ?", filter.ResourceType)
	}
	if filter.ActorID != nil {
		q = q.Where("actor_id = ?", *filter.ActorID)
	}
	if filter.From != nil {
		q = q.Where("created_at >= ?", *filter.From)
	}
	if filter.To != nil {
		q = q.Where("created_at <= ?", *filter.To)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
