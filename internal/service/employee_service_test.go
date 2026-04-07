package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/event"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/service"
	"github.com/ovander/parashift/internal/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestEmitter() *event.Emitter {
	e := event.NewEmitter()
	return e
}

func newTestLogger() *logrus.Entry {
	l := logrus.New()
	l.SetLevel(logrus.FatalLevel) // suppress output during tests
	return l.WithField("test", true)
}

// ─── GetByID ──────────────────────────────────────────────────────────────────

func TestEmployeeService_GetByID_Found(t *testing.T) {
	tenantID := uuid.New()
	emp := testutil.NewEmployee(tenantID)

	repo := &testutil.MockEmployeeRepo{
		GetByIDFn: func(_ context.Context, tID, id uuid.UUID) (*model.Employee, error) {
			assert.Equal(t, tenantID, tID)
			assert.Equal(t, emp.ID, id)
			return emp, nil
		},
	}
	svc := service.NewEmployeeService(repo, &testutil.MockContractRepo{}, newTestEmitter(), newTestLogger())

	got, err := svc.GetByID(context.Background(), tenantID, emp.ID)
	require.NoError(t, err)
	assert.Equal(t, emp.ID, got.ID)
	assert.Equal(t, emp.Name, got.Name)
}

func TestEmployeeService_GetByID_NotFound(t *testing.T) {
	tenantID := uuid.New()
	repo := &testutil.MockEmployeeRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Employee, error) {
			return nil, nil
		},
	}
	svc := service.NewEmployeeService(repo, &testutil.MockContractRepo{}, newTestEmitter(), newTestLogger())

	_, err := svc.GetByID(context.Background(), tenantID, uuid.New())
	require.Error(t, err)
}

func TestEmployeeService_GetByID_RepoError(t *testing.T) {
	tenantID := uuid.New()
	repo := &testutil.MockEmployeeRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Employee, error) {
			return nil, errors.New("db error")
		},
	}
	svc := service.NewEmployeeService(repo, &testutil.MockContractRepo{}, newTestEmitter(), newTestLogger())

	_, err := svc.GetByID(context.Background(), tenantID, uuid.New())
	require.Error(t, err)
}

// ─── GetByAuthID ──────────────────────────────────────────────────────────────

func TestEmployeeService_GetByAuthID_Found(t *testing.T) {
	tenantID := uuid.New()
	emp := testutil.NewEmployee(tenantID)

	repo := &testutil.MockEmployeeRepo{
		GetByAuthIDFn: func(_ context.Context, authID string) (*model.Employee, error) {
			assert.Equal(t, emp.AuthID, authID)
			return emp, nil
		},
	}
	svc := service.NewEmployeeService(repo, &testutil.MockContractRepo{}, newTestEmitter(), newTestLogger())

	got, err := svc.GetByAuthID(context.Background(), emp.AuthID)
	require.NoError(t, err)
	assert.Equal(t, emp.AuthID, got.AuthID)
}

func TestEmployeeService_GetByAuthID_NotFound(t *testing.T) {
	repo := &testutil.MockEmployeeRepo{
		GetByAuthIDFn: func(_ context.Context, _ string) (*model.Employee, error) {
			return nil, nil
		},
	}
	svc := service.NewEmployeeService(repo, &testutil.MockContractRepo{}, newTestEmitter(), newTestLogger())

	_, err := svc.GetByAuthID(context.Background(), "unknown-sub")
	require.Error(t, err)
}

// ─── List ─────────────────────────────────────────────────────────────────────

func TestEmployeeService_List(t *testing.T) {
	tenantID := uuid.New()
	employees := []*model.Employee{
		testutil.NewEmployee(tenantID),
		testutil.NewEmployee(tenantID),
	}

	repo := &testutil.MockEmployeeRepo{
		ListFn: func(_ context.Context, tID uuid.UUID, page, pageSize int) ([]*model.Employee, int64, error) {
			assert.Equal(t, tenantID, tID)
			return employees, 2, nil
		},
	}
	svc := service.NewEmployeeService(repo, &testutil.MockContractRepo{}, newTestEmitter(), newTestLogger())

	got, total, err := svc.List(context.Background(), tenantID, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, got, 2)
}

