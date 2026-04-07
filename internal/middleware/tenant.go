package middleware

import (
	"net/http"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/repo"
	"github.com/sirupsen/logrus"
)

// TenantMiddleware resolves the authenticated employee from the JWT sub claim.
type TenantMiddleware struct {
	empRepo repo.EmployeeRepository
	logger  *logrus.Entry
}

// NewTenantMiddleware creates a new TenantMiddleware.
func NewTenantMiddleware(empRepo repo.EmployeeRepository, logger *logrus.Entry) *TenantMiddleware {
	return &TenantMiddleware{empRepo: empRepo, logger: logger}
}

// Handler is the chi-compatible middleware function.
func (m *TenantMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		role := ctxutil.GetUserRole(ctx)

		// Platform admins bypass tenant DB lookup
		if role == "admin" {
			next.ServeHTTP(w, r)
			return
		}

		// Resolve employee from Socrate sub claim
		sub := ctxutil.GetUserSub(ctx)
		if sub == "" {
			apierror.Unauthorized("missing user identity").WriteJSON(w)
			return
		}

		emp, err := m.empRepo.GetByAuthID(ctx, sub)
		if err != nil {
			m.logger.WithError(err).WithField("sub", sub).Error("db error resolving employee for sub")
			apierror.Internal("internal error").WriteJSON(w)
			return
		}
		if emp == nil {
			m.logger.WithField("sub", sub).Warn("no employee found for sub — user not provisioned")
			apierror.NotFound("employee", sub).WriteJSON(w)
			return
		}

		// Inject the employee's store as the tenant, their DB UUID as the user ID,
		// and their DB position for RBAC.
		// Position (manager|employee) controls access — job_role controls shift eligibility.
		// The Socrate JWT has no tenant claim and carries only the generic "user" role;
		// the source of truth for all three is the employee record.
		ctx = ctxutil.WithTenantID(ctx, emp.TenantID)
		ctx = ctxutil.WithUserID(ctx, emp.ID)
		ctx = ctxutil.WithUserRole(ctx, emp.Position)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
