package handler

import (
	"net/http"

	"github.com/ovander/parashift/internal/pkg"
)

// OptionsHandler serves all selectable option lists so the frontend never
// needs to hardcode enum values, dropdown labels, or business-logic strings.
//
// Design rules:
//   - Every value here is the canonical wire value used in API payloads.
//   - Labels are the human-readable strings displayed in the UI.
//   - The order of items defines their display order.
//   - Values are stable — never removed without a deprecation cycle.
type OptionsHandler struct{}

// NewOptionsHandler creates a new OptionsHandler.
func NewOptionsHandler() *OptionsHandler {
	return &OptionsHandler{}
}

// option is a generic label/value pair used by all option lists.
type option struct {
	Value any    `json:"value"` // string or int, stable wire identifier
	Label string `json:"label"` // localised display string
}

// optionWithDescription extends option with an optional description.
type optionWithDescription struct {
	Value       any    `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// optionsResponse is the full options payload returned by GET /api/v1/options.
type optionsResponse struct {
	EmployeePositions  []optionWithDescription `json:"employee_positions"`  // manager|employee — RBAC access level
	JobRoles           []optionWithDescription `json:"job_roles"`           // pharmacist|animator|... — shift eligibility
	LeaveTypes         []option                `json:"leave_types"`
	RuleTypes          []optionWithDescription `json:"rule_types"`
	RuleSeverities     []optionWithDescription `json:"rule_severities"`
	DaysOfWeek         []option                `json:"days_of_week"`
	ShiftStatuses      []option                `json:"shift_statuses"`
	AssignmentStatuses []option                `json:"assignment_statuses"`
	PlanStates         []option                `json:"plan_states"`
}

// staticOptions is the single authoritative source for all selectable values.
// When a new role, status, or type is added to the backend domain models the
// corresponding entry MUST be added here in the same commit.
var staticOptions = optionsResponse{

	// ── Employee positions ──────────────────────────────────────────────────────
	// Controls RBAC access. Matches the Position field in model/employee.go and
	// the RBAC role map in middleware/rbac.go.
	EmployeePositions: []optionWithDescription{
		{Value: "manager", Label: "Manager", Description: "Can manage schedules, leaves, and store configuration"},
		{Value: "employee", Label: "Employee", Description: "Standard access — can view schedules and request leave"},
	},

	// ── Job roles ───────────────────────────────────────────────────────────────
	// Controls shift eligibility. Matches the JobRole field in model/employee.go.
	// Add new roles here and in coverage requirement configuration.
	JobRoles: []optionWithDescription{
		{Value: "pharmacist", Label: "Pharmacist", Description: "Licensed pharmacist — eligible for pharmacist-only shifts"},
		{Value: "pharmacist_assistant", Label: "Pharmacist assistant", Description: "Assistant pharmacist — supports licensed pharmacist"},
		{Value: "animator", Label: "Animator", Description: "Community or health animator"},
		{Value: "logistics_agent", Label: "Logistics agent", Description: "Stock and supply chain management"},
		{Value: "cashier", Label: "Cashier", Description: "Point of sale and customer checkout"},
		{Value: "technician", Label: "Technician", Description: "Pharmacy technician — preparation and dispensing support"},
	},

	// ── Leave types ─────────────────────────────────────────────────────────────
	// Matches LeaveTypeVacation/Sick/Other in model/leave_request.go and the
	// validation in leave_service.go.
	LeaveTypes: []option{
		{Value: "vacation", Label: "Vacation"},
		{Value: "sick", Label: "Sick leave"},
		{Value: "other", Label: "Other"},
	},

	// ── Rule types ──────────────────────────────────────────────────────────────
	// Matches RuleType* constants in model/rule.go and validateRuleType() in
	// rule_service.go.
	RuleTypes: []optionWithDescription{
		{Value: "MAX_HOURS", Label: "Max Hours", Description: "Maximum weekly hours per employee"},
		{Value: "MIN_REST", Label: "Min Rest", Description: "Minimum rest hours between consecutive shifts"},
		{Value: "NO_OVERLAP", Label: "No Overlap", Description: "Employee cannot have two overlapping shifts"},
		{Value: "COVERAGE", Label: "Coverage", Description: "Minimum staffing level for a shift slot"},
		{Value: "ROLE", Label: "Role", Description: "Required role or qualification for a shift"},
	},

	// ── Rule severities ─────────────────────────────────────────────────────────
	// Matches RuleSeverity* constants in model/rule.go and validateSeverity().
	RuleSeverities: []optionWithDescription{
		{Value: "BLOCKING", Label: "Blocking", Description: "Operation is rejected when this rule fires"},
		{Value: "WARNING", Label: "Warning", Description: "Operation is allowed but a warning is returned"},
		{Value: "INFO", Label: "Info", Description: "Informational — no action is blocked"},
	},

	// ── Days of week ────────────────────────────────────────────────────────────
	// Values match Go's time.Weekday() integers: Sunday=0, Monday=1 … Saturday=6.
	// Displayed in ISO week order (Mon–Sun) for UI consistency.
	DaysOfWeek: []option{
		{Value: 1, Label: "Monday"},
		{Value: 2, Label: "Tuesday"},
		{Value: 3, Label: "Wednesday"},
		{Value: 4, Label: "Thursday"},
		{Value: 5, Label: "Friday"},
		{Value: 6, Label: "Saturday"},
		{Value: 0, Label: "Sunday"},
	},

	// ── Shift statuses ──────────────────────────────────────────────────────────
	// Matches ShiftStatus* constants in model/shift_instance.go.
	ShiftStatuses: []option{
		{Value: "DRAFT", Label: "Draft"},
		{Value: "PUBLISHED", Label: "Published"},
	},

	// ── Assignment statuses ─────────────────────────────────────────────────────
	// Matches AssignmentStatus* constants in model/shift_assignment.go.
	AssignmentStatuses: []option{
		{Value: "confirmed", Label: "Confirmed"},
		{Value: "pending", Label: "Pending"},
		{Value: "cancelled", Label: "Cancelled"},
	},

	// ── Schedule plan states ─────────────────────────────────────────────────────
	// Matches PlanState* constants in model/schedule_plan.go.
	PlanStates: []option{
		{Value: "DRAFT", Label: "Draft"},
		{Value: "PUBLISHED", Label: "Published"},
		{Value: "LIVE", Label: "Live"},
		{Value: "ARCHIVED", Label: "Archived"},
	},
}

// GetOptions returns all selectable option lists for the frontend.
//
//	GET /api/v1/options
func (h *OptionsHandler) GetOptions(w http.ResponseWriter, r *http.Request) {
	pkg.WriteJSON(w, http.StatusOK, staticOptions)
}
