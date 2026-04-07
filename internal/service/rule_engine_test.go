package service_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/service"
	"github.com/ovander/parashift/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRuleEngine creates a RuleEngine wired to the provided mocks.
func newRuleEngine(
	ruleRepo *testutil.MockRuleRepo,
	assignRepo *testutil.MockShiftAssignmentRepo,
	empRepo *testutil.MockEmployeeRepo,
) *service.RuleEngine {
	return service.NewRuleEngine(ruleRepo, assignRepo, empRepo, newTestLogger())
}

func ruleJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

// ─── RoleRule ─────────────────────────────────────────────────────────────────

func TestRuleEngine_RoleRule_Pass(t *testing.T) {
	tenantID := uuid.New()
	shift := testutil.NewShiftInstance(tenantID)
	emp := testutil.NewEmployee(tenantID)
	emp.JobRole = "cashier"

	cfg := ruleJSON(t, model.RoleConfig{RequiredRole: "cashier"})
	rules := []*model.Rule{
		{
			TenantScoped:  model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
			Type:          model.RuleTypeRole,
			Configuration: cfg,
			Severity:      model.RuleSeverityBlocking,
			Enabled:       true,
		},
	}

	ruleRepo := &testutil.MockRuleRepo{
		ListEnabledFn: func(_ context.Context, _ uuid.UUID) ([]*model.Rule, error) {
			return rules, nil
		},
	}

	eng := newRuleEngine(ruleRepo, &testutil.MockShiftAssignmentRepo{}, &testutil.MockEmployeeRepo{})
	violations, err := eng.EvaluateAssignment(context.Background(), tenantID, shift, emp)
	require.NoError(t, err)
	assert.Empty(t, violations)
}

func TestRuleEngine_RoleRule_Violation(t *testing.T) {
	tenantID := uuid.New()
	shift := testutil.NewShiftInstance(tenantID)
	emp := testutil.NewEmployee(tenantID)
	emp.JobRole = "pharmacist" // different from required "cashier" → triggers violation

	cfg := ruleJSON(t, model.RoleConfig{RequiredRole: "cashier"})
	rules := []*model.Rule{
		{
			TenantScoped:  model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
			Type:          model.RuleTypeRole,
			Configuration: cfg,
			Severity:      model.RuleSeverityBlocking,
			Enabled:       true,
		},
	}

	ruleRepo := &testutil.MockRuleRepo{
		ListEnabledFn: func(_ context.Context, _ uuid.UUID) ([]*model.Rule, error) {
			return rules, nil
		},
	}

	eng := newRuleEngine(ruleRepo, &testutil.MockShiftAssignmentRepo{}, &testutil.MockEmployeeRepo{})
	violations, err := eng.EvaluateAssignment(context.Background(), tenantID, shift, emp)
	require.NoError(t, err)
	require.Len(t, violations, 1)
	assert.Equal(t, model.RuleTypeRole, violations[0].RuleType)
	assert.Equal(t, model.RuleSeverityBlocking, violations[0].Severity)
}

// ─── NoOverlapRule ────────────────────────────────────────────────────────────

func TestRuleEngine_NoOverlapRule_Conflict(t *testing.T) {
	tenantID := uuid.New()
	shift := testutil.NewShiftInstance(tenantID)
	emp := testutil.NewEmployee(tenantID)

	rules := []*model.Rule{
		{
			TenantScoped:  model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
			Type:          model.RuleTypeNoOverlap,
			Configuration: []byte("{}"),
			Severity:      model.RuleSeverityBlocking,
			Enabled:       true,
		},
	}
	ruleRepo := &testutil.MockRuleRepo{
		ListEnabledFn: func(_ context.Context, _ uuid.UUID) ([]*model.Rule, error) {
			return rules, nil
		},
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ExistsConflictFn: func(_ context.Context, _, _ uuid.UUID, _ time.Time, _, _ string, _ *uuid.UUID) (bool, error) {
			return true, nil
		},
	}

	eng := newRuleEngine(ruleRepo, assignRepo, &testutil.MockEmployeeRepo{})
	violations, err := eng.EvaluateAssignment(context.Background(), tenantID, shift, emp)
	require.NoError(t, err)
	require.Len(t, violations, 1)
	assert.Equal(t, model.RuleTypeNoOverlap, violations[0].RuleType)
}

// ─── MaxHoursRule ─────────────────────────────────────────────────────────────

func TestRuleEngine_MaxHoursRule_Exceeded(t *testing.T) {
	tenantID := uuid.New()
	shift := testutil.NewShiftInstance(tenantID)
	shift.StartTime = "09:00"
	shift.EndTime = "17:00" // 8 h shift
	emp := testutil.NewEmployee(tenantID)

	// 5 existing assignments this week × 8 h average + 8 h proposed = 48 h > 40
	existingAssignments := make([]*model.ShiftAssignment, 5)
	for i := range existingAssignments {
		existingAssignments[i] = testutil.NewShiftAssignment(tenantID, uuid.New(), emp.ID)
	}

	cfg := ruleJSON(t, model.MaxHoursConfig{MaxWeeklyHours: 40})
	rules := []*model.Rule{
		{
			TenantScoped:  model.TenantScoped{ID: uuid.New(), TenantID: tenantID},
			Type:          model.RuleTypeMaxHours,
			Configuration: cfg,
			Severity:      model.RuleSeverityWarning,
			Enabled:       true,
		},
	}
	ruleRepo := &testutil.MockRuleRepo{
		ListEnabledFn: func(_ context.Context, _ uuid.UUID) ([]*model.Rule, error) {
			return rules, nil
		},
	}
	assignRepo := &testutil.MockShiftAssignmentRepo{
		ListByEmployeeFn: func(_ context.Context, _, _ uuid.UUID, _, _ time.Time) ([]*model.ShiftAssignment, error) {
			return existingAssignments, nil
		},
	}

	eng := newRuleEngine(ruleRepo, assignRepo, &testutil.MockEmployeeRepo{})
	violations, err := eng.EvaluateAssignment(context.Background(), tenantID, shift, emp)
	require.NoError(t, err)
	require.Len(t, violations, 1)
	assert.Equal(t, model.RuleTypeMaxHours, violations[0].RuleType)
	assert.Equal(t, model.RuleSeverityWarning, violations[0].Severity)
}

// ─── Graceful degradation on repo error ───────────────────────────────────────

func TestRuleEngine_RepoError_DegradeGracefully(t *testing.T) {
	tenantID := uuid.New()
	shift := testutil.NewShiftInstance(tenantID)
	emp := testutil.NewEmployee(tenantID)

	ruleRepo := &testutil.MockRuleRepo{
		ListEnabledFn: func(_ context.Context, _ uuid.UUID) ([]*model.Rule, error) {
			return nil, assert.AnError
		},
	}

	eng := newRuleEngine(ruleRepo, &testutil.MockShiftAssignmentRepo{}, &testutil.MockEmployeeRepo{})
	// Should not return an error — rule engine degrades gracefully.
	violations, err := eng.EvaluateAssignment(context.Background(), tenantID, shift, emp)
	require.NoError(t, err)
	assert.Empty(t, violations)
}
