package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
)

// CreateShiftSlotRequest is the HTTP request body for POST /templates.
type CreateShiftSlotRequest struct {
	Scheme       string `json:"scheme"`        // A|B
	DayOfWeek    int    `json:"day_of_week"`   // 1=Monday … 7=Sunday
	StartTime    string `json:"start_time"`    // HH:MM
	EndTime      string `json:"end_time"`      // HH:MM
	RequiredRole string `json:"required_role"` // e.g. "pharmacist"
}

// ShiftSlotResponse is the canonical HTTP response shape for a ShiftSlot.
type ShiftSlotResponse struct {
	ID           uuid.UUID `json:"id"`
	StoreID      uuid.UUID `json:"store_id"`
	Scheme       string    `json:"scheme"`
	DayOfWeek    int       `json:"day_of_week"`
	StartTime    string    `json:"start_time"`
	EndTime      string    `json:"end_time"`
	RequiredRole string    `json:"required_role"`
	CreatedAt    string    `json:"created_at"`
	UpdatedAt    string    `json:"updated_at"`
}

// ToShiftSlotResponse converts a model.ShiftSlot to a ShiftSlotResponse.
func ToShiftSlotResponse(s *model.ShiftSlot) ShiftSlotResponse {
	return ShiftSlotResponse{
		ID:           s.ID,
		StoreID:      s.TenantID,
		Scheme:       s.Scheme,
		DayOfWeek:    s.DayOfWeek,
		StartTime:    s.StartTime,
		EndTime:      s.EndTime,
		RequiredRole: s.RequiredRole,
		CreatedAt:    s.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    s.UpdatedAt.Format(time.RFC3339),
	}
}
