package handler

import (
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

// SwapHandler handles swap request-related HTTP requests.
type SwapHandler struct {
	svc *service.SwapService
}

// NewSwapHandler creates a new SwapHandler.
func NewSwapHandler(svc *service.SwapService) *SwapHandler {
	return &SwapHandler{svc: svc}
}

// toSwapResponse maps *model.SwapRequest → dto.SwapRequestResponse.
func toSwapResponse(s *model.SwapRequest) dto.SwapRequestResponse {
	return dto.SwapRequestResponse{
		ID:               s.ID,
		TenantID:         s.TenantID,
		RequesterID:      s.RequesterID,
		TargetEmployeeID: s.TargetEmployeeID,
		ShiftInstanceID:  s.ShiftInstanceID,
		TargetShiftID:    s.TargetShiftID,
		Status:           s.Status,
		Note:             s.Note,
		ReviewedBy:       s.ReviewedBy,
		CreatedAt:        s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:        s.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// Create creates a new swap request.
func (h *SwapHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req dto.CreateSwapRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body"))
		return
	}

	if req.ShiftInstanceID == uuid.Nil {
		pkg.WriteError(w, apierror.BadRequest("shift_instance_id is required"))
		return
	}

	// Requester is the currently authenticated user
	requesterID := ctxutil.GetUserID(ctx)

	swapRequest, err := h.svc.CreateSwapRequest(ctx, storeID, requesterID, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusCreated, toSwapResponse(swapRequest))
}

// List returns swap requests with pagination and optional status filter.
func (h *SwapHandler) List(w http.ResponseWriter, r *http.Request) {
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

	status := r.URL.Query().Get("status")
	params := pagination.Parse(r)

	swapRequests, total, err := h.svc.ListByStore(ctx, storeID, status, params.Page, params.PerPage)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	responses := make([]dto.SwapRequestResponse, len(swapRequests))
	for i, sr := range swapRequests {
		responses[i] = toSwapResponse(sr)
	}

	pkg.WriteJSON(w, http.StatusOK, pagination.NewPagedResponse(responses, params, total))
}

// Get returns a specific swap request by ID.
func (h *SwapHandler) Get(w http.ResponseWriter, r *http.Request) {
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

	swapID, err := uuid.Parse(chi.URLParam(r, "swapId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid swap request ID"))
		return
	}

	swapRequest, err := h.svc.GetSwapRequest(ctx, storeID, swapID)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, toSwapResponse(swapRequest))
}

// Review reviews a swap request (approve or deny).
func (h *SwapHandler) Review(w http.ResponseWriter, r *http.Request) {
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

	swapID, err := uuid.Parse(chi.URLParam(r, "swapId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid swap request ID"))
		return
	}

	var req dto.ReviewSwapRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body"))
		return
	}

	if req.Status == "" {
		pkg.WriteError(w, apierror.BadRequest("status is required"))
		return
	}

	swapRequest, err := h.svc.ReviewSwapRequest(ctx, storeID, swapID, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, toSwapResponse(swapRequest))
}
