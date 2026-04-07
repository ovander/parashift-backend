package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/pagination"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/pkg"
	"github.com/ovander/parashift/internal/repo"
	"github.com/ovander/parashift/internal/service"
)

// AdminEmployeeHandler handles cross-tenant employee CRUD for the admin role.
// All routes require PermManageStore (verified in the router middleware chain).
type AdminEmployeeHandler struct {
	svc *service.EmployeeService
}

// NewAdminEmployeeHandler creates a new AdminEmployeeHandler.
func NewAdminEmployeeHandler(svc *service.EmployeeService) *AdminEmployeeHandler {
	return &AdminEmployeeHandler{svc: svc}
}

// List returns all employees across all stores with optional filtering.
//
//	GET /api/v1/admin/employees
//	Query params: role, store_id, status (active|pending|unclaimed), integrity (noStore|noRole|expiredTokens), page, per_page
func (h *AdminEmployeeHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params := pagination.Parse(r)

	filter := repo.EmployeeFilter{
		JobRole:   r.URL.Query().Get("job_role"),
		Status:    r.URL.Query().Get("status"),
		Integrity: r.URL.Query().Get("filter"),
	}
	if rawStore := r.URL.Query().Get("store_id"); rawStore != "" {
		if id, err := uuid.Parse(rawStore); err == nil {
			filter.StoreID = &id
		}
	}

	employees, total, err := h.svc.ListAll(ctx, filter, params.Page, params.PerPage)
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

// Get returns a single employee by ID (global lookup).
//
//	GET /api/v1/admin/employees/{employeeId}
func (h *AdminEmployeeHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := uuid.Parse(chi.URLParam(r, "employeeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid employee ID"))
		return
	}

	emp, err := h.svc.GetByIDGlobal(ctx, id)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}
	pkg.WriteJSON(w, http.StatusOK, toEmployeeResponse(emp))
}

// Create creates a new employee in a specified store (store_id required in body).
//
//	POST /api/v1/admin/employees
func (h *AdminEmployeeHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req dto.CreateEmployeeRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body"))
		return
	}
	if req.StoreID == nil {
		pkg.WriteError(w, apierror.BadRequest("store_id is required"))
		return
	}

	emp, err := h.svc.Create(ctx, *req.StoreID, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}
	pkg.WriteJSON(w, http.StatusCreated, toEmployeeResponse(emp))
}

// Update updates any employee regardless of store.
//
//	PUT /api/v1/admin/employees/{employeeId}
func (h *AdminEmployeeHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := uuid.Parse(chi.URLParam(r, "employeeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid employee ID"))
		return
	}

	var req dto.UpdateEmployeeRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body"))
		return
	}

	emp, err := h.svc.UpdateGlobal(ctx, id, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}
	pkg.WriteJSON(w, http.StatusOK, toEmployeeResponse(emp))
}

// Delete soft-deletes any employee regardless of store.
//
//	DELETE /api/v1/admin/employees/{employeeId}
func (h *AdminEmployeeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := uuid.Parse(chi.URLParam(r, "employeeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid employee ID"))
		return
	}

	if err := h.svc.DeleteGlobal(ctx, id); err != nil {
		pkg.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
