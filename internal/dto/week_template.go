package dto

import (
	"github.com/google/uuid"
)

// WeekTemplateEntry represents a single shift entry in a week template.
type WeekTemplateEntry struct {
	WeekType  string `json:"week_type" binding:"required"` // A|B
	DayOfWeek int    `json:"day_of_week" binding:"required,gte=0,lte=6"`
	StartTime string `json:"start_time" binding:"required"`
	EndTime   string `json:"end_time" binding:"required"`
	Role      string `json:"role"` // job role for the slot (e.g. pharmacist_assistant); empty = any role
}

// WeekTemplateResponse is the DTO for returning week template information.
type WeekTemplateResponse struct {
	EmployeeID uuid.UUID            `json:"employee_id"`
	WeekType   string               `json:"week_type"`
	Entries    []WeekTemplateEntry `json:"entries"`
}

// UpsertWeekTemplateRequest is the DTO for upserting week templates for an employee.
type UpsertWeekTemplateRequest struct {
	EmployeeID uuid.UUID            `json:"employee_id" binding:"required"`
	Templates  []WeekTemplateEntry `json:"templates" binding:"required,min=1"`
}
