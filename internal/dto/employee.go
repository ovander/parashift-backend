package dto

import (
	"github.com/google/uuid"
	"time"
)

// CreateEmployeeRequest is the DTO for creating a new employee.
//
// Priority for binding the Socrate identity:
//  1. email    — ParaShift calls InviteUserAsService; Socrate creates the account
//               and sends an invite email. auth_id is derived from the response.
//  2. auth_id  — admin already knows the Socrate sub (numeric string); bound directly.
//  3. neither  — a one-time claim_token is generated as a fallback; admin shares the
//               URL manually.
type CreateEmployeeRequest struct {
	Name      string     `json:"name" binding:"required"`
	Position  string     `json:"position" binding:"required"` // manager|employee — RBAC access level
	JobRole   string     `json:"job_role" binding:"required"` // pharmacist|animator|logistics_agent|... — shift eligibility
	StartDate time.Time  `json:"start_date" binding:"required"`
	Email     string     `json:"email"`    // preferred: triggers Socrate invite
	AuthID    string     `json:"auth_id"`  // fallback: direct sub binding
	StoreID   *uuid.UUID `json:"store_id"` // required for admin cross-tenant creation
}

// UpdateEmployeeRequest is the DTO for updating an existing employee.
type UpdateEmployeeRequest struct {
	Name      *string    `json:"name"`
	Position  *string    `json:"position"` // manager|employee
	JobRole   *string    `json:"job_role"` // pharmacist|animator|logistics_agent|...
	StartDate *time.Time `json:"start_date"`
}

// EmployeeResponse is the DTO for returning employee information.
type EmployeeResponse struct {
	ID         uuid.UUID  `json:"id"`
	TenantID   uuid.UUID  `json:"tenant_id"`
	Name       string     `json:"name"`
	Position   string     `json:"position"`  // manager|employee
	JobRole    string     `json:"job_role"`  // pharmacist|animator|logistics_agent|...
	ContractID *uuid.UUID `json:"contract_id"`
	StartDate  string     `json:"start_date"`
	Email      string     `json:"email,omitempty"`
	AuthID     string     `json:"auth_id"`
	ClaimToken *string    `json:"claim_token,omitempty"` // present only while the invite is unclaimed (no email path)
	StoreName  string     `json:"store_name,omitempty"`  // populated in cross-tenant (admin) responses
	CreatedAt  string     `json:"created_at"`
	UpdatedAt  string     `json:"updated_at"`
}
