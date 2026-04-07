package e2e_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/testutil"
)

// TestE2E_SchedulePlan_GetOrCreate tests creating or retrieving a plan for a week.
func TestE2E_SchedulePlan_GetOrCreate(t *testing.T) {
	mocks := emptyMocks()
	ts := newTestServer(t, mocks)
	defer ts.Close()

	tenantID := uuid.New()
	userID := uuid.New()
	storeID := tenantID // StoreID == TenantID

	withManagerEmp(mocks, tenantID, "user123")
	client := newClient(ts, tenantID, userID, "user123", "manager")

	// Plan doesn't exist yet
	weekStr := "2026-04-27" // Monday
	resp := client.do(t, "GET", "/stores/"+storeID.String()+"/plans?week="+weekStr, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var plan dto.SchedulePlanResponse
	decode(t, resp, &plan)
	if plan.State != "DRAFT" {
		t.Fatalf("expected DRAFT state, got %s", plan.State)
	}
}

// TestE2E_SchedulePlan_Publish tests publishing a plan.
func TestE2E_SchedulePlan_Publish(t *testing.T) {
	mocks := emptyMocks()

	planID := uuid.New()
	plan := &model.SchedulePlan{
		TenantScoped: model.TenantScoped{
			ID:        planID,
			TenantID:  uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		WeekStart:   time.Now(),
		State:       model.PlanStateDraft,
		Snapshots:   "[]",
		OverrideLog: "[]",
	}

	mocks.schedulePlan.GetByIDFn = func(ctx context.Context, tenantID, id uuid.UUID) (*model.SchedulePlan, error) {
		if id == planID {
			return plan, nil
		}
		return nil, nil
	}

	mocks.schedulePlan.UpdateFn = func(ctx context.Context, p *model.SchedulePlan) error {
		plan = p
		return nil
	}

	mocks.shift.SetStatusByDateRangeFn = func(ctx context.Context, tenantID uuid.UUID, from, to time.Time, status string) (int64, error) {
		return 5, nil
	}

	mocks.shift.ListByDateRangeFn = func(ctx context.Context, tenantID uuid.UUID, from, to time.Time, page, pageSize int) ([]*model.ShiftInstance, int64, error) {
		return []*model.ShiftInstance{}, 0, nil
	}

	ts := newTestServer(t, mocks)
	defer ts.Close()

	tenantID := plan.TenantID
	userID := uuid.New()
	storeID := tenantID

	withManagerEmp(mocks, tenantID, "user123")
	client := newClient(ts, tenantID, userID, "user123", "manager")

	resp := client.do(t, "POST", "/stores/"+storeID.String()+"/plans/"+planID.String()+"/publish",
		dto.PublishPlanRequest{Note: "Ready for publication"})

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var published dto.SchedulePlanResponse
	decode(t, resp, &published)
	if published.State != "PUBLISHED" {
		t.Fatalf("expected PUBLISHED state, got %s", published.State)
	}
}

// TestE2E_SchedulePlan_PublishAlreadyPublished tests publishing a non-DRAFT plan.
func TestE2E_SchedulePlan_PublishAlreadyPublished(t *testing.T) {
	mocks := emptyMocks()

	planID := uuid.New()
	publishedTime := time.Now()
	publishedBy := uuid.New()

	plan := &model.SchedulePlan{
		TenantScoped: model.TenantScoped{
			ID:        planID,
			TenantID:  uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		WeekStart:   time.Now(),
		State:       model.PlanStatePublished,
		PublishedAt: &publishedTime,
		PublishedBy: &publishedBy,
		Snapshots:   "[]",
		OverrideLog: "[]",
	}

	mocks.schedulePlan.GetByIDFn = func(ctx context.Context, tenantID, id uuid.UUID) (*model.SchedulePlan, error) {
		if id == planID {
			return plan, nil
		}
		return nil, nil
	}

	ts := newTestServer(t, mocks)
	defer ts.Close()

	tenantID := plan.TenantID
	userID := uuid.New()
	storeID := tenantID

	withManagerEmp(mocks, tenantID, "user123")
	client := newClient(ts, tenantID, userID, "user123", "manager")

	resp := client.do(t, "POST", "/stores/"+storeID.String()+"/plans/"+planID.String()+"/publish",
		dto.PublishPlanRequest{})

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

// TestE2E_SchedulePlan_GetHistory tests retrieving plan history.
func TestE2E_SchedulePlan_GetHistory(t *testing.T) {
	mocks := emptyMocks()

	planID := uuid.New()
	plan := &model.SchedulePlan{
		TenantScoped: model.TenantScoped{
			ID:        planID,
			TenantID:  uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		WeekStart:   time.Now(),
		State:       model.PlanStatePublished,
		Snapshots:   `[{"version":1,"captured_at":"2026-04-27T10:00:00Z","shift_count":5,"published_by":"00000000-0000-0000-0000-000000000001"}]`,
		OverrideLog: "[]",
	}

	mocks.schedulePlan.GetByIDFn = func(ctx context.Context, tenantID, id uuid.UUID) (*model.SchedulePlan, error) {
		if id == planID {
			return plan, nil
		}
		return nil, nil
	}

	ts := newTestServer(t, mocks)
	defer ts.Close()

	tenantID := plan.TenantID
	userID := uuid.New()
	storeID := tenantID

	withManagerEmp(mocks, tenantID, "user123")
	client := newClient(ts, tenantID, userID, "user123", "manager")

	resp := client.do(t, "GET", "/stores/"+storeID.String()+"/plans/"+planID.String()+"/history", nil)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// TestE2E_SchedulePlan_Rollback tests rolling back a plan.
func TestE2E_SchedulePlan_Rollback(t *testing.T) {
	mocks := emptyMocks()

	planID := uuid.New()
	plan := &model.SchedulePlan{
		TenantScoped: model.TenantScoped{
			ID:        planID,
			TenantID:  uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		WeekStart:   time.Now(),
		State:       model.PlanStatePublished,
		Snapshots:   "[]",
		OverrideLog: "[]",
	}

	mocks.schedulePlan.GetByIDFn = func(ctx context.Context, tenantID, id uuid.UUID) (*model.SchedulePlan, error) {
		if id == planID {
			return plan, nil
		}
		return nil, nil
	}

	mocks.schedulePlan.UpdateFn = func(ctx context.Context, p *model.SchedulePlan) error {
		plan = p
		return nil
	}

	mocks.shift.SetStatusByDateRangeFn = func(ctx context.Context, tenantID uuid.UUID, from, to time.Time, status string) (int64, error) {
		return 5, nil
	}

	ts := newTestServer(t, mocks)
	defer ts.Close()

	tenantID := plan.TenantID
	userID := uuid.New()
	storeID := tenantID

	withManagerEmp(mocks, tenantID, "user123")
	client := newClient(ts, tenantID, userID, "user123", "manager")

	resp := client.do(t, "POST", "/stores/"+storeID.String()+"/plans/"+planID.String()+"/rollback",
		dto.RollbackPlanRequest{})

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var rolled dto.SchedulePlanResponse
	decode(t, resp, &rolled)
	if rolled.State != "DRAFT" {
		t.Fatalf("expected DRAFT state, got %s", rolled.State)
	}
}

// TestE2E_SchedulePlan_Employee_Forbidden tests that employees cannot publish.
func TestE2E_SchedulePlan_Employee_Forbidden(t *testing.T) {
	mocks := emptyMocks()

	planID := uuid.New()
	tenantID := uuid.New()

	mocks.schedulePlan.GetByIDFn = func(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*model.SchedulePlan, error) {
		return nil, nil
	}

	ts := newTestServer(t, mocks)
	defer ts.Close()

	userID := uuid.New()
	storeID := tenantID

	// Client with employee role
	empUser := testutil.NewEmployee(tenantID)
	empUser.AuthID = "user123"
	empUser.Position = "employee"
	mocks.emp.GetByAuthIDFn = func(_ context.Context, authID string) (*model.Employee, error) {
		if authID == "user123" {
			return empUser, nil
		}
		return nil, nil
	}
	client := newClient(ts, tenantID, userID, "user123", "employee")

	resp := client.do(t, "POST", "/stores/"+storeID.String()+"/plans/"+planID.String()+"/publish",
		dto.PublishPlanRequest{})

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}
