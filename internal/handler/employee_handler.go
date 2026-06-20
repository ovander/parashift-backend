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

// EmployeeHandler handles employee-related HTTP requests.
type EmployeeHandler struct {
	svc *service.EmployeeService
}

// NewEmployeeHandler creates a new EmployeeHandler.
func NewEmployeeHandler(svc *service.EmployeeService) *EmployeeHandler {
	return &EmployeeHandler{svc: svc}
}

// toEmployeeResponse maps *model.Employee → dto.EmployeeResponse.
// StoreName is populated from the virtual field when set via a JOIN query (admin cross-tenant).
func toEmployeeResponse(e *model.Employee) dto.EmployeeResponse {
	return dto.EmployeeResponse{
		ID:         e.ID,
		TenantID:   e.TenantID,
		Name:       e.Name,
		Position:   e.Position,
		JobRole:    e.JobRole,
		ContractID: e.ContractID,
		StartDate:  e.StartDate.Format("2006-01-02"),
		Email:      e.Email,
		AuthID:     e.AuthID,
		ClaimToken: e.ClaimToken,
		StoreName:  e.StoreName,
		CreatedAt:  e.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:  e.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// List returns all employees in a store with pagination.
func (h *EmployeeHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}

	if storeID != tenantID && ctxutil.GetUserRole(ctx) != "admin" {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	params := pagination.Parse(r)

	// Service: List(ctx, tenantID, page, pageSize) → ([]*model.Employee, int64, error)
	employees, total, err := h.svc.List(ctx, storeID, params.Page, params.PerPage)
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

// Get returns a specific employee by ID.
func (h *EmployeeHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}

	if storeID != tenantID && ctxutil.GetUserRole(ctx) != "admin" {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	employeeID, err := uuid.Parse(chi.URLParam(r, "employeeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid employee ID").WithKey("errors.invalidEmployeeId"))
		return
	}

	// Service: GetByID(ctx, tenantID, id) — tenantID is the store (storeID == tenantID)
	employee, err := h.svc.GetByID(ctx, storeID, employeeID)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, toEmployeeResponse(employee))
}

// Create creates a new employee in a store.
func (h *EmployeeHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}

	if storeID != tenantID && ctxutil.GetUserRole(ctx) != "admin" {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	// Decode canonical DTO: Name string, Role string, StartDate time.Time, AuthID string
	var req dto.CreateEmployeeRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body").WithKey("errors.invalidInput"))
		return
	}

	// Privilege guard: only an admin may create a manager (SEC-7).
	if !guardManagerPosition(w, r, req.Position) {
		return
	}

	// Service: Create(ctx, tenantID, req dto.CreateEmployeeRequest)
	employee, err := h.svc.Create(ctx, storeID, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusCreated, toEmployeeResponse(employee))
}

// Update updates an existing employee.
func (h *EmployeeHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}

	if storeID != tenantID && ctxutil.GetUserRole(ctx) != "admin" {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	employeeID, err := uuid.Parse(chi.URLParam(r, "employeeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid employee ID").WithKey("errors.invalidEmployeeId"))
		return
	}

	// Decode canonical DTO: Name *string, Role *string, StartDate *time.Time
	var req dto.UpdateEmployeeRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body").WithKey("errors.invalidInput"))
		return
	}

	// Privilege guard: a manager cannot promote an employee to manager (SEC-7).
	reqPosition := ""
	if req.Position != nil {
		reqPosition = *req.Position
	}
	if !guardManagerPosition(w, r, reqPosition) {
		return
	}

	// Service: Update(ctx, tenantID, id, req dto.UpdateEmployeeRequest)
	employee, err := h.svc.Update(ctx, storeID, employeeID, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, toEmployeeResponse(employee))
}

// Delete deletes an employee.
func (h *EmployeeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}

	if storeID != tenantID && ctxutil.GetUserRole(ctx) != "admin" {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	employeeID, err := uuid.Parse(chi.URLParam(r, "employeeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid employee ID").WithKey("errors.invalidEmployeeId"))
		return
	}

	// Service: Delete(ctx, tenantID, id)
	if err := h.svc.Delete(ctx, storeID, employeeID); err != nil {
		pkg.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
