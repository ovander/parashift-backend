package model

import (
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"time"
)

// TimeRange represents a time slot with start and end times.
type TimeRange struct {
	Start string `json:"start"` // HH:MM
	End   string `json:"end"`   // HH:MM
}

// Availability represents an employee's available time slots for a specific date.
type Availability struct {
	TenantScoped
	EmployeeID uuid.UUID      `gorm:"type:uuid;not null;index"`
	Date       time.Time      `gorm:"type:date;not null;index"`
	TimeRanges datatypes.JSON `gorm:"type:jsonb"` // []TimeRange
	Note       string
}
