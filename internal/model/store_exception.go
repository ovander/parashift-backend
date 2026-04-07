package model

import "time"

const (
	// ExceptionExtraOpen marks a day the store opens exceptionally (e.g. a December Sunday).
	ExceptionExtraOpen = "EXTRA_OPEN"
	// ExceptionForcedClosed marks a day the store is closed despite normally being open
	// (e.g. inventory day, team training).
	ExceptionForcedClosed = "FORCED_CLOSED"
)

// StoreException records a one-off deviation from the store's regular opening schedule.
// It is tenant-scoped and date-specific.
type StoreException struct {
	TenantScoped
	// Date is the affected day.
	Date time.Time `gorm:"type:date;not null;index"`
	// Type is either EXTRA_OPEN or FORCED_CLOSED.
	Type string `gorm:"not null"`
	// Note is an optional human-readable reason (e.g. "Ouverture exceptionnelle Noël").
	Note string
}
