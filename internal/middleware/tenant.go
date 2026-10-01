package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/backendkit/socrate"
	"github.com/ovander/parashift/internal/repo"
	"github.com/sirupsen/logrus"
)

// ProfileReader reads the signed-in user's Socrate profile with the bearer
// token jwtauth put in the request context. Satisfied by *socrate.Client
// (GET /api/profile), which returns the e-mail address and whether Socrate
// has verified it.
type ProfileReader interface {
	GetProfile(ctx context.Context) (*socrate.FullProfile, error)
}

// TenantMiddleware resolves the authenticated employee from the JWT sub claim.
type TenantMiddleware struct {
	empRepo  repo.EmployeeRepository
	logger   *logrus.Entry
	profiles ProfileReader // nil → no auto-link by e-mail
}

// NewTenantMiddleware creates a new TenantMiddleware. profiles nil disables
// the auto-link by e-mail (Socrate not configured, tests).
func NewTenantMiddleware(empRepo repo.EmployeeRepository, logger *logrus.Entry, profiles ProfileReader) *TenantMiddleware {
	return &TenantMiddleware{empRepo: empRepo, logger: logger, profiles: profiles}
}

// verifiedEmail returns the signed-in user's e-mail address when Socrate has
// verified it, and "" otherwise or on any error (auto-link is best-effort).
// An unverified address is never used: anyone can register an address at
// Socrate, and linking on it would hand them the employee with that address
// (compat report row S8).
func (m *TenantMiddleware) verifiedEmail(ctx context.Context, sub string) string {
	if m.profiles == nil {
		return ""
	}
	p, err := m.profiles.GetProfile(ctx)
	if err != nil {
		m.logger.WithError(err).WithField("sub", sub).Warn("auto-link: Socrate profile lookup failed")
		return ""
	}
	if p == nil || !p.IsVerified {
		return ""
	}
	return strings.TrimSpace(p.Email)
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
			apierror.Unauthorized("missing user identity").WithKey("errors.accessDenied").WriteJSON(w)
			return
		}

		emp, err := m.empRepo.GetByAuthID(ctx, sub)
		if err != nil {
			m.logger.WithError(err).WithField("sub", sub).Error("db error resolving employee for sub")
			apierror.Internal("internal error").WithKey("errors.unknown").WriteJSON(w)
			return
		}

		if emp == nil {
			// Auto-link: if no employee is bound to this auth_id yet, read the user's
			// e-mail from their Socrate profile (access tokens carry none) and, when
			// Socrate has verified it, bind the unlinked employee with that address,
			// so every subsequent request goes through the fast GetByAuthID path.
			if email := m.verifiedEmail(ctx, sub); email != "" {
				emp, err = m.empRepo.GetByEmail(ctx, email)
				if err != nil {
					m.logger.WithError(err).WithFields(logrus.Fields{"sub": sub, "email": email}).
						Error("db error during email-based auto-link lookup")
					apierror.Internal("internal error").WithKey("errors.unknown").WriteJSON(w)
					return
				}
				if emp != nil {
					// Found a match — bind the Socrate sub to this employee record.
					emp.AuthID = sub
					if updateErr := m.empRepo.Update(ctx, emp); updateErr != nil {
						m.logger.WithError(updateErr).WithFields(logrus.Fields{"sub": sub, "email": email, "employee_id": emp.ID}).
							Error("failed to auto-link employee auth_id")
						apierror.Internal("internal error").WithKey("errors.unknown").WriteJSON(w)
						return
					}
					m.logger.WithFields(logrus.Fields{"sub": sub, "email": email, "employee_id": emp.ID}).
						Info("auto-linked employee to Socrate sub via email match")
				}
			}
		}

		if emp == nil {
			m.logger.WithField("sub", sub).Warn("no employee found for sub — user not provisioned")
			apierror.NotFound("employee", sub).WithKey("errors.notFound").WriteJSON(w)
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
