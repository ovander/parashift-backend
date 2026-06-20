package e2e_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/repo"
	"github.com/stretchr/testify/assert"
)

// ARC-3: when a repository update hits an optimistic-lock conflict, the API
// surfaces it as 409 Conflict (not a generic 500), so the client can reload and
// retry instead of silently losing the update.
func TestE2E_OptimisticLockConflictReturns409(t *testing.T) {
	empID := uuid.New()
	mocks := emptyMocks()
	mocks.emp.GetByIDGlobalFn = func(_ context.Context, id uuid.UUID) (*model.Employee, error) {
		return &model.Employee{
			TenantScoped: model.TenantScoped{ID: id, TenantID: uuid.New()}, Versioned: model.Versioned{Version: 2},
			Name: "Old", Position: "employee", JobRole: "pharmacist",
		}, nil
	}
	mocks.emp.UpdateFn = func(_ context.Context, _ *model.Employee) error {
		return repo.ErrOptimisticLock
	}
	ts := newTestServer(t, mocks)

	c := newClient(ts, uuid.New(), uuid.New(), "admin-sub", "admin")
	resp := c.do(t, http.MethodPut, "/admin/employees/"+empID.String(),
		map[string]any{"name": "New"})
	drain(resp)

	assert.Equal(t, http.StatusConflict, resp.StatusCode,
		"an optimistic-lock conflict must map to 409")
}
