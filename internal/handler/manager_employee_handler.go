package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/backendkit/pagination"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/pkg"
	"github.com/ovander/parashift/internal/service"
)

// ManagerEmployeeHandler handles store-scoped employee CRUD for the manager role.
// The store (tenant) ID is taken from the JWT context (no URL param needed).
// All routes require TenantMiddleware and the manager or admin role.
type ManagerEmployeeHandler struct {
	svc *service.EmployeeService
}

// NewManagerEmployeeHandler creates a new ManagerEmployeeHandler.
func NewManagerEmployeeHandler(svc *service.EmployeeService) *ManagerEmployeeHandler {
	return &ManagerEmployeeHandler{svc: svc}
}

// List returns all employees in the manager's own store.
//
//	GET /api/v1/manager/employees
func (h *ManagerEmployeeHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)
	params := pagination.Parse(r)

	employees, total, err := h.svc.List(ctx, tenantID, params.Page, params.PerPage)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	responses := make([]dto.EmployeeResponse, len(employees))
	for i, emp := range employees {
		responses[i] = toEmployeeResponse(emp)
	}
	pkg.WriteJSON(w, http.StatusOK, pagination.NewPagedResponse(responses, params, total))
}

// Get returns a specific employee in the manager's own store.
//
//	GET /api/v1/manager/employees/{employeeId}
func (h *ManagerEmployeeHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "employeeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid employee ID").WithKey("errors.invalidEmployeeId"))
		return
	}

	emp, err := h.svc.GetByID(ctx, tenantID, id)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}
	pkg.WriteJSON(w, http.StatusOK, toEmployeeResponse(emp))
}

// Create creates a new employee in the manager's own store.
//
//	POST /api/v1/manager/employees
func (h *ManagerEmployeeHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	var req dto.CreateEmployeeRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body").WithKey("errors.invalidInput"))
		return
	}
	// Manager cannot create other managers — restrict to employee position (SEC-7).
	if !guardManagerPosition(w, r, req.Position) {
		return
	}

	emp, err := h.svc.Create(ctx, tenantID, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}
	pkg.WriteJSON(w, http.StatusCreated, toEmployeeResponse(emp))
}

// Update updates an employee in the manager's own store.
//
//	PUT /api/v1/manager/employees/{employeeId}
func (h *ManagerEmployeeHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "employeeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid employee ID").WithKey("errors.invalidEmployeeId"))
		return
	}

	var req dto.UpdateEmployeeRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body").WithKey("errors.invalidInput"))
		return
	}
	// Prevent managers from promoting employees to manager (SEC-7).
	reqPosition := ""
	if req.Position != nil {
		reqPosition = *req.Position
	}
	if !guardManagerPosition(w, r, reqPosition) {
		return
	}

	emp, err := h.svc.Update(ctx, tenantID, id, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}
	pkg.WriteJSON(w, http.StatusOK, toEmployeeResponse(emp))
}

// Delete removes an employee from the manager's own store.
//
//	DELETE /api/v1/manager/employees/{employeeId}
func (h *ManagerEmployeeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "employeeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid employee ID").WithKey("errors.invalidEmployeeId"))
		return
	}

	if err := h.svc.Delete(ctx, tenantID, id); err != nil {
		pkg.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
