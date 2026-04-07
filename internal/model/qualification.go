package model

import (
	"github.com/google/uuid"
	"time"
)

// Qualification defines a certification or licence that may be required for certain shift roles.
type Qualification struct {
	TenantScoped
	Name            string `gorm:"not null"`
	IssuingBody     string
	RequiredForRole string // if set, auto-required for shifts of this role
}

// EmployeeQualification links an employee to a held qualification.
type EmployeeQualification struct {
	TenantScoped
	EmployeeID      uuid.UUID  `gorm:"type:uuid;not null;index"`
	QualificationID uuid.UUID  `gorm:"type:uuid;not null;index"`
	IssueDate       *time.Time `gorm:"type:date"`
	ExpiryDate      *time.Time `gorm:"type:date"`
	Verified        bool       `gorm:"default:false"`
	DocumentURL     string
}

// IsExpired returns true if ExpiryDate is set and is in the past.
func (eq *EmployeeQualification) IsExpired() bool {
	if eq.ExpiryDate == nil {
		return false
	}
	return eq.ExpiryDate.Before(time.Now())
}

// ExpiresWithinDays returns true if the qualification expires within the given number of days.
func (eq *EmployeeQualification) ExpiresWithinDays(days int) bool {
	if eq.ExpiryDate == nil {
		return false
	}
	return eq.ExpiryDate.Before(time.Now().AddDate(0, 0, days))
}
