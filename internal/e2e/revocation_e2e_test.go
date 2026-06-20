package e2e_test

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// POST /me/sessions/revoke raises the caller's own revocation floor.
func TestE2E_RevokeMySessions(t *testing.T) {
	var revokedSub atomic.Value
	mocks := emptyMocks()
	mocks.tokenRevocation.UpsertFn = func(_ context.Context, sub string, _ time.Time) error {
		revokedSub.Store(sub)
		return nil
	}
	ts := newTestServer(t, mocks)

	resp := rawGet2(t, http.MethodPost, ts.URL+"/api/v1/me/sessions/revoke", map[string]string{
		hTenantID: uuid.New().String(),
		hUserID:   uuid.New().String(),
		hSub:      "caller-sub",
		hRole:     "admin", // keeps the header-injected tenant; avoids employee lookup
	})
	drain(resp)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	require.NotNil(t, revokedSub.Load())
	assert.Equal(t, "caller-sub", revokedSub.Load(), "a caller may only revoke their own subject")
}

// POST /admin/employees/{id}/sessions/revoke revokes the target employee's
// Socrate subject (their AuthID).
func TestE2E_AdminRevokeEmployeeSessions(t *testing.T) {
	empID := uuid.New()
	var revokedSub atomic.Value
	mocks := emptyMocks()
	mocks.emp.GetByIDGlobalFn = func(_ context.Context, id uuid.UUID) (*model.Employee, error) {
		require.Equal(t, empID, id)
		return &model.Employee{TenantScoped: model.TenantScoped{ID: empID, TenantID: uuid.New()}, AuthID: "target-sub"}, nil
	}
	mocks.tokenRevocation.UpsertFn = func(_ context.Context, sub string, _ time.Time) error {
		revokedSub.Store(sub)
		return nil
	}
	ts := newTestServer(t, mocks)

	resp := rawGet2(t, http.MethodPost,
		ts.URL+"/api/v1/admin/employees/"+empID.String()+"/sessions/revoke",
		map[string]string{
			hSub:  "admin-sub",
			hRole: "admin",
		})
	drain(resp)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	require.NotNil(t, revokedSub.Load())
	assert.Equal(t, "target-sub", revokedSub.Load())
}

// An employee who never claimed their account (empty AuthID) has no live
// sessions: the admin revoke is a no-op success and never writes a floor.
func TestE2E_AdminRevokeUnclaimedEmployeeIsNoop(t *testing.T) {
	empID := uuid.New()
	var upsertCalled atomic.Bool
	mocks := emptyMocks()
	mocks.emp.GetByIDGlobalFn = func(_ context.Context, id uuid.UUID) (*model.Employee, error) {
		return &model.Employee{TenantScoped: model.TenantScoped{ID: id, TenantID: uuid.New()}, AuthID: ""}, nil
	}
	mocks.tokenRevocation.UpsertFn = func(_ context.Context, _ string, _ time.Time) error {
		upsertCalled.Store(true)
		return nil
	}
	ts := newTestServer(t, mocks)

	resp := rawGet2(t, http.MethodPost,
		ts.URL+"/api/v1/admin/employees/"+empID.String()+"/sessions/revoke",
		map[string]string{hSub: "admin-sub", hRole: "admin"})
	drain(resp)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.False(t, upsertCalled.Load(), "no floor should be written for an unclaimed employee")
}

// rawGet2 issues an arbitrary-method request with explicit headers (no automatic
// tenant header), mirroring rawGet for non-GET verbs.
func rawGet2(t *testing.T, method, url string, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, url, nil)
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}
