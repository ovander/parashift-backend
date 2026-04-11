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

// WeekTemplateHandler handles week template-related HTTP requests.
type WeekTemplateHandler struct {
	svc *service.ScheduleService
}

// NewWeekTemplateHandler creates a new WeekTemplateHandler.
func NewWeekTemplateHandler(svc *service.ScheduleService) *WeekTemplateHandler {
	return &WeekTemplateHandler{svc: svc}
}

// GetTemplates returns week templates for an employee.
func (h *WeekTemplateHandler) GetTemplates(w http.ResponseWriter, r *http.Request) {
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

	employeeID, err := uuid.Parse(chi.URLParam(r, "employeeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid employee ID").WithKey("errors.invalidEmployeeId"))
		return
	}

	// Service: GetWeekTemplates(ctx, tenantID, employeeID) → (dto.WeekTemplateResponse, error)
	response, err := h.svc.GetWeekTemplates(ctx, storeID, employeeID)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, response)
}

// UpsertTemplates creates or updates week templates for an employee.
func (h *WeekTemplateHandler) UpsertTemplates(w http.ResponseWriter, r *http.Request) {
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

	employeeID, err := uuid.Parse(chi.URLParam(r, "employeeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid employee ID").WithKey("errors.invalidEmployeeId"))
		return
	}

	// Decode canonical DTO: Templates []WeekTemplateEntry
	var req dto.UpsertWeekTemplateRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body").WithKey("errors.invalidInput"))
		return
	}

	if len(req.Templates) == 0 {
		pkg.WriteError(w, apierror.BadRequest("templates cannot be empty").WithKey("errors.invalidInput"))
		return
	}

	// Service: UpsertWeekTemplates(ctx, tenantID, employeeID, req dto.UpsertWeekTemplateRequest) → error
	if err := h.svc.UpsertWeekTemplates(ctx, storeID, employeeID, req); err != nil {
		pkg.WriteError(w, err)
		return
	}

	// Return the updated templates
	response, err := h.svc.GetWeekTemplates(ctx, storeID, employeeID)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, response)
}
