package router

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"

	"github.com/ovander/backendkit/httpware"
	"github.com/ovander/parashift/internal/config"
	"github.com/ovander/parashift/internal/handler"
	"github.com/ovander/parashift/internal/middleware"
	"github.com/ovander/parashift/internal/pkg/metrics"
	"github.com/ovander/parashift/internal/pkg/tracing"
)

// Middleware holds all HTTP middleware instances.
type Middleware struct {
	// Auth is the authentication middleware. In production this wraps jwtauth.Middleware.Handler;
	// in tests it can be replaced with a lightweight stub that injects auth context directly.
	Auth           func(http.Handler) http.Handler
	Tenant         *middleware.TenantMiddleware
	RBAC           *middleware.RBACMiddleware
	GeneralLimiter *httpware.RateLimiter
	Logger         *logrus.Logger
}

// NewRouter creates and configures the main HTTP router.
func NewRouter(cfg *config.Config, handlers *handler.HandlerBundle, mw Middleware) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.CORS(cfg.AllowedOrigins))
	r.Use(httpware.RequestID)
	// Socrate calls made on a user's behalf (code exchange, refresh, revocation)
	// carry the browser's address, resolved from a loopback-trusted
	// X-Forwarded-For; never the browser's own header (report row S5).
	r.Use(middleware.SocrateClientAttribution())
	r.Use(httpware.Logger(mw.Logger))
	// Tracing runs after Logger (so it enriches the request logger with trace_id)
	// and outside Recover (so panics surface as 5xx spans). It is a no-op span when
	// tracing is disabled, so it is always safe to install (OBS-3).
	r.Use(tracing.Middleware)
	r.Use(httpware.SecurityHeaders)
	r.Use(httpware.BodyLimit(cfg.MaxRequestBodyBytes))
	r.Use(httpware.Recover(mw.Logger.WithField("component", "recover")))
	r.Use(middleware.Locale) // T1.5: inject Accept-Language locale into request context

	// Prometheus RED metrics (OBS-2). Gated by METRICS_ENABLED. When enabled, a
	// middleware records request rate/errors/duration for every matched route and
	// /metrics exposes the exposition format for scraping. The endpoint is kept at
	// root (no auth) so an in-cluster Prometheus can scrape it; restrict exposure
	// at the network layer rather than with app auth.
	if cfg.MetricsEnabled {
		mc := metrics.New()
		r.Use(mc.Middleware)
		r.Method(http.MethodGet, "/metrics", mc.Handler())
		mw.Logger.Info("metrics: Prometheus /metrics endpoint enabled")
	}

	// Public routes (no auth required) — kept at root so auth store can call them
	// without the /api/v1 prefix.
	r.Get("/healthz", handlers.Health.Check)
	r.Get("/readyz", handlers.Health.Ready)
	r.Get("/api/version", handlers.Version.Get)

	// Auth endpoints are public but rate-limited to resist brute-force and
	// credential-stuffing. A per-IP (not per-tenant) limiter is used here because
	// tenant context is not yet available at this stage of the request lifecycle.
	// Keying per source IP (20 req/s, burst 40) stops automated attacks from a
	// single origin without letting one abuser lock out everyone else (SEC-3).
	authLim := newIPRateLimiter(20, 40).middleware
	r.With(authLim).Post("/auth/callback", handlers.Auth.Callback)
	r.With(authLim).Post("/auth/refresh", handlers.Auth.Refresh)
	r.Post("/auth/logout", handlers.Auth.Logout)

	// All versioned API routes live under /api/v1 to match frontend axios calls.
	r.Route("/api/v1", func(r chi.Router) {

		// Dev-only: decode a JWT without validation to inspect alg/kid/claims.
		// Served only with ENV=development, never by default (report row S7).
		if cfg.IsDevelopment() {
			r.Get("/debug/token", handlers.Debug.DecodeToken)
		}

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(mw.Auth)
			r.Use(mw.GeneralLimiter.Handler)

			// Claim route: auth required but NO TenantMiddleware — the user has no Employee
			// record yet; they are binding themselves to one via the invite token.
			// Per-IP limited to blunt invite-token brute-force/enumeration (SEC-6),
			// since no tenant context exists here for the per-tenant limiter.
			claimLim := newIPRateLimiter(5, 10).middleware
			r.With(claimLim, httpware.Timeout(5*time.Second)).Post("/claim/{token}", handlers.Claim.Claim)

			r.Use(mw.Tenant.Handler)

			// Standard CRUD operations (5s timeout)
			r.With(httpware.Timeout(5 * time.Second)).Group(func(r chi.Router) {

				// Fail closed: every handler in this group is tenant-scoped, so reject
				// any request that reached here without a tenant in context rather than
				// letting it run against the nil tenant (backendkit v1.8.0). Admins are
				// not affected — their cross-tenant surface lives under /admin, which is
				// intentionally outside this group.
				r.Use(httpware.RequireTenant)

				// Global option lists — single source of truth for all selectable values.
				// No RBAC required: any authenticated user may read options.
				r.Get("/options", handlers.Options.GetOptions)

				// Public holiday reference data (French government API, cached in DB)
				r.Get("/public-holidays", handlers.PublicHoliday.List)

				// Current user endpoints
				r.Get("/me", handlers.Me.GetProfile)
				r.Patch("/me/locale", handlers.Me.UpdateLocale)
				r.Get("/me/schedule", handlers.Me.GetMySchedule)
				r.Get("/me/schedule.ics", handlers.Me.ExportICS)
				// Self-service "log out everywhere": revoke all of the caller's sessions.
				r.Post("/me/sessions/revoke", handlers.Revocation.RevokeMine)

				// Store endpoints
				r.Get("/stores/me", handlers.Store.GetMyStore)
				r.With(mw.RBAC.Require(middleware.PermManageStore)).Put("/stores/me", handlers.Store.UpdateMyStore)

				// Employee routes
				r.Route("/stores/{storeId}/employees", func(r chi.Router) {
					r.With(mw.RBAC.Require(middleware.PermViewEmployees)).Get("/", handlers.Employee.List)
					r.With(mw.RBAC.Require(middleware.PermManageEmployees)).Post("/", handlers.Employee.Create)
					r.With(mw.RBAC.Require(middleware.PermViewEmployees)).Get("/{employeeId}", handlers.Employee.Get)
					r.With(mw.RBAC.Require(middleware.PermManageEmployees)).Put("/{employeeId}", handlers.Employee.Update)
					r.With(mw.RBAC.Require(middleware.PermManageEmployees)).Delete("/{employeeId}", handlers.Employee.Delete)

					// Week templates
					r.Get("/{employeeId}/week-templates", handlers.WeekTemplate.GetTemplates)
					r.With(mw.RBAC.Require(middleware.PermManageSchedule)).Put("/{employeeId}/week-templates", handlers.WeekTemplate.UpsertTemplates)

					// Availability
					r.Post("/{employeeId}/availability", handlers.Availability.SetAvailability)
					r.Get("/{employeeId}/availability", handlers.Availability.GetAvailability)
					r.Get("/{employeeId}/availability/range", handlers.Availability.ListAvailability)

					// Employee-scoped schedule and leave views (manager access)
					r.With(mw.RBAC.Require(middleware.PermViewSchedule)).Get("/{employeeId}/schedule", handlers.Schedule.ListEmployeeSchedule)
					r.With(mw.RBAC.Require(middleware.PermManageLeave)).Get("/{employeeId}/leave-requests", handlers.Leave.ListByEmployee)

					// Employee-scoped qualifications routes (Sprint 2)
					r.Get("/{employeeId}/qualifications", handlers.Qualification.ListForEmployee)
					r.With(mw.RBAC.Require(middleware.PermManageEmployees)).Post("/{employeeId}/qualifications", handlers.Qualification.AddToEmployee)
					r.With(mw.RBAC.Require(middleware.PermManageEmployees)).Put("/{employeeId}/qualifications/{eqId}", handlers.Qualification.UpdateForEmployee)
					r.With(mw.RBAC.Require(middleware.PermManageEmployees)).Delete("/{employeeId}/qualifications/{eqId}", handlers.Qualification.RemoveFromEmployee)
				})

				// Schedule routes
				r.Route("/stores/{storeId}", func(r chi.Router) {
					r.With(mw.RBAC.Require(middleware.PermViewSchedule)).Get("/schedule", handlers.Schedule.GetSchedule)
					r.With(mw.RBAC.Require(middleware.PermManageSchedule)).Post("/schedule/publish", handlers.Schedule.PublishSchedule)
					// Planner collection endpoints — return flat arrays for a given week.
					r.With(mw.RBAC.Require(middleware.PermViewSchedule)).Get("/shifts", handlers.Schedule.ListShifts)
					r.With(mw.RBAC.Require(middleware.PermViewSchedule)).Get("/assignments", handlers.Schedule.ListAssignments)
					r.With(mw.RBAC.Require(middleware.PermManageSchedule)).Post("/shifts", handlers.Schedule.CreateShift)
					r.With(mw.RBAC.Require(middleware.PermViewSchedule)).Get("/shifts/{shiftId}", handlers.Schedule.GetShift)
					r.With(mw.RBAC.Require(middleware.PermManageSchedule)).Put("/shifts/{shiftId}", handlers.Schedule.UpdateShift)
					r.With(mw.RBAC.Require(middleware.PermManageSchedule)).Delete("/shifts/{shiftId}", handlers.Schedule.DeleteShift)
					r.With(mw.RBAC.Require(middleware.PermViewSchedule)).Get("/shifts/{shiftId}/assignments", handlers.Schedule.GetAssignments)
					r.With(mw.RBAC.Require(middleware.PermManageSchedule)).Post("/assignments", handlers.Schedule.CreateAssignment)
					r.With(mw.RBAC.Require(middleware.PermManageSchedule)).Delete("/assignments/{assignmentId}", handlers.Schedule.DeleteAssignment)
					r.With(mw.RBAC.Require(middleware.PermManageSchedule)).Delete("/assignments", handlers.Schedule.ResetWeekAssignments)
					r.With(mw.RBAC.Require(middleware.PermManageSchedule)).Post("/shifts/regenerate", handlers.Schedule.RegenerateWeek)
					r.With(mw.RBAC.Require(middleware.PermManageSchedule)).Post("/shifts/generate", handlers.Schedule.GenerateSchedule)

					// Coverage routes
					r.With(mw.RBAC.Require(middleware.PermViewCoverage)).Get("/coverage", handlers.Coverage.GetCoverage)
					r.With(mw.RBAC.Require(middleware.PermViewCoverage)).Get("/coverage/gaps", handlers.Coverage.GetGaps)
					r.With(mw.RBAC.Require(middleware.PermManageStore)).Get("/coverage-requirements", handlers.Coverage.ListRequirements)
					r.With(mw.RBAC.Require(middleware.PermManageStore)).Post("/coverage-requirements", handlers.Coverage.CreateRequirement)
					r.With(mw.RBAC.Require(middleware.PermManageStore)).Put("/coverage-requirements/{reqId}", handlers.Coverage.UpdateRequirement)
					r.With(mw.RBAC.Require(middleware.PermManageStore)).Delete("/coverage-requirements/{reqId}", handlers.Coverage.DeleteRequirement)

					// Leave request routes
					r.Post("/leave-requests", handlers.Leave.Create)
					r.With(mw.RBAC.Require(middleware.PermViewLeave)).Get("/leave-requests", handlers.Leave.List)
					r.Get("/leave-requests/{leaveId}", handlers.Leave.Get)
					r.Delete("/leave-requests/{leaveId}", handlers.Leave.Delete)
					r.With(mw.RBAC.Require(middleware.PermManageLeave)).Put("/leave-requests/{leaveId}/review", handlers.Leave.Review)
					r.With(mw.RBAC.Require(middleware.PermManageLeave)).Get("/leave-requests/{leaveId}/impact", handlers.Leave.GetImpact)

					// Swap request routes
					r.Post("/swap-requests", handlers.Swap.Create)
					r.Get("/swap-requests", handlers.Swap.List)
					r.Get("/swap-requests/{swapId}", handlers.Swap.Get)
					r.With(mw.RBAC.Require(middleware.PermManageSwap)).Put("/swap-requests/{swapId}/review", handlers.Swap.Review)

					// Scheduling rules (Phase 2)
					r.With(mw.RBAC.Require(middleware.PermManageStore)).Route("/rules", func(r chi.Router) {
						r.Get("/", handlers.Rule.List)
						r.Post("/", handlers.Rule.Create)
						r.Get("/{ruleId}", handlers.Rule.Get)
						r.Put("/{ruleId}", handlers.Rule.Update)
						r.Delete("/{ruleId}", handlers.Rule.Delete)
					})

					// Store exception routes (exceptional openings / forced closures)
					r.With(mw.RBAC.Require(middleware.PermViewSchedule)).Get("/exceptions", handlers.StoreException.List)
					r.With(mw.RBAC.Require(middleware.PermManageSchedule)).Post("/exceptions", handlers.StoreException.Create)
					r.With(mw.RBAC.Require(middleware.PermManageSchedule)).Delete("/exceptions/{exceptionId}", handlers.StoreException.Delete)

					// Shift slot template routes
					r.With(mw.RBAC.Require(middleware.PermViewSchedule)).Get("/templates", handlers.ShiftSlot.List)
					r.With(mw.RBAC.Require(middleware.PermManageSchedule)).Post("/templates", handlers.ShiftSlot.Create)
					r.With(mw.RBAC.Require(middleware.PermManageSchedule)).Delete("/templates/{templateId}", handlers.ShiftSlot.Delete)
					r.With(mw.RBAC.Require(middleware.PermManageSchedule)).Post("/templates/{templateId}/publish", handlers.ShiftSlot.Publish)

					// Schedule plan lifecycle routes (Sprint 1)
					r.With(mw.RBAC.Require(middleware.PermViewSchedule)).Get("/plans", handlers.SchedulePlan.Get)
					r.With(mw.RBAC.Require(middleware.PermViewSchedule)).Get("/plans/{planId}", handlers.SchedulePlan.GetByID)
					r.With(mw.RBAC.Require(middleware.PermManageSchedule)).Post("/plans/{planId}/publish", handlers.SchedulePlan.Publish)
					r.With(mw.RBAC.Require(middleware.PermManageSchedule)).Post("/plans/{planId}/override", handlers.SchedulePlan.RecordOverride)
					r.With(mw.RBAC.Require(middleware.PermViewSchedule)).Get("/plans/{planId}/history", handlers.SchedulePlan.GetHistory)
					r.With(mw.RBAC.Require(middleware.PermManageSchedule)).Post("/plans/{planId}/rollback", handlers.SchedulePlan.Rollback)

					// Store-level qualifications routes (Sprint 2)
					r.With(mw.RBAC.Require(middleware.PermManageStore)).Get("/qualifications", handlers.Qualification.List)
					r.With(mw.RBAC.Require(middleware.PermManageStore)).Post("/qualifications", handlers.Qualification.Create)
					r.With(mw.RBAC.Require(middleware.PermManageStore)).Put("/qualifications/{qualId}", handlers.Qualification.Update)
					r.With(mw.RBAC.Require(middleware.PermManageStore)).Delete("/qualifications/{qualId}", handlers.Qualification.Delete)
					r.With(mw.RBAC.Require(middleware.PermManageStore)).Get("/qualifications/expiring", handlers.Qualification.ListExpiring)

					// Planning model metrics routes (Sprint 3)
					r.With(mw.RBAC.Require(middleware.PermViewSchedule)).Get("/models/{scheme}/metrics", handlers.PlanningModelMetric.GetMetrics)

					// AI Engine (Phase 3) — only registered when handler is available
					if handlers.AI != nil {
						r.With(mw.RBAC.Require(middleware.PermViewSchedule)).Get("/ai/suggest-assignment", handlers.AI.SuggestAssignment)
						r.With(mw.RBAC.Require(middleware.PermViewSchedule)).Get("/ai/optimize", handlers.AI.OptimizeSchedule)
						r.With(mw.RBAC.Require(middleware.PermViewSchedule)).Get("/ai/insights", handlers.AI.ListInsights)
						r.With(mw.RBAC.Require(middleware.PermManageSchedule)).Put("/ai/insights/{insightId}/dismiss", handlers.AI.DismissInsight)
					}
				})
			})

			// Admin routes (10s timeout for potentially longer operations).
			// Gated by PermPlatformAdmin — granted ONLY to the platform "admin" role —
			// so tenant-scoped managers (who hold PermManageStore) cannot reach the
			// cross-tenant admin surface (employee/store/audit CRUD across tenants).
			r.With(httpware.Timeout(10*time.Second)).Route("/admin", func(r chi.Router) {
				r.Use(mw.RBAC.Require(middleware.PermPlatformAdmin))

				// Dashboard
				r.Get("/dashboard", handlers.AdminDashboard.GetDashboard)
				// Legacy stats endpoint (kept for backward compatibility)
				r.Get("/stats", handlers.Admin.GetStats)

				// Static metadata (roles, statuses, integrity filters)
				r.Get("/metadata", handlers.Metadata.GetMetadata)

				// Store CRUD
				r.Get("/stores", handlers.Store.List)
				r.Post("/stores", handlers.Store.Create)
				r.Get("/stores/{storeId}", handlers.Store.Get)
				r.Put("/stores/{storeId}", handlers.Store.Update)
				r.Delete("/stores/{storeId}", handlers.Store.Delete)

				// Cross-tenant employee CRUD (any role, any store)
				r.Get("/employees", handlers.AdminEmployee.List)
				r.Post("/employees", handlers.AdminEmployee.Create)
				r.Get("/employees/{employeeId}", handlers.AdminEmployee.Get)
				r.Put("/employees/{employeeId}", handlers.AdminEmployee.Update)
				r.Delete("/employees/{employeeId}", handlers.AdminEmployee.Delete)
				// Admin-driven instant session revocation for a target employee.
				r.Post("/employees/{employeeId}/sessions/revoke", handlers.Revocation.RevokeEmployee)

				// Manager-specific CRUD (role=manager subset)
				r.Get("/managers", handlers.AdminManager.List)
				r.Post("/managers", handlers.AdminManager.Create)
				r.Get("/managers/{managerId}", handlers.AdminManager.Get)
				r.Put("/managers/{managerId}", handlers.AdminManager.Update)
				r.Delete("/managers/{managerId}", handlers.AdminManager.Delete)
				r.Post("/managers/{managerId}/resend-invite", handlers.AdminManager.ResendInvite)

				// Audit log query endpoint
				r.Get("/audit-logs", handlers.Admin.ListAuditLogs)
				r.Get("/audit-logs/tenant", handlers.Admin.ListAuditLogsForTenant)
			})

			// Manager routes — store-scoped via JWT context (TenantMiddleware already applied)
			r.With(httpware.Timeout(5*time.Second)).Route("/manager", func(r chi.Router) {
				r.Use(httpware.RequireTenant) // tenant-scoped: never run without a tenant
				r.Use(mw.RBAC.Require(middleware.PermManageEmployees))
				r.Get("/employees", handlers.ManagerEmployee.List)
				r.Post("/employees", handlers.ManagerEmployee.Create)
				r.Get("/employees/{employeeId}", handlers.ManagerEmployee.Get)
				r.Put("/employees/{employeeId}", handlers.ManagerEmployee.Update)
				r.Delete("/employees/{employeeId}", handlers.ManagerEmployee.Delete)
			})
		})
	})

	return r
}
