package dto

import (
	"time"

	"github.com/google/uuid"
)

// ── Shift Instances — HTTP request types (canonical API) ──────────────────────

// CreateShiftRequest is the canonical HTTP request DTO for creating a shift.
// Date is an ISO 8601 string (YYYY-MM-DD); the service layer parses it.
type CreateShiftRequest struct {
	Date          string     `json:"date"       binding:"required"` // YYYY-MM-DD
	StartTime     string     `json:"start_time" binding:"required"` // HH:MM
	EndTime       string     `json:"end_time"   binding:"required"` // HH:MM
	Role          string     `json:"role"       binding:"required"`
	RequiredCount *int       `json:"required_count"` // defaults to 1
	TemplateID    *uuid.UUID `json:"template_id"`
}

// UpdateShiftRequest is the canonical HTTP request DTO for partially updating a shift.
// All fields are optional; only non-nil fields are applied.
type UpdateShiftRequest struct {
	StartTime     *string `json:"start_time"`
	EndTime       *string `json:"end_time"`
	Role          *string `json:"role"`
	RequiredCount *int    `json:"required_count"`
	Status        *string `json:"status"` // DRAFT|PUBLISHED
}

// ── Shift Instances — internal service-layer request types ───────────────────
//
// The service and handler layers were designed before the canonical API
// was formalised. These types remain as the internal contract between
// handler → service. They will be unified with the canonical types in a
// future refactor that migrates the service to string dates.

// CreateShiftInstanceRequest is used by ScheduleService.CreateShift.
// Handlers that use binding:"required" on time.Time rely on gin's time
// parsing — keep this type until the service layer is migrated.
type CreateShiftInstanceRequest struct {
	Date                  time.Time  `json:"date" binding:"required"`
	StartTime             string     `json:"start_time" binding:"required"`
	EndTime               string     `json:"end_time" binding:"required"`
	Role                  *string    `json:"role"`
	RequiredQualification *string    `json:"required_qualification"`
	Source                *string    `json:"source"` // defaults to "MANUAL"
	SourceTemplateID      *uuid.UUID `json:"source_template_id"`
}

// UpdateShiftInstanceRequest is used by ScheduleService.UpdateShift.
// All fields are optional pointers.
type UpdateShiftInstanceRequest struct {
	Date                  *time.Time `json:"date"`
	StartTime             *string    `json:"start_time"`
	EndTime               *string    `json:"end_time"`
	Role                  *string    `json:"role"`
	RequiredQualification *string    `json:"required_qualification"`
	Source                *string    `json:"source"`
	SourceTemplateID      *uuid.UUID `json:"source_template_id"`
}

// ── Shift Instances — HTTP response type ──────────────────────────────────────

// ShiftInstanceResponse is the canonical HTTP response shape for a shift instance.
// Field names match the OpenAPI spec and the TypeScript canonical types exactly.
type ShiftInstanceResponse struct {
	ID            uuid.UUID  `json:"id"`
	StoreID       uuid.UUID  `json:"store_id"`
	Date          string     `json:"date"`       // YYYY-MM-DD
	StartTime     string     `json:"start_time"` // HH:MM
	EndTime       string     `json:"end_time"`   // HH:MM
	Role          string     `json:"role"`
	RequiredCount int        `json:"required_count"`
	Status        string     `json:"status"` // DRAFT|PUBLISHED
	Source        string     `json:"source"` // MANUAL|TEMPLATE|OVERRIDE
	TemplateID    *uuid.UUID `json:"template_id,omitempty"`
	NeedsCover    bool       `json:"needs_cover"`
	CreatedAt     string     `json:"created_at"`
	UpdatedAt     string     `json:"updated_at"`
}

// ── Assignments ───────────────────────────────────────────────────────────────

// CreateAssignmentRequest is the DTO for assigning an employee to a shift.
// Used by both the HTTP handler and the service layer.
type CreateAssignmentRequest struct {
	ShiftID    uuid.UUID `json:"shift_id"    binding:"required"`
	EmployeeID uuid.UUID `json:"employee_id" binding:"required"`
}

// AssignmentResponse is the canonical HTTP response shape for an assignment.
//
// Denormalized shift-time fields (ShiftDate, ShiftStartTime, ShiftEndTime) are
// included so callers never need a follow-up request to render the assignment.
//
// Violations is non-empty only when the assignment was persisted despite one or
// more WARNING/INFO rule violations.  BLOCKING violations prevent persistence
// entirely and are returned as a 422 ValidationErrorResponse instead.
type AssignmentResponse struct {
	ID             uuid.UUID          `json:"id"`
	StoreID        uuid.UUID          `json:"store_id"`
	ShiftID        uuid.UUID          `json:"shift_id"`
	EmployeeID     uuid.UUID          `json:"employee_id"`
	Status         string             `json:"status"`           // confirmed|pending|cancelled
	ShiftDate      string             `json:"shift_date"`       // YYYY-MM-DD
	ShiftStartTime string             `json:"shift_start_time"` // HH:MM
	ShiftEndTime   string             `json:"shift_end_time"`   // HH:MM
	Violations     []RuleViolationDTO `json:"violations"`
	CreatedAt      string             `json:"created_at"`
	UpdatedAt      string             `json:"updated_at"`
}

// RuleViolationDTO is the DTO representation of a rule violation.
//
// Unlike model.RuleViolation (which carries only rule-level context),
// RuleViolationDTO includes ShiftID and EmployeeID so the frontend can map
// violations to calendar cells without reconstructing composite keys.
type RuleViolationDTO struct {
	RuleID     uuid.UUID `json:"rule_id"`
	RuleType   string    `json:"rule_type"`
	Severity   string    `json:"severity"` // BLOCKING|WARNING|INFO
	Message    string    `json:"message"`
	ShiftID    uuid.UUID `json:"shift_id"`
	EmployeeID uuid.UUID `json:"employee_id"`
}

// ValidationErrorResponse is returned as HTTP 422 when an assignment is
// rejected by one or more BLOCKING rule violations.
type ValidationErrorResponse struct {
	Code       string             `json:"code"` // always "RULE_VIOLATION"
	Message    string             `json:"message"`
	Violations []RuleViolationDTO `json:"violations"`
}

// ── Schedule generation (internal / template-driven) ─────────────────────────

// GenerateScheduleRequest is the DTO for generating shift instances from
// week templates. This is an internal manager action, not part of the
// assignment flow.
type GenerateScheduleRequest struct {
	StoreID  uuid.UUID `json:"store_id"`
	DateFrom time.Time `json:"-"` // parsed from date_from string in handler
	DateTo   time.Time `json:"-"` // parsed from date_to string in handler
}
