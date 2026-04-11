package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/backendkit/pagination"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/pkg"
	"github.com/ovander/parashift/internal/service"
)

// AIHandler handles AI Engine HTTP requests.
type AIHandler struct {
	svc *service.AIService
}

// NewAIHandler creates a new AIHandler.
func NewAIHandler(svc *service.AIService) *AIHandler {
	return &AIHandler{svc: svc}
}

// SuggestAssignment returns AI-ranked employee suggestions for a shift.
// GET /stores/{storeId}/ai/suggest-assignment?shift_id=<uuid>
func (h *AIHandler) SuggestAssignment(w http.ResponseWriter, r *http.Request) {
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

	shiftIDStr := r.URL.Query().Get("shift_id")
	shiftID, err := uuid.Parse(shiftIDStr)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("shift_id query param is required and must be a valid UUID").WithKey("errors.invalidInput"))
		return
	}

	resp, err := h.svc.SuggestAssignment(ctx, storeID, dto.SuggestAssignmentRequest{ShiftID: shiftID})
	if err != nil {
		pkg.WriteError(w, err)
		return
	}
	pkg.WriteJSON(w, http.StatusOK, resp)
}

// OptimizeSchedule returns AI-proposed improvements for a date range.
// GET /stores/{storeId}/ai/optimize?from=YYYY-MM-DD&to=YYYY-MM-DD
func (h *AIHandler) OptimizeSchedule(w http.ResponseWriter, r *http.Request) {
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
	toStr := r.URL.Query().Get("to")
	if fromStr == "" || toStr == "" {
		pkg.WriteError(w, apierror.BadRequest("'from' and 'to' query params are required (YYYY-MM-DD)").WithKey("errors.invalidInput"))
		return
	}

	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("'from' must be in YYYY-MM-DD format").WithKey("errors.invalidInput"))
		return
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("'to' must be in YYYY-MM-DD format").WithKey("errors.invalidInput"))
		return
	}
	if to.Before(from) {
		pkg.WriteError(w, apierror.BadRequest("'to' must be on or after 'from'").WithKey("errors.invalidInput"))
		return
	}

	resp, err := h.svc.OptimizeSchedule(ctx, storeID, dto.OptimizeScheduleRequest{DateFrom: from, DateTo: to})
	if err != nil {
		pkg.WriteError(w, err)
		return
	}
	pkg.WriteJSON(w, http.StatusOK, resp)
}

// ListInsights returns active AI-generated insights for the store.
// GET /stores/{storeId}/ai/insights
func (h *AIHandler) ListInsights(w http.ResponseWriter, r *http.Request) {
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

	params := pagination.Parse(r)
	insights, total, err := h.svc.ListInsights(ctx, storeID, params.Page, params.PerPage)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	responses := make([]dto.AIInsightResponse, len(insights))
	for i, ins := range insights {
		responses[i] = dto.ToAIInsightResponse(ins)
	}
	pkg.WriteJSON(w, http.StatusOK, pagination.NewPagedResponse(responses, params, total))
}

// DismissInsight marks an insight as dismissed.
// PUT /stores/{storeId}/ai/insights/{insightId}/dismiss
func (h *AIHandler) DismissInsight(w http.ResponseWriter, r *http.Request) {
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

	insightID, err := uuid.Parse(chi.URLParam(r, "insightId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid insight ID").WithKey("errors.invalidInsightId"))
		return
	}

	if err := h.svc.DismissInsight(ctx, storeID, insightID); err != nil {
		pkg.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
