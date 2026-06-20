// Package e2e contains end-to-end tests for the ParaShift HTTP API.
//
// Unlike unit tests (which mock at the service or repo level), these tests
// exercise the full HTTP stack — router, middleware (tenant + RBAC), handler,
// and service — using mock repositories as the only test boundary.
//
// # Test auth middleware
//
// The production JWT middleware is replaced with a lightweight stub that reads
// auth claims from four request headers:
//
//	X-Test-TenantID  — UUID of the store/tenant
//	X-Test-UserID    — UUID of the requesting user
//	X-Test-Sub       — Auth subject string (used by TenantMiddleware)
//	X-Test-UserRole  — Role string (employee | manager | admin)
//
// This lets tests set any identity without a running JWKS server.
package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/backendkit/httpware"
	"github.com/ovander/parashift/internal/config"
	"github.com/ovander/parashift/internal/event"
	"github.com/ovander/parashift/internal/handler"
	"github.com/ovander/parashift/internal/middleware"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/router"
	"github.com/ovander/parashift/internal/service"
	"github.com/ovander/parashift/internal/testutil"
	"github.com/sirupsen/logrus"
)

// ─── Auth header constants ────────────────────────────────────────────────────

const (
	hTenantID = "X-Test-TenantID"
	hUserID   = "X-Test-UserID"
	hSub      = "X-Test-Sub"
	hRole     = "X-Test-Role"
)

// testAuthMiddleware injects auth context values from the custom test headers,
// replacing the production jwtauth middleware.
func testAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if v := r.Header.Get(hTenantID); v != "" {
			if id, err := uuid.Parse(v); err == nil {
				ctx = ctxutil.WithTenantID(ctx, id)
			}
		}
		if v := r.Header.Get(hUserID); v != "" {
			if id, err := uuid.Parse(v); err == nil {
				ctx = ctxutil.WithUserID(ctx, id)
			}
		}
		ctx = ctxutil.WithUserSub(ctx, r.Header.Get(hSub))
		ctx = ctxutil.WithUserRole(ctx, r.Header.Get(hRole))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ─── Mock bundle ──────────────────────────────────────────────────────────────

// testMocks is a flat collection of all mock repositories used in E2E tests.
// Each test builds the mocks it needs and passes the bundle to newTestServer.
type testMocks struct {
	store                 *testutil.MockStoreRepo
	emp                   *testutil.MockEmployeeRepo
	contract              *testutil.MockContractRepo
	shift                 *testutil.MockShiftInstanceRepo
	assign                *testutil.MockShiftAssignmentRepo
	tmpl                  *testutil.MockWeekTemplateRepo
	coverage              *testutil.MockCoverageRequirementRepo
	avail                 *testutil.MockAvailabilityRepo
	leave                 *testutil.MockLeaveRequestRepo
	swap                  *testutil.MockSwapRequestRepo
	auditLog              *testutil.MockAuditLogRepo
	rule                  *testutil.MockRuleRepo
	aiInsight             *testutil.MockAIInsightRepo
	slot                  *testutil.MockShiftSlotRepo
	schedulePlan          *testutil.MockSchedulePlanRepo
	qualification         *testutil.MockQualificationRepo
	employeeQualification *testutil.MockEmployeeQualificationRepo
	planningModelMetric   *testutil.MockPlanningModelMetricRepo
	publicHoliday         *testutil.MockPublicHolidayRepo
	tokenRevocation       *testutil.MockTokenRevocationRepo
}

