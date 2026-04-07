package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/repo"
)

// ─── Core types ───────────────────────────────────────────────────────────────

// EvaluationInput is the context passed to every rule evaluator. Shared data
// (e.g. recent assignments) is pre-loaded once by the engine and passed here so
// each evaluator is a pure function — it reads from this struct, never the DB.
type EvaluationInput struct {
	TenantID uuid.UUID
	Action   string // "assign" | "swap"

	Shift    *model.ShiftInstance
	Employee *model.Employee

	// RecentAssignments holds the employee's assignments in a ±14-day window around
	// the proposed shift. Populated once by the engine; used by MaxHours and MinRest.
	RecentAssignments []*model.ShiftAssignment
}

// EvaluatorOutput is what each evaluator returns.
type EvaluatorOutput struct {
	Passed  bool
	Message string // populated on failure only
}

// ─── Interface ────────────────────────────────────────────────────────────────

// RuleEvaluator is the interface all concrete rule types must implement.
// Adding a new constraint type is done by implementing this interface and
// registering it in DefaultRegistry (or a custom registry at startup).
type RuleEvaluator interface {
	// Code returns the rule type identifier matching one of the model.RuleType* constants.
	Code() string
	// Evaluate checks whether the proposed scheduling action satisfies this rule.
	// It must not return a non-nil error for business failures — only for unexpected
	// infrastructure errors. Business failures are expressed via Passed: false.
	Evaluate(ctx context.Context, input EvaluationInput, rawConfig []byte) (EvaluatorOutput, error)
}

// ─── Registry ─────────────────────────────────────────────────────────────────

// RuleRegistry maps rule type codes to their evaluator implementations.
// It is the extension point for adding custom rules without modifying the engine.
type RuleRegistry struct {
	evaluators map[string]RuleEvaluator
}

// NewRuleRegistry returns an empty, ready-to-use registry.
func NewRuleRegistry() *RuleRegistry {
	return &RuleRegistry{evaluators: make(map[string]RuleEvaluator)}
}

// Register adds (or replaces) an evaluator. Safe to call multiple times for
// the same code — last registration wins.
func (r *RuleRegistry) Register(e RuleEvaluator) {
	r.evaluators[e.Code()] = e
}

// Get looks up an evaluator by its type code.
func (r *RuleRegistry) Get(code string) (RuleEvaluator, bool) {
	e, ok := r.evaluators[code]
	return e, ok
}

// DefaultRegistry returns a registry pre-populated with all built-in evaluators.
// It is the entry-point used by NewRuleEngine.
func DefaultRegistry(assignRepo repo.ShiftAssignmentRepository) *RuleRegistry {
	r := NewRuleRegistry()
	r.Register(&RoleEvaluator{})
	r.Register(&NoOverlapEvaluator{assignRepo: assignRepo})
	r.Register(&MaxHoursEvaluator{})
	r.Register(&MinRestEvaluator{})
	return r
}

// ─── RoleEvaluator ────────────────────────────────────────────────────────────

// RoleEvaluator ensures the employee holds the qualification required by the shift.
type RoleEvaluator struct{}

func (e *RoleEvaluator) Code() string { return model.RuleTypeRole }

func (e *RoleEvaluator) Evaluate(_ context.Context, input EvaluationInput, rawConfig []byte) (EvaluatorOutput, error) {
	var cfg model.RoleConfig
	if err := json.Unmarshal(rawConfig, &cfg); err != nil {
		return EvaluatorOutput{Passed: true}, nil // skip malformed config gracefully
	}

	required := cfg.RequiredRole
	if required == "" {
		required = input.Shift.RequiredQualification
	}
	if required == "" {
		return EvaluatorOutput{Passed: true}, nil // no requirement — always pass
	}

	if input.Employee.JobRole != required {
		return EvaluatorOutput{
			Passed: false,
			Message: fmt.Sprintf(
				"employee %q has job role %q but shift requires %q",
				input.Employee.Name, input.Employee.JobRole, required,
			),
		}, nil
	}
	return EvaluatorOutput{Passed: true}, nil
}

// ─── NoOverlapEvaluator ───────────────────────────────────────────────────────

// NoOverlapEvaluator prevents an employee from being double-booked on the same
// date and time. It delegates to the repository's optimised overlap query.
type NoOverlapEvaluator struct {
	assignRepo repo.ShiftAssignmentRepository
}

func (e *NoOverlapEvaluator) Code() string { return model.RuleTypeNoOverlap }

func (e *NoOverlapEvaluator) Evaluate(ctx context.Context, input EvaluationInput, _ []byte) (EvaluatorOutput, error) {
	hasConflict, err := e.assignRepo.ExistsConflict(
		ctx,
		input.TenantID,
		input.Employee.ID,
		input.Shift.Date,
		input.Shift.StartTime,
		input.Shift.EndTime,
		nil,
	)
	if err != nil {
		return EvaluatorOutput{}, fmt.Errorf("no_overlap: conflict check: %w", err)
	}
	if hasConflict {
		return EvaluatorOutput{
			Passed: false,
			Message: fmt.Sprintf(
				"employee %q already has an overlapping assignment on %s %s–%s",
				input.Employee.Name,
				input.Shift.Date.Format("2006-01-02"),
				input.Shift.StartTime,
				input.Shift.EndTime,
			),
		}, nil
	}
	return EvaluatorOutput{Passed: true}, nil
}

