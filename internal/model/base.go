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

// GetID exposes the primary key (used by the optimistic-update helper).
func (t *TenantScoped) GetID() uuid.UUID { return t.ID }

// Versioned is an opt-in mixin adding optimistic concurrency control (ARC-3).
// Only models whose table carries a `version` column embed it; each versioned
// update bumps the version and guards on the prior value, so a concurrent writer
// working from a stale copy is rejected instead of silently clobbering the row.
type Versioned struct {
	Version int `gorm:"not null;default:0"`
}

// GetVersion returns the current optimistic-lock version.
func (v *Versioned) GetVersion() int { return v.Version }

// SetVersion sets the optimistic-lock version.
func (v *Versioned) SetVersion(n int) { v.Version = n }
