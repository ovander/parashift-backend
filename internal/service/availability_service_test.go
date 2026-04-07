package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/service"
	"github.com/ovander/parashift/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAvailabilityService(avRepo *testutil.MockAvailabilityRepo, empRepo *testutil.MockEmployeeRepo) *service.AvailabilityService {
	return service.NewAvailabilityService(avRepo, empRepo, newTestEmitter(), newTestLogger())
}

// ─── SetAvailability ──────────────────────────────────────────────────────────

func TestAvailabilityService_SetAvailability(t *testing.T) {
	tenantID := uuid.New()
	employeeID := uuid.New()
	upserted := false

	avRepo := &testutil.MockAvailabilityRepo{
		UpsertFn: func(_ context.Context, a *model.Availability) error {
			upserted = true
			assert.Equal(t, tenantID, a.TenantID)
			assert.Equal(t, employeeID, a.EmployeeID)
			return nil
		},
	}
	empRepo := &testutil.MockEmployeeRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Employee, error) {
			return testutil.NewEmployee(tenantID), nil
		},
	}
	svc := newAvailabilityService(avRepo, empRepo)

	req := dto.SetAvailabilityRequest{
		Date: time.Now(),
		TimeRanges: []dto.TimeSlot{
			{Start: "09:00", End: "17:00"},
		},
		Note: func() *string { s := "Available"; return &s }(),
	}
	got, err := svc.SetAvailability(context.Background(), tenantID, employeeID, req)
	require.NoError(t, err)
	assert.True(t, upserted)
	assert.Equal(t, employeeID, got.EmployeeID)
}

func TestAvailabilityService_SetAvailability_UpsertError(t *testing.T) {
	tenantID := uuid.New()
	employeeID := uuid.New()

	avRepo := &testutil.MockAvailabilityRepo{
		UpsertFn: func(_ context.Context, _ *model.Availability) error {
			return errors.New("db error")
		},
	}
	empRepo := &testutil.MockEmployeeRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Employee, error) {
			return testutil.NewEmployee(tenantID), nil
		},
	}
	svc := newAvailabilityService(avRepo, empRepo)

	_, err := svc.SetAvailability(context.Background(), tenantID, employeeID, dto.SetAvailabilityRequest{
		Date: time.Now(),
	})
	require.Error(t, err)
}

// ─── GetAvailability ──────────────────────────────────────────────────────────

func TestAvailabilityService_GetAvailability_Found(t *testing.T) {
	tenantID := uuid.New()
	employeeID := uuid.New()
	av := testutil.NewAvailability(tenantID, employeeID)
	date := time.Now()

	avRepo := &testutil.MockAvailabilityRepo{
		GetByEmployeeDateFn: func(_ context.Context, tID, eID uuid.UUID, d time.Time) (*model.Availability, error) {
			assert.Equal(t, tenantID, tID)
			assert.Equal(t, employeeID, eID)
			return av, nil
		},
	}
	svc := newAvailabilityService(avRepo, &testutil.MockEmployeeRepo{})

	got, err := svc.GetAvailability(context.Background(), tenantID, employeeID, date)
	require.NoError(t, err)
	assert.Equal(t, av.ID, got.ID)
}

func TestAvailabilityService_GetAvailability_NotFound(t *testing.T) {
	avRepo := &testutil.MockAvailabilityRepo{
		GetByEmployeeDateFn: func(_ context.Context, _, _ uuid.UUID, _ time.Time) (*model.Availability, error) {
			return nil, nil
		},
	}
	svc := newAvailabilityService(avRepo, &testutil.MockEmployeeRepo{})

	_, err := svc.GetAvailability(context.Background(), uuid.New(), uuid.New(), time.Now())
	require.Error(t, err)
}

// ─── ListAvailability ─────────────────────────────────────────────────────────

func TestAvailabilityService_ListAvailability(t *testing.T) {
	tenantID := uuid.New()
	employeeID := uuid.New()
	avs := []*model.Availability{
		testutil.NewAvailability(tenantID, employeeID),
		testutil.NewAvailability(tenantID, employeeID),
	}

	avRepo := &testutil.MockAvailabilityRepo{
		ListByEmployeeFn: func(_ context.Context, tID, eID uuid.UUID, from, to time.Time) ([]*model.Availability, error) {
			assert.Equal(t, tenantID, tID)
			assert.Equal(t, employeeID, eID)
			return avs, nil
		},
	}
	svc := newAvailabilityService(avRepo, &testutil.MockEmployeeRepo{})

	from := time.Now()
	to := from.AddDate(0, 0, 7)
	got, err := svc.ListAvailability(context.Background(), tenantID, employeeID, from, to)
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestAvailabilityService_ListAvailability_RepoError(t *testing.T) {
	avRepo := &testutil.MockAvailabilityRepo{
		ListByEmployeeFn: func(_ context.Context, _, _ uuid.UUID, _, _ time.Time) ([]*model.Availability, error) {
			return nil, errors.New("db error")
		},
	}
	svc := newAvailabilityService(avRepo, &testutil.MockEmployeeRepo{})

	_, err := svc.ListAvailability(context.Background(), uuid.New(), uuid.New(), time.Now(), time.Now())
	require.Error(t, err)
}
