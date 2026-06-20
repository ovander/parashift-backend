package model

import (
	"github.com/google/uuid"
	"time"
)

const (
	// AssignmentStatusConfirmed indicates the assignment is confirmed
	AssignmentStatusConfirmed = "confirmed"
	// AssignmentStatusCancelled indicates the assignment has been cancelled
	AssignmentStatusCancelled = "cancelled"
	// AssignmentStatusPending indicates the assignment is pending approval
	AssignmentStatusPending = "pending"
)

// ShiftAssignment represents an assignment of an employee to a shift.
//
// ShiftDate, ShiftStartTime, and ShiftEndTime are denormalized copies of the
// corresponding fields from the ShiftInstance. They are populated at creation
// time so the rule engine can calculate real hours without an extra JOIN.
type ShiftAssignment struct {
	TenantScoped
	Versioned
	ShiftInstanceID uuid.UUID `gorm:"type:uuid;not null;index"`
	EmployeeID      uuid.UUID `gorm:"type:uuid;not null;index"`
	Status          string    `gorm:"not null;default:'confirmed'"` // confirmed|cancelled|pending
	AssignedBy      uuid.UUID `gorm:"type:uuid;not null"`
	AssignedAt      time.Time `gorm:"not null"`

	// Denormalized shift time data — allows accurate hours calculation without a JOIN.
	ShiftDate      time.Time `gorm:"type:date;not null;default:'0001-01-01'"`
	ShiftStartTime string    `gorm:"not null;default:''"`
	ShiftEndTime   string    `gorm:"not null;default:''"`
}
