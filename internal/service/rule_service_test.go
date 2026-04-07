package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/service"
	"github.com/ovander/parashift/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRuleService(repo *testutil.MockRuleRepo) *service.RuleService {
	return service.NewRuleService(repo, newTestLogger())
}

func TestRuleService_List(t *testing.T) {
	tenantID := uuid.New()
	rules := []*model.Rule{
		{TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: tenantID}, Type: model.RuleTypeNoOverlap},
	}
	repo := &testutil.MockRuleRepo{
		ListFn: func(_ context.Context, _ uuid.UUID) ([]*model.Rule, error) {
			return rules, nil
		},
	}
	svc := newRuleService(repo)
	got, err := svc.List(context.Background(), tenantID)
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

func TestRuleService_Create_Valid(t *testing.T) {
	tenantID := uuid.New()
	created := false
	repo := &testutil.MockRuleRepo{
		CreateFn: func(_ context.Context, r *model.Rule) error {
			created = true
			assert.Equal(t, model.RuleTypeMaxHours, r.Type)
			assert.Equal(t, model.RuleSeverityWarning, r.Severity)
			return nil
		},
	}
	svc := newRuleService(repo)
	rule, err := svc.Create(context.Background(), tenantID, dto.CreateRuleRequest{
		Type:          model.RuleTypeMaxHours,
		Configuration: map[string]interface{}{"max_weekly_hours": 40.0},
		Severity:      model.RuleSeverityWarning,
	})
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, model.RuleTypeMaxHours, rule.Type)
}

func TestRuleService_Create_InvalidType(t *testing.T) {
	svc := newRuleService(&testutil.MockRuleRepo{})
	_, err := svc.Create(context.Background(), uuid.New(), dto.CreateRuleRequest{
		Type:          "INVALID_TYPE",
		Severity:      model.RuleSeverityWarning,
		Configuration: map[string]interface{}{},
	})
	require.Error(t, err)
}

func TestRuleService_Create_InvalidSeverity(t *testing.T) {
	svc := newRuleService(&testutil.MockRuleRepo{})
	_, err := svc.Create(context.Background(), uuid.New(), dto.CreateRuleRequest{
		Type:          model.RuleTypeNoOverlap,
		Severity:      "INVALID",
		Configuration: map[string]interface{}{},
	})
	require.Error(t, err)
}

func TestRuleService_Create_MaxHours_MissingConfig(t *testing.T) {
	svc := newRuleService(&testutil.MockRuleRepo{})
	_, err := svc.Create(context.Background(), uuid.New(), dto.CreateRuleRequest{
		Type:          model.RuleTypeMaxHours,
		Severity:      model.RuleSeverityWarning,
		Configuration: map[string]interface{}{}, // missing max_weekly_hours
	})
	require.Error(t, err)
}

func TestRuleService_Delete_NotFound(t *testing.T) {
	repo := &testutil.MockRuleRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Rule, error) {
			return nil, nil // not found
		},
	}
	svc := newRuleService(repo)
	err := svc.Delete(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
}

func TestRuleService_GetByID_Found(t *testing.T) {
	tenantID := uuid.New()
	ruleID := uuid.New()
	rule := &model.Rule{TenantScoped: model.TenantScoped{ID: ruleID, TenantID: tenantID}, Type: model.RuleTypeRole}
	repo := &testutil.MockRuleRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Rule, error) {
			return rule, nil
		},
	}
	svc := newRuleService(repo)
	got, err := svc.GetByID(context.Background(), tenantID, ruleID)
	require.NoError(t, err)
	assert.Equal(t, ruleID, got.ID)
}
