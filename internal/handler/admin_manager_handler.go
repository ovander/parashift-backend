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

// AdminManagerHandler handles manager-specific CRUD (role=manager) for the admin.
// It is a thin wrapper over AdminEmployeeHandler that hard-codes role=manager.
// All routes require PermManageStore.
type AdminManagerHandler struct {
	svc *service.EmployeeService
}

// NewAdminManagerHandler creates a new AdminManagerHandler.
func NewAdminManagerHandler(svc *service.EmployeeService) *AdminManagerHandler {
	return &AdminManagerHandler{svc: svc}
}

// List returns all employees with role=manager across all stores.
//
//	GET /api/v1/admin/managers
func (h *AdminManagerHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params := pagination.Parse(r)

	filter := repo.EmployeeFilter{Position: "manager"}
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

// Get returns a single manager by ID.
//
//	GET /api/v1/admin/managers/{managerId}
func (h *AdminManagerHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := uuid.Parse(chi.URLParam(r, "managerId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid manager ID"))
		return
	}

	emp, err := h.svc.GetByIDGlobal(ctx, id)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}
	if emp.Position != "manager" {
		pkg.WriteError(w, apierror.NotFound("manager", id.String()))
		return
	}
	pkg.WriteJSON(w, http.StatusOK, toEmployeeResponse(emp))
}

// Create creates a new manager in a specified store (store_id required in body).
// Role is forced to "manager" regardless of what is supplied in the request.
//
//	POST /api/v1/admin/managers
func (h *AdminManagerHandler) Create(w http.ResponseWriter, r *http.Request) {
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
	// Always manager position.
	req.Position = "manager"

	emp, err := h.svc.Create(ctx, *req.StoreID, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}
	pkg.WriteJSON(w, http.StatusCreated, toEmployeeResponse(emp))
}

// Update updates a manager's profile (role stays manager).
//
//	PUT /api/v1/admin/managers/{managerId}
func (h *AdminManagerHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := uuid.Parse(chi.URLParam(r, "managerId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid manager ID"))
		return
	}

	var req dto.UpdateEmployeeRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body"))
		return
	}
	// Prevent position demotion via this endpoint.
	managerPosition := "manager"
	req.Position = &managerPosition

	emp, err := h.svc.UpdateGlobal(ctx, id, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}
	pkg.WriteJSON(w, http.StatusOK, toEmployeeResponse(emp))
}

// Delete removes a manager record.
//
//	DELETE /api/v1/admin/managers/{managerId}
func (h *AdminManagerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := uuid.Parse(chi.URLParam(r, "managerId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid manager ID"))
		return
	}

	if err := h.svc.DeleteGlobal(ctx, id); err != nil {
		pkg.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ResendInvite resends the invite to a manager.
//
//   - Managers not yet in Socrate: creates the Socrate account and sends the invite
//     email (InviteUserAsService path). Returns {email_sent: true} on success or
//     {email_sent: false, claim_token: "..."} when Socrate is unavailable.
//   - Managers already in Socrate: sends a magic-link email. Returns {email_sent: true}.
//
// POST /api/v1/admin/managers/{managerId}/resend-invite
func (h *AdminManagerHandler) ResendInvite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := uuid.Parse(chi.URLParam(r, "managerId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid manager ID"))
		return
	}

	result, err := h.svc.ResendInvite(ctx, id)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}
	pkg.WriteJSON(w, http.StatusOK, result)
}
