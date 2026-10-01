# ParaShift Go Backend - Complete Architecture

## Overview
This document describes the complete handler, middleware, router, and bootstrap layers implemented for the ParaShift Go backend (`github.com/ovander/parashift`).

## Architecture Principles (Strictly Enforced)
- **Handlers are thin HTTP adapters**: Parse request → call service → write JSON
- **No business logic in handlers**: All logic lives in services
- **Always use `r.Context()`**: Never `context.Background()`
- **All errors via apierror constructors**: Consistent error handling
- **Pagination on all list endpoints**: Using `pagination.Parse()` + `pagination.NewPagedResponse()`
- **Never return bare arrays**: All responses wrapped in structured types

## File Structure

### `/internal/pkg/`
- **json.go**: Helper functions for JSON encoding/decoding
  - `WriteJSON(w, status, v)`: Writes JSON response with given status code
  - `DecodeJSON(r, v)`: Decodes JSON request body

### `/internal/middleware/`
- **permissions.go**: RBAC permission constants and role-permission map
  - Permissions: `PermViewSchedule`, `PermManageSchedule`, `PermManageEmployees`, `PermManageStore`, `PermManageLeave`, `PermViewCoverage`, `PermManageSwap`
  - Role mappings:
    - `employee`: View schedule & coverage only
    - `manager`: All permissions except admin-level operations
    - `admin`: All permissions

- **tenant.go**: TenantMiddleware
  - Resolves JWT `store_id` claim (tenant context)
  - For non-admin users: Verifies employee record exists and belongs to tenant
  - Admin users bypass employee lookup
  - Returns 401 if employee not found, 403 if tenant mismatch

- **rbac.go**: RBACMiddleware
  - Wraps `httpware.RBAC` with ParaShift role-permission mappings
  - `Require(perm)` method returns middleware requiring specific permission

### `/internal/handler/`
All handlers follow thin HTTP adapter pattern. Each handler has:
- Constructor: `NewXxxHandler(svc *service.XxxService)`
- Methods: Parse request → validate → call service → write JSON response
- Private helper: `toXxxResponse(model)` for model-to-DTO conversion

**HealthHandler**
- `Check()`: GET /healthz — Returns {"status": "ok"}

**StoreHandler**
- `List()`: GET /admin/stores — Paginated list of all stores
- `Get()`: GET /admin/stores/{storeId} — Get specific store
- `Create()`: POST /admin/stores — Create new store
- `Update()`: PUT /admin/stores/{storeId} — Update store
- `Delete()`: DELETE /admin/stores/{storeId} — Delete store
- `GetMyStore()`: GET /stores/me — Get authenticated user's store
- `UpdateMyStore()`: PUT /stores/me — Update authenticated user's store

**EmployeeHandler**
- `List()`: GET /stores/{storeId}/employees — Paginated employee list
- `Get()`: GET /stores/{storeId}/employees/{employeeId} — Get employee
- `Create()`: POST /stores/{storeId}/employees — Create employee
- `Update()`: PUT /stores/{storeId}/employees/{employeeId} — Update employee
- `Delete()`: DELETE /stores/{storeId}/employees/{employeeId} — Delete employee
- Enforces: storeId must match tenant OR user must be admin

**ScheduleHandler**
- `GetSchedule()`: GET /stores/{storeId}/schedule?from=YYYY-MM-DD&to=YYYY-MM-DD — Paginated schedule
- `CreateShift()`: POST /stores/{storeId}/shifts — Create shift
- `GetShift()`: GET /stores/{storeId}/shifts/{shiftId} — Get shift
- `UpdateShift()`: PUT /stores/{storeId}/shifts/{shiftId} — Update shift
- `DeleteShift()`: DELETE /stores/{storeId}/shifts/{shiftId} — Delete shift
- `GenerateSchedule()`: POST /stores/{storeId}/schedule/generate — A/B projection
- `CreateAssignment()`: POST /stores/{storeId}/assignments — Assign employee to shift
- `GetAssignments()`: GET /stores/{storeId}/shifts/{shiftId}/assignments — List assignments