// emptyMocks returns a bundle where every Fn field is nil (no-op defaults).
func emptyMocks() *testMocks {
	return &testMocks{
		store:                 &testutil.MockStoreRepo{},
		emp:                   &testutil.MockEmployeeRepo{},
		contract:              &testutil.MockContractRepo{},
		shift:                 &testutil.MockShiftInstanceRepo{},
		assign:                &testutil.MockShiftAssignmentRepo{},
		tmpl:                  &testutil.MockWeekTemplateRepo{},
		coverage:              &testutil.MockCoverageRequirementRepo{},
		avail:                 &testutil.MockAvailabilityRepo{},
		leave:                 &testutil.MockLeaveRequestRepo{},
		swap:                  &testutil.MockSwapRequestRepo{},
		auditLog:              &testutil.MockAuditLogRepo{},
		rule:                  &testutil.MockRuleRepo{},
		aiInsight:             &testutil.MockAIInsightRepo{},
		slot:                  &testutil.MockShiftSlotRepo{},
		schedulePlan:          &testutil.MockSchedulePlanRepo{},
		qualification:         &testutil.MockQualificationRepo{},
		employeeQualification: &testutil.MockEmployeeQualificationRepo{},
		planningModelMetric:   &testutil.MockPlanningModelMetricRepo{},
		publicHoliday:         &testutil.MockPublicHolidayRepo{},
		tokenRevocation:       &testutil.MockTokenRevocationRepo{},
	}
}

// ─── Server builder ───────────────────────────────────────────────────────────

func newTestLogger() *logrus.Entry {
	l := logrus.New()
	l.SetLevel(logrus.FatalLevel)
	return l.WithField("e2e", true)
}

// newTestServer wires all services and handlers with the provided mock repos
// and spins up an httptest.Server running the full chi router.
// The caller is responsible for calling ts.Close() (use t.Cleanup).
func newTestServer(t *testing.T, mocks *testMocks) *httptest.Server {
	t.Helper()

	logger := newTestLogger()
	emitter := event.NewEmitter()

	storeSvc := service.NewStoreService(mocks.store, emitter, logger.WithField("svc", "store"))
	empSvc := service.NewEmployeeService(mocks.emp, mocks.contract, emitter, logger.WithField("svc", "employee"))
	scheduleSvc := service.NewScheduleService(
		mocks.shift, mocks.assign, mocks.tmpl,
		mocks.emp, mocks.leave, mocks.avail,
		mocks.store,
		emitter, logger.WithField("svc", "schedule"),
	)
	coverageSvc := service.NewCoverageService(
		mocks.coverage, mocks.shift, mocks.assign,
		mocks.emp, emitter, logger.WithField("svc", "coverage"),
	)
	availSvc := service.NewAvailabilityService(mocks.avail, mocks.emp, emitter, logger.WithField("svc", "avail"))
	leaveSvc := service.NewLeaveService(mocks.leave, mocks.emp, mocks.assign, mocks.shift, emitter, logger.WithField("svc", "leave"))
	swapSvc := service.NewSwapService(mocks.swap, mocks.shift, mocks.assign, mocks.emp, nil, emitter, logger.WithField("svc", "swap"))
	adminSvc := service.NewAdminService(mocks.auditLog, nil, logger.WithField("svc", "admin"))
	ruleSvc := service.NewRuleService(mocks.rule, logger.WithField("svc", "rule"))
	shiftSlotSvc := service.NewShiftSlotService(mocks.slot, mocks.shift, mocks.store, logger.WithField("svc", "shift_slot"))
	metricSvc := service.NewPlanningModelMetricService(mocks.planningModelMetric, logger.WithField("svc", "planning_model_metric"))
	schedulePlanSvc := service.NewSchedulePlanService(mocks.schedulePlan, mocks.shift, logger.WithField("svc", "schedule_plan")).
		WithMetricService(metricSvc)
	qualSvc := service.NewQualificationService(mocks.qualification, mocks.employeeQualification, logger.WithField("svc", "qualification"))
	holidaySvc := service.NewPublicHolidayService(mocks.publicHoliday, logger.WithField("svc", "public_holiday"))
	scheduleSvc.WithPublicHolidayService(holidaySvc)
	revocationSvc := service.NewRevocationService(mocks.tokenRevocation, logger.WithField("svc", "revocation"))

	svcBundle := &service.ServiceBundle{
		Emitter:             emitter,
		Store:               storeSvc,
		Employee:            empSvc,
		Schedule:            scheduleSvc,
		Coverage:            coverageSvc,
		Availability:        availSvc,
		Leave:               leaveSvc,
		Swap:                swapSvc,
		Admin:               adminSvc,
		Rule:                ruleSvc,
		ShiftSlot:           shiftSlotSvc,
		SchedulePlan:        schedulePlanSvc,
		Qualification:       qualSvc,
		PlanningModelMetric: metricSvc,
		PublicHoliday:       holidaySvc,
		Revocation:          revocationSvc,
	}

	cfg := &config.Config{MaxRequestBodyBytes: 10 << 20}

	// Pass nil for db — e2e tests use mock repos, not a real DB connection.
	// HealthHandler.Ready handles nil db gracefully (returns 503 with "not_configured").
	handlers := handler.NewHandlerBundle(svcBundle, cfg, nil, handler.BuildInfo{Version: "test", Commit: "test", BuildTime: "test"})

	rbacMW := middleware.NewRBACMiddleware(logger.WithField("mw", "rbac"))
	tenantMW := middleware.NewTenantMiddleware(mocks.emp, logger.WithField("mw", "tenant"), "" /* no userinfo in tests */)
	limiter := httpware.NewRateLimiter(10000, 20000)

	mw := router.Middleware{
		Auth:           testAuthMiddleware,
		Tenant:         tenantMW,
		RBAC:           rbacMW,
		GeneralLimiter: limiter,
		Logger:         logger.Logger,
	}

	ts := httptest.NewServer(router.NewRouter(cfg, handlers, mw))
	t.Cleanup(func() {
		ts.Close()
		emitter.Close()
		limiter.Stop()
	})
	return ts
}

