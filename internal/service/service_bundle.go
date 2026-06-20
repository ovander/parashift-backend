package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ovander/backendkit/aigateway"
	"github.com/ovander/backendkit/socrate"
	"github.com/ovander/parashift/internal/config"
	"github.com/ovander/parashift/internal/event"
	"github.com/ovander/parashift/internal/repo"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ServiceBundle holds all service instances.
type ServiceBundle struct {
	Emitter             *event.Emitter
	Store               *StoreService
	Employee            *EmployeeService
	Schedule            *ScheduleService
	Coverage            *CoverageService
	Availability        *AvailabilityService
	Leave               *LeaveService
	Swap                *SwapService
	Admin               *AdminService
	AdminDashboard      *AdminDashboardService
	Rule                *RuleService
	ShiftSlot           *ShiftSlotService
	AI                  *AIService
	RuleEngine          *RuleEngine
	SchedulePlan        *SchedulePlanService
	Qualification       *QualificationService
	PlanningModelMetric *PlanningModelMetricService
	PublicHoliday       *PublicHolidayService
	StoreException      *StoreExceptionService
	Revocation          *RevocationService
	DB                  *gorm.DB
}

// NewServiceBundle wires all services with their dependencies.
// cfg is optional (pass nil to skip AI gateway construction, e.g. in tests).
func NewServiceBundle(repos *repo.RepoBundle, logger *logrus.Entry, cfgs ...*config.Config) *ServiceBundle {
	emitter := event.NewEmitter()
	emitter.SetLogger(logger.WithField("component", "event"))

	// Wire audit subscriber synchronously (OBS-1): inline writes are never
	// dropped under burst, never lost on crash, and avoid the shared-pointer
	// data race of async dispatch. Panics are isolated by the emitter.
	auditSub := event.NewAuditSubscriber(repos.DB, logger.WithField("component", "audit"))
	emitter.Subscribe(auditSub)

	// Build the rule engine (shared by Schedule and Swap services).
	ruleEngine := NewRuleEngine(
		repos.Rule, repos.ShiftAssignment, repos.Employee,
		logger.WithField("component", "rule_engine"),
	)

	schedule := NewScheduleService(
		repos.ShiftInstance, repos.ShiftAssignment, repos.WeekTemplate,
		repos.Employee, repos.LeaveRequest, repos.Availability,
		repos.Store,
		emitter, logger.WithField("service", "schedule"),
	).WithRuleEngine(ruleEngine).WithTxRunner(repos.WithTx)

	swap := NewSwapService(
		repos.SwapRequest, repos.ShiftInstance, repos.ShiftAssignment, repos.Employee,
		repos.DB, emitter, logger.WithField("service", "swap"),
	).WithRuleEngine(ruleEngine)

	// Build AI service — requires an API key. When not configured the service is
	// still created but gateway calls will fail gracefully (degrade to nil results).
	var aiSvc *AIService
	var cfg *config.Config
	if len(cfgs) > 0 && cfgs[0] != nil {
		cfg = cfgs[0]
	}

	// Build the Socrate client for service-account operations (invite emails, etc.).
	// A nil client is safe — EmployeeService degrades gracefully to the claim-token path.
	var socrateClient *socrate.Client
	if cfg != nil && cfg.Socrate.ClientID != "" && cfg.Socrate.ClientSecret != "" {
		// If SOCRATE_APP_ID is not set, resolve it automatically from the service
		// token's sub claim (format: "app:<numeric_id>").  This avoids a manual
		// copy-paste step and removes the need to re-set the ID after a Socrate
		// DB reset or app re-registration.
		appID := cfg.Socrate.AppID
		if appID == "" {
			if resolved, err := resolveAppIDFromToken(cfg.Socrate.BaseURL, cfg.Socrate.ClientID, cfg.Socrate.ClientSecret); err == nil {
				appID = resolved
				logger.WithField("app_id", appID).Info("SOCRATE_APP_ID resolved automatically from service token sub claim")
			} else {
				logger.WithError(err).Warn("SOCRATE_APP_ID not set and auto-resolution failed — service-account calls will fail at runtime")
			}
		}
		if sc, err := socrate.NewClient(socrate.ClientConfig{
			BaseURL:      cfg.Socrate.BaseURL,
			AdminBaseURL: cfg.Socrate.AdminURL,
			ClientID:     cfg.Socrate.ClientID,
			ClientSecret: cfg.Socrate.ClientSecret,
			AppID:        appID,
		}); err == nil {
			socrateClient = sc
		}
	}

	var gateway *aigateway.Client
	if cfg != nil && cfg.AI.AnthropicAPIKey != "" {
		gateway = aigateway.New(aigateway.Config{
			Provider:   "claude",
			APIKey:     cfg.AI.AnthropicAPIKey,
			Model:      cfg.AI.Model,
			MaxTokens:  cfg.AI.MaxTokens,
			TimeoutSec: cfg.AI.TimeoutSec,
		}, logger.WithField("component", "aigateway"))
	}

	if gateway != nil {
		aiSvc = NewAIService(
			gateway,
			repos.AIInsight, repos.Employee, repos.ShiftInstance,
			repos.ShiftAssignment, repos.CoverageRequirement, repos.Contract,
			logger.WithField("service", "ai"),
		)
	}

	// Create planning model metric service
	metricSvc := NewPlanningModelMetricService(repos.PlanningModelMetric, logger.WithField("service", "planning_model_metric"))

	// Create schedule plan service and wire metric service
	schedulePlanSvc := NewSchedulePlanService(repos.SchedulePlan, repos.ShiftInstance, logger.WithField("service", "schedule_plan")).
		WithMetricService(metricSvc)

	// Create qualification service
	qualSvc := NewQualificationService(repos.Qualification, repos.EmployeeQualification, logger.WithField("service", "qualification"))

	// Create public holiday service
	publicHolidaySvc := NewPublicHolidayService(repos.PublicHoliday, logger.WithField("service", "public_holiday"))

	// Create store exception service
	storeExceptionSvc := NewStoreExceptionService(repos.StoreException)

	// Create revocation service (instant session revocation; wired into the auth
	// middleware via jwtauth.WithRevocationCheck in bootstrap).
	revocationSvc := NewRevocationService(repos.TokenRevocation, logger.WithField("service", "revocation"))

	// Attach optional services to the schedule service
	schedule.WithPublicHolidayService(publicHolidaySvc)
	schedule.WithStoreExceptionRepo(repos.StoreException)

	return &ServiceBundle{
		Emitter: emitter,
		Store:   NewStoreService(repos.Store, emitter, logger.WithField("service", "store")),
		Employee: NewEmployeeService(repos.Employee, repos.Contract, emitter, logger.WithField("service", "employee")).
			WithSocrateClient(socrateClient),
		Schedule: schedule,
		Coverage: NewCoverageService(
			repos.CoverageRequirement, repos.ShiftInstance, repos.ShiftAssignment,
			repos.Employee, emitter, logger.WithField("service", "coverage"),
		).WithHolidayService(publicHolidaySvc).
			WithStoreExceptionRepo(repos.StoreException),
		Availability:       NewAvailabilityService(repos.Availability, repos.Employee, emitter, logger.WithField("service", "availability")),
		Leave:              NewLeaveService(repos.LeaveRequest, repos.Employee, repos.ShiftAssignment, repos.ShiftInstance, emitter, logger.WithField("service", "leave")),
		Swap:               swap,
		Admin:              NewAdminService(repos.AuditLog, repos.DB, logger.WithField("service", "admin")),
		AdminDashboard:     NewAdminDashboardService(repos.DB, logger.WithField("service", "admin_dashboard")),
		Rule:               NewRuleService(repos.Rule, logger.WithField("service", "rule")),
		ShiftSlot:          NewShiftSlotService(repos.ShiftSlot, repos.ShiftInstance, repos.Store, logger.WithField("service", "shift_slot")),
		AI:                 aiSvc,
		RuleEngine:         ruleEngine,
		SchedulePlan:       schedulePlanSvc,
		Qualification:      qualSvc,
		PlanningModelMetric: metricSvc,
		PublicHoliday:       publicHolidaySvc,
		StoreException:      storeExceptionSvc,
		Revocation:         revocationSvc,
		DB:                  repos.DB,
	}
}

