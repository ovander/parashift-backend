package dto

import (
	"github.com/google/uuid"
)

// CreateSwapRequest is the DTO for creating a shift swap request.
type CreateSwapRequest struct {
	TargetEmployeeID *uuid.UUID `json:"target_employee_id"` // nil for swap with open slot
	ShiftInstanceID  uuid.UUID  `json:"shift_instance_id" binding:"required"`
	TargetShiftID    *uuid.UUID `json:"target_shift_id"`
	Note             *string    `json:"note"`
}

// ReviewSwapRequest is the DTO for reviewing a swap request.
type ReviewSwapRequest struct {
	Status string `json:"status" binding:"required"` // accepted|rejected
}

// SwapRequestResponse is the DTO for returning swap request information.
type SwapRequestResponse struct {
	ID               uuid.UUID  `json:"id"`
	TenantID         uuid.UUID  `json:"tenant_id"`
	RequesterID      uuid.UUID  `json:"requester_id"`
	TargetEmployeeID *uuid.UUID `json:"target_employee_id"`
	ShiftInstanceID  uuid.UUID  `json:"shift_instance_id"`
	TargetShiftID    *uuid.UUID `json:"target_shift_id"`
	Status           string     `json:"status"`
	Note             string     `json:"note"`
	ReviewedBy       *uuid.UUID `json:"reviewed_by"`
	CreatedAt        string     `json:"created_at"`
	UpdatedAt        string     `json:"updated_at"`
}
