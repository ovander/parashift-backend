package middleware

import "github.com/ovander/backendkit/httpware"

// Permission constants for ParaShift RBAC.
const (
	PermViewSchedule    httpware.Permission = "schedule:view"
	PermManageSchedule  httpware.Permission = "schedule:manage"
	PermViewEmployees   httpware.Permission = "employees:view"   // read-only: list + get colleagues
	PermManageEmployees httpware.Permission = "employees:manage" // write: create, update, delete
	PermManageStore     httpware.Permission = "store:manage"
	PermViewLeave       httpware.Permission = "leave:view"   // read own leave requests
	PermManageLeave     httpware.Permission = "leave:manage" // read all + approve/reject
	PermViewCoverage    httpware.Permission = "coverage:view"
	PermManageSwap      httpware.Permission = "swap:manage"

	// PermPlatformAdmin gates the cross-tenant /admin surface. It is granted
	// ONLY to the platform "admin" role (never to a store "manager"), so that
	// tenant-scoped managers cannot reach cross-tenant admin endpoints.
	PermPlatformAdmin httpware.Permission = "platform:admin"
)
