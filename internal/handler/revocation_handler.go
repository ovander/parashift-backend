package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/pkg"
	"github.com/ovander/parashift/internal/service"
)

// RevocationHandler exposes instant session revocation: a user can revoke their
// own sessions, and a platform admin can revoke any employee's sessions. Both
// raise the subject's token revocation floor so previously-issued tokens stop
// being accepted before their natural expiry (backendkit v1.8.0).
type RevocationHandler struct {
	revSvc *service.RevocationService
	empSvc *service.EmployeeService
}

// NewRevocationHandler creates a new RevocationHandler.
func NewRevocationHandler(revSvc *service.RevocationService, empSvc *service.EmployeeService) *RevocationHandler {
	return &RevocationHandler{revSvc: revSvc, empSvc: empSvc}
}

// RevokeMine revokes all of the authenticated caller's own sessions ("log out
// everywhere"). The subject comes from the validated token, so a caller can only
// ever revoke themselves.
//
//	POST /api/v1/me/sessions/revoke
func (h *RevocationHandler) RevokeMine(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sub := ctxutil.GetUserSub(ctx)
	if sub == "" {
		pkg.WriteError(w, apierror.Unauthorized("missing user identity").WithKey("errors.accessDenied"))
		return
	}
	if err := h.revSvc.RevokeSessions(ctx, sub); err != nil {
		pkg.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RevokeEmployee revokes all sessions of a target employee. Platform-admin only
// (gated by the /admin route group). The employee is resolved cross-tenant by ID;
// its Socrate subject (AuthID) is the revocation key.
//
//	POST /api/v1/admin/employees/{employeeId}/sessions/revoke
func (h *RevocationHandler) RevokeEmployee(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := uuid.Parse(chi.URLParam(r, "employeeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid employee id").WithKey("errors.invalidInput"))
		return
	}

	emp, err := h.empSvc.GetByIDGlobal(ctx, id)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}
	if emp.AuthID == "" {
		// The employee has never claimed their account, so there are no live
		// sessions to revoke. Treat as a no-op success.
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := h.revSvc.RevokeSessions(ctx, emp.AuthID); err != nil {
		pkg.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
