package middleware

import (
	"net/http"

	"github.com/ovander/backendkit/httpware"
	"github.com/sirupsen/logrus"
)

// RBACMiddleware wraps httpware.RBAC with the ParaShift role-permission map.
type RBACMiddleware struct {
	rbac *httpware.RBAC
}

// NewRBACMiddleware creates the RBAC middleware with ParaShift's role map.
func NewRBACMiddleware(logger *logrus.Entry) *RBACMiddleware {
	roleMap := httpware.RoleMap{
		// Positions control RBAC; job_role is orthogonal and controls shift eligibility only.
		"employee": []httpware.Permission{PermViewSchedule, PermViewCoverage, PermViewEmployees, PermViewLeave},
		"manager": []httpware.Permission{
			PermViewSchedule, PermViewCoverage, PermViewEmployees,
			PermManageSchedule, PermManageEmployees,
			PermViewLeave, PermManageLeave, PermManageSwap, PermManageStore,
		},
		"admin": []httpware.Permission{
			PermViewSchedule, PermViewCoverage, PermViewEmployees,
			PermManageSchedule, PermManageEmployees,
			PermViewLeave, PermManageLeave, PermManageSwap, PermManageStore,
		},
	}
	return &RBACMiddleware{rbac: httpware.NewRBAC(roleMap, logger)}
}

// Require returns middleware that enforces the given permission.
func (m *RBACMiddleware) Require(perm httpware.Permission) func(http.Handler) http.Handler {
	return m.rbac.Require(perm)
}
