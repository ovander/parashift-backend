package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/backendkit/httpware"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// newRBAC builds an RBACMiddleware with a silent logger for testing.
func newRBAC() *RBACMiddleware {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	return NewRBACMiddleware(logrus.NewEntry(logger))
}

// serveWithRole runs a request carrying the given role through the permission
// gate and returns the resulting HTTP status code. A 200 means allowed.
func serveWithRole(t *testing.T, role string, perm httpware.Permission) int {
	t.Helper()
	rbac := newRBAC()
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := rbac.Require(perm)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(ctxutil.WithUserRole(req.Context(), role))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

// TestRBAC_PlatformAdmin_OnlyAdminRole is the SEC-1 regression guard: the
// cross-tenant /admin surface is gated by PermPlatformAdmin, which must be
// held ONLY by the platform "admin" role — never by manager or employee.
func TestRBAC_PlatformAdmin_OnlyAdminRole(t *testing.T) {
	assert.Equal(t, http.StatusOK, serveWithRole(t, "admin", PermPlatformAdmin),
		"admin must be allowed PermPlatformAdmin")
	assert.Equal(t, http.StatusForbidden, serveWithRole(t, "manager", PermPlatformAdmin),
		"manager must NOT hold PermPlatformAdmin (cross-tenant escalation guard)")
	assert.Equal(t, http.StatusForbidden, serveWithRole(t, "employee", PermPlatformAdmin),
		"employee must NOT hold PermPlatformAdmin")
}

// TestRBAC_ManagerKeepsStoreManage confirms the fix does not strip managers of
// the tenant-scoped store:manage permission they legitimately need.
func TestRBAC_ManagerKeepsStoreManage(t *testing.T) {
	assert.Equal(t, http.StatusOK, serveWithRole(t, "manager", PermManageStore),
		"manager must retain PermManageStore for its own tenant routes")
	assert.Equal(t, http.StatusForbidden, serveWithRole(t, "employee", PermManageStore),
		"employee must not have PermManageStore")
}
