package e2e_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rawGet issues a GET with arbitrary auth headers (no automatic tenant header),
// so we can exercise the fail-closed RequireTenant guard directly.
func rawGet(t *testing.T, url string, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// backendkit v1.8.0 RequireTenant: a tenant-scoped route must never run without a
// tenant in context. A platform admin bypasses TenantMiddleware (no tenant is
// resolved), so hitting a tenant-scoped CRUD route as an admin without a tenant
// header is rejected with 401 rather than running against the nil tenant.
func TestE2E_RequireTenant_BlocksTenantlessRequest(t *testing.T) {
	ts := newTestServer(t, emptyMocks())

	resp := rawGet(t, ts.URL+"/api/v1/options", map[string]string{
		hSub:  "admin-sub",
		hRole: "admin", // admin → TenantMiddleware bypasses, leaving no tenant in ctx
		// deliberately NO X-Test-TenantID
	})
	drain(resp)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"tenant-scoped route must reject a request with no tenant in context")
}

// With a tenant present in context the same route is reachable (RequireTenant is a
// pass-through), proving the guard is not over-blocking legitimate traffic.
func TestE2E_RequireTenant_AllowsTenantScopedRequest(t *testing.T) {
	ts := newTestServer(t, emptyMocks())

	resp := rawGet(t, ts.URL+"/api/v1/options", map[string]string{
		hTenantID: uuid.New().String(),
		hUserID:   uuid.New().String(),
		hSub:      "admin-sub",
		hRole:     "admin", // admin keeps the header-injected tenant (no DB lookup)
	})
	drain(resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"a request carrying a tenant must pass the RequireTenant guard")
}