**WeekTemplateHandler**
- `GetTemplates()`: GET /stores/{storeId}/employees/{employeeId}/week-templates — Get templates
- `UpsertTemplates()`: PUT /stores/{storeId}/employees/{employeeId}/week-templates — Create/update templates

**CoverageHandler**
- `ListRequirements()`: GET /stores/{storeId}/coverage-requirements — Paginated requirements
- `CreateRequirement()`: POST /stores/{storeId}/coverage-requirements — Create requirement
- `UpdateRequirement()`: PUT /stores/{storeId}/coverage-requirements/{reqId} — Update requirement
- `DeleteRequirement()`: DELETE /stores/{storeId}/coverage-requirements/{reqId} — Delete requirement
- `GetCoverage()`: GET /stores/{storeId}/coverage?from=YYYY-MM-DD&to=YYYY-MM-DD — Coverage analysis
- `GetGaps()`: GET /stores/{storeId}/coverage/gaps?from=YYYY-MM-DD&to=YYYY-MM-DD — Coverage gaps

**AvailabilityHandler**
- `SetAvailability()`: POST /stores/{storeId}/employees/{employeeId}/availability — Set availability
- `GetAvailability()`: GET /stores/{storeId}/employees/{employeeId}/availability?date=YYYY-MM-DD — Get for date
- `ListAvailability()`: GET /stores/{storeId}/employees/{employeeId}/availability/range?from=...&to=... — Paginated range

**LeaveHandler**
- `Create()`: POST /stores/{storeId}/leave-requests — Create leave request
- `List()`: GET /stores/{storeId}/leave-requests?status=pending — Paginated, filterable by status
- `Get()`: GET /stores/{storeId}/leave-requests/{leaveId} — Get leave request
- `Review()`: PUT /stores/{storeId}/leave-requests/{leaveId}/review — Manager approval/denial

**SwapHandler**
- `Create()`: POST /stores/{storeId}/swap-requests — Create swap request
- `List()`: GET /stores/{storeId}/swap-requests?status=pending — Paginated, filterable by status
- `Get()`: GET /stores/{storeId}/swap-requests/{swapId} — Get swap request
- `Review()`: PUT /stores/{storeId}/swap-requests/{swapId}/review — Manager approval/denial

**MeHandler** (Authenticated user endpoints)
- `GetProfile()`: GET /me — Current user's employee profile
- `GetMySchedule()`: GET /me/schedule?from=YYYY-MM-DD&to=YYYY-MM-DD — User's assigned shifts (paginated)
- `ExportICS()`: GET /me/schedule.ics?from=YYYY-MM-DD&to=YYYY-MM-DD — ICS calendar export

**HandlerBundle**
- Holds references to all handler instances
- `NewHandlerBundle(svc *service.ServiceBundle)` initializes all handlers

### `/internal/router/router.go`
Complete chi router with:
- Health check endpoint (no auth)
- Middleware stack (auth → tenant → RBAC)
- Standard routes (5s timeout): Schedule, Employees, Coverage, Leave, Swap, Availability, Me endpoints
- Admin routes (10s timeout): Store management

Key middleware composition:
1. Global: RequestID, SecurityHeaders, BodyLimit, Recover
2. Auth group: JWT auth, rate limiting, tenant validation
3. RBAC: Permission-based access control per route

### `/cmd/server/bootstrap.go`
Strict initialization order (12 steps):
1. Logger setup (JSON formatter, configurable level)
2. Config validation (already loaded)
3. JWT auth initialization (JWKS validation)
4. Database connection (PostgreSQL with connection pooling)
5. SQL migrations (if AutoMigrate enabled)
6. GORM AutoMigrate (dev/staging only)
7. Repository bundle initialization
8. Service bundle initialization
9. Handler bundle initialization
10. Middleware initialization
11. Router setup
12. HTTP server creation

