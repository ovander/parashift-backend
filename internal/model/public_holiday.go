package model

import "time"

// PublicHoliday represents an official public holiday for a given zone (e.g. "metropole").
// These are not tenant-scoped — they are global reference data fetched from the
// French government API (calendrier.api.gouv.fr) and cached in the database.
type PublicHoliday struct {
	// Date is the holiday date (stored as DATE, no time component).
	Date time.Time `gorm:"primaryKey;type:date"`
	// Zone is the geographic zone, e.g. "metropole", "alsace-moselle", "guadeloupe".
	Zone string `gorm:"primaryKey;not null;default:'metropole'"`
	// Name is the official French name of the holiday, e.g. "1er janvier".
	Name string `gorm:"not null"`
}
