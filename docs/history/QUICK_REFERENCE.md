# ParaShift Backend - Quick Reference Guide

## Starting the Server

```bash
cd cmd/server
go run main.go
```

Server starts on configured port (default from config).

## Environment Variables / Configuration

Required in `config.yml` or env:
```
PORT=8080
LOG_LEVEL=info
DATABASE_URL=postgres://user:pass@localhost/parashift
JWKS_URL=https://socrate.example.com/.well-known/jwks.json
JWKS_ISSUER=https://socrate.example.com
AUTO_MIGRATE=true
MAX_REQUEST_BODY_BYTES=1048576
```

## API Authentication

All endpoints except `/healthz` require JWT token in `Authorization: Bearer <token>` header.

Token must contain claims:
- `sub`: User UUID (Socrate AuthID)
- `store_id`: Tenant UUID (Store ID)
- `role`: One of: `employee`, `manager`, `admin`

## Health Check

```bash
curl http://localhost:8080/healthz
# Response: {"status":"ok"}
```

## Common Endpoints

### User Profile
```bash
GET /me
# Returns current user's employee record
```

### User Schedule
```bash
GET /me/schedule?from=2025-01-01&to=2025-01-31&page=1&pageSize=20
# Returns user's assigned shifts (paginated)
```

### Schedule ICS Export
```bash
GET /me/schedule.ics?from=2025-01-01&to=2025-01-31
# Returns calendar in ICS format
```

### List Employees
```bash
GET /stores/{storeId}/employees?page=1&pageSize=50
# Requires: PermManageEmployees
```

### Create Employee
```bash
POST /stores/{storeId}/employees
{
  "auth_id": "uuid",
  "first_name": "John",
  "last_name": "Doe",
  "email": "john@example.com",
  "role": "employee",
  "phone_number": "+1234567890",
  "start_date": "2025-01-01T00:00:00Z"
}
```

### Get Schedule
```bash
GET /stores/{storeId}/schedule?from=2025-01-01&to=2025-01-31&page=1&pageSize=20
# Returns shifts with assignments (paginated)
```

### Create Shift
```bash
POST /stores/{storeId}/shifts
{
  "start_time": "2025-01-15T09:00:00Z",
  "end_time": "2025-01-15T17:00:00Z",
  "role": "cashier"
}
```

### Assign Employee to Shift
```bash
POST /stores/{storeId}/assignments
{
  "shift_id": "uuid",
  "employee_id": "uuid"
}
```

### Set Employee Availability
```bash
POST /stores/{storeId}/employees/{employeeId}/availability
{
  "date": "2025-01-15",
  "status": "available",
  "notes": "Prefer morning shift"
}
```

### Create Leave Request
```bash
POST /stores/{storeId}/leave-requests
{
  "employee_id": "uuid",  # Optional for employees (uses self)
  "start_date": "2025-01-20T00:00:00Z",
  "end_date": "2025-01-24T00:00:00Z",
  "reason": "Vacation"
}
```

### Review Leave Request (Manager)
```bash
PUT /stores/{storeId}/leave-requests/{leaveId}/review
{
  "status": "approved",
  "comments": "Approved"
}
# Status: approved | denied
```

### Create Swap Request
```bash
POST /stores/{storeId}/swap-requests
{
  "requestor_id": "uuid",
  "target_employee_id": "uuid",
  "requestor_shift_id": "uuid",
  "target_shift_id": "uuid"
}
```

### Coverage Analysis
```bash
GET /stores/{storeId}/coverage?from=2025-01-01&to=2025-01-31
# Returns coverage data with required vs assigned counts
```

### Coverage Gaps
```bash
GET /stores/{storeId}/coverage/gaps?from=2025-01-01&to=2025-01-31
# Returns gaps in coverage by role
```

## Error Response Format

All errors return consistent JSON:
```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "store not found"
  }
}
```

Common error codes:
- `NOT_FOUND` (404): Resource not found
- `BAD_REQUEST` (400): Invalid input
- `UNAUTHORIZED` (401): Missing/invalid auth
- `FORBIDDEN` (403): Permission denied
- `CONFLICT` (409): Resource conflict
- `INTERNAL` (500): Server error

## Pagination

All list endpoints support:
- `?page=1` (default: 1)
- `?pageSize=50` (default: 20, max: configurable)

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

## Permissions

### Employee Role
Can:
- View own schedule
- View own availability
- Create/view leave requests (own)
- Create/view swap requests (own)
- View coverage

### Manager Role
Can:
- Do everything employee can do
- Manage employees (CRUD)
- Manage schedules (shifts, assignments)
- Manage leave requests (review)
- Manage swap requests (review)
- Manage coverage requirements
- Update own store settings

### Admin Role
Can:
- Everything (all endpoints)
- Create/manage stores
- Manage all users and stores
- System administration

## Date Formats

All dates in API:
- ISO 8601: `2025-01-15T09:00:00Z`
- Query params: `YYYY-MM-DD` (e.g., `?from=2025-01-15&to=2025-01-31`)

## Rate Limiting

- Rate limit: 100 requests per second per IP
- Burst: 200 requests allowed
- Headers returned:
  - `X-RateLimit-Limit`: 100
  - `X-RateLimit-Remaining`: Requests left
  - `X-RateLimit-Reset`: Unix timestamp

## Timeouts

- Standard CRUD: 5 seconds
- Admin operations: 10 seconds
- Health check: no timeout
- ICS export: 5 seconds

## Common Issues

### 401 Unauthorized
- Missing/invalid JWT token
- Token expired
- Employee record not found

### 403 Forbidden
- User doesn't have required permission
- User trying to access different store
- Employee not found in store

### 400 Bad Request
- Invalid JSON
- Missing required fields
- Invalid date format (use YYYY-MM-DD or ISO 8601)
- Invalid UUID format

## Development

### Running Tests
```bash
go test ./...
```

### Code Structure
```
cmd/server/
  main.go      - Entry point
  bootstrap.go - Initialization

internal/
  config/      - Configuration
  handler/     - HTTP handlers
  middleware/  - HTTP middleware
  router/      - Route definitions
  service/     - Business logic
  repo/        - Data access
  model/       - Data models
  event/       - Event system
  pkg/         - Utilities
```

### Adding New Endpoints

1. Create handler method in appropriate handler file
2. Add route in `internal/router/router.go`
3. Add RBAC permission check if needed
4. Implement service method
5. Write tests

### Adding New Permissions

1. Add constant in `internal/middleware/permissions.go`
2. Update `RolePermissionMap` for roles
3. Use `mw.RBAC.Require(newPermission)` in router

## Logging

All logs are JSON formatted:
```json
{
  "timestamp": "2025-01-15T10:30:00Z",
  "level": "info",
  "message": "user accessed schedule",
  "component": "handler",
  "request_id": "uuid"
}
```

Set log level in config: `debug`, `info`, `warn`, `error`

## Database

Using PostgreSQL with GORM.

Connection pooling configured:
- Max open: 25
- Max idle: 5
- Max lifetime: 5 minutes

Migrations in `migrations/` directory (golang-migrate format).

## Security

Enforced security:
- JWT validation (JWKS)
- Tenant isolation (all requests scoped to store_id)
- RBAC (role-based access control)
- Rate limiting
- Security headers (HSTS, X-Content-Type-Options, etc.)
- Request body size limits
- Panic recovery
- Request timeouts
- SQL injection prevention (GORM parameterized queries)
