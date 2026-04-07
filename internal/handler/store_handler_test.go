package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/handler"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/service"
	"github.com/ovander/parashift/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStoreHandler(repo *testutil.MockStoreRepo) *handler.StoreHandler {
	svc := service.NewStoreService(repo, newTestEmitter(), newTestLogger())
	return handler.NewStoreHandler(svc)
}

// ─── Get ──────────────────────────────────────────────────────────────────────

func TestStoreHandler_Get_OK(t *testing.T) {
	store := testutil.NewStore()
	repo := &testutil.MockStoreRepo{
		GetByIDFn: func(_ context.Context, _ uuid.UUID) (*model.Store, error) {
			return store, nil
		},
	}
	h := newStoreHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/stores/"+store.ID.String(), nil)
	req = withChiURLParam(req, "storeId", store.ID.String())
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp dto.StoreResponse
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Equal(t, store.ID, resp.ID)
	assert.Equal(t, store.Name, resp.Name)
}

func TestStoreHandler_Get_InvalidUUID(t *testing.T) {
	h := newStoreHandler(&testutil.MockStoreRepo{})

	req := httptest.NewRequest(http.MethodGet, "/stores/bad-uuid", nil)
	req = withChiURLParam(req, "storeId", "bad-uuid")
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestStoreHandler_Get_NotFound(t *testing.T) {
	repo := &testutil.MockStoreRepo{
		GetByIDFn: func(_ context.Context, _ uuid.UUID) (*model.Store, error) {
			return nil, nil
		},
	}
	h := newStoreHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/stores/"+uuid.New().String(), nil)
	req = withChiURLParam(req, "storeId", uuid.New().String())
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ─── Create ───────────────────────────────────────────────────────────────────

func TestStoreHandler_Create_OK(t *testing.T) {
	store := testutil.NewStore()
	repo := &testutil.MockStoreRepo{
		CreateFn: func(_ context.Context, s *model.Store) error {
			s.ID = store.ID
			return nil
		},
	}
	h := newStoreHandler(repo)

	body := jsonBody(dto.CreateStoreRequest{
		Name:     "Pharmacie Test",
		Timezone: "Europe/Brussels",
	})
	req := httptest.NewRequest(http.MethodPost, "/stores", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp dto.StoreResponse
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Equal(t, "Pharmacie Test", resp.Name)
}

func TestStoreHandler_Create_MissingName(t *testing.T) {
	h := newStoreHandler(&testutil.MockStoreRepo{})

	body := jsonBody(dto.CreateStoreRequest{Timezone: "UTC"}) // name missing
	req := httptest.NewRequest(http.MethodPost, "/stores", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestStoreHandler_Create_RepoError(t *testing.T) {
	repo := &testutil.MockStoreRepo{
		CreateFn: func(_ context.Context, _ *model.Store) error {
			return errors.New("db error")
		},
	}
	h := newStoreHandler(repo)

	body := jsonBody(dto.CreateStoreRequest{Name: "X", Timezone: "UTC"})
	req := httptest.NewRequest(http.MethodPost, "/stores", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// ─── Update ───────────────────────────────────────────────────────────────────

func TestStoreHandler_Update_OK(t *testing.T) {
	store := testutil.NewStore()
	newName := "New Name"
	repo := &testutil.MockStoreRepo{
		GetByIDFn: func(_ context.Context, _ uuid.UUID) (*model.Store, error) {
			return store, nil
		},
		UpdateFn: func(_ context.Context, s *model.Store) error {
			s.Name = newName
			return nil
		},
	}
	h := newStoreHandler(repo)

	body := jsonBody(dto.UpdateStoreRequest{Name: &newName})
	req := httptest.NewRequest(http.MethodPut, "/stores/"+store.ID.String(), body)
	req.Header.Set("Content-Type", "application/json")
	req = withChiURLParam(req, "storeId", store.ID.String())
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp dto.StoreResponse
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Equal(t, newName, resp.Name)
}

// ─── Delete ───────────────────────────────────────────────────────────────────

func TestStoreHandler_Delete_OK(t *testing.T) {
	storeID := uuid.New()
	deleted := false
	repo := &testutil.MockStoreRepo{
		DeleteFn: func(_ context.Context, id uuid.UUID) error {
			deleted = true
			assert.Equal(t, storeID, id)
			return nil
		},
	}
	h := newStoreHandler(repo)

	req := httptest.NewRequest(http.MethodDelete, "/stores/"+storeID.String(), nil)
	req = withChiURLParam(req, "storeId", storeID.String())
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.True(t, deleted)
}

// ─── List ─────────────────────────────────────────────────────────────────────

func TestStoreHandler_List_OK(t *testing.T) {
	stores := []*model.Store{testutil.NewStore(), testutil.NewStore()}
	repo := &testutil.MockStoreRepo{
		ListFn: func(_ context.Context, _, _ int) ([]*model.Store, int64, error) {
			return stores, 2, nil
		},
	}
	h := newStoreHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/stores", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

// ─── GetMyStore ───────────────────────────────────────────────────────────────

func TestStoreHandler_GetMyStore_OK(t *testing.T) {
	store := testutil.NewStore()
	repo := &testutil.MockStoreRepo{
		GetByIDFn: func(_ context.Context, id uuid.UUID) (*model.Store, error) {
			assert.Equal(t, store.ID, id)
			return store, nil
		},
	}
	h := newStoreHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/stores/me", nil)
	req = req.WithContext(managerCtx(store.ID))
	rr := httptest.NewRecorder()
	h.GetMyStore(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp dto.StoreResponse
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Equal(t, store.ID, resp.ID)
}
