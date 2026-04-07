package model

import (
	"github.com/google/uuid"
)

// Contract represents an employment contract with hours and type information.
// The unique index enforces one active contract per employee (employee IDs are globally unique UUIDs).
type Contract struct {
	TenantScoped
	EmployeeID  uuid.UUID `gorm:"type:uuid;not null;uniqueIndex"`
	Type        string    `gorm:"not null"` // full-time|part-time
	WeeklyHours float64   `gorm:"not null"`
}
