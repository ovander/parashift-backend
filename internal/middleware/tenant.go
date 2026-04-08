package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/repo"
	"github.com/sirupsen/logrus"
)

// TenantMiddleware resolves the authenticated employee from the JWT sub claim.
type TenantMiddleware struct {
	empRepo        repo.EmployeeRepository
	logger         *logrus.Entry
	socrateBaseURL string        // e.g. "https://golfperformance.fr" — used for /oauth/userinfo
	httpClient     *http.Client
}

// NewTenantMiddleware creates a new TenantMiddleware.
// socrateBaseURL is the public OAuth base URL used to call /oauth/userinfo when
// the access token doesn't carry an email claim (which is the common case).
// Pass "" to disable the userinfo lookup (auto-link will be silently skipped).
func NewTenantMiddleware(empRepo repo.EmployeeRepository, logger *logrus.Entry, socrateBaseURL string) *TenantMiddleware {
	return &TenantMiddleware{
		empRepo:        empRepo,
		logger:         logger,
		socrateBaseURL: strings.TrimRight(socrateBaseURL, "/"),
		httpClient:     &http.Client{Timeout: 3 * time.Second},
	}
}

// fetchEmailFromUserinfo calls Socrate's /oauth/userinfo endpoint with the
// Bearer token already present in the Authorization header of r.
// Returns "" on any error (auto-link is best-effort).
func (m *TenantMiddleware) fetchEmailFromUserinfo(r *http.Request) string {
	if m.socrateBaseURL == "" {
		return ""
	}
	bearer := r.Header.Get("Authorization")
	if bearer == "" {
		return ""
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet,
		m.socrateBaseURL+"/oauth/userinfo", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", bearer)

	resp, err := m.httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return ""
	}
	defer resp.Body.Close()

	var profile struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return ""
	}
	return profile.Email
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
			// Auto-link: if no employee is bound to this auth_id yet, look up the email
			// from Socrate's /oauth/userinfo endpoint (access tokens don't carry email in
			// their JWT claims — only ID tokens do). On a match we bind the two records
			// together so every subsequent request goes through the fast GetByAuthID path.
			email := ctxutil.GetUserEmail(ctx) // populated only when an ID token is used
			if email == "" {
				email = m.fetchEmailFromUserinfo(r)
			}
			if email != "" {
				emp, err = m.empRepo.GetByEmail(ctx, email)
				if err != nil {
					m.logger.WithError(err).WithFields(logrus.Fields{"sub": sub, "email": email}).
						Error("db error during email-based auto-link lookup")
					apierror.Internal("internal error").WriteJSON(w)
					return
				}
				if emp != nil {
					// Found a match — bind the Socrate sub to this employee record.
					emp.AuthID = sub
					if updateErr := m.empRepo.Update(ctx, emp); updateErr != nil {
						m.logger.WithError(updateErr).WithFields(logrus.Fields{"sub": sub, "email": email, "employee_id": emp.ID}).
							Error("failed to auto-link employee auth_id")
						apierror.Internal("internal error").WriteJSON(w)
						return
					}
					m.logger.WithFields(logrus.Fields{"sub": sub, "email": email, "employee_id": emp.ID}).
						Info("auto-linked employee to Socrate sub via email match")
				}
			}
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