// ─── MaxHoursEvaluator ────────────────────────────────────────────────────────

// MaxHoursEvaluator checks that adding the proposed shift won't push the employee
// over their configured maximum weekly hours.
//
// It uses the denormalized ShiftDate/ShiftStartTime/ShiftEndTime fields on each
// pre-loaded assignment so no extra query is needed.
type MaxHoursEvaluator struct{}

func (e *MaxHoursEvaluator) Code() string { return model.RuleTypeMaxHours }

func (e *MaxHoursEvaluator) Evaluate(_ context.Context, input EvaluationInput, rawConfig []byte) (EvaluatorOutput, error) {
	var cfg model.MaxHoursConfig
	if err := json.Unmarshal(rawConfig, &cfg); err != nil || cfg.MaxWeeklyHours <= 0 {
		return EvaluatorOutput{Passed: true}, nil
	}

	weekStart, weekEnd := weekBounds(input.Shift.Date)

	// Sum actual hours from the pre-loaded assignments that fall within this week.
	var workedHours float64
	for _, a := range input.RecentAssignments {
		if !a.ShiftDate.IsZero() &&
			!a.ShiftDate.Before(weekStart) &&
			!a.ShiftDate.After(weekEnd) {
			workedHours += shiftHours(a.ShiftStartTime, a.ShiftEndTime)
		}
	}
	// Add the proposed shift's own hours.
	workedHours += shiftHours(input.Shift.StartTime, input.Shift.EndTime)

	if workedHours > cfg.MaxWeeklyHours {
		return EvaluatorOutput{
			Passed: false,
			Message: fmt.Sprintf(
				"employee %q would reach %.1f h this week (limit %.0f h)",
				input.Employee.Name, workedHours, cfg.MaxWeeklyHours,
			),
		}, nil
	}
	return EvaluatorOutput{Passed: true}, nil
}

// ─── MinRestEvaluator ─────────────────────────────────────────────────────────

// MinRestEvaluator ensures there is a sufficient rest gap between any pair of
// consecutive shifts. Both directions are checked: rest before the proposed shift
// and rest after it.
type MinRestEvaluator struct{}

func (e *MinRestEvaluator) Code() string { return model.RuleTypeMinRest }

func (e *MinRestEvaluator) Evaluate(_ context.Context, input EvaluationInput, rawConfig []byte) (EvaluatorOutput, error) {
	var cfg model.MinRestConfig
	if err := json.Unmarshal(rawConfig, &cfg); err != nil || cfg.MinRestHours <= 0 {
		return EvaluatorOutput{Passed: true}, nil
	}

	minRest := time.Duration(cfg.MinRestHours) * time.Hour
	proposedStart := parseDayTime(input.Shift.Date, input.Shift.StartTime)
	proposedEnd := parseDayTime(input.Shift.Date, input.Shift.EndTime)

	for _, a := range input.RecentAssignments {
		if a.ShiftStartTime == "" || a.ShiftEndTime == "" || a.ShiftDate.IsZero() {
			continue // skip assignments without denormalized time data
		}
		existingEnd := parseDayTime(a.ShiftDate, a.ShiftEndTime)
		existingStart := parseDayTime(a.ShiftDate, a.ShiftStartTime)

		// Gap between an existing shift ending and the proposed shift starting.
		if existingEnd.Before(proposedStart) {
			if gap := proposedStart.Sub(existingEnd); gap < minRest {
				return EvaluatorOutput{
					Passed: false,
					Message: fmt.Sprintf(
						"employee %q has only %.1f h rest before proposed shift (minimum %.0f h)",
						input.Employee.Name, gap.Hours(), cfg.MinRestHours,
					),
				}, nil
			}
		}

		// Gap between the proposed shift ending and an existing shift starting.
		if proposedEnd.Before(existingStart) {
			if gap := existingStart.Sub(proposedEnd); gap < minRest {
				return EvaluatorOutput{
					Passed: false,
					Message: fmt.Sprintf(
						"employee %q would have only %.1f h rest after proposed shift (minimum %.0f h)",
						input.Employee.Name, gap.Hours(), cfg.MinRestHours,
					),
				}, nil
			}
		}
	}
	return EvaluatorOutput{Passed: true}, nil
}

// ─── Shared schedule helpers ──────────────────────────────────────────────────

// weekBounds returns the Monday 00:00:00 UTC and Sunday 23:59:59 UTC for the
// ISO week that contains date.
func weekBounds(date time.Time) (time.Time, time.Time) {
	weekday := int(date.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday → 7 so Monday is 1
	}
	monday := date.AddDate(0, 0, -(weekday - 1))
	start := time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 6)
	end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 0, time.UTC)
	return start, end
}

// shiftHours returns the decimal-hour duration of a HH:MM start/end pair.
func shiftHours(start, end string) float64 {
	sh, sm := parseHHMM(start)
	eh, em := parseHHMM(end)
	diff := float64(eh*60+em-sh*60-sm) / 60.0
	if diff < 0 {
		diff += 24 // overnight shift
	}
	return diff
}

func parseHHMM(t string) (int, int) {
	var h, m int
	fmt.Sscanf(t, "%d:%d", &h, &m) //nolint:errcheck
	return h, m
}

func parseDayTime(date time.Time, hhmm string) time.Time {
	h, m := parseHHMM(hhmm)
	return time.Date(date.Year(), date.Month(), date.Day(), h, m, 0, 0, time.UTC)
}