// resolveAppIDFromToken exchanges client credentials for a service token and
// extracts the numeric app ID from the JWT sub claim ("app:<id>").
// This allows SOCRATE_APP_ID to be omitted from the environment — the ID is
// always present in the token Socrate issues, so we never need to hard-code it.
func resolveAppIDFromToken(baseURL, clientID, clientSecret string) (string, error) {
	tokenURL := strings.TrimRight(baseURL, "/") + "/oauth/token"
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.AccessToken == "" {
		return "", fmt.Errorf("token exchange returned no access_token (HTTP %d)", resp.StatusCode)
	}

	// Decode the JWT payload (middle section, base64url, no padding).
	parts := strings.Split(tok.AccessToken, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("service token is not a valid JWT")
	}
	padded := parts[1]
	switch len(padded) % 4 {
	case 2:
		padded += "=="
	case 3:
		padded += "="
	}
	payload, err := base64.URLEncoding.DecodeString(padded)
	if err != nil {
		return "", fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var claims struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Sub == "" {
		return "", fmt.Errorf("JWT payload has no sub claim")
	}

	// sub format: "app:<numeric_id>"
	numericID := strings.TrimPrefix(claims.Sub, "app:")
	if numericID == claims.Sub || numericID == "" {
		return "", fmt.Errorf("unexpected sub format: %q (expected \"app:<id>\")", claims.Sub)
	}
	return numericID, nil
}
