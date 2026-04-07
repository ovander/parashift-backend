package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/repo"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// AdminService exposes admin-only operations such as audit log queries.
type AdminService struct {
	auditRepo repo.AuditLogRepository
	db        *gorm.DB
	logger    *logrus.Entry
}

// NewAdminService creates a new AdminService.
func NewAdminService(auditRepo repo.AuditLogRepository, db *gorm.DB, logger *logrus.Entry) *AdminService {
	return &AdminService{
		auditRepo: auditRepo,
		db:        db,
		logger:    logger,
	}
}

// AdminStats holds cross-tenant platform-wide metrics for the admin dashboard.
type AdminStats struct {
	TotalStores          int64 `json:"total_stores"`
	TotalEmployees       int64 `json:"total_employees"`
	PendingLeaveRequests int64 `json:"pending_leave_requests"`
	PendingSwapRequests  int64 `json:"pending_swap_requests"`
}

// GetStats computes platform-wide counts visible to the super-admin.
func (s *AdminService) GetStats(ctx context.Context) (*AdminStats, error) {
	var stats AdminStats

	if err := s.db.WithContext(ctx).Table("stores").
		Where("deleted_at IS NULL").Count(&stats.TotalStores).Error; err != nil {
		s.logger.WithError(err).Error("failed to count stores")
		return nil, apierror.Internal("failed to get stats")
	}
	if err := s.db.WithContext(ctx).Table("employees").
		Where("deleted_at IS NULL").Count(&stats.TotalEmployees).Error; err != nil {
		s.logger.WithError(err).Error("failed to count employees")
		return nil, apierror.Internal("failed to get stats")
	}
	if err := s.db.WithContext(ctx).Table("leave_requests").
		Where("status = ? AND deleted_at IS NULL", "pending").Count(&stats.PendingLeaveRequests).Error; err != nil {
		s.logger.WithError(err).Error("failed to count pending leave requests")
		return nil, apierror.Internal("failed to get stats")
	}
	if err := s.db.WithContext(ctx).Table("swap_requests").
		Where("status = ? AND deleted_at IS NULL", "pending").Count(&stats.PendingSwapRequests).Error; err != nil {
		s.logger.WithError(err).Error("failed to count pending swap requests")
		return nil, apierror.Internal("failed to get stats")
	}

	return &stats, nil
}

// AuditLogEntry is the enriched view of an audit log record with actor name resolved.
type AuditLogEntry struct {
	*model.AuditLog
	ActorName string `json:"actor_name"` // resolved from employees; empty if actor not found
}

// AuditLogFilter holds optional filters for ListAuditLogs.
type AuditLogFilter struct {
	Action       string
	ResourceType string
	ActorID      *uuid.UUID
	From         *time.Time
	To           *time.Time
	// StoreID overrides the tenant scope for super-admin queries.
	// When set the handler substitutes it for the calling tenant's ID.
	StoreID *uuid.UUID
}

// ListAuditLogs retrieves paginated audit logs for a tenant, enriching each entry
// with the actor's resolved name.
func (s *AdminService) ListAuditLogs(ctx context.Context, tenantID uuid.UUID, filter AuditLogFilter, page, pageSize int) ([]*AuditLogEntry, int64, error) {
	logger := ctxutil.GetLogger(ctx)

	logs, total, err := s.auditRepo.List(ctx, tenantID, repo.AuditLogFilter{
		Action:       filter.Action,
		ResourceType: filter.ResourceType,
		ActorID:      filter.ActorID,
		From:         filter.From,
		To:           filter.To,
	}, page, pageSize)
	if err != nil {
		logger.WithError(err).Error("failed to list audit logs")
		return nil, 0, apierror.Internal("failed to list audit logs")
	}

	actorNames := s.resolveActorNames(ctx, logs)

	entries := make([]*AuditLogEntry, len(logs))
	for i, l := range logs {
		entries[i] = &AuditLogEntry{
			AuditLog:  l,
			ActorName: actorNames[l.ActorID],
		}
	}
	return entries, total, nil
}

// resolveActorNames performs a single IN query to map actor UUIDs → names for the
// current page of logs. Non-fatal: returns empty map on DB error or nil db.
func (s *AdminService) resolveActorNames(ctx context.Context, logs []*model.AuditLog) map[uuid.UUID]string {
	if len(logs) == 0 || s.db == nil {
		return nil
	}

	seen := make(map[uuid.UUID]struct{}, len(logs))
	ids := make([]uuid.UUID, 0, len(logs))
	for _, l := range logs {
		if _, ok := seen[l.ActorID]; !ok {
			seen[l.ActorID] = struct{}{}
			ids = append(ids, l.ActorID)
		}
	}

	type row struct {
		ID   uuid.UUID
		Name string
	}
	var rows []row
	if err := s.db.WithContext(ctx).
		Table("employees").
		Select("id, name").
		Where("id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return nil
	}

	result := make(map[uuid.UUID]string, len(rows))
	for _, r := range rows {
		result[r.ID] = r.Name
	}
	return result
}
