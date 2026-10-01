package middleware

// Auth middleware: backendkit/jwtauth validates the Socrate access token
// (RS256 against the JWKS, issuer, audience, expiry, revocation), then the
// role is scoped to Parashift.

import (
	"net/http"

	"github.com/ovander/backendkit/ctxutil"
)

// AppRole scopes the role that later checks see (TenantMiddleware's platform-
// admin bypass, GET /me, RBAC) to Parashift. It wraps the jwtauth handler,
// which has put the token's claims in the context:
//
//   - the role becomes app_roles[clientID], the role Socrate grants for
//     Parashift's client; a user without one is a plain "user";
//   - the token's top-level role claim is never used: Socrate gives its global
//     admins "admin" on every app, even without a membership (compat report
//     row S4), and on a shared Socrate that is not a Parashift role.
//
// Store roles (manager, employee) still come from the employee record, set by
// TenantMiddleware. With an empty clientID (development without Socrate) the
// role is left as the token has it.
func AppRole(clientID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if clientID == "" {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := ctxutil.GetAppRoles(r.Context())[clientID]
			if role == "" {
				role = "user"
			}
			next.ServeHTTP(w, r.WithContext(ctxutil.WithUserRole(r.Context(), role)))
		})
	}
}
