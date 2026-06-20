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

// LeaveHandler handles leave request-related HTTP requests.
type LeaveHandler struct {
	svc *service.LeaveService
}

// NewLeaveHandler creates a new LeaveHandler.
func NewLeaveHandler(svc *service.LeaveService) *LeaveHandler {
	return &LeaveHandler{svc: svc}
}

// Create creates a new leave request.
func (h *LeaveHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	// Decode canonical DTO fields + optional employee_id (for managers acting on behalf of employees)
	var req struct {
		EmployeeID *uuid.UUID `json:"employee_id,omitempty"`
		dto.CreateLeaveRequest
	}
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body").WithKey("errors.invalidInput"))
		return
	}

	// Determine employee ID. By default a user files leave for themselves; an
	// explicit employee_id in the body (manager acting on behalf) is only
	// honored for managers/admins — an employee may not forge leave for a
	// colleague (SEC-5).
	callerID := ctxutil.GetUserID(ctx)
	empID := callerID
	if req.EmployeeID != nil {
		if !isManagerRole(ctxutil.GetUserRole(ctx)) && *req.EmployeeID != callerID {
			pkg.WriteError(w, apierror.Forbidden("cannot create leave for another employee").WithKey("errors.accessDenied"))
			return
		}
		empID = *req.EmployeeID
	}

	// Service: CreateLeaveRequest(ctx, tenantID, employeeID, req dto.CreateLeaveRequest)
	leaveRequest, err := h.svc.CreateLeaveRequest(ctx, storeID, empID, req.CreateLeaveRequest)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusCreated, toLeaveRequestResponse(leaveRequest))
}

// List returns leave requests with pagination and optional status filter.
// Managers and admins receive all store leave requests; employees receive only
// their own (scoped by the current user's ID from the JWT).
func (h *LeaveHandler) List(w http.ResponseWriter, r *http.Request) {
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
	role   := ctxutil.GetUserRole(ctx)

	var leaveRequests []*model.LeaveRequest
	var total         int64

	if role == "manager" || role == "admin" {
		// Managers/admins see all leave requests for the store.
		status := r.URL.Query().Get("status")
		leaveRequests, total, err = h.svc.ListByStore(ctx, storeID, status, params.Page, params.PerPage)
	} else {
		// Employees see only their own leave requests.
		empID := ctxutil.GetUserID(ctx)
		leaveRequests, total, err = h.svc.ListByEmployee(ctx, storeID, empID, params.Page, params.PerPage)
	}
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	responses := make([]dto.LeaveRequestResponse, len(leaveRequests))
	for i, lr := range leaveRequests {
		responses[i] = toLeaveRequestResponse(lr)
	}

	pkg.WriteJSON(w, http.StatusOK, pagination.NewPagedResponse(responses, params, total))
}

// Get returns a specific leave request by ID.
func (h *LeaveHandler) Get(w http.ResponseWriter, r *http.Request) {
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

	leaveID, err := uuid.Parse(chi.URLParam(r, "leaveId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid leave request ID").WithKey("errors.invalidInput"))
		return
	}

	// Service: GetLeaveRequest(ctx, tenantID, id) — tenantID scopes the lookup
	leaveRequest, err := h.svc.GetLeaveRequest(ctx, storeID, leaveID)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	// Object-level authz: an employee may only read their own leave (SEC-5).
	if !enforceSelfOrManager(w, r, leaveRequest.EmployeeID) {
		return
	}

	pkg.WriteJSON(w, http.StatusOK, toLeaveRequestResponse(leaveRequest))
}

// Delete cancels (removes) a pending leave request.
func (h *LeaveHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

	leaveID, err := uuid.Parse(chi.URLParam(r, "leaveId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid leave request ID").WithKey("errors.invalidInput"))
		return
	}

	// Object-level authz: load first so an employee can only cancel their own
	// leave; managers/admins may cancel any in the tenant (SEC-5).
	existing, err := h.svc.GetLeaveRequest(ctx, storeID, leaveID)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}
	if !enforceSelfOrManager(w, r, existing.EmployeeID) {
		return
	}

	if err := h.svc.DeleteLeaveRequest(ctx, storeID, leaveID); err != nil {
		pkg.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetImpact returns a preview of what will happen to the schedule if the leave is approved.
// Managers call this before clicking Approve to see which shifts will be cancelled and
// whether any shifts will be left uncovered.
func (h *LeaveHandler) GetImpact(w http.ResponseWriter, r *http.Request) {
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

	leaveID, err := uuid.Parse(chi.URLParam(r, "leaveId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid leave request ID").WithKey("errors.invalidInput"))
		return
	}

	impact, err := h.svc.GetLeaveImpact(ctx, storeID, leaveID)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, impact)
}

// Review reviews a leave request (approve or reject).
func (h *LeaveHandler) Review(w http.ResponseWriter, r *http.Request) {
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

	leaveID, err := uuid.Parse(chi.URLParam(r, "leaveId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid leave request ID").WithKey("errors.invalidInput"))
		return
	}

	// Decode canonical DTO: Status string (approved|rejected)
	var req dto.ReviewLeaveRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body").WithKey("errors.invalidInput"))
		return
	}

	// Service: ReviewLeaveRequest(ctx, tenantID, id, req dto.ReviewLeaveRequest)
	leaveRequest, err := h.svc.ReviewLeaveRequest(ctx, storeID, leaveID, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, toLeaveRequestResponse(leaveRequest))
}

// ListByEmployee returns leave requests for a specific employee with pagination.
// Managers use this to view an individual employee's leave history.
func (h *LeaveHandler) ListByEmployee(w http.ResponseWriter, r *http.Request) {
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

	params := pagination.Parse(r)

	// Service: ListByEmployee(ctx, tenantID, employeeID, page, pageSize) → ([]*model.LeaveRequest, int64, error)
	leaveRequests, total, err := h.svc.ListByEmployee(ctx, storeID, employeeID, params.Page, params.PerPage)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	responses := make([]dto.LeaveRequestResponse, len(leaveRequests))
	for i, lr := range leaveRequests {
		responses[i] = toLeaveRequestResponse(lr)
	}

	pkg.WriteJSON(w, http.StatusOK, pagination.NewPagedResponse(responses, params, total))
}

// toLeaveRequestResponse maps *model.LeaveRequest → dto.LeaveRequestResponse.
func toLeaveRequestResponse(l *model.LeaveRequest) dto.LeaveRequestResponse {
	reviewedAt := (*string)(nil)
	if l.ReviewedAt != nil {
		s := l.ReviewedAt.Format("2006-01-02T15:04:05Z07:00")
		reviewedAt = &s
	}
	reason := ""
	if l.Reason != "" {
		reason = l.Reason
	}
	return dto.LeaveRequestResponse{
		ID:         l.ID,
		TenantID:   l.TenantID,
		EmployeeID: l.EmployeeID,
		StartDate:  l.StartDate.Format("2006-01-02"),
		EndDate:    l.EndDate.Format("2006-01-02"),
		Type:       l.Type,
		Status:     l.Status,
		Reason:     reason,
		ReviewedBy: l.ReviewedBy,
		ReviewedAt: reviewedAt,
		CreatedAt:  l.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:  l.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
