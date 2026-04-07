package model

import (
	"github.com/google/uuid"
	"time"
)

const (
	// SourceTemplate indicates the shift was generated from a template
	SourceTemplate = "TEMPLATE"
	// SourceOverride indicates the shift is an override/exception
	SourceOverride = "OVERRIDE"
	// SourceManual indicates the shift was manually created
	SourceManual = "MANUAL"
)

const (
	// ShiftStatusDraft indicates the shift is in draft (not yet visible to employees).
	ShiftStatusDraft = "DRAFT"
	// ShiftStatusPublished indicates the shift is published and visible to employees.
	ShiftStatusPublished = "PUBLISHED"
)

// ShiftInstance represents a specific shift on a given date and time.
type ShiftInstance struct {
	TenantScoped
	Date                  time.Time  `gorm:"type:date;not null;index"`
	StartTime             string     `gorm:"not null"` // HH:MM
	EndTime               string     `gorm:"not null"` // HH:MM
	Role                  string     // role required for this shift
	RequiredQualification string     // e.g. "pharmacist"
	Source                string     `gorm:"not null;default:'MANUAL'"` // TEMPLATE|OVERRIDE|MANUAL
	SourceTemplateID      *uuid.UUID `gorm:"type:uuid"`
	// Status tracks whether the shift is visible to employees.
	Status string `gorm:"not null;default:'DRAFT'"` // DRAFT|PUBLISHED
	// NeedsCover is set when an assignment for this shift was cancelled due to leave,
	// signalling to the planner that a replacement needs to be found.
	NeedsCover bool `gorm:"not null;default:false"`
}
