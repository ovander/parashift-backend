package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/sirupsen/logrus"

	"github.com/ovander/parashift/internal/event"
	"github.com/ovander/parashift/internal/testutil"
)

// ─── Logger / emitter helpers ────────────────────────────────────────────────

func newTestEmitter() *event.Emitter { return event.NewEmitter() }

func newTestLogger() *logrus.Entry {
	l := logrus.New()
	l.SetLevel(logrus.FatalLevel)
	return l.WithField("test", true)
}

// ─── Context helpers ─────────────────────────────────────────────────────────

// managerCtx returns a context with tenant=storeID, role=manager — passes validateStoreTenant.
func managerCtx(storeID uuid.UUID) context.Context {
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, storeID)
	ctx = ctxutil.WithUserID(ctx, uuid.New())
	ctx = ctxutil.WithUserSub(ctx, "sub-manager")
	ctx = ctxutil.WithUserRole(ctx, "manager")
	return ctx
}

// employeeCtx returns a context for a regular employee.
func employeeCtx(storeID, userID uuid.UUID, sub string) context.Context {
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, storeID)
	ctx = ctxutil.WithUserID(ctx, userID)
	ctx = ctxutil.WithUserSub(ctx, sub)
	ctx = ctxutil.WithUserRole(ctx, "employee")
	return ctx
}

// ─── HTTP test helpers ────────────────────────────────────────────────────────

func jsonBody(v any) *bytes.Buffer {
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(v)
	return &buf
}

func withChiURLParam(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func withChiURLParams(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func decodeJSON(rr *httptest.ResponseRecorder, v any) error {
	return json.NewDecoder(rr.Body).Decode(v)
}

// ─── Shared mock repos ────────────────────────────────────────────────────────

// all-noop mock bundles; tests override individual Fn fields as needed.
func emptyMocks() (
	*testutil.MockStoreRepo,
	*testutil.MockEmployeeRepo,
	*testutil.MockContractRepo,
	*testutil.MockShiftInstanceRepo,
	*testutil.MockShiftAssignmentRepo,
	*testutil.MockWeekTemplateRepo,
	*testutil.MockCoverageRequirementRepo,
	*testutil.MockAvailabilityRepo,
	*testutil.MockLeaveRequestRepo,
	*testutil.MockSwapRequestRepo,
) {
	return &testutil.MockStoreRepo{},
		&testutil.MockEmployeeRepo{},
		&testutil.MockContractRepo{},
		&testutil.MockShiftInstanceRepo{},
		&testutil.MockShiftAssignmentRepo{},
		&testutil.MockWeekTemplateRepo{},
		&testutil.MockCoverageRequirementRepo{},
		&testutil.MockAvailabilityRepo{},
		&testutil.MockLeaveRequestRepo{},
		&testutil.MockSwapRequestRepo{}
}
