package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
)

// ── Rule CRUD — canonical API types ──────────────────────────────────────────

// UpsertRuleRequest is the canonical HTTP request body for PUT /rules/{id}.
// All fields are required — PUT semantics replace the entire rule definition.
type UpsertRuleRequest struct {
	RuleType      string                 `json:"rule_type"      binding:"required"`
	IsEnabled     bool                   `json:"is_enabled"`
	Severity      string                 `json:"severity"       binding:"required,oneof=BLOCKING WARNING INFO"`
	Configuration map[string]interface{} `json:"configuration"  binding:"required"`
}

// ── Rule CRUD — internal service-layer request types ─────────────────────────
//
// The handler and service layers predate the canonical UpsertRuleRequest.
// These types remain as the internal contract and will be unified in a
// future migration.

// CreateRuleRequest is used by handlers and RuleService.Create.
// Field names use the canonical snake_case names from the frontend Rule type
// (rule_type, is_enabled) so that request and response shapes are consistent.
type CreateRuleRequest struct {
	Type          string                 `json:"rule_type"`               // COVERAGE|ROLE|MAX_HOURS|NO_OVERLAP|MIN_REST
	Configuration map[string]interface{} `json:"configuration"`            // rule-type-specific JSON object
	Severity      string                 `json:"severity"`                 // BLOCKING|WARNING|INFO
	Enabled       *bool                  `json:"is_enabled,omitempty"`
	Description   string                 `json:"description,omitempty"`
}

// UpdateRuleRequest is used by handlers and RuleService.Update.
// All fields are optional — only provided fields are applied.
type UpdateRuleRequest struct {
	Configuration map[string]interface{} `json:"configuration,omitempty"`
	Severity      string                 `json:"severity,omitempty"`
	Enabled       *bool                  `json:"is_enabled,omitempty"`
	Description   string                 `json:"description,omitempty"`
}

// ── Rule HTTP response ────────────────────────────────────────────────────────

// RuleResponse is the canonical HTTP response shape for a Rule.
// Field names match the OpenAPI spec and the TypeScript canonical types exactly.
type RuleResponse struct {
	ID            uuid.UUID       `json:"id"`
	StoreID       uuid.UUID       `json:"store_id"`
	RuleType      string          `json:"rule_type"`
	IsEnabled     bool            `json:"is_enabled"`
	Severity      string          `json:"severity"`
	Description   string          `json:"description"`
	Configuration json.RawMessage `json:"configuration"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
}

// ToRuleResponse converts a model.Rule to a canonical RuleResponse.
func ToRuleResponse(r *model.Rule) RuleResponse {
	return RuleResponse{
		ID:            r.ID,
		StoreID:       r.TenantID, // TenantID is the canonical store_id at DB level
		RuleType:      r.Type,
		IsEnabled:     r.Enabled,
		Severity:      r.Severity,
		Description:   r.Description,
		Configuration: json.RawMessage(r.Configuration),
		CreatedAt:     r.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     r.UpdatedAt.Format(time.RFC3339),
	}
}

// Note: RuleViolationDTO and ValidationErrorResponse are defined in schedule.go
// so that AssignmentResponse can reference them without a circular dependency.
// Do not redefine them here.
