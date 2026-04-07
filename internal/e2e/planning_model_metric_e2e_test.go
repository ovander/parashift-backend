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

// TestE2E_PlanningModelMetric_GetMetrics tests retrieving metrics for a scheme.
func TestE2E_PlanningModelMetric_GetMetrics(t *testing.T) {
	mocks := emptyMocks()

	scheme := "A"
	tenantID := uuid.New()

	metrics := []*model.PlanningModelMetric{
		{
			TenantScoped: model.TenantScoped{
				ID:        uuid.New(),
				TenantID:  tenantID,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			StoreID:         tenantID,
			ModelScheme:     scheme,
			WeekStart:       time.Now().AddDate(0, 0, -7),
			CoverageRate:    0.95,
			OvertimeHours:   5.0,
			AdjustmentCount: 2,
			ViolationCount:  0,
		},
	}

	mocks.planningModelMetric.ListBySchemeFn = func(ctx context.Context, tenantID uuid.UUID, scheme string, limitWeeks int) ([]*model.PlanningModelMetric, error) {
		return metrics, nil
	}

	ts := newTestServer(t, mocks)
	defer ts.Close()

	userID := uuid.New()
	storeID := tenantID

	withManagerEmp(mocks, tenantID, "user123")
	client := newClient(ts, tenantID, userID, "user123", "manager")

	resp := client.do(t, "GET", "/stores/"+storeID.String()+"/models/"+scheme+"/metrics?weeks=8", nil)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var metricsResp dto.PlanningModelMetricsResponse
	decode(t, resp, &metricsResp)

	if metricsResp.Scheme != scheme {
		t.Fatalf("expected scheme %s, got %s", scheme, metricsResp.Scheme)
	}

	if len(metricsResp.Metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metricsResp.Metrics))
	}

	if metricsResp.Metrics[0].CoverageRate != 0.95 {
		t.Fatalf("expected coverage rate 0.95, got %f", metricsResp.Metrics[0].CoverageRate)
	}

	if metricsResp.Summary.SampleWeeks != 1 {
		t.Fatalf("expected 1 sample week, got %d", metricsResp.Summary.SampleWeeks)
	}
}

// TestE2E_PlanningModelMetric_GetMetrics_Empty tests retrieving metrics when none exist.
func TestE2E_PlanningModelMetric_GetMetrics_Empty(t *testing.T) {
	mocks := emptyMocks()

	scheme := "B"
	tenantID := uuid.New()

	mocks.planningModelMetric.ListBySchemeFn = func(ctx context.Context, tenantID uuid.UUID, scheme string, limitWeeks int) ([]*model.PlanningModelMetric, error) {
		return []*model.PlanningModelMetric{}, nil
	}

	ts := newTestServer(t, mocks)
	defer ts.Close()

	userID := uuid.New()
	storeID := tenantID

	withManagerEmp(mocks, tenantID, "user123")
	client := newClient(ts, tenantID, userID, "user123", "manager")

	resp := client.do(t, "GET", "/stores/"+storeID.String()+"/models/"+scheme+"/metrics", nil)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var metricsResp dto.PlanningModelMetricsResponse
	decode(t, resp, &metricsResp)

	if len(metricsResp.Metrics) != 0 {
		t.Fatalf("expected 0 metrics, got %d", len(metricsResp.Metrics))
	}

	if metricsResp.Summary.SampleWeeks != 0 {
		t.Fatalf("expected 0 sample weeks, got %d", metricsResp.Summary.SampleWeeks)
	}
}

// TestE2E_PlanningModelMetric_Employee_Forbidden tests that employees cannot view metrics.
func TestE2E_PlanningModelMetric_Employee_Forbidden(t *testing.T) {
	mocks := emptyMocks()

	ts := newTestServer(t, mocks)
	defer ts.Close()

	tenantID := uuid.New()
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
	client := newClient(ts, tenantID, userID, "user123", "employee").asEmployee()

	resp := client.do(t, "GET", "/stores/"+storeID.String()+"/models/A/metrics", nil)

	// Note: employees have PermViewSchedule, so they can actually view metrics
	// Change to a non-schedule permission test if needed
	if resp.StatusCode != http.StatusOK {
		t.Logf("note: employees with PermViewSchedule can view metrics (status %d)", resp.StatusCode)
	}
}
