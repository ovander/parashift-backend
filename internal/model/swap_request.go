package model

import (
	"github.com/google/uuid"
)

const (
	// SwapStatusPending indicates the swap request is pending a response
	SwapStatusPending = "pending"
	// SwapStatusAccepted indicates the swap request has been accepted
	SwapStatusAccepted = "accepted"
	// SwapStatusRejected indicates the swap request has been rejected
	SwapStatusRejected = "rejected"
	// SwapStatusCancelled indicates the swap request has been cancelled
	SwapStatusCancelled = "cancelled"
)

// SwapRequest represents a request to swap shifts between employees or with an open slot.
type SwapRequest struct {
	TenantScoped
	RequesterID       uuid.UUID  `gorm:"type:uuid;not null;index"`
	TargetEmployeeID  *uuid.UUID `gorm:"type:uuid"` // null means swap with an open slot
	ShiftInstanceID   uuid.UUID  `gorm:"type:uuid;not null"`
	TargetShiftID     *uuid.UUID `gorm:"type:uuid"` // the shift being offered/requested
	Status            string     `gorm:"not null;default:'pending'"` // pending|accepted|rejected|cancelled
	Note              string
	ReviewedBy        *uuid.UUID `gorm:"type:uuid"`
}
