package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/pkg"
	"github.com/ovander/parashift/internal/service"
)

// ShiftSlotHandler handles store-level shift slot template HTTP requests.
type ShiftSlotHandler struct {
	svc *service.ShiftSlotService
}

// NewShiftSlotHandler creates a new ShiftSlotHandler.
func NewShiftSlotHandler(svc *service.ShiftSlotService) *ShiftSlotHandler {
	return &ShiftSlotHandler{svc: svc}
}

// List returns all shift slots for the store.
func (h *ShiftSlotHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID"))
		return
	}
	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store"))
		return
	}

	slots, err := h.svc.List(ctx, storeID)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}
	responses := make([]dto.ShiftSlotResponse, len(slots))
	for i, sl := range slots {
		responses[i] = dto.ToShiftSlotResponse(sl)
	}
	pkg.WriteJSON(w, http.StatusOK, responses)
}

// Create adds a new shift slot to the store's template.
func (h *ShiftSlotHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID"))
		return
	}
	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store"))
		return
	}

	var req dto.CreateShiftSlotRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body"))
		return
	}

	slot, err := h.svc.Create(ctx, storeID, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}
	pkg.WriteJSON(w, http.StatusCreated, dto.ToShiftSlotResponse(slot))
}

// Delete removes a shift slot.
func (h *ShiftSlotHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID"))
		return
	}
	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store"))
		return
	}

	slotID, err := uuid.Parse(chi.URLParam(r, "templateId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid slot ID"))
		return
	}

	if err := h.svc.Delete(ctx, storeID, slotID); err != nil {
		pkg.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Publish generates ShiftInstances for the next 4 weeks from all slots of the
// same scheme as the referenced slot.
func (h *ShiftSlotHandler) Publish(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID"))
		return
	}
	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store"))
		return
	}

	slotID, err := uuid.Parse(chi.URLParam(r, "templateId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid slot ID"))
		return
	}

	count, err := h.svc.PublishScheme(ctx, storeID, slotID)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}
	pkg.WriteJSON(w, http.StatusOK, map[string]int{"shifts_created": count})
}
