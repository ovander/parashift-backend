package dto

import (
	"github.com/google/uuid"
	"time"
)

// TimeSlot represents a time range in an availability request.
type TimeSlot struct {
	Start string `json:"start" binding:"required"` // HH:MM
	End   string `json:"end" binding:"required"`   // HH:MM
}

// SetAvailabilityRequest is the DTO for setting employee availability.
type SetAvailabilityRequest struct {
	EmployeeID uuid.UUID  `json:"employee_id" binding:"required"`
	Date       time.Time  `json:"date" binding:"required"`
	TimeRanges []TimeSlot `json:"time_ranges" binding:"required,min=1"`
	Note       *string    `json:"note"`
}

// AvailabilityResponse is the DTO for returning availability information.
type AvailabilityResponse struct {
	ID         uuid.UUID `json:"id"`
	TenantID   uuid.UUID `json:"tenant_id"`
	EmployeeID uuid.UUID `json:"employee_id"`
	Date       string    `json:"date"`
	TimeRanges []TimeSlot `json:"time_ranges"`
	Note       string    `json:"note"`
	CreatedAt  string    `json:"created_at"`
	UpdatedAt  string    `json:"updated_at"`
}