**AppResources struct** holds:
- `DB *gorm.DB`
- `Services *service.ServiceBundle`
- `Server *http.Server`
- `Logger *logrus.Logger`

### `/cmd/server/main.go`
Entry point:
1. Load and validate config
2. Bootstrap application
3. Start server in goroutine
4. Wait for SIGINT/SIGTERM
5. Graceful shutdown (30s timeout):
   - Drain HTTP connections
   - Close event emitter
   - Close database
6. Exit

## Request Flow

1. **HTTP Request arrives** → Router matches route
2. **Middleware chain executes**:
   - RequestID (tracing)
   - SecurityHeaders
   - BodyLimit
   - Recover (panics)
   - JWT auth (validates JWKS)
   - Rate limiting
   - Tenant middleware (validates store access)
   - RBAC middleware (validates permissions)
   - Route timeout
3. **Handler executes**:
   - Parses request body/params
   - Validates input
   - Calls service method
   - Maps result to DTO
   - Writes JSON response
4. **Response sent** to client

## Error Handling

All errors use `apierror` constructors:
- `apierror.NotFound(resource, id)` → 404 + structured error
- `apierror.BadRequest(msg)` → 400
- `apierror.Unauthorized(msg)` → 401
- `apierror.Forbidden(msg)` → 403
- `apierror.Conflict(msg)` → 409
- `apierror.Internal(msg)` → 500
- `apierror.WriteJSON(w, err)` → writes JSON error response

All errors are written via `apierror.WriteJSON()` for consistent JSON format:
```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "store not found"
  }
}
```

## Pagination

All list endpoints use:
```go
params := pagination.Parse(r, 20)  // default page size 20
results, total, err := service.List(ctx, params.Page, params.PageSize)
pkg.WriteJSON(w, http.StatusOK, pagination.NewPagedResponse(results, total, params))
```

Query parameters:
- `?page=1&pageSize=50` (defaults to page=1, pageSize=20)

Response format:
```json
{
  "items": [...],
  "total": 100,
  "page": 1,
  "pageSize": 50,
  "pages": 2
}
```

## Context Usage

All handlers use `r.Context()` for:
- `ctxutil.GetTenantID(ctx)` → Store UUID from JWT
- `ctxutil.GetUserID(ctx)` → User UUID from JWT (Socrate `sub` claim)
- `ctxutil.GetUserRole(ctx)` → Role string from JWT
- `ctxutil.GetLogger(ctx)` → Structured logger for this request

## Security Features

1. **JWT Authentication**: Validates JWKS tokens from Socrate
2. **Tenant Isolation**: TenantMiddleware ensures users can only access their store
3. **RBAC**: Permission-based access control on every protected endpoint
4. **Rate Limiting**: 100 rps with 200 burst per IP
5. **Security Headers**: X-Content-Type-Options, X-Frame-Options, etc.
6. **Request Body Limit**: Prevents oversized payloads
7. **Panic Recovery**: All panics caught and logged
8. **Timeout Protection**: 5s for CRUD, 10s for admin, prevents hanging

## Configuration

All configuration via `config.Config` struct:
- `Port`: Server port
- `LogLevel`: Logging level (debug/info/warn/error)
- `DatabaseURL`: PostgreSQL connection string
- `AutoMigrate`: Run migrations on startup
- `MaxRequestBodyBytes`: Request size limit
- `JWKS.URL`: JWKS endpoint
- `JWKS.Issuer`: JWT issuer claim
- `DBPool.*`: Connection pool settings

## Testing Considerations

All handlers are designed to be easily testable:
- Services are injected (not instantiated in handlers)
- All context values are via standard `ctxutil` functions
- HTTP responses are consistent JSON format
- Errors follow standard pattern
- No global state or singletons