func TestEmployeeService_List_RepoError(t *testing.T) {
	repo := &testutil.MockEmployeeRepo{
		ListFn: func(_ context.Context, _ uuid.UUID, _, _ int) ([]*model.Employee, int64, error) {
			return nil, 0, errors.New("db down")
		},
	}
	svc := service.NewEmployeeService(repo, &testutil.MockContractRepo{}, newTestEmitter(), newTestLogger())

	_, _, err := svc.List(context.Background(), uuid.New(), 1, 20)
	require.Error(t, err)
}

// ─── Create ───────────────────────────────────────────────────────────────────

func TestEmployeeService_Create(t *testing.T) {
	tenantID := uuid.New()
	created := false

	repo := &testutil.MockEmployeeRepo{
		CreateFn: func(_ context.Context, e *model.Employee) error {
			created = true
			assert.Equal(t, tenantID, e.TenantID)
			assert.Equal(t, "Bob Martin", e.Name)
			return nil
		},
	}
	svc := service.NewEmployeeService(repo, &testutil.MockContractRepo{}, newTestEmitter(), newTestLogger())

	req := dto.CreateEmployeeRequest{
		Name:      "Bob Martin",
		Position:  "employee",
		JobRole:   "pharmacist",
		StartDate: time.Now(),
		AuthID:    "sub-bob",
	}
	emp, err := svc.Create(context.Background(), tenantID, req)
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, "Bob Martin", emp.Name)
	assert.Equal(t, tenantID, emp.TenantID)
}

func TestEmployeeService_Create_RepoError(t *testing.T) {
	repo := &testutil.MockEmployeeRepo{
		CreateFn: func(_ context.Context, _ *model.Employee) error {
			return errors.New("unique constraint violation")
		},
	}
	svc := service.NewEmployeeService(repo, &testutil.MockContractRepo{}, newTestEmitter(), newTestLogger())

	_, err := svc.Create(context.Background(), uuid.New(), dto.CreateEmployeeRequest{
		Name: "Dup", Position: "employee", JobRole: "cashier", StartDate: time.Now(), AuthID: "sub-dup",
	})
	require.Error(t, err)
}

// ─── Update ───────────────────────────────────────────────────────────────────

func TestEmployeeService_Update_PartialFields(t *testing.T) {
	tenantID := uuid.New()
	emp := testutil.NewEmployee(tenantID)
	newName := "Updated Name"

	empRepo := &testutil.MockEmployeeRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Employee, error) {
			return emp, nil
		},
		UpdateFn: func(_ context.Context, e *model.Employee) error {
			assert.Equal(t, newName, e.Name)
			return nil
		},
	}
	svc := service.NewEmployeeService(empRepo, &testutil.MockContractRepo{}, newTestEmitter(), newTestLogger())

	req := dto.UpdateEmployeeRequest{Name: &newName}
	got, err := svc.Update(context.Background(), tenantID, emp.ID, req)
	require.NoError(t, err)
	assert.Equal(t, newName, got.Name)
}

func TestEmployeeService_Update_NotFound(t *testing.T) {
	empRepo := &testutil.MockEmployeeRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Employee, error) {
			return nil, nil
		},
	}
	svc := service.NewEmployeeService(empRepo, &testutil.MockContractRepo{}, newTestEmitter(), newTestLogger())

	name := "x"
	_, err := svc.Update(context.Background(), uuid.New(), uuid.New(), dto.UpdateEmployeeRequest{Name: &name})
	require.Error(t, err)
}

// ─── Delete ───────────────────────────────────────────────────────────────────

func TestEmployeeService_Delete(t *testing.T) {
	tenantID := uuid.New()
	empID := uuid.New()
	deleted := false

	empRepo := &testutil.MockEmployeeRepo{
		DeleteFn: func(_ context.Context, tID, id uuid.UUID) error {
			deleted = true
			assert.Equal(t, tenantID, tID)
			assert.Equal(t, empID, id)
			return nil
		},
	}
	svc := service.NewEmployeeService(empRepo, &testutil.MockContractRepo{}, newTestEmitter(), newTestLogger())

	err := svc.Delete(context.Background(), tenantID, empID)
	require.NoError(t, err)
	assert.True(t, deleted)
}

func TestEmployeeService_Delete_RepoError(t *testing.T) {
	empRepo := &testutil.MockEmployeeRepo{
		DeleteFn: func(_ context.Context, _, _ uuid.UUID) error {
			return errors.New("not found")
		},
	}
	svc := service.NewEmployeeService(empRepo, &testutil.MockContractRepo{}, newTestEmitter(), newTestLogger())

	err := svc.Delete(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
}
