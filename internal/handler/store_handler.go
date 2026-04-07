package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/backendkit/pagination"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/pkg"
	"github.com/ovander/parashift/internal/service"
)

// StoreHandler handles store-related HTTP requests.
type StoreHandler struct {
	svc *service.StoreService
}

// NewStoreHandler creates a new StoreHandler.
func NewStoreHandler(svc *service.StoreService) *StoreHandler {
	return &StoreHandler{svc: svc}
}

// toStoreResponse maps *model.Store → dto.StoreResponse.
func toStoreResponse(s *model.Store) dto.StoreResponse {
	var hours []dto.OpeningHourSlot
	if len(s.OpeningHours) > 0 {
		_ = json.Unmarshal(s.OpeningHours, &hours)
	}
	return dto.StoreResponse{
		ID:           s.ID,
		Name:         s.Name,
		OpeningHours: hours,
		Timezone:     s.Timezone,
		CreatedAt:    s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:    s.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// List returns all stores with pagination (admin only).
func (h *StoreHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params := pagination.Parse(r)

	stores, total, err := h.svc.List(ctx, params.Page, params.PerPage)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	responses := make([]dto.StoreResponse, len(stores))
	for i, s := range stores {
		responses[i] = toStoreResponse(s)
	}

	pkg.WriteJSON(w, http.StatusOK, pagination.NewPagedResponse(responses, params, total))
}

// Get returns a specific store by ID (admin only).
func (h *StoreHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID"))
		return
	}

	store, err := h.svc.GetByID(ctx, storeID)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, toStoreResponse(store))
}

// Create creates a new store (admin only).
func (h *StoreHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req dto.CreateStoreRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body"))
		return
	}

	if req.Name == "" {
		pkg.WriteError(w, apierror.BadRequest("name is required"))
		return
	}

	store, err := h.svc.Create(ctx, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusCreated, toStoreResponse(store))
}

// Update updates an existing store (admin only).
func (h *StoreHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID"))
		return
	}

	var req dto.UpdateStoreRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body"))
		return
	}

	store, err := h.svc.Update(ctx, storeID, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, toStoreResponse(store))
}

// Delete deletes a store (admin only).
func (h *StoreHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID"))
		return
	}

	if err := h.svc.Delete(ctx, storeID); err != nil {
		pkg.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetMyStore returns the authenticated user's store.
func (h *StoreHandler) GetMyStore(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	store, err := h.svc.GetByID(ctx, tenantID)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, toStoreResponse(store))
}

// UpdateMyStore updates the authenticated user's store.
func (h *StoreHandler) UpdateMyStore(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	var req dto.UpdateStoreRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body"))
		return
	}

	store, err := h.svc.Update(ctx, tenantID, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, toStoreResponse(store))
}

