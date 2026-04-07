package model

import (
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"time"
)

// AuditLog records all significant changes in the system.
// Unlike other models, it does NOT embed TenantScoped and is NOT soft-deleted.
type AuditLog struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID     uuid.UUID      `gorm:"type:uuid;not null;index"`
	ActorID      uuid.UUID      `gorm:"type:uuid;not null"`
	Action       string         `gorm:"not null"` // event type string, e.g. "leave.updated"
	ResourceType string         `gorm:"not null;default:''"` // e.g. "leave_request", "shift", "employee"
	ResourceID   string         `gorm:"not null;default:''"` // UUID of the affected resource (as string)
	Before       datatypes.JSON `gorm:"type:jsonb"`
	After        datatypes.JSON `gorm:"type:jsonb"`
	CreatedAt    time.Time      `gorm:"not null"`
}
