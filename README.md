# Parashift API

[![CI](https://github.com/ovander/parashift-backend/actions/workflows/ci.yml/badge.svg)](https://github.com/ovander/parashift-backend/actions/workflows/ci.yml)
[![License: AGPL-3.0](https://img.shields.io/github/license/ovander/parashift-backend)](LICENSE)
[![Latest tag](https://img.shields.io/github/v/tag/ovander/parashift-backend?sort=semver&label=version)](https://github.com/ovander/parashift-backend/tags)
[![Go version](https://img.shields.io/github/go-mod/go-version/ovander/parashift-backend)](go.mod)

> Rule-checked, AI-assisted shift planning for pharmacies: every assignment is validated against
> the store's rules before it is saved.

Parashift plans the week of a pharmacy team: stores and their opening hours, employees and
contracts, week templates, shifts and assignments, coverage requirements, leave and swap
requests. A configurable rule engine checks every assignment as it is made (role, overlap,
weekly hours, rest), and an AI layer suggests who could take a shift, without ever deciding.
This repository is the Go REST API; the web app is
[`ovander/parashift-frontend`](https://github.com/ovander/parashift-frontend), and sign-in is
Socrate, the suite's OAuth 2.1 / OpenID Connect server, reached through
[`backendkit`](https://github.com/ovander/backendkit).

---

## Table of contents

- [Why Parashift](#why-parashift)
- [Features](#features)
- [Tech stack](#tech-stack)
- [Architecture](#architecture)
- [Project structure](#project-structure)
- [Getting started](#getting-started)
- [Environment variables](#environment-variables)
- [API overview](#api-overview)
- [Rule engine](#rule-engine)
- [AI engine](#ai-engine)
- [Testing](#testing)
- [Deployment](#deployment)
- [Security](#security)
- [Status](#status)
- [Contributing](#contributing)
- [License](#license)

---

## Why Parashift

A pharmacy cannot open without a pharmacist on shift, and labour law caps weekly hours and
imposes rest between shifts. Spreadsheets check none of it; enterprise workforce suites are built
for HR departments, not for a manager planning between two customers. Parashift puts the
constraints where the decision is made:

- **Rules are data, checked at assignment time.** Each store configures its rules; a blocking
  violation rejects the assignment, a warning is returned with it.
- **AI advises, people decide.** Suggestions come with a confidence score and a reason, and the
  AI never writes to the schedule. Without the AI provider, a deterministic scorer still answers.
- **Stores are isolated.** Every row belongs to a store (tenant); the tenant comes from the
  signed-in employee's record, never from the request.

---

## Features

- **Scheduling:** shifts, assignments, week generation from templates (A/B rotation), publication,
  plan lifecycle (publish, override after publication, history, rollback), store exceptions
  (exceptional openings and closures) and French public holidays.
- **Rule engine:** role, no overlap, maximum weekly hours, minimum rest; severity `BLOCKING`,
  `WARNING` or `INFO`; configurable per store.
- **Coverage:** requirements per time slot, coverage status and gaps.
- **People:** employees, managers, contracts, qualifications (with expiry), availability,
  invitations through Socrate or a claim link.
- **Leave and swaps:** requests, review, and the impact of a leave on coverage.
- **AI:** ranked suggestions for a shift, schedule optimisation, insights (coverage, rest,
  fairness), through `backendkit/aigateway`.
- **Self-service:** `GET /api/v1/me`, my schedule and its `.ics` export, preferred locale, and
  "sign out everywhere" (token revocation).
- **Platform administration:** cross-store `/api/v1/admin` for stores, managers, employees,
  dashboard and the audit log.
- **Operations:** health and readiness probes, version endpoint, Prometheus metrics, OpenTelemetry
  tracing, Sentry.

---

## Tech stack

| Component | Technology |
|---|---|
| Language | Go (`go 1.25` language version; built and tested with Go 1.27.1, pinned by `toolchain` in `go.mod`) |
| HTTP | chi v5, `backendkit/httpware` (request ID, logging, security headers, body limit, recover, timeouts, rate limit, RBAC) |
| Database | PostgreSQL 16, GORM, SQL migrations with golang-migrate (`migrations/`) |
| Identity | Socrate via `backendkit`: `jwtauth` (RS256, JWKS, issuer, audience, revocation check), `socrate.Client` (OAuth and service-account calls) |
| AI | `backendkit/aigateway` (Anthropic Claude), heuristic fallback |
| Observability | logrus, Prometheus (`METRICS_ENABLED`), OpenTelemetry OTLP/HTTP, Sentry |
| Tests | `testing`, testify, go-sqlmock, mockery mocks |
| Lint | golangci-lint v2.14.0 (`.golangci.yml`) |

---

## Architecture

```
browser ── https://parashift.vandermoten.eu ── Caddy ─┬─ /api/* /auth/* → this API (127.0.0.1:$PORT)
                                                      └─ everything else → the SPA (static files)

API ── OAuth (token, refresh, revoke, profile, JWKS) → https://socrate.vandermoten.eu
    └─ admin API (service-account calls: invitations) → SOCRATE_ADMIN_URL (SSH tunnel to Socrate's loopback)
```

Every request runs through: CORS → request ID → client attribution → logger → tracing → security
headers → body limit → recover → locale; then, under `/api/v1`, authentication (`jwtauth` and the
Parashift app role), the general rate limit, the tenant middleware (resolves the employee, their
store and position) and RBAC per route.

- **Layers.** Handlers parse and answer; services hold the business logic; repositories hold the
  SQL. Handlers never call repositories.
- **Tenancy.** `TenantMiddleware` finds the employee whose `auth_id` is the token's `sub` and
  puts their store (tenant), ID and position in the context; every repository call takes the
  tenant from there. Store-scoped routes also check that the `{storeId}` in the URL is the
  caller's store.
- **Roles.** Store roles (`manager`, `employee`) come from the employee record. The platform
  `admin` role comes only from Socrate's `app_roles` entry for Parashift's client, never from the
  token's top-level `role`.
- **Client attribution.** One address, from `X-Forwarded-For` trusted only from a loopback peer
  (Caddy), keys the per-IP rate limits and is sent to Socrate on calls made for a user.
- **Fail-open rule engine.** An infrastructure error during rule evaluation logs a warning and
  lets the assignment through, so a database hiccup does not stop a store from planning.

Sign-in is moving to a Backend-for-Frontend under `/bff`, so that no token reaches the browser;
the plan and the finding it fixes are in [`docs/SOCRATE-COMPAT-REPORT.md`](docs/SOCRATE-COMPAT-REPORT.md).

---

## Project structure

```
cmd/server/          main, bootstrap (config → Socrate check → DB → migrations → services → router)
internal/
  config/            environment configuration and its validation
  dto/               request and response types
  handler/           HTTP handlers (thin)
  middleware/        auth role, tenant, RBAC, client attribution, CORS, locale
  model/             GORM models
  repo/              repository interfaces and GORM implementations (mocks/ by mockery)
  router/            routes, per-IP limiter
  service/           business logic, rule engine, AI engine
  event/             event emitter and the audit-log subscriber
  pkg/               JSON helpers, metrics, tracing
  e2e/               end-to-end tests of the router with mocked repositories
migrations/          numbered SQL migrations (up and down)
scripts/             push.sh (build and upload), deploy-backend.sh (on the VPS)
docs/                compatibility report; history/ (unmaintained notes from the first build)
```

---

## Getting started

### Prerequisites

- Go 1.25 or later (the `toolchain` line in `go.mod` downloads 1.27.1), Docker for the local database, and the
  [`migrate`](https://github.com/golang-migrate/migrate) CLI for manual migrations.
- A Socrate client for sign-in, to go past the public routes.

### Run

```bash
git clone https://github.com/ovander/parashift-backend && cd parashift-backend
cp .env.example .env            # set ENV=development and the values below
make db-up                      # PostgreSQL 16 in Docker (parashift / parashift)
make run                        # http://localhost:4000
```

Migrations run with `RUN_MIGRATIONS=true`, or once with `go run ./cmd/server migrate`.
`AUTO_MIGRATE=true` (GORM schema sync) is for development only and refused in production.

---

## Environment variables

The server reads the environment (and a `.env` file, for development). `ENV` is required.
Production (`ENV=production`) refuses to start when a required value is missing or malformed.

| Variable | Meaning | Default / example |
|---|---|---|
| `ENV` | `development`, `production` or `test`. Required; `/api/v1/debug/token` exists only in `development` | — |
| `PORT` | Listen port | `4000` |
| `BIND_ADDR` | Interface to listen on; empty counts as unset | `127.0.0.1` in production, every interface otherwise |
| `LOG_LEVEL` | `debug`, `info`, `warn`, `error` | `info` |
| `DATABASE_URL` | PostgreSQL URL; `sslmode=disable` is refused in production | — (required) |
| `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, `DB_CONN_MAX_LIFETIME` | Connection pool (lifetime in seconds) | `25`, `5`, `300` |
| `RUN_MIGRATIONS` | Apply `migrations/` at start-up | `false` |
| `AUTO_MIGRATE` | GORM schema sync; development only | `false` |
| `ALLOWED_ORIGINS` | CORS origins, comma-separated; production needs an https one | `http://localhost:3000` |
| `APP_BASE_URL` | This API's public URL | `http://localhost:4000` |
| `MAX_REQUEST_BODY_BYTES` | Request body limit | `10485760` |
| `SOCRATE_BASE_URL` | Socrate's public URL, also the expected issuer; https in production, no trailing slash | `https://socrate.vandermoten.eu` |
| `SOCRATE_JWKS_URL` | JWKS for token validation | `https://socrate.vandermoten.eu/.well-known/jwks.json` |
| `SOCRATE_ISSUER` | Optional; defaults to `SOCRATE_BASE_URL` and must equal it in production | — |
| `SOCRATE_ADMIN_URL` | Socrate's admin API, for service-account calls (invitations). Required in production, never derived | `http://127.0.0.1:18082` |
| `SOCRATE_APP_ID` | Parashift's numeric app ID at Socrate. Required in production | `7` |
| `SOCRATE_CLIENT_ID` | OAuth client ID | — (required) |
| `SOCRATE_CLIENT_SECRET` | OAuth client secret; on the server only | — (required) |
| `SOCRATE_REDIRECT_URL` | Redirect URI used when the SPA sends none | `https://parashift.vandermoten.eu/callback` |
| `ANTHROPIC_API_KEY` | Enables the AI engine (heuristic fallback without it) | — |
| `AI_MODEL`, `AI_MAX_TOKENS`, `AI_TIMEOUT_SEC` | AI settings | `claude-sonnet-4-6`, `2000`, `30` |
| `METRICS_ENABLED` | Serve Prometheus `/metrics` (restrict it at the network layer) | `false` |
| `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_INSECURE`, `OTEL_TRACES_SAMPLER_ARG` | OpenTelemetry tracing; off when the endpoint is empty | —, `false`, `1.0` |
| `SENTRY_DSN` | Sentry error monitoring; off when empty | — |

Never paste Socrate's generated env template: its names differ (`SOCRATE_ADMIN_BASE_URL`) and
its empty secret would erase the real one. Compare names with this table.

---

## API overview

Public: `GET /healthz`, `GET /readyz`, `GET /api/version`, and the sign-in routes
`POST /auth/callback`, `/auth/refresh`, `/auth/logout` (rate-limited per IP). Everything under
`/api/v1` needs a Socrate access token (`Authorization: Bearer …`).

| Area | Routes (under `/api/v1`) |
|---|---|
| Self | `GET /me`, `PATCH /me/locale`, `GET /me/schedule`, `GET /me/schedule.ics`, `POST /me/sessions/revoke` |
| Invitations | `POST /claim/{token}` |
| Reference data | `GET /options`, `GET /public-holidays` |
| Store | `GET /stores/me`, `PUT /stores/me` |
| Employees | `/stores/{storeId}/employees[/{employeeId}]`, with week templates, availability, qualifications, schedule and leave per employee |
| Schedule | `/stores/{storeId}/schedule`, `/shifts`, `/assignments`, `/shifts/generate`, `/shifts/regenerate`, `/schedule/publish` |
| Plans | `/stores/{storeId}/plans[/{planId}]` with `publish`, `override`, `history`, `rollback` |
| Rules, coverage | `/stores/{storeId}/rules`, `/coverage`, `/coverage/gaps`, `/coverage-requirements` |
| Leave, swaps | `/stores/{storeId}/leave-requests`, `/swap-requests` (with review and impact) |
| Templates, exceptions | `/stores/{storeId}/templates`, `/exceptions`, `/qualifications`, `/models/{scheme}/metrics` |
| AI | `/stores/{storeId}/ai/suggest-assignment`, `/ai/optimize`, `/ai/insights` |
| Manager | `/manager/employees` |
| Platform admin | `/admin/dashboard`, `/admin/stores`, `/admin/managers`, `/admin/employees`, `/admin/audit-logs`, `/admin/metadata` |

The router, `internal/router/router.go`, is authoritative, including the permission each route
requires (`schedule:manage`, `employees:manage`, `store:manage`, `leave:manage`, `swap:manage`,
`platform:admin`, …).

---

## Rule engine

`ScheduleService` calls `RuleEngine.EvaluateAssignment` before saving an assignment. The engine
loads the store's enabled rules, pre-loads the employee's assignments within ±14 days in one
query, and runs each rule's `RuleEvaluator`:

| Type | Configuration | Checks |
|---|---|---|
| `ROLE` | `required_role` (optional) | The employee's role fits the shift |
| `NO_OVERLAP` | — | No two overlapping shifts for one employee |
| `MAX_HOURS` | `max_weekly_hours` | Hours per ISO week |
| `MIN_REST` | `min_rest_hours` | Rest between consecutive shifts |

`BLOCKING` rejects the assignment (409, with the violation's message and i18n key); `WARNING` and `INFO` are returned
with the saved assignment. A new rule type is one `RuleEvaluator` (`Code()`, `Evaluate()`)
registered in the registry. On an infrastructure error the engine fails open and logs a warning.

---

## AI engine

- **Suggestions** (`SuggestAssignment`): the top three employees for a shift, with a confidence
  score and a reason, cached for ten minutes per shift. Without the AI provider, a heuristic
  scores role match (50 points) and hours balance against the contract (50 points).
- **Optimisation** (`OptimizeSchedule`): proposed reassignments for a date range.
- **Insights** (`GenerateInsights`): coverage, rest and fairness observations saved as
  dismissible records; meant to run from an external scheduler.

The AI never writes to the schedule, and rule enforcement does not depend on it.

---

## Testing

```bash
go build ./... && go vet ./...
go test -race ./...              # unit, handler and router end-to-end tests; no database needed
golangci-lint run ./...          # v2.14.0
```

CI (`.github/workflows/ci.yml`) runs lint, build, vet and race tests with coverage, and on pull
requests checks that every added or changed Go file is gofmt-formatted. The local gate is in
[`CLAUDE.md`](CLAUDE.md).

---

## Deployment

The API runs as a systemd service (`parashift`) on the apps VPS behind Caddy, listening on
`127.0.0.1:$PORT`; its environment is `/opt/apps/parashift/env/.env`.

```bash
scripts/push.sh v0.3.0           # build the binary, upload it and the migrations
# then on the VPS:
sudo /opt/apps/parashift/deploy-backend.sh v0.3.0
```

`deploy-backend.sh` stops the service, installs the release, runs the migrations, switches the
`current` link, restarts, and checks `http://127.0.0.1:$PORT/healthz`; it rolls back when the
check fails. `push.sh` reads the VPS address from `~/.config/parashift/deploy.env` (or the file
named by `PARASHIFT_DEPLOY_ENV`), never from the repository:

```bash
SSH_USER=deploy
SSH_HOST=vps.example.com
SSH_PORT=22
```

Caddy proxies `/api/*` and `/auth/*` to `127.0.0.1:$PORT` (not `localhost`, which may resolve to
`::1`) and must not set `trusted_proxies`. A release is an annotated `vX.Y.Z` tag on `main` with
its `CHANGELOG.md` section; pushing the tag publishes a GitHub Release with Linux binaries
(`.github/workflows/release.yml`). Back up the database before a release with a migration, and
deploy the API before the web app when the API changes.

---

## Security

Please report vulnerabilities privately; see [`SECURITY.md`](SECURITY.md). The identity-provider
audit, its findings and their status are in
[`docs/SOCRATE-COMPAT-REPORT.md`](docs/SOCRATE-COMPAT-REPORT.md).

- Tokens: RS256 against Socrate's JWKS, with issuer, audience (`SOCRATE_CLIENT_ID`), expiry and a
  revocation floor per subject (`token_revocations`), so "sign out everywhere" and admin revocation
  take effect before tokens expire.
- Roles: platform admin only from `app_roles[SOCRATE_CLIENT_ID]`; store roles from the database.
- Tenancy: the store comes from the employee record; store-scoped routes check the URL against it.
- Every Socrate call goes through `backendkit`; secrets come from the environment only.
- Errors: internal and upstream details are logged, never returned; every error carries an i18n
  key for the web app.
- Known gap, being closed: `/auth/callback` and `/auth/refresh` return tokens to the browser until
  sign-in moves to the Backend-for-Frontend.

---

## Status

Deployed at `https://parashift.vandermoten.eu`. In progress: sign-in through a
Backend-for-Frontend (no token in the browser), then removal of the `/auth` token routes. Known
limits: one PostgreSQL primary, no outbound webhooks yet, AI insights need an external scheduler,
and AI quality improves with a few weeks of history.

---

## Contributing

Contributions are welcome; see [`CONTRIBUTING.md`](CONTRIBUTING.md). Every change goes through a
pull request with green CI and a `CHANGELOG.md` line.

---

## License

[GNU Affero General Public License v3.0](LICENSE) (AGPL-3.0-only). If you run a modified version
as a network service, you must offer its users the source of your version.
