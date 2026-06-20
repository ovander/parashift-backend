package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/pkg"
	"github.com/ovander/parashift/internal/service"
)

// ClaimHandler handles invite-claim requests.
// The claim endpoint is protected by auth (JWT required) but does NOT use TenantMiddleware,
// because the user has no Employee record yet — they are in the process of creating one.
type ClaimHandler struct {
	svc *service.EmployeeService
}

// NewClaimHandler creates a new ClaimHandler.
func NewClaimHandler(svc *service.EmployeeService) *ClaimHandler {
	return &ClaimHandler{svc: svc}
}

// Claim binds the authenticated user's Socrate sub to the employee invite identified by
// the claim token in the URL path. The token is invalidated after a successful claim.
//
//	POST /api/v1/claim/{token}
func (h *ClaimHandler) Claim(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := chi.URLParam(r, "token")
	if token == "" {
		pkg.WriteError(w, apierror.BadRequest("missing claim token").WithKey("errors.invalidInput"))
		return
	}

	sub := ctxutil.GetUserSub(ctx)
	if sub == "" {
		pkg.WriteError(w, apierror.Unauthorized("missing user identity").WithKey("errors.accessDenied"))
		return
	}

	// callerEmail comes from the verified token's email claim when present; the
	// service enforces it against the invited employee's email (SEC-6).
	callerEmail := ctxutil.GetUserEmail(ctx)
	emp, err := h.svc.ClaimByToken(ctx, token, sub, callerEmail)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, toEmployeeResponse(emp))
}
