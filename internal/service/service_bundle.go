package service

import (
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
	// SocrateClient makes every call to Socrate (OAuth and service-account
	// admin API). nil when SOCRATE_CLIENT_ID / SOCRATE_CLIENT_SECRET are unset.
	SocrateClient *socrate.Client
	DB            *gorm.DB
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

	// Build the Socrate client: OAuth calls (code exchange, refresh, revocation,
	// profile) and service-account calls (invite emails, magic links).
	// A nil client is safe — EmployeeService degrades gracefully to the claim-token path.
	socrateClient := newSocrateClient(cfg, logger)

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
		Availability:        NewAvailabilityService(repos.Availability, repos.Employee, emitter, logger.WithField("service", "availability")),
		Leave:               NewLeaveService(repos.LeaveRequest, repos.Employee, repos.ShiftAssignment, repos.ShiftInstance, emitter, logger.WithField("service", "leave")),
		Swap:                swap,
		Admin:               NewAdminService(repos.AuditLog, repos.DB, logger.WithField("service", "admin")),
		AdminDashboard:      NewAdminDashboardService(repos.DB, logger.WithField("service", "admin_dashboard")),
		Rule:                NewRuleService(repos.Rule, logger.WithField("service", "rule")),
		ShiftSlot:           NewShiftSlotService(repos.ShiftSlot, repos.ShiftInstance, repos.Store, logger.WithField("service", "shift_slot")),
		AI:                  aiSvc,
		RuleEngine:          ruleEngine,
		SchedulePlan:        schedulePlanSvc,
		Qualification:       qualSvc,
		PlanningModelMetric: metricSvc,
		PublicHoliday:       publicHolidaySvc,
		StoreException:      storeExceptionSvc,
		Revocation:          revocationSvc,
		SocrateClient:       socrateClient,
		DB:                  repos.DB,
	}
}

// newSocrateClient builds the backendkit Socrate client, or returns nil when
// the client credentials are not configured.
//
// The app ID is SOCRATE_APP_ID, never derived: a service account cannot look
// up its own app ID at Socrate (GET /api/admin/apps is for human admins), and
// the app-scoped service routes need it (compat report row K2).
func newSocrateClient(cfg *config.Config, logger *logrus.Entry) *socrate.Client {
	if cfg == nil || cfg.Socrate.ClientID == "" || cfg.Socrate.ClientSecret == "" {
		return nil
	}
	if cfg.Socrate.AppID == "" {
		logger.Warn("SOCRATE_APP_ID not set — invite emails and other service-account calls will fail")
	}
	sc, err := socrate.NewClient(socrate.ClientConfig{
		BaseURL:      cfg.Socrate.BaseURL,
		AdminBaseURL: cfg.Socrate.AdminURL,
		ClientID:     cfg.Socrate.ClientID,
		ClientSecret: cfg.Socrate.ClientSecret,
		AppID:        cfg.Socrate.AppID,
	})
	if err != nil {
		logger.WithError(err).Warn("Socrate client not configured")
		return nil
	}
	return sc
}
