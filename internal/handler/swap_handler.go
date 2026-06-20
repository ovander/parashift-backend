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
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}

	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	var req dto.CreateSwapRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body").WithKey("errors.invalidInput"))
		return
	}

	if req.ShiftInstanceID == uuid.Nil {
		pkg.WriteError(w, apierror.BadRequest("shift_instance_id is required").WithKey("errors.invalidInput"))
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
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}

	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	status := r.URL.Query().Get("status")
	params := pagination.Parse(r)

	// Employees see only their own swap requests; managers/admins see the store.
	var swapRequests []*model.SwapRequest
	var total int64
	if isManagerRole(ctxutil.GetUserRole(ctx)) {
		swapRequests, total, err = h.svc.ListByStore(ctx, storeID, status, params.Page, params.PerPage)
	} else {
		swapRequests, total, err = h.svc.ListByEmployee(ctx, storeID, ctxutil.GetUserID(ctx), params.Page, params.PerPage)
	}
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
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}

	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	swapID, err := uuid.Parse(chi.URLParam(r, "swapId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid swap request ID").WithKey("errors.invalidInput"))
		return
	}

	swapRequest, err := h.svc.GetSwapRequest(ctx, storeID, swapID)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	// Object-level authz: only the requester, the swap target, or a
	// manager/admin may read a swap request (SEC-5).
	if !isManagerRole(ctxutil.GetUserRole(ctx)) {
		callerID := ctxutil.GetUserID(ctx)
		isParty := swapRequest.RequesterID == callerID ||
			(swapRequest.TargetEmployeeID != nil && *swapRequest.TargetEmployeeID == callerID)
		if !isParty {
			pkg.WriteError(w, apierror.Forbidden("access denied to this resource").WithKey("errors.accessDenied"))
			return
		}
	}

	pkg.WriteJSON(w, http.StatusOK, toSwapResponse(swapRequest))
}

// Review reviews a swap request (approve or deny).
func (h *SwapHandler) Review(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}

	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	swapID, err := uuid.Parse(chi.URLParam(r, "swapId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid swap request ID").WithKey("errors.invalidInput"))
		return
	}

	var req dto.ReviewSwapRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body").WithKey("errors.invalidInput"))
		return
	}

	if req.Status == "" {
		pkg.WriteError(w, apierror.BadRequest("status is required").WithKey("errors.invalidInput"))
		return
	}

	swapRequest, err := h.svc.ReviewSwapRequest(ctx, storeID, swapID, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, toSwapResponse(swapRequest))
}
