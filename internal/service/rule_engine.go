package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/repo"
	"github.com/sirupsen/logrus"
)

// RuleEngine loads enabled rules from the DB and evaluates them through a pluggable
// RuleRegistry. The engine always degrades gracefully: infrastructure errors never
// block a scheduling operation (fail-open principle).
type RuleEngine struct {
	ruleRepo   repo.RuleRepository
	assignRepo repo.ShiftAssignmentRepository
	empRepo    repo.EmployeeRepository
	registry   *RuleRegistry
	logger     *logrus.Entry
}

// NewRuleEngine creates a RuleEngine wired to the DefaultRegistry.
func NewRuleEngine(
	ruleRepo repo.RuleRepository,
	assignRepo repo.ShiftAssignmentRepository,
	empRepo repo.EmployeeRepository,
	logger *logrus.Entry,
) *RuleEngine {
	return &RuleEngine{
		ruleRepo:   ruleRepo,
		assignRepo: assignRepo,
		empRepo:    empRepo,
		registry:   DefaultRegistry(assignRepo),
		logger:     logger,
	}
}

// NewRuleEngineWithRegistry creates a RuleEngine with a custom registry.
// Useful for tests or to register additional evaluators at startup.
func NewRuleEngineWithRegistry(
	ruleRepo repo.RuleRepository,
	assignRepo repo.ShiftAssignmentRepository,
	empRepo repo.EmployeeRepository,
	registry *RuleRegistry,
	logger *logrus.Entry,
) *RuleEngine {
	return &RuleEngine{
		ruleRepo:   ruleRepo,
		assignRepo: assignRepo,
		empRepo:    empRepo,
		registry:   registry,
		logger:     logger,
	}
}

// EvaluateAssignment runs all enabled rules against a proposed (shift, employee)
// pair and returns any violations. PASS results are logged at debug level for
// audit purposes. BLOCKING violations must prevent the operation; WARNING/INFO
// are advisory.
//
// The engine pre-loads the employee's recent assignments once (±14-day window)
// and passes them to every evaluator — avoiding N separate repo queries.
func (e *RuleEngine) EvaluateAssignment(
	ctx context.Context,
	tenantID uuid.UUID,
	shift *model.ShiftInstance,
	employee *model.Employee,
) ([]model.RuleViolation, error) {
	logger := ctxutil.GetLogger(ctx)

	rules, err := e.ruleRepo.ListEnabled(ctx, tenantID)
	if err != nil {
		logger.WithError(err).Warn("rule engine: failed to load rules, skipping evaluation")
		return nil, nil // fail-open: don't block the operation
	}
	if len(rules) == 0 {
		return nil, nil
	}

	// Pre-load the employee's recent assignments once — shared across all evaluators.
	recent, err := e.preloadAssignments(ctx, tenantID, employee.ID, shift.Date)
	if err != nil {
		logger.WithError(err).Warn("rule engine: failed to pre-load assignments, continuing without them")
		recent = nil
	}

	input := EvaluationInput{
		TenantID:          tenantID,
		Action:            "assign",
		Shift:             shift,
		Employee:          employee,
		RecentAssignments: recent,
	}

	results := make([]model.RuleResult, 0, len(rules))
	for _, rule := range rules {
		result := e.run(ctx, rule, input)
		results = append(results, result)

		// Structured audit log for every evaluation regardless of outcome.
		e.logger.WithFields(logrus.Fields{
			"rule_id":   result.RuleID,
			"rule_type": result.RuleType,
			"severity":  result.Severity,
			"status":    result.Status,
			"employee":  employee.Name,
		}).Debug("rule engine: evaluation result")
	}

	return model.FilterViolations(results), nil
}

// EvaluateShiftCreation checks rules that apply at shift-creation time.
// No structural rules are evaluated at this stage (no employee is assigned yet).
// Coverage rules are handled by the CoverageEngine post-assignment.
func (e *RuleEngine) EvaluateShiftCreation(
	_ context.Context,
	_ uuid.UUID,
	_ *model.ShiftInstance,
) ([]model.RuleViolation, error) {
	return nil, nil
}

// ─── Internal ─────────────────────────────────────────────────────────────────

// run invokes the evaluator for the given rule and wraps the output into a
// RuleResult. On any infrastructure error, it fails open (status = PASS).
func (e *RuleEngine) run(ctx context.Context, rule *model.Rule, input EvaluationInput) model.RuleResult {
	base := model.RuleResult{
		RuleID:   rule.ID.String(),
		RuleType: rule.Type,
		Severity: rule.Severity,
	}

	evaluator, ok := e.registry.Get(rule.Type)
	if !ok {
		e.logger.Warnf("rule engine: no evaluator for type %q — skipping", rule.Type)
		base.Status = model.RuleStatusPass
		return base
	}

	out, err := evaluator.Evaluate(ctx, input, []byte(rule.Configuration))
	if err != nil {
		e.logger.WithError(err).Warnf("rule engine: evaluator %q error — failing open", rule.Type)
		base.Status = model.RuleStatusPass
		return base
	}

	if out.Passed {
		base.Status = model.RuleStatusPass
	} else {
		base.Status = model.RuleStatusFail
		base.Message = out.Message
	}
	return base
}

// preloadAssignments fetches the employee's confirmed assignments in a ±14-day
// window around shiftDate. This single query feeds MaxHoursEvaluator (weekly
// window) and MinRestEvaluator (consecutive-day gap check).
func (e *RuleEngine) preloadAssignments(
	ctx context.Context,
	tenantID, employeeID uuid.UUID,
	shiftDate time.Time,
) ([]*model.ShiftAssignment, error) {
	from := shiftDate.AddDate(0, 0, -14)
	to := shiftDate.AddDate(0, 0, 14)
	return e.assignRepo.ListByEmployee(ctx, tenantID, employeeID, from, to)
}
