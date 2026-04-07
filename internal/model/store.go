package model

import (
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"time"
)

// OpeningHoursSlot represents a store's opening hours for a specific day.
type OpeningHoursSlot struct {
	DayOfWeek int    `json:"day_of_week"` // 0=Sunday, 1=Monday...6=Saturday
	OpenTime  string `json:"open_time"`   // HH:MM
	CloseTime string `json:"close_time"` // HH:MM
}

// Store is the root tenant entity. It does NOT embed TenantScoped.
type Store struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name         string         `gorm:"not null"`
	OpeningHours datatypes.JSON `gorm:"type:jsonb"` // []OpeningHoursSlot
	Timezone     string         `gorm:"not null;default:'Europe/Brussels'"`
	// ABWeekAnchor is the reference date used to compute A/B week types for the
	// entire store. When nil, each employee's own StartDate is used as the anchor.
	ABWeekAnchor *time.Time     `gorm:"type:date"`
	CreatedAt    time.Time      `gorm:"not null"`
	UpdatedAt    time.Time      `gorm:"not null"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}
