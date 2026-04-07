# ParaShift

[![Build](https://img.shields.io/badge/build-passing-brightgreen)](#)
[![Coverage](https://img.shields.io/badge/coverage-80%25-yellowgreen)](#)
[![License](https://img.shields.io/badge/license-MIT-blue)](#)

**Rule-driven and AI-assisted workforce management for parapharmacies.**

ParaShift is a production-grade backend system that handles shift scheduling, coverage management, and compliance enforcement for multi-store parapharmacy networks. It exposes a RESTful API consumed by web and mobile frontends.

---

## Table of Contents

1. [Project Overview](#1-project-overview)
2. [Product Positioning](#2-product-positioning)
3. [Core Scheduling Flow](#3-core-scheduling-flow)
4. [Architectural Principles](#4-architectural-principles)
5. [Features](#5-features)
6. [Architecture](#6-architecture)
7. [Tech Stack](#7-tech-stack)
8. [Getting Started](#8-getting-started)
9. [Configuration](#9-configuration)
10. [API Overview](#10-api-overview)
11. [Rule Engine](#11-rule-engine)
12. [AI Engine](#12-ai-engine)
13. [Security](#13-security)
14. [Known Limitations](#14-known-limitations)
15. [Why Fail-Open?](#15-why-fail-open)
16. [Scalability](#16-scalability)
17. [Development Guidelines](#17-development-guidelines)
18. [Roadmap](#18-roadmap)
19. [License](#19-license)

---

## 1. Project Overview

Parapharmacies operate under strict staffing constraints: qualified pharmacists must be on shift, rest periods between consecutive shifts must be respected, weekly hour limits must not be exceeded, and store coverage requirements must be met at all times.

ParaShift centralises these constraints in a configurable rule engine evaluated at scheduling time. A secondary AI engine provides ranked employee suggestions and schedule-optimisation recommendations, acting as a decision-support tool — managers remain in full control.

**Key properties:**

- Multi-tenant: each store is an isolated tenant; data never crosses store boundaries
- API-first: all operations are driven through a documented REST API
- Fail-open rule engine: infrastructure failures never block scheduling operations
- AI as assistant: the AI layer suggests, never decides

---

## 2. Product Positioning

ParaShift occupies the space between lightweight scheduling tools and heavyweight enterprise workforce management systems — and is deliberately better than both in its target context.

**What it replaces**

Most scheduling operations at this scale run on spreadsheets, consumer-grade tools, or manual coordination. These approaches lack structured rule enforcement, audit trails, and any meaningful decision-support layer. ParaShift replaces them with an API-backed, compliance-aware planning workflow.

**What it avoids**

Enterprise WFM platforms (SAP, Kronos, UKG) are built for HR departments, not frontline operations. They are expensive to deploy, slow to configure, and structurally unsuited to the fast iteration cadence of independent retail or parapharmacy networks. ParaShift is purpose-built for operations teams who need to act fast within real regulatory constraints.

**Target segment**

- Multi-location parapharmacy networks and retail pharmacy chains
- Hospitality and F&B operations with qualification-dependent shift staffing
- Any compliance-heavy environment where scheduling errors carry legal or operational risk (EU Working Time Directive, sector-specific rest period mandates)

**Key differentiators**

- **Rule engine as first-class system** — constraints are not hardcoded business logic but configurable, database-backed, independently extensible evaluators
- **AI as decision support, not automation** — AI suggestions augment manager judgment; they never modify schedules autonomously or override rule enforcement
- **Multi-tenant by design** — store isolation is enforced at every system layer, not as an afterthought

---

## 3. Core Scheduling Flow

The following describes the primary backend workflow triggered when a manager assigns an employee to a shift. This is the most critical and frequently exercised code path in the system.

1. **Assignment request received** — the client sends `POST /stores/{storeId}/assignments` with a `shiftId` and `employeeId`. `TenantMiddleware` validates that the authenticated user belongs to the store in the URL. `RBACMiddleware` confirms the `manage_schedule` permission.

2. **ScheduleService takes control** — the handler delegates to `ScheduleService.CreateAssignment`, which owns the full transactional lifecycle of the operation. No business logic runs in the handler layer.

3. **Rule engine evaluates constraints** — `RuleEngine.EvaluateAssignment` loads all enabled rules for the tenant, pre-fetches the employee's recent assignments in a single ±14-day window query, and runs each active rule through its registered `RuleEvaluator`.

4. **Violations classified** — each evaluator returns PASS or FAIL. FAIL results are converted to `RuleViolation` values tagged with severity: `BLOCKING`, `WARNING`, or `INFO`. BLOCKING violations cause immediate abort; WARNING and INFO violations are collected and returned alongside a successful response.

5. **Assignment persisted or rejected** — if no BLOCKING violations exist, the assignment is written to the database. The response includes any WARNING or INFO violations so the frontend can surface them. If BLOCKING violations exist, the service returns 422 with the full violation list and no write occurs.

6. **Coverage recalculated** — after a successful write, the coverage engine recalculates staffing ratios for the affected time slots. Updated coverage data is available immediately via the coverage endpoints.

7. **AI suggestions available on demand** — independently of the assignment flow, managers can request `GET /ai/suggest-assignment?shiftId=<uuid>` at any point. The AI engine ranks available employees and returns confidence-scored recommendations. This call is fully decoupled from the rule engine and never blocks or modifies any assignment.

---

## 4. Architectural Principles

These are the non-negotiable design decisions that govern the codebase. They exist to maintain correctness, testability, and extensibility as the feature set evolves.

- **API-first.** Every feature is designed around its API contract first. The REST interface is the system boundary — internal implementation details are never exposed and can evolve independently.

- **Strict layer separation.** Handlers parse requests and write responses. Services own business logic. Repositories own data access. Nothing crosses these boundaries. A service never constructs an HTTP response; a handler never touches a database.

- **Stateless by design.** The server holds no mutable in-memory state between requests beyond short-lived caches (AI result cache, JWKS key cache). Any number of replicas can run behind a load balancer with no coordination and no sticky sessions required.

- **Fail-open for availability.** The rule engine and AI engine are designed to degrade gracefully under infrastructure failures. A database outage during rule evaluation allows the scheduling operation to proceed rather than bringing operations to a halt. See [Section 15](#15-why-fail-open) for the rationale.

- **Multi-tenant isolation as a correctness invariant.** Tenant isolation is not a feature — it is a structural guarantee enforced at the middleware, handler, and repository layers simultaneously. A bug in one layer cannot leak data if the other two layers are correct.

---

## 5. Features

**Shift scheduling** — create, update, and delete shift instances; assign employees to shifts; bulk-generate schedules from weekly templates; publish schedules to make them visible to employees.

**Rule-based validation** — a configurable, DB-driven rule engine evaluates every assignment against active rules. Rules cover role requirements, weekly hour caps, minimum rest periods, and overlap prevention. Severity levels (BLOCKING / WARNING / INFO) determine whether a violation rejects the operation or only produces a warning.

**Coverage management** — store managers define minimum staffing requirements per time slot. A coverage engine calculates gaps in real time and surfaces them via a dedicated endpoint.

**A/B schedule support** — employees can have two weekly schedule templates (scheme A and scheme B) that alternate week-by-week, supporting non-uniform rotation patterns common in parapharmacy staffing.

**AI-assisted suggestions** — when assigning a shift, the AI engine ranks available employees by suitability. If the LLM gateway is unavailable, a deterministic heuristic scorer (role match + hours-balance) provides guaranteed suggestions. Managers see ranked candidates with confidence scores and reasoning.

**AI insights** — a background job periodically analyses the current schedule and persists insights about coverage gaps, rest violations, and fairness anomalies. Managers can review and dismiss insights.

**Leave and swap management** — employees submit leave requests and shift-swap requests. Managers review and approve or reject them. Approved swaps automatically update assignments and refresh denormalized shift-time data.

**Multi-store support** — up to 50 stores with full data isolation. A platform admin layer manages store provisioning.

---

## 6. Architecture

### High-level diagram

```
  Clients (web / mobile)
         │
         ▼
  ┌──────────────────────────────────────┐
  │         HTTP API (Chi router)        │
  │  Auth MW → Tenant MW → RBAC MW       │
  └────────────────┬─────────────────────┘
                   │
       ┌───────────┼───────────┐
       ▼           ▼           ▼
  ┌─────────┐ ┌─────────┐ ┌─────────┐
  │ Handler │ │ Handler │ │ Handler │  ...
  └────┬────┘ └────┬────┘ └────┬────┘
       │            │           │
       ▼            ▼           ▼
  ┌──────────────────────────────────┐
  │           Service Layer          │
  │  ScheduleService  RuleEngine     │
  │  AIService        CoverageEngine │
  │  LeaveService     SwapService    │
  └──────────────────┬───────────────┘
                     │
       ┌─────────────┼─────────────┐
       ▼             ▼             ▼
  ┌─────────┐  ┌──────────┐  ┌──────────────┐
  │  Repos  │  │  AI GW   │  │   Socrate    │
  │ (GORM)  │  │  (LLM)   │  │ (OAuth2/OIDC)│
  └────┬────┘  └──────────┘  └──────────────┘
       │
       ▼
  ┌──────────┐
  │PostgreSQL│
  └──────────┘
```

This architecture was chosen deliberately to optimise for three properties: **testability** (each layer is independently mockable — handlers use `httptest`, services use mock repositories, no running infrastructure required); **extensibility** (the rule engine and AI engine are service-layer components with defined interfaces — new evaluators and AI sub-systems plug in without touching the HTTP or data layers); and **operational simplicity** (a stateless server against a single managed database is the right default topology for a 5–50 store deployment, with a clear growth path to read replicas and horizontal scale as needed).

### Backend layers

**Handler layer** — thin HTTP adapters. Responsible for request parsing, parameter validation, and response serialisation. No business logic.

**Service layer** — all business logic lives here. Services are injected with repository interfaces and cross-cutting concerns (logger, event emitter). The rule engine and AI engine are service-layer components.

**Repository layer** — GORM-backed implementations of typed repository interfaces. Each model has its own repository. Tests use in-memory mock implementations of the same interfaces.

### Rule Engine

The rule engine is invoked by `ScheduleService` on every assignment operation. It loads enabled rules from the database, pre-fetches the employee's recent assignments in a single query, then runs each rule through a `RuleEvaluator` registered in a `RuleRegistry`. Adding a new rule type requires only implementing the `RuleEvaluator` interface and registering it — no changes to the engine itself. See [Section 11](#11-rule-engine) for details.

### AI Engine

The AI engine communicates with an LLM via `backendkit/aigateway`. All three sub-engines (recommendation, optimisation, insight) degrade gracefully: if the gateway call fails, `SuggestAssignment` falls back to a deterministic heuristic scorer; optimisation and insight calls log a warning and return empty results without blocking the caller. See [Section 12](#12-ai-engine) for details.

### Authentication (Socrate)

ParaShift delegates all identity management to **Socrate**, an external OAuth2/OIDC provider. The backend validates JWT access tokens using Socrate's JWKS endpoint. No passwords or credentials are stored in ParaShift's database. Role and tenant context are extracted from JWT claims on every request.

---

## 7. Tech Stack

| Component | Technology |
|---|---|
| Language | Go 1.25 |
| HTTP router | Chi v5 |
| ORM | GORM |
| Database | PostgreSQL 16 |
| Migrations | golang-migrate |
| Authentication | OAuth2/OIDC via Socrate; JWT validation via JWKS |
| AI gateway | backendkit/aigateway (Anthropic Claude) |
| Logging | Logrus (structured JSON in production) |
| Testing | testing + testify |
| Container | Docker + Docker Compose |

---

## 8. Getting Started

### Prerequisites

- Go 1.25+
- Docker and Docker Compose
- `golang-migrate` CLI (for manual migrations)
- A running Socrate instance (or a stub for local development)

### Installation

```bash
git clone https://github.com/ovander/parashift.git
cd parashift
cp .env.example .env   # edit values before proceeding
```

### Environment variables

```dotenv
# Application
ENV=development
PORT=8080
LOG_LEVEL=info
AUTO_MIGRATE=true
ALLOWED_ORIGINS=http://localhost:3000
APP_BASE_URL=http://localhost:8080

# Database
DATABASE_URL=postgres://parashift:parashift@localhost:5432/parashift?sslmode=disable
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5
DB_CONN_MAX_LIFETIME=300

# Authentication (Socrate)
SOCRATE_BASE_URL=https://auth.example.com
SOCRATE_ADMIN_URL=https://auth-admin.example.com
SOCRATE_CLIENT_ID=parashift
SOCRATE_CLIENT_SECRET=<secret>
SOCRATE_APP_ID=parashift
SOCRATE_REDIRECT_URL=http://localhost:8080/auth/callback

# JWT / JWKS
JWKS_URL=https://auth.example.com/.well-known/jwks.json
JWKS_ISSUER=https://auth.example.com

# AI (optional — heuristic fallback is used when unset)
ANTHROPIC_API_KEY=
AI_MODEL=claude-opus-4-6
AI_MAX_TOKENS=1024
AI_TIMEOUT_SEC=30
```

### Running locally

```bash
# Start PostgreSQL
make docker-up        # or: docker compose up -d db

# Run the server (applies migrations automatically when AUTO_MIGRATE=true)
make run
```

### Running tests

```bash
make test             # go test -race ./...
make test-verbose     # verbose output
make vet              # go vet
make lint             # golangci-lint (requires golangci-lint installed)
```

### Database migrations (manual)

```bash
export DATABASE_URL=postgres://parashift:parashift@localhost:5432/parashift?sslmode=disable

make migrate-up       # apply all pending migrations
make migrate-down     # roll back one migration
make migrate-create   # scaffold a new migration file
```

---

## 9. Configuration

### Rule configuration

Rules are stored in the `rules` table and managed via the Rules API. Each rule has a `type`, a JSON `configuration` object, a `severity`, and an `enabled` flag. Rules are scoped to a tenant (store).

Built-in rule types:

| Type | Configuration keys | Description |
|---|---|---|
| `ROLE` | `required_role` (optional) | Employee role must match the shift's required qualification |
| `MAX_HOURS` | `max_weekly_hours` (float) | Maximum hours per employee per ISO week |
| `MIN_REST` | `min_rest_hours` (float) | Minimum gap in hours between consecutive shifts |
| `NO_OVERLAP` | — | Employee cannot hold two overlapping shifts |
| `COVERAGE` | — | Minimum staffing levels (evaluated post-assignment) |

### Store setup

Stores are provisioned via the admin API (`POST /admin/stores`). Each store is an isolated tenant. Employees, rules, shifts, and schedules are all scoped to a single store and never visible across store boundaries.

### Multi-tenant behaviour

Tenant isolation is enforced at three levels:

1. **JWT claim** — the tenant ID is extracted from the authenticated user's JWT on every request by `TenantMiddleware`.
2. **URL validation** — handler code verifies that the `{storeId}` path parameter matches the tenant ID in the request context.
3. **Repository queries** — every repository method accepts a `tenantID` parameter included as a mandatory `WHERE` clause condition.

---

## 10. API Overview

All endpoints require a valid JWT bearer token issued by Socrate, except `GET /healthz`.

### Authentication

ParaShift does not implement a login endpoint. Clients obtain tokens from Socrate using the standard OAuth2 authorisation code flow and present them as `Authorization: Bearer <token>` headers.

```
GET /healthz                     → 200 OK (no auth required)
GET /me                          → current user profile
GET /me/schedule                 → authenticated employee's own schedule
GET /me/schedule.ics             → iCal export
```

### Scheduling endpoints

```
GET    /stores/{storeId}/schedule                         → paginated shift list
POST   /stores/{storeId}/schedule/generate                → generate from week templates
POST   /stores/{storeId}/schedule/publish                 → publish schedule

POST   /stores/{storeId}/shifts                           → create shift
GET    /stores/{storeId}/shifts/{shiftId}                 → get shift
PUT    /stores/{storeId}/shifts/{shiftId}                 → update shift
DELETE /stores/{storeId}/shifts/{shiftId}                 → delete shift

GET    /stores/{storeId}/shifts/{shiftId}/assignments     → list assignments for a shift
POST   /stores/{storeId}/assignments                      → create assignment (runs rule engine)
DELETE /stores/{storeId}/assignments/{assignmentId}       → remove assignment
```

### Rule endpoints

```
GET    /stores/{storeId}/rules               → list all rules
POST   /stores/{storeId}/rules               → create rule
GET    /stores/{storeId}/rules/{ruleId}      → get rule
PUT    /stores/{storeId}/rules/{ruleId}      → update rule
DELETE /stores/{storeId}/rules/{ruleId}      → delete rule
```

Example — create a `MAX_HOURS` rule:

```bash
curl -X POST https://api.example.com/stores/{storeId}/rules \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "MAX_HOURS",
    "configuration": { "max_weekly_hours": 40 },
    "severity": "WARNING",
    "enabled": true,
    "description": "Maximum 40 hours per week"
  }'
```

### AI endpoints

```
GET /stores/{storeId}/ai/suggest-assignment?shiftId=<uuid>      → ranked employee suggestions
GET /stores/{storeId}/ai/optimize?from=2025-01-06&to=2025-01-12 → optimisation recommendations
GET /stores/{storeId}/ai/insights                               → list active insights
PUT /stores/{storeId}/ai/insights/{insightId}/dismiss           → dismiss an insight
```

### Coverage endpoints

```
GET /stores/{storeId}/coverage                          → coverage status
GET /stores/{storeId}/coverage/gaps                     → gaps vs. requirements

GET    /stores/{storeId}/coverage-requirements          → list requirements
POST   /stores/{storeId}/coverage-requirements          → create requirement
PUT    /stores/{storeId}/coverage-requirements/{reqId}  → update requirement
DELETE /stores/{storeId}/coverage-requirements/{reqId}  → delete requirement
```

---

## 11. Rule Engine

The rule engine is the **core system of record for scheduling constraints**. It is not an add-on feature — it is the mechanism by which ParaShift operationalises compliance. Every assignment, regardless of source (manual, template-generated, or AI-suggested), passes through the rule engine before being committed.

Three properties define how the engine is built: **dynamic configurability** (rules are stored in the database and evaluated at runtime — no deployment is required to change thresholds or enable new constraints); **extensibility via the evaluator interface** (new rule types are added by implementing a single interface and registering it at startup, with no changes to the engine loop); and **independence from business workflows** (the engine is a pure function of its inputs — it has no knowledge of UI state, AI suggestions, or user identity beyond what is passed to it explicitly).

### Purpose

The rule engine enforces scheduling constraints at assignment time. It prevents invalid schedules from being created (BLOCKING rules) and surfaces advisories for managers to review (WARNING / INFO rules).

### Severity model

| Severity | Behaviour |
|---|---|
| `BLOCKING` | The assignment is rejected; the API returns 422 with the violation list |
| `WARNING` | The assignment succeeds; violations are returned alongside the 200 response |
| `INFO` | The assignment succeeds; informational violations are included in the response |

### Execution flow

1. `ScheduleService.CreateAssignment` calls `RuleEngine.EvaluateAssignment`.
2. The engine fetches all enabled rules for the tenant from the database.
3. It pre-loads the employee's assignments in a ±14-day window around the proposed shift (one query, shared across all evaluators).
4. For each rule, the registered `RuleEvaluator` is retrieved from the `RuleRegistry` and called with `Evaluate(ctx, input, config)`.
5. Each evaluator returns a `RuleResult` (PASS or FAIL). PASS results are logged at DEBUG for audit; FAIL results accumulate.
6. `FilterViolations` converts FAIL results to `RuleViolation` values returned to the service.
7. If any BLOCKING violation exists, the service aborts the assignment and returns the violations to the handler.
8. On infrastructure errors (DB failure, evaluator panic), the engine fails open: the operation proceeds without violations.

### Adding a new rule type

Implement the `RuleEvaluator` interface and register it at startup:

```go
type RuleEvaluator interface {
    Code() string
    Evaluate(ctx context.Context, input EvaluationInput, rawConfig []byte) (EvaluatorOutput, error)
}

// In DefaultRegistry or at server startup:
registry.Register(&MyNewEvaluator{})
```

No changes to the engine loop are required.

### Example rule (MIN_REST)

```json
{
  "type": "MIN_REST",
  "configuration": { "min_rest_hours": 11 },
  "severity": "BLOCKING",
  "enabled": true,
  "description": "EU Working Time Directive — 11 h daily rest"
}
```

---

## 12. AI Engine

The AI engine's role is precisely scoped: **it augments manager decisions, it never enforces them**. AI suggestions exist in a separate decision space from rule enforcement — a manager can ignore every suggestion without consequence, and the AI engine never writes to the schedule autonomously.

This separation is a deliberate architectural choice. Rule compliance is a correctness concern; scheduling optimisation is a quality concern. Conflating them would compromise the predictability of both. The rule engine is deterministic and auditable; the AI engine is probabilistic and advisory. Keeping them architecturally separate preserves each property cleanly.

The engine provides **deterministic fallback by design**. When the LLM gateway is unavailable, `SuggestAssignment` returns heuristic-scored results — not an error. Managers always receive a ranked candidate list. The AI system's reliability guarantee is that it never degrades to zero output when eligible employees exist.

### Overview

The AI engine is an **assistive** layer. It produces ranked suggestions and schedule insights that a manager can act on. It never modifies schedules autonomously.

The engine is composed of three sub-systems:

**Recommendation** (`SuggestAssignment`) — given a shift, ranks all employees by suitability and returns the top 3 candidates with confidence scores and reasons. Results are cached per (tenant, shift) with a 10-minute TTL.

**Optimisation** (`OptimizeSchedule`) — given a date range, analyses the current schedule and returns a list of recommended reassignments that would improve coverage, rest distribution, and fairness.

**Insight** (`GenerateInsights`) — intended to be run as a background job, analyses the upcoming week and persists structured observations (coverage gaps, rest violations, fairness anomalies) as dismissible `AIInsight` records.

### Scoring logic (heuristic fallback)

When the LLM gateway is unavailable, `SuggestAssignment` falls back to a deterministic heuristic scorer:

- **Role match (50 pts)** — employees whose role matches the shift's required qualification receive full points. Employees with a mismatched role (when one is required) are excluded entirely.
- **Hours balance (50 pts)** — inversely proportional to hours already worked relative to the employee's contracted weekly target (default 40 h). An employee at 0 h worked scores 50 pts; one at target scores 0 pts; overtime is penalised further.

The top 3 scored employees are returned. The heuristic guarantees a non-empty suggestion list as long as any eligible employee exists.

### Role of AI vs Rules

| | Rule Engine | AI Engine |
|---|---|---|
| Purpose | Enforce hard and soft constraints | Provide scheduling recommendations |
| Invocation | Automatic on every assignment | On-demand by managers |
| Output | Violations that may block operations | Suggestions with confidence scores |
| Failure mode | Fail-open (no violation on error) | Heuristic scorer fallback |
| Decision authority | System enforces | Manager decides |

---

## 13. Security

### Authentication

All API requests are authenticated via JWT bearer tokens issued by **Socrate** (OAuth2/OIDC). The backend validates tokens by fetching Socrate's JWKS endpoint at startup and caching signing keys. Token claims supply user identity, role, and tenant — all three are established from the token on every request, before any handler logic runs.

No session state is maintained server-side. The backend is fully stateless with respect to authentication: there are no sessions, no server-side token stores, and no revocation lists. Token validity is determined exclusively by JWT signature verification and claim validation against the configured issuer. Short-lived access tokens and identity-provider-managed refresh are the intended token lifecycle model.

### Authorisation

Role-based access control is enforced by `RBACMiddleware`:

| Permission | Scope |
|---|---|
| `manage_schedule` | Create/modify shifts and assignments |
| `view_schedule` | Read schedule and assignment data |
| `manage_employees` | Create/modify employee records |
| `manage_store` | Store settings, rules, coverage requirements |
| `manage_leave` | Review and approve leave requests |
| `manage_swap` | Review and approve swap requests |

### Multi-tenant isolation

Tenant isolation is enforced at every layer — middleware, handler, and repository. A request authenticated for store A can never read or modify data belonging to store B. This triple-layer enforcement means a single layer failure (e.g., a missing handler check) cannot alone produce a data leak — the repository layer will still reject the query.

### Additional practices

- Security headers (`X-Content-Type-Options`, `X-Frame-Options`, `Strict-Transport-Security`) are applied globally.
- Request bodies are size-limited (default 10 MB, configurable via `MAX_REQUEST_BODY_BYTES`).
- All `/admin` endpoints require the `manage_store` permission.
- All credentials, API keys, and secrets are supplied via environment variables — never hardcoded.

---

## 14. Known Limitations

These are honest constraints of the current system. Most are acknowledged in the [Roadmap](#18-roadmap).

- **Single-region deployment assumption.** The system is designed for a single PostgreSQL primary with optional read replicas. There is no built-in support for multi-region active-active topologies or cross-region failover. Geographic distribution requires external infrastructure (connection poolers, global load balancers) not provided by this codebase.

- **No horizontal database sharding.** All tenants share a single database schema. At very high store counts (well above 50) or with very high write throughput, the single-primary PostgreSQL model will require re-evaluation. The `tenant_id` indexing strategy is a foundation for future sharding but sharding is not implemented.

- **AI output quality depends on input data.** The LLM-backed suggestion and insight engines produce lower-quality output when historical assignment data is sparse or when employee profiles are incomplete. New deployments in their first 4–6 weeks of operation will see degraded AI quality; the heuristic fallback remains reliable regardless.

- **Fail-open rule engine may allow suboptimal scheduling under failure.** When the rule engine encounters an infrastructure error (database timeout, evaluator panic), it allows the assignment to proceed without violations. This is the correct trade-off for operational continuity but means that, under a database degradation scenario, rules are temporarily not enforced. See [Section 15](#15-why-fail-open) for rationale.

- **No event streaming by default.** The internal event emitter is in place but outbound webhooks and streaming integrations (payroll systems, HR platforms) are not yet implemented. This is a planned roadmap item.

- **No built-in background job scheduler.** AI insight generation is designed to be triggered by an external scheduler (cron, Kubernetes CronJob). There is no embedded job runner or retry mechanism in the current release.

---

## 15. Why Fail-Open?

The rule engine and AI engine both follow a **fail-open** principle: when an infrastructure error occurs during evaluation, the scheduling operation is permitted to proceed rather than being blocked.

**The trade-off being made**

Fail-open prioritises **availability over strict enforcement** in the event of an infrastructure failure. This means a database timeout during rule evaluation will not prevent a manager from assigning a shift. The alternative — fail-closed, where any evaluation error blocks the assignment — would give infrastructure failures the ability to halt scheduling operations entirely.

**Why this is the right choice for this domain**

Scheduling is a time-sensitive operational activity. A store that cannot assign shifts because the rule engine's database query timed out is in a worse position than one that assigns with temporarily unenforced constraints. Managers have domain expertise and are not blind to the rules — they will schedule appropriately in the absence of automated enforcement. The risk of a suboptimal schedule for one shift is significantly lower than the risk of an operations team unable to staff their store.

**Mitigations**

- All infrastructure errors that trigger fail-open behaviour are logged at WARNING level with full context, creating an audit trail.
- The JWKS key cache and AI result cache have TTLs designed to outlast transient outages without requiring live connectivity on every request.
- Fail-open applies only to infrastructure failures — it does not apply to evaluator logic errors, which are handled as rule violations in the normal path.

---

## 16. Scalability

### Expected scale

ParaShift is designed for networks of up to 50 stores with approximately 500 employees total. A single well-tuned instance running against a managed PostgreSQL cluster is the expected production topology.

### Stateless backend

The HTTP server holds no mutable in-memory state between requests beyond short-lived caches (AI narration cache, JWKS key cache). Multiple replicas can run behind a load balancer without sticky sessions.

### Database

The connection pool is configurable via `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, and `DB_CONN_MAX_LIFETIME`. All repository queries are scoped by `tenant_id`, which is indexed on every tenant-scoped table. Migrations run at startup and are idempotent.

Heavy read paths (schedule views, coverage analysis) benefit from PostgreSQL read replicas if throughput grows beyond a single primary.

---

## 17. Development Guidelines

### Project structure

```
cmd/server/          → main package, bootstrap, logger initialisation
internal/
  config/            → environment-based configuration
  dto/               → request/response types (no business logic)
  handler/           → HTTP handlers (thin adapters)
  middleware/        → auth, tenant, RBAC middleware
  model/             → domain models (GORM structs, domain constants)
  repo/              → repository interfaces and GORM implementations
  router/            → Chi router wiring
  service/           → business logic, rule engine, AI engine
  testutil/          → shared test helpers, mock repositories
migrations/          → SQL migration files (golang-migrate)
```

### Idiomatic Go practices

- Interfaces are defined by consumers (`repo/interfaces.go`), not by implementors.
- Services depend on repository interfaces — concrete GORM types never leak into the service layer.
- Errors from external systems are wrapped with `%w` and converted to typed `apierror` values at service boundaries.
- Context is threaded through every function call; `ctxutil.GetLogger(ctx)` retrieves a request-scoped logger with pre-populated fields.

### Testing strategy

Unit tests are co-located with the code under test (`*_test.go`). Repository calls are mocked via structs in `internal/testutil/mocks.go` with optional `Fn` fields that individual tests override. Handler tests use `httptest.NewRecorder` and inject chi route context directly — no running HTTP server required.

```bash
make test    # go test -race ./...
```

### Logging

Logrus is used with structured fields throughout. In development, output is human-readable; in production (`ENV=production`), it is JSON. Request-scoped fields (request ID, tenant ID, user ID) are injected by middleware and available anywhere via `ctxutil.GetLogger(ctx)`.

### Error handling

- Repository errors are wrapped and propagated to the service layer.
- Services convert repository errors to typed `apierror` values (`NotFound`, `Internal`, `BadRequest`, `Forbidden`).
- Handlers call `pkg.WriteError(w, err)`, which maps `apierror` types to HTTP status codes and serialises a consistent JSON error body.
- The rule engine and AI engine follow a **fail-open** principle: infrastructure errors produce log warnings but do not propagate errors to callers and do not block scheduling operations.

---

## 18. Roadmap

**Rule Engine expansion** — additional built-in rule types (maximum consecutive working days, qualification expiry); a simplified configuration UI for non-technical managers; per-store rule templates.

**AI optimisation** — richer prompts incorporating employee preferences, historical acceptance rates, and contract types; confidence calibration from outcome feedback.

**Predictive features** — forecast staffing needs from historical demand patterns; proactive alerts for under-staffed weeks before the schedule is published.

**Audit trail surface** — expose the structured per-rule PASS/FAIL evaluation log to managers via the API and a dedicated audit screen.

**Webhook / event streaming** — push schedule change events to external systems (payroll, HR) via outbound webhooks backed by the internal event emitter.

---

## 19. License

This project is licensed under the [MIT License](LICENSE).
