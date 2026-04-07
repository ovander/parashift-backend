package model

import "gorm.io/datatypes"

const (
	// Rule types
	RuleTypeCoverage  = "COVERAGE"   // minimum staffing for a shift slot
	RuleTypeRole      = "ROLE"       // required role/qualification for a shift
	RuleTypeMaxHours  = "MAX_HOURS"  // maximum weekly hours per employee
	RuleTypeNoOverlap = "NO_OVERLAP" // employee cannot have two overlapping shifts
	RuleTypeMinRest   = "MIN_REST"   // minimum rest hours between consecutive shifts

	// Severity levels
	RuleSeverityBlocking = "BLOCKING" // operation is rejected
	RuleSeverityWarning  = "WARNING"  // operation allowed but warning is returned
	RuleSeverityInfo     = "INFO"     // informational only

	// Rule evaluation status
	RuleStatusPass = "PASS"
	RuleStatusFail = "FAIL"
)

// Rule is a configurable scheduling constraint scoped to a tenant/store.
type Rule struct {
	TenantScoped
	Type          string         `gorm:"not null"`                    // COVERAGE|ROLE|MAX_HOURS|NO_OVERLAP|MIN_REST
	Configuration datatypes.JSON `gorm:"type:jsonb"`                  // rule-type-specific JSON config
	Severity      string         `gorm:"not null;default:'WARNING'"`  // BLOCKING|WARNING|INFO
	Enabled       bool           `gorm:"not null;default:true"`
	Description   string
}

// RuleViolation describes a single rule that was violated during evaluation.
// It is the external representation (failures only) returned to callers.
type RuleViolation struct {
	RuleID   string `json:"rule_id"`
	RuleType string `json:"rule_type"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// IsBlocking returns true when the violation should prevent the operation.
func (v RuleViolation) IsBlocking() bool { return v.Severity == RuleSeverityBlocking }

// RuleResult is the internal outcome of evaluating a single rule. It includes
// both PASS and FAIL outcomes so the engine can build a full audit trail.
type RuleResult struct {
	RuleID   string `json:"rule_id"`
	RuleType string `json:"rule_type"`
	Severity string `json:"severity"`
	Status   string `json:"status"` // PASS | FAIL
	Message  string `json:"message,omitempty"`
}

// IsBlocking returns true when this is a blocking failure.
func (r RuleResult) IsBlocking() bool {
	return r.Severity == RuleSeverityBlocking && r.Status == RuleStatusFail
}

// FilterViolations extracts FAIL results and converts them to RuleViolation values.
func FilterViolations(results []RuleResult) []RuleViolation {
	var violations []RuleViolation
	for _, r := range results {
		if r.Status == RuleStatusFail {
			violations = append(violations, RuleViolation{
				RuleID:   r.RuleID,
				RuleType: r.RuleType,
				Severity: r.Severity,
				Message:  r.Message,
			})
		}
	}
	return violations
}

// HasBlocking returns true if any result is a blocking failure.
func HasBlocking(results []RuleResult) bool {
	for _, r := range results {
		if r.IsBlocking() {
			return true
		}
	}
	return false
}

// ─── Rule configuration structs ───────────────────────────────────────────────
// These are used when unmarshaling the Configuration JSON field.

// MaxHoursConfig is the configuration for a MAX_HOURS rule.
type MaxHoursConfig struct {
	MaxWeeklyHours float64 `json:"max_weekly_hours"` // e.g. 40
}

// MinRestConfig is the configuration for a MIN_REST rule.
type MinRestConfig struct {
	MinRestHours float64 `json:"min_rest_hours"` // e.g. 11
}

// RoleConfig is the configuration for a ROLE rule.
type RoleConfig struct {
	RequiredRole string `json:"required_role"` // e.g. "pharmacist"
}
