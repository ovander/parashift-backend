package model

import "github.com/google/uuid"

const (
	// SourceSlot indicates the shift was generated from a store-level ShiftSlot template.
	SourceSlot = "SLOT"
)

// ShiftSlot is a store-level recurring shift template slot used to define the
// standard shift pattern for an A or B schedule week. Each slot describes one
// shift occurrence (day + time + required role) within a scheme week.
type ShiftSlot struct {
	TenantScoped
	Scheme                  string     `gorm:"not null"` // A|B
	DayOfWeek               int        `gorm:"not null"` // 1=Monday … 7=Sunday (matches frontend convention)
	StartTime               string     `gorm:"not null"` // HH:MM
	EndTime                 string     `gorm:"not null"` // HH:MM
	RequiredRole            string     `gorm:"not null"` // e.g. "pharmacist"
	RequiredQualificationID *uuid.UUID `gorm:"type:uuid"`
}
