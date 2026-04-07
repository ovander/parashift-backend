package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
)

// ── Schedule suggestions — canonical API types ────────────────────────────────

// ScheduleSuggestion is one AI/heuristic-proposed employee-to-shift assignment.
// It is advisory only — callers must confirm via POST /assignments.
//
// EmployeeName is denormalized for display.  If the caller needs the full
// Employee object it should look it up by EmployeeID from its local state.
//
// Confidence is an internal scoring signal (0.0–1.0) populated by the
// heuristic engine.  It is intentionally omitted from the OpenAPI spec and the
// canonical TypeScript types because it is not stable enough to expose as a
// contract; include it in logs and internal tooling only.
type ScheduleSuggestion struct {
	ShiftID      uuid.UUID `json:"shift_id"`
	EmployeeID   uuid.UUID `json:"employee_id"`
	EmployeeName string    `json:"employee_name"`
	Reason       string    `json:"reason"`
	// Confidence is populated internally but omitted from the public API response.
	// Use json:"-" so it is never serialised to HTTP clients.
	Confidence float64 `json:"-"`
}

// SuggestWeekResponse is the envelope returned by GET /schedule/suggest.
// Suggestions are ordered in descending recommendation priority.
type SuggestWeekResponse struct {
	Suggestions []ScheduleSuggestion `json:"suggestions"`
}

// ── Single-shift suggestions (internal / backward-compatible) ─────────────────

// SuggestAssignmentRequest asks the AI to recommend the best employees for one
// specific shift. Used internally by AIService.SuggestAssignment.
type SuggestAssignmentRequest struct {
	ShiftID uuid.UUID `json:"shift_id"`
}

// SuggestAssignmentResponse wraps AI-ranked employee suggestions for a shift.
// Used internally — the new week-level API uses SuggestWeekResponse instead.
type SuggestAssignmentResponse struct {
	ShiftID     uuid.UUID            `json:"shift_id"`
	Suggestions []ScheduleSuggestion `json:"suggestions"`
}

// ── Schedule optimization (internal / admin action) ───────────────────────────

// OptimizeScheduleRequest asks the AI to propose improvements over a date range.
type OptimizeScheduleRequest struct {
	DateFrom time.Time `json:"date_from"`
	DateTo   time.Time `json:"date_to"`
}

// OptimizeScheduleSuggestion is one AI-proposed schedule change.
type OptimizeScheduleSuggestion struct {
	ShiftID               uuid.UUID `json:"shift_id"`
	RecommendedEmployeeID uuid.UUID `json:"recommended_employee_id"`
	Reason                string    `json:"reason"`
}

// OptimizeScheduleResponse wraps AI optimization suggestions for a date range.
type OptimizeScheduleResponse struct {
	DateFrom    time.Time                    `json:"date_from"`
	DateTo      time.Time                    `json:"date_to"`
	Suggestions []OptimizeScheduleSuggestion `json:"suggestions"`
}

// ── AI Insights ───────────────────────────────────────────────────────────────

// AIInsightResponse is the HTTP response shape for a single AIInsight.
type AIInsightResponse struct {
	ID             uuid.UUID  `json:"id"`
	StoreID        uuid.UUID  `json:"store_id"`
	Type           string     `json:"type"`
	Message        string     `json:"message"`
	Recommendation string     `json:"recommendation"`
	RelatedShiftID *uuid.UUID `json:"related_shift_id,omitempty"`
	RelatedEmpID   *uuid.UUID `json:"related_emp_id,omitempty"`
	Dismissed      bool       `json:"dismissed"`
	CreatedAt      string     `json:"created_at"`
}

// ToAIInsightResponse converts a model.AIInsight to an AIInsightResponse.
func ToAIInsightResponse(i *model.AIInsight) AIInsightResponse {
	return AIInsightResponse{
		ID:             i.ID,
		StoreID:        i.TenantID, // TenantID is the canonical store_id at DB level
		Type:           i.Type,
		Message:        i.Message,
		Recommendation: i.Recommendation,
		RelatedShiftID: i.RelatedShiftID,
		RelatedEmpID:   i.RelatedEmpID,
		Dismissed:      i.Dismissed,
		CreatedAt:      i.CreatedAt.Format(time.RFC3339),
	}
}
