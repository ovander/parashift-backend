package handler

import (
	"gorm.io/gorm"

	"github.com/ovander/parashift/internal/config"
	"github.com/ovander/parashift/internal/service"
)

// HandlerBundle holds all HTTP handlers.
type HandlerBundle struct {
	Auth                *AuthHandler
	Version             *VersionHandler
	Health              *HealthHandler
	Store               *StoreHandler
	Employee            *EmployeeHandler
	AdminEmployee       *AdminEmployeeHandler
	AdminManager        *AdminManagerHandler
	ManagerEmployee     *ManagerEmployeeHandler
	Metadata            *MetadataHandler
	Options             *OptionsHandler
	Claim               *ClaimHandler
	Schedule            *ScheduleHandler
	WeekTemplate        *WeekTemplateHandler
	Coverage            *CoverageHandler
	Availability        *AvailabilityHandler
	Leave               *LeaveHandler
	Swap                *SwapHandler
	Me                  *MeHandler
	Admin               *AdminHandler
	AdminDashboard      *AdminDashboardHandler
	Rule                *RuleHandler
	ShiftSlot           *ShiftSlotHandler
	AI                  *AIHandler
	Debug               *DebugHandler
	SchedulePlan        *SchedulePlanHandler
	Qualification       *QualificationHandler
	PlanningModelMetric *PlanningModelMetricHandler
	PublicHoliday       *PublicHolidayHandler
	StoreException      *StoreExceptionHandler
	Revocation          *RevocationHandler
}

// BuildInfo holds the values injected at link time via -ldflags.
type BuildInfo struct {
	Version   string
	Commit    string
	BuildTime string
}

// NewHandlerBundle creates a new HandlerBundle with all handlers initialized.
// db is passed to the HealthHandler so /readyz can verify the database is reachable.
func NewHandlerBundle(svc *service.ServiceBundle, cfg *config.Config, db *gorm.DB, build BuildInfo) *HandlerBundle {
	b := &HandlerBundle{
		Auth:                NewAuthHandler(socrateTokens(svc), cfg.Socrate.RedirectURL),
		Version:             NewVersionHandler(build.Version, build.Commit, build.BuildTime),
		Health:              NewHealthHandler(db),
		Store:               NewStoreHandler(svc.Store),
		Employee:            NewEmployeeHandler(svc.Employee),
		AdminEmployee:       NewAdminEmployeeHandler(svc.Employee),
		AdminManager:        NewAdminManagerHandler(svc.Employee),
		ManagerEmployee:     NewManagerEmployeeHandler(svc.Employee),
		Metadata:            NewMetadataHandler(),
		Options:             NewOptionsHandler(),
		Claim:               NewClaimHandler(svc.Employee),
		Schedule:            NewScheduleHandler(svc.Schedule),
		WeekTemplate:        NewWeekTemplateHandler(svc.Schedule),
		Coverage:            NewCoverageHandler(svc.Coverage),
		Availability:        NewAvailabilityHandler(svc.Availability),
		Leave:               NewLeaveHandler(svc.Leave),
		Swap:                NewSwapHandler(svc.Swap),
		Me:                  NewMeHandler(svc.Employee, svc.Schedule),
		Admin:               NewAdminHandler(svc.Admin),
		AdminDashboard:      NewAdminDashboardHandler(svc.AdminDashboard),
		Rule:                NewRuleHandler(svc.Rule),
		ShiftSlot:           NewShiftSlotHandler(svc.ShiftSlot),
		Debug:               NewDebugHandler(),
		SchedulePlan:        NewSchedulePlanHandler(svc.SchedulePlan),
		Qualification:       NewQualificationHandler(svc.Qualification),
		PlanningModelMetric: NewPlanningModelMetricHandler(svc.PlanningModelMetric),
		PublicHoliday:       NewPublicHolidayHandler(svc.PublicHoliday),
		StoreException:      NewStoreExceptionHandler(svc.StoreException),
		Revocation:          NewRevocationHandler(svc.Revocation, svc.Employee),
	}
	// AI handler is optional — only created when the AI service is available.
	if svc.AI != nil {
		b.AI = NewAIHandler(svc.AI)
	}
	return b
}

// socrateTokens returns the bundle's Socrate client as SocrateTokens, or a nil
// interface (not a typed nil) when it is not configured.
func socrateTokens(svc *service.ServiceBundle) SocrateTokens {
	if svc == nil || svc.SocrateClient == nil {
		return nil
	}
	return svc.SocrateClient
}
