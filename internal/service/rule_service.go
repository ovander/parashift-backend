package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/repo"
	"github.com/sirupsen/logrus"
)

// RuleService provides CRUD operations for scheduling rules.
type RuleService struct {
	ruleRepo repo.RuleRepository
	logger   *logrus.Entry
}

// NewRuleService creates a new RuleService.
func NewRuleService(ruleRepo repo.RuleRepository, logger *logrus.Entry) *RuleService {
	return &RuleService{
		ruleRepo: ruleRepo,
		logger:   logger,
	}
}

// List returns all rules for a tenant.
func (s *RuleService) List(ctx context.Context, tenantID uuid.UUID) ([]*model.Rule, error) {
	logger := ctxutil.GetLogger(ctx)

	rules, err := s.ruleRepo.List(ctx, tenantID)
	if err != nil {
		logger.WithError(err).Error("failed to list rules")
		return nil, apierror.Internal("failed to list rules")
	}
	return rules, nil
}

// GetByID returns a single rule by ID.
func (s *RuleService) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.Rule, error) {
	logger := ctxutil.GetLogger(ctx)

	rule, err := s.ruleRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		logger.WithError(err).Error("failed to get rule")
		return nil, apierror.Internal("failed to get rule")
	}
	if rule == nil {
		return nil, apierror.NotFound("rule", id.String())
	}
	return rule, nil
}

// Create creates a new scheduling rule.
func (s *RuleService) Create(ctx context.Context, tenantID uuid.UUID, req dto.CreateRuleRequest) (*model.Rule, error) {
	logger := ctxutil.GetLogger(ctx)

	if err := validateRuleType(req.Type); err != nil {
		return nil, err
	}
	if err := validateSeverity(req.Severity); err != nil {
		return nil, err
	}

	// Validate configuration JSON is valid for the given type.
	if err := validateRuleConfig(req.Type, req.Configuration); err != nil {
		return nil, err
	}

	cfgBytes, err := json.Marshal(req.Configuration)
	if err != nil {
		return nil, apierror.BadRequest("invalid configuration JSON")
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	rule := &model.Rule{
		TenantScoped: model.TenantScoped{
			ID:        uuid.New(),
			TenantID:  tenantID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Type:          req.Type,
		Configuration: cfgBytes,
		Severity:      req.Severity,
		Enabled:       enabled,
		Description:   req.Description,
	}

	if err := s.ruleRepo.Create(ctx, rule); err != nil {
		logger.WithError(err).Error("failed to create rule")
		return nil, apierror.Internal("failed to create rule")
	}
	return rule, nil
}

// Update modifies an existing rule.
func (s *RuleService) Update(ctx context.Context, tenantID, id uuid.UUID, req dto.UpdateRuleRequest) (*model.Rule, error) {
	logger := ctxutil.GetLogger(ctx)

	rule, err := s.ruleRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		logger.WithError(err).Error("failed to get rule")
		return nil, apierror.Internal("failed to get rule")
	}
	if rule == nil {
		return nil, apierror.NotFound("rule", id.String())
	}

	if req.Severity != "" {
		if err := validateSeverity(req.Severity); err != nil {
			return nil, err
		}
		rule.Severity = req.Severity
	}
	if req.Configuration != nil {
		if err := validateRuleConfig(rule.Type, req.Configuration); err != nil {
			return nil, err
		}
		cfgBytes, err := json.Marshal(req.Configuration)
		if err != nil {
			return nil, apierror.BadRequest("invalid configuration JSON")
		}
		rule.Configuration = cfgBytes
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	if req.Description != "" {
		rule.Description = req.Description
	}
	rule.UpdatedAt = time.Now()

	if err := s.ruleRepo.Update(ctx, rule); err != nil {
		logger.WithError(err).Error("failed to update rule")
		return nil, apierror.Internal("failed to update rule")
	}
	return rule, nil
}

// Delete removes a rule.
func (s *RuleService) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	logger := ctxutil.GetLogger(ctx)

	rule, err := s.ruleRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		logger.WithError(err).Error("failed to get rule")
		return apierror.Internal("failed to get rule")
	}
	if rule == nil {
		return apierror.NotFound("rule", id.String())
	}

	if err := s.ruleRepo.Delete(ctx, tenantID, id); err != nil {
		logger.WithError(err).Error("failed to delete rule")
		return apierror.Internal("failed to delete rule")
	}
	return nil
}

// ─── Validation helpers ────────────────────────────────────────────────────────

func validateRuleType(t string) error {
	switch t {
	case model.RuleTypeCoverage, model.RuleTypeRole, model.RuleTypeMaxHours,
		model.RuleTypeNoOverlap, model.RuleTypeMinRest:
		return nil
	default:
		return apierror.BadRequest("invalid rule type: " + t)
	}
}

func validateSeverity(s string) error {
	switch s {
	case model.RuleSeverityBlocking, model.RuleSeverityWarning, model.RuleSeverityInfo:
		return nil
	default:
		return apierror.BadRequest("invalid severity: " + s)
	}
}

// validateRuleConfig checks that the provided configuration map is valid for the rule type.
func validateRuleConfig(ruleType string, cfg map[string]interface{}) error {
	switch ruleType {
	case model.RuleTypeMaxHours:
		if _, ok := cfg["max_weekly_hours"]; !ok {
			return apierror.BadRequest("max_hours rule requires 'max_weekly_hours' in configuration")
		}
	case model.RuleTypeMinRest:
		if _, ok := cfg["min_rest_hours"]; !ok {
			return apierror.BadRequest("min_rest rule requires 'min_rest_hours' in configuration")
		}
	case model.RuleTypeRole:
		// required_role is optional (falls back to shift's own qualification)
	case model.RuleTypeNoOverlap, model.RuleTypeCoverage:
		// no required configuration keys
	}
	return nil
}
