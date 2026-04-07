package model

import (
	"github.com/google/uuid"
	"time"
)

const (
	PlanStateDraft     = "DRAFT"
	PlanStatePublished = "PUBLISHED"
	PlanStateLive      = "LIVE"
	PlanStateArchived  = "ARCHIVED"
)

// OverrideEntry records a post-publication edit for audit purposes.
type OverrideEntry struct {
	ShiftID   uuid.UUID `json:"shift_id"`
	EditorID  uuid.UUID `json:"editor_id"`
	Reason    string    `json:"reason,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// PlanSnapshot captures the summary state of the plan at a publish moment.
type PlanSnapshot struct {
	Version     int       `json:"version"`
	CapturedAt  time.Time `json:"captured_at"`
	ShiftCount  int       `json:"shift_count"`
	PublishedBy uuid.UUID `json:"published_by"`
}

// SchedulePlan represents the week-level planning lifecycle state for a store.
type SchedulePlan struct {
	TenantScoped
	StoreID        uuid.UUID  `gorm:"type:uuid;not null;index"`
	WeekStart      time.Time  `gorm:"type:date;not null;index"` // always a Monday
	State          string     `gorm:"not null;default:'DRAFT'"`
	PublishedAt    *time.Time
	PublishedBy    *uuid.UUID `gorm:"type:uuid"`
	Snapshots      string     `gorm:"type:text;default:'[]'"` // JSON []PlanSnapshot
	OverrideLog    string     `gorm:"type:text;default:'[]'"` // JSON []OverrideEntry
	ModelScheme    string     `gorm:"default:''"`              // optional: "A", "B", or custom name
}
