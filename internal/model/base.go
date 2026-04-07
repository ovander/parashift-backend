package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

// TenantScoped is the base struct for all tenant-scoped entities.
// It includes a TenantID field for multi-tenant support.
type TenantScoped struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID  uuid.UUID      `gorm:"type:uuid;not null;index"`
	CreatedAt time.Time      `gorm:"not null"`
	UpdatedAt time.Time      `gorm:"not null"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