// ─── Test client ─────────────────────────────────────────────────────────────

// testClient wraps an httptest.Server and sends authenticated requests.
type testClient struct {
	server   *httptest.Server
	tenantID uuid.UUID
	userID   uuid.UUID
	sub      string
	role     string
}

func newClient(ts *httptest.Server, tenantID, userID uuid.UUID, sub, role string) *testClient {
	return &testClient{server: ts, tenantID: tenantID, userID: userID, sub: sub, role: role}
}

// asManager returns a copy of the client acting as a manager.
func (c *testClient) asManager() *testClient {
	cp := *c
	cp.role = "manager"
	return &cp
}

// asEmployee returns a copy of the client acting as an employee.
func (c *testClient) asEmployee() *testClient {
	cp := *c
	cp.role = "employee"
	return &cp
}

// withTenant returns a copy of the client scoped to a different tenant.
func (c *testClient) withTenant(tenantID uuid.UUID) *testClient {
	cp := *c
	cp.tenantID = tenantID
	return &cp
}

// do performs a request against the test server, returning the response.
// body may be nil (no body sent) or any JSON-encodable value.
// Paths are automatically prefixed with /api/v1 unless they start with
// /healthz or /auth/ (those remain at the root level).
func (c *testClient) do(t *testing.T, method, path string, body any) *http.Response {
	t.Helper()

	// Prepend the API version prefix for all versioned routes.
	if !strings.HasPrefix(path, "/healthz") && !strings.HasPrefix(path, "/auth/") {
		path = "/api/v1" + path
	}

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(context.Background(), method, c.server.URL+path, bodyReader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	req.Header.Set(hTenantID, c.tenantID.String())
	req.Header.Set(hUserID, c.userID.String())
	req.Header.Set(hSub, c.sub)
	req.Header.Set(hRole, c.role)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request %s %s: %v", method, path, err)
	}
	return resp
}

// decode reads the response body as JSON into v.
func decode(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

// drain discards the response body (required to allow connection reuse).
func drain(resp *http.Response) {
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
}

// ─── Common mock setups ───────────────────────────────────────────────────────

// withManagerEmp configures empRepo so TenantMiddleware accepts the manager sub.
// The returned employee has TenantID == tenantID and Role == "manager".
// Role must be "manager" (not the testutil default "pharmacist") so that the
// TenantMiddleware role-injection grants manager-level RBAC permissions.
func withManagerEmp(mocks *testMocks, tenantID uuid.UUID, sub string) *model.Employee {
	emp := testutil.NewEmployee(tenantID)
	emp.AuthID = sub
	emp.Position = "manager" // TenantMiddleware injects this into the request context.
	mocks.emp.GetByAuthIDFn = func(_ context.Context, authID string) (*model.Employee, error) {
		if authID == sub {
			return emp, nil
		}
		return nil, nil
	}
	return emp
}
