package model

import (
	"github.com/google/uuid"
	"time"
)

const (
	// LeaveTypeVacation indicates a vacation leave
	LeaveTypeVacation = "vacation"
	// LeaveTypeSick indicates a sick leave
	LeaveTypeSick = "sick"
	// LeaveTypeOther indicates other types of leave
	LeaveTypeOther = "other"

	// LeaveStatusPending indicates the leave request is pending approval
	LeaveStatusPending = "pending"
	// LeaveStatusApproved indicates the leave request has been approved
	LeaveStatusApproved = "approved"
	// LeaveStatusRejected indicates the leave request has been rejected
	LeaveStatusRejected = "rejected"
)

// LeaveRequest represents an employee's request for time off.
type LeaveRequest struct {
	TenantScoped
	EmployeeID  uuid.UUID  `gorm:"type:uuid;not null;index"`
	StartDate   time.Time  `gorm:"type:date;not null"`
	EndDate     time.Time  `gorm:"type:date;not null"`
	Type        string     `gorm:"not null"` // vacation|sick|other
	Status      string     `gorm:"not null;default:'pending'"` // pending|approved|rejected
	Reason      string
	ReviewedBy  *uuid.UUID `gorm:"type:uuid"`
	ReviewedAt  *time.Time
}
