package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/pkg"
	"github.com/ovander/parashift/internal/service"
)

// CoverageHandler handles coverage-related HTTP requests.
type CoverageHandler struct {
	svc *service.CoverageService
}

// NewCoverageHandler creates a new CoverageHandler.
func NewCoverageHandler(svc *service.CoverageService) *CoverageHandler {
	return &CoverageHandler{svc: svc}
}

// ListRequirements returns all coverage requirements for a store.
func (h *CoverageHandler) ListRequirements(w http.ResponseWriter, r *http.Request) {
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

	// Service: ListRequirements(ctx, tenantID) → ([]*model.CoverageRequirement, error)
	requirements, err := h.svc.ListRequirements(ctx, storeID)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	responses := make([]dto.CoverageRequirementResponse, len(requirements))
	for i, req := range requirements {
		responses[i] = toCoverageRequirementResponse(req)
	}

	pkg.WriteJSON(w, http.StatusOK, responses)
}

// CreateRequirement creates a new coverage requirement.
func (h *CoverageHandler) CreateRequirement(w http.ResponseWriter, r *http.Request) {
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

	// Decode canonical DTO: DayOfWeek int, StartTime/EndTime string, MinStaff int, RequiredRole *string
	var req dto.CoverageRequirementRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body").WithKey("errors.invalidInput"))
		return
	}

	// Service: CreateRequirement(ctx, tenantID, req dto.CoverageRequirementRequest)
	requirement, err := h.svc.CreateRequirement(ctx, storeID, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusCreated, toCoverageRequirementResponse(requirement))
}

// UpdateRequirement updates an existing coverage requirement.
func (h *CoverageHandler) UpdateRequirement(w http.ResponseWriter, r *http.Request) {
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

	reqID, err := uuid.Parse(chi.URLParam(r, "reqId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid requirement ID").WithKey("errors.invalidInput"))
		return
	}

	var req dto.CoverageRequirementRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body").WithKey("errors.invalidInput"))
		return
	}

	// Service: UpdateRequirement(ctx, tenantID, id, req dto.CoverageRequirementRequest)
	requirement, err := h.svc.UpdateRequirement(ctx, storeID, reqID, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, toCoverageRequirementResponse(requirement))
}

// DeleteRequirement deletes a coverage requirement.
func (h *CoverageHandler) DeleteRequirement(w http.ResponseWriter, r *http.Request) {
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

	reqID, err := uuid.Parse(chi.URLParam(r, "reqId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid requirement ID").WithKey("errors.invalidInput"))
		return
	}

	// Service: DeleteRequirement(ctx, tenantID, id)
	if err := h.svc.DeleteRequirement(ctx, storeID, reqID); err != nil {
		pkg.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetCoverage returns coverage analysis for a date range.
func (h *CoverageHandler) GetCoverage(w http.ResponseWriter, r *http.Request) {
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
		pkg.WriteError(w, apierror.BadRequest("from and to query parameters are required").WithKey("errors.missingParams"))
		return
	}

	from, err := parseDate(fromStr)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid from date format, use YYYY-MM-DD").WithKey("errors.invalidDateRange"))
		return
	}

	to, err := parseDate(toStr)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid to date format, use YYYY-MM-DD").WithKey("errors.invalidDateRange"))
		return
	}

	// Returns dto.CoverageReport{Items []CoverageSlot, TotalSlots int, GapCount int}
	coverage, err := h.svc.ComputeForDateRange(ctx, storeID, from, to)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, coverage)
}

// GetGaps returns only coverage slots that have gaps (non-OK status).
func (h *CoverageHandler) GetGaps(w http.ResponseWriter, r *http.Request) {
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
		pkg.WriteError(w, apierror.BadRequest("from and to query parameters are required").WithKey("errors.missingParams"))
		return
	}

	from, err := parseDate(fromStr)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid from date format, use YYYY-MM-DD").WithKey("errors.invalidDateRange"))
		return
	}

	to, err := parseDate(toStr)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid to date format, use YYYY-MM-DD").WithKey("errors.invalidDateRange"))
		return
	}

	// Returns []dto.CoverageSlot — only slots with Status != OK
	gaps, err := h.svc.GetGaps(ctx, storeID, from, to)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, gaps)
}

// toCoverageRequirementResponse maps *model.CoverageRequirement → dto.CoverageRequirementResponse.
func toCoverageRequirementResponse(m *model.CoverageRequirement) dto.CoverageRequirementResponse {
	return dto.CoverageRequirementResponse{
		ID:           m.ID,
		StoreID:      m.TenantID,
		DayOfWeek:    m.DayOfWeek,
		StartTime:    m.StartTime,
		EndTime:      m.EndTime,
		MinStaff:     m.MinStaff,
		RequiredRole: m.RequiredRole,
		CreatedAt:    m.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:    m.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
