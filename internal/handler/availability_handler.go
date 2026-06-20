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

// AvailabilityHandler handles availability-related HTTP requests.
type AvailabilityHandler struct {
	svc *service.AvailabilityService
}

// NewAvailabilityHandler creates a new AvailabilityHandler.
func NewAvailabilityHandler(svc *service.AvailabilityService) *AvailabilityHandler {
	return &AvailabilityHandler{svc: svc}
}

// SetAvailability sets or updates an employee's availability.
func (h *AvailabilityHandler) SetAvailability(w http.ResponseWriter, r *http.Request) {
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

	if !enforceSelfOrManager(w, r, employeeID) {
		return
	}

	var req dto.SetAvailabilityRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body").WithKey("errors.invalidInput"))
		return
	}

	availability, err := h.svc.SetAvailability(ctx, tenantID, employeeID, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusCreated, toAvailabilityResponse(availability))
}

// GetAvailability returns availability for a specific date.
func (h *AvailabilityHandler) GetAvailability(w http.ResponseWriter, r *http.Request) {
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

	if !enforceSelfOrManager(w, r, employeeID) {
		return
	}

	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		pkg.WriteError(w, apierror.BadRequest("date query parameter is required").WithKey("errors.invalidInput"))
		return
	}

	date, err := parseDate(dateStr)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid date format, use YYYY-MM-DD").WithKey("errors.invalidInput"))
		return
	}

	// Service signature: GetAvailability(ctx, tenantID, employeeID, date)
	availability, err := h.svc.GetAvailability(ctx, tenantID, employeeID, date)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, toAvailabilityResponse(availability))
}

// ListAvailability returns availability for a date range.
func (h *AvailabilityHandler) ListAvailability(w http.ResponseWriter, r *http.Request) {
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

	if !enforceSelfOrManager(w, r, employeeID) {
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

	// Service signature: ListAvailability(ctx, tenantID, employeeID, from, to) — no pagination, returns 2 values
	availabilities, err := h.svc.ListAvailability(ctx, tenantID, employeeID, from, to)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	responses := make([]dto.AvailabilityResponse, len(availabilities))
	for i, a := range availabilities {
		responses[i] = toAvailabilityResponse(a)
	}

	params := pagination.Parse(r)
	pkg.WriteJSON(w, http.StatusOK, pagination.NewPagedResponse(responses, params, int64(len(availabilities))))
}

// toAvailabilityResponse converts a model.Availability to dto.AvailabilityResponse.
func toAvailabilityResponse(a *model.Availability) dto.AvailabilityResponse {
	// Deserialize TimeRanges from JSON
	var timeRanges []dto.TimeSlot
	_ = json.Unmarshal(a.TimeRanges, &timeRanges)

	return dto.AvailabilityResponse{
		ID:         a.ID,
		TenantID:   a.TenantID,
		EmployeeID: a.EmployeeID,
		Date:       a.Date.Format("2006-01-02"),
		TimeRanges: timeRanges,
		Note:       a.Note,
		CreatedAt:  a.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:  a.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
