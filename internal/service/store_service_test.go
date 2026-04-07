package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/service"
	"github.com/ovander/parashift/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── GetByID ──────────────────────────────────────────────────────────────────

func TestStoreService_GetByID_Found(t *testing.T) {
	store := testutil.NewStore()
	repo := &testutil.MockStoreRepo{
		GetByIDFn: func(_ context.Context, id uuid.UUID) (*model.Store, error) {
			assert.Equal(t, store.ID, id)
			return store, nil
		},
	}
	svc := service.NewStoreService(repo, newTestEmitter(), newTestLogger())

	got, err := svc.GetByID(context.Background(), store.ID)
	require.NoError(t, err)
	assert.Equal(t, store.ID, got.ID)
	assert.Equal(t, store.Name, got.Name)
}

func TestStoreService_GetByID_NotFound(t *testing.T) {
	repo := &testutil.MockStoreRepo{
		GetByIDFn: func(_ context.Context, _ uuid.UUID) (*model.Store, error) {
			return nil, nil
		},
	}
	svc := service.NewStoreService(repo, newTestEmitter(), newTestLogger())

	_, err := svc.GetByID(context.Background(), uuid.New())
	require.Error(t, err)
}

func TestStoreService_GetByID_RepoError(t *testing.T) {
	repo := &testutil.MockStoreRepo{
		GetByIDFn: func(_ context.Context, _ uuid.UUID) (*model.Store, error) {
			return nil, errors.New("db error")
		},
	}
	svc := service.NewStoreService(repo, newTestEmitter(), newTestLogger())

	_, err := svc.GetByID(context.Background(), uuid.New())
	require.Error(t, err)
}

// ─── List ─────────────────────────────────────────────────────────────────────

func TestStoreService_List(t *testing.T) {
	stores := []*model.Store{testutil.NewStore(), testutil.NewStore()}
	repo := &testutil.MockStoreRepo{
		ListFn: func(_ context.Context, page, pageSize int) ([]*model.Store, int64, error) {
			assert.Equal(t, 1, page)
			assert.Equal(t, 20, pageSize)
			return stores, int64(len(stores)), nil
		},
	}
	svc := service.NewStoreService(repo, newTestEmitter(), newTestLogger())

	got, total, err := svc.List(context.Background(), 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, got, 2)
}

func TestStoreService_List_RepoError(t *testing.T) {
	repo := &testutil.MockStoreRepo{
		ListFn: func(_ context.Context, _, _ int) ([]*model.Store, int64, error) {
			return nil, 0, errors.New("db error")
		},
	}
	svc := service.NewStoreService(repo, newTestEmitter(), newTestLogger())

	_, _, err := svc.List(context.Background(), 1, 20)
	require.Error(t, err)
}

// ─── Create ───────────────────────────────────────────────────────────────────

func TestStoreService_Create(t *testing.T) {
	created := false
	repo := &testutil.MockStoreRepo{
		CreateFn: func(_ context.Context, s *model.Store) error {
			created = true
			assert.Equal(t, "Pharmacie Test", s.Name)
			assert.Equal(t, "Europe/Brussels", s.Timezone)
			return nil
		},
	}
	svc := service.NewStoreService(repo, newTestEmitter(), newTestLogger())

	req := dto.CreateStoreRequest{
		Name:     "Pharmacie Test",
		Timezone: "Europe/Brussels",
		OpeningHours: []dto.OpeningHourSlot{
			{DayOfWeek: 1, OpenTime: "09:00", CloseTime: "18:00"},
		},
	}
	got, err := svc.Create(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, "Pharmacie Test", got.Name)
}

func TestStoreService_Create_RepoError(t *testing.T) {
	repo := &testutil.MockStoreRepo{
		CreateFn: func(_ context.Context, _ *model.Store) error {
			return errors.New("db error")
		},
	}
	svc := service.NewStoreService(repo, newTestEmitter(), newTestLogger())

	_, err := svc.Create(context.Background(), dto.CreateStoreRequest{Name: "X", Timezone: "UTC"})
	require.Error(t, err)
}

// ─── Update ───────────────────────────────────────────────────────────────────

func TestStoreService_Update(t *testing.T) {
	store := testutil.NewStore()
	newName := "New Name"

	repo := &testutil.MockStoreRepo{
		GetByIDFn: func(_ context.Context, _ uuid.UUID) (*model.Store, error) {
			return store, nil
		},
		UpdateFn: func(_ context.Context, s *model.Store) error {
			assert.Equal(t, newName, s.Name)
			return nil
		},
	}
	svc := service.NewStoreService(repo, newTestEmitter(), newTestLogger())

	got, err := svc.Update(context.Background(), store.ID, dto.UpdateStoreRequest{Name: &newName})
	require.NoError(t, err)
	assert.Equal(t, newName, got.Name)
}

func TestStoreService_Update_NotFound(t *testing.T) {
	repo := &testutil.MockStoreRepo{
		GetByIDFn: func(_ context.Context, _ uuid.UUID) (*model.Store, error) {
			return nil, nil
		},
	}
	svc := service.NewStoreService(repo, newTestEmitter(), newTestLogger())

	name := "x"
	_, err := svc.Update(context.Background(), uuid.New(), dto.UpdateStoreRequest{Name: &name})
	require.Error(t, err)
}

// ─── Delete ───────────────────────────────────────────────────────────────────

func TestStoreService_Delete(t *testing.T) {
	storeID := uuid.New()
	deleted := false
	repo := &testutil.MockStoreRepo{
		DeleteFn: func(_ context.Context, id uuid.UUID) error {
			deleted = true
			assert.Equal(t, storeID, id)
			return nil
		},
	}
	svc := service.NewStoreService(repo, newTestEmitter(), newTestLogger())

	err := svc.Delete(context.Background(), storeID)
	require.NoError(t, err)
	assert.True(t, deleted)
}
