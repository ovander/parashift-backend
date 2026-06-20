package model

import (
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"time"
)

// Employee represents a pharmacy employee with contract and schedule information.
type Employee struct {
	TenantScoped
	Name        string         `gorm:"not null"`
	Position    string         `gorm:"not null;default:'employee'"` // manager|employee — controls RBAC access
	JobRole     string         `gorm:"not null;default:''"` // pharmacist|animator|logistics_agent|... — controls shift eligibility
	ContractID  *uuid.UUID     `gorm:"type:uuid"`
	StartDate   time.Time      `gorm:"not null"` // A/B week anchor date
	Email       string         `gorm:"default:''"` // used to send Socrate invite
	AuthID      string         `gorm:"default:'';index" json:"-"` // Socrate sub claim; secret — never serialize (SEC-2)
	ClaimToken  *string        `gorm:"uniqueIndex" json:"-"`       // one-time invite token; secret — never serialize (SEC-2)
	Locale      string         `gorm:"not null;default:'fr'"` // UI locale — 'fr' | 'en'
	Preferences datatypes.JSON `gorm:"type:jsonb"` // flexible per-employee preferences (shift preferences, notifications, etc.)

	// Virtual fields — populated by JOIN queries, not persisted to DB.
	StoreName string `gorm:"-" json:"-"`
}

// WeekType returns "A" or "B" for the calendar week that contains date.
//
// Algorithm: ISO week numbers (1–52/53) are used to determine A/B parity.
// The employee's startDate fixes which parity is "A": any ISO week that has
// the same odd/even parity as the startDate's ISO week is "A"; the other
// parity is "B". Because every day within a Mon–Sun ISO week shares the same
// week number, all days in a week are guaranteed to get the same type.
func WeekType(date time.Time, startDate time.Time) string {
	// Normalise to UTC midnight.
	d := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	s := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, time.UTC)

	_, dateWeek  := d.ISOWeek()
	_, startWeek := s.ISOWeek()

	if dateWeek%2 == startWeek%2 {
		return "A"
	}
	return "B"
}
