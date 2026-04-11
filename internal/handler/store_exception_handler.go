package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/pkg"
	"github.com/ovander/parashift/internal/service"
)

// StoreExceptionHandler manages store-level schedule exceptions (EXTRA_OPEN / FORCED_CLOSED).
type StoreExceptionHandler struct {
	svc *service.StoreExceptionService
}

// NewStoreExceptionHandler creates a new StoreExceptionHandler.
func NewStoreExceptionHandler(svc *service.StoreExceptionService) *StoreExceptionHandler {
	return &StoreExceptionHandler{svc: svc}
}

type storeExceptionResponse struct {
	ID       uuid.UUID `json:"id"`
	StoreID  uuid.UUID `json:"store_id"`
	Date     string    `json:"date"` // YYYY-MM-DD
	Type     string    `json:"type"` // EXTRA_OPEN | FORCED_CLOSED
	Note     string    `json:"note,omitempty"`
}

type createStoreExceptionRequest struct {
	Date string `json:"date"` // YYYY-MM-DD
	Type string `json:"type"` // EXTRA_OPEN | FORCED_CLOSED
	Note string `json:"note"`
}

// List returns all store exceptions for a given date range.
// Route: GET /stores/{storeId}/exceptions?from=YYYY-MM-DD&to=YYYY-MM-DD
func (h *StoreExceptionHandler) List(w http.ResponseWriter, r *http.Request) {
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

	fromStr := r.URL.Query().Get("from")
	toStr   := r.URL.Query().Get("to")
	if fromStr == "" || toStr == "" {
		pkg.WriteError(w, apierror.BadRequest("from and to query params are required (YYYY-MM-DD)").WithKey("errors.invalidInput"))
		return
	}
	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid from date").WithKey("errors.invalidInput"))
		return
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid to date").WithKey("errors.invalidInput"))
		return
	}

	exceptions, err := h.svc.List(ctx, storeID, from, to)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	out := make([]storeExceptionResponse, len(exceptions))
	for i, e := range exceptions {
		out[i] = storeExceptionResponse{
			ID:      e.ID,
			StoreID: e.TenantID,
			Date:    e.Date.Format("2006-01-02"),
			Type:    e.Type,
			Note:    e.Note,
		}
	}
	pkg.WriteJSON(w, http.StatusOK, out)
}

// Create adds a new store exception.
// Route: POST /stores/{storeId}/exceptions
func (h *StoreExceptionHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req createStoreExceptionRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body").WithKey("errors.invalidInput"))
		return
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid date format — expected YYYY-MM-DD").WithKey("errors.invalidInput"))
		return
	}
	if req.Type != model.ExceptionExtraOpen && req.Type != model.ExceptionForcedClosed {
		pkg.WriteError(w, apierror.BadRequest("type must be EXTRA_OPEN or FORCED_CLOSED").WithKey("errors.invalidInput"))
		return
	}

	exc := &model.StoreException{
		TenantScoped: model.TenantScoped{TenantID: storeID},
		Date:         date,
		Type:         req.Type,
		Note:         req.Note,
	}
	if err := h.svc.Create(ctx, exc); err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusCreated, storeExceptionResponse{
		ID:      exc.ID,
		StoreID: exc.TenantID,
		Date:    exc.Date.Format("2006-01-02"),
		Type:    exc.Type,
		Note:    exc.Note,
	})
}

// Delete removes a store exception.
// Route: DELETE /stores/{storeId}/exceptions/{exceptionId}
func (h *StoreExceptionHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

	exceptionID, err := uuid.Parse(chi.URLParam(r, "exceptionId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid exception ID").WithKey("errors.invalidInput"))
		return
	}

	if err := h.svc.Delete(ctx, storeID, exceptionID); err != nil {
		pkg.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
