package dto

import (
	"github.com/google/uuid"
)

// OpeningHourSlot is used in store DTOs.
type OpeningHourSlot struct {
	DayOfWeek int    `json:"day_of_week"`
	OpenTime  string `json:"open_time"`
	CloseTime string `json:"close_time"`
}

// CreateStoreRequest is the DTO for creating a new store.
type CreateStoreRequest struct {
	Name         string             `json:"name" binding:"required"`
	OpeningHours []OpeningHourSlot  `json:"opening_hours"`
	Timezone     string             `json:"timezone" binding:"required"`
}

// UpdateStoreRequest is the DTO for updating an existing store.
type UpdateStoreRequest struct {
	Name         *string            `json:"name"`
	OpeningHours []OpeningHourSlot  `json:"opening_hours"`
	Timezone     *string            `json:"timezone"`
}

// StoreResponse is the DTO for returning store information.
type StoreResponse struct {
	ID           uuid.UUID         `json:"id"`
	Name         string            `json:"name"`
	OpeningHours []OpeningHourSlot `json:"opening_hours"`
	Timezone     string            `json:"timezone"`
	CreatedAt    string            `json:"created_at"`
	UpdatedAt    string            `json:"updated_at"`
}
