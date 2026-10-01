# CLAUDE.md — parashift-backend

Standing instructions for Claude Code in this repository. Read this file before any change. The
SPA lives in `ovander/parashift-frontend`; many changes touch both.

## Project in one paragraph

Parashift plans shifts for pharmacies: stores, employees and contracts, week templates, shift
generation under scheduling rules, coverage, leave and swap requests, and AI suggestions. This
repository is the Go API: chi router, GORM on PostgreSQL 16, SQL migrations with golang-migrate,
authentication by the Socrate OAuth 2.1 / OIDC provider (`https://socrate.vandermoten.eu`) through
`github.com/ovander/backendkit`. It is multi-tenant: a store is a tenant, and every business row
belongs to one.

## Sources of truth, in order

1. The code. Read it before proposing changes; do not describe code you have not opened. The
   router (`internal/router/router.go`) is authoritative for routes.
2. `docs/SOCRATE-COMPAT-REPORT.md` (findings, dated status table, PR plan) for the
   identity-provider contract and known issues.
3. Ascenda's retrospective (`ovander/ascenda-backend`, `docs/SOCRATE-MIGRATION-2026-10-01.md`):
   the lessons of the same move. Apply them; do not rediscover them.

## Hard rules

- **Layering.** `handler` → `service` → `repo`; handlers do not call repositories.
- **Tenant scoping.** Every repository call takes the tenant from the request context
  (`ctxutil.GetTenantID`), never from the request body. Tenant-scoped route groups use
  `httpware.RequireTenant`; the cross-tenant surface is `/api/v1/admin`, gated by
  `PermPlatformAdmin`.
- **Roles.** Read the role from the context the middleware sets (`ctxutil.GetUserRole`); store
  roles come from the employee record (`TenantMiddleware`). Keep the audience check
  (`jwtauth.WithAudience(SOCRATE_CLIENT_ID)`). Never trust the token's top-level `role` claim for a
  Parashift role: Socrate serves several apps and gives its global admins `admin` on every app
  (`app_roles[SOCRATE_CLIENT_ID]` is Parashift's own; report row S4).
- **Socrate** is reached only through `backendkit` (`jwtauth` for tokens, `socrate.Client` for
  every call, `bff` for sessions). No hand-written requests to its OAuth or admin endpoints,
  apart from one documented start-up reachability check. If backendkit lacks something, propose
  the backendkit change; do not work around it here. Service-account calls use
  `/api/apps/{id}/service/*` only.
- **Tokens** never reach the browser once the BFF lands (report rows S2, S3); never add a route
  that returns one.
- **Migrations.** A schema change is a new numbered pair in `migrations/` (`make migrate-create`)
  with a working `down`. Never edit a released migration. `AUTO_MIGRATE` is development-only.
- **Never weaken a gate** to get green: no skipped or deleted tests, no `//nolint` or `t.Skip`
  without a one-line reason, no `continue-on-error`, no required check removed.
- **Secrets** never enter the repository: no `.env`, keys or tokens; `.env.example` holds
  placeholders only. When a value must be shown, show its first four characters.
- **Scope.** One change per PR; do not widen a PR with unrelated fixes (open a separate one).

## Local gate (the same checks as CI)

```bash
git diff --name-only --diff-filter=AM origin/main...HEAD -- '*.go' | xargs -r gofmt -l   # must print nothing
go build ./...
go vet ./...
go test -race -covermode=atomic -coverprofile=coverage.out ./...
golangci-lint run ./...        # v2.14.0, built with Go 1.27.1
govulncheck ./...              # no reachable vulnerability
```

Go 1.27.1 is pinned by the `toolchain` line in `go.mod`; CI, the Dockerfile and the release
workflow use it, and CI fails if they drift. Install the linter with the same toolchain:
`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0`.

Run it, **check the exit code of each step**, and push only when all pass; never chain a push
after a command that may fail. The whole tree is not gofmt-clean yet (57 files on 2026-10-01): new
and changed files must be.

## Git workflow

- Branch from `main`: `feat/…`, `fix/…`, `chore/…`, `ci/…`, `docs/…`. Conventional Commits.
- Open a PR; never push to `main`, never force-push a shared branch, never merge with red CI.
  The owner merges, tags and deploys.
- Each PR adds a line under `## [Unreleased]` in `CHANGELOG.md`, and says in its body what it
  changes, how it was tested, and any deploy note (migration, env variable, order with the
  frontend). Fixing a report row updates its line in the report's status table.

## Releases and deploys (the owner runs them)

- A release is an annotated tag `vX.Y.Z` on an updated `main`, with the `[Unreleased]` section
  moved under the new version. Tag only the merged release commit:
  `grep -q "^## \[X.Y.Z\]" CHANGELOG.md && git tag -a vX.Y.Z -m vX.Y.Z`. Pushing the tag runs
  `.github/workflows/release.yml` (Linux binaries; fails when the changelog section is missing).
- `scripts/push.sh <tag>` builds and uploads the binary and migrations to the apps VPS (SSH
  settings in `~/.config/parashift/deploy.env`, never in the repository); then
  `sudo /opt/apps/parashift/deploy-backend.sh <tag>` on the VPS migrates, switches and restarts.
  Back up the database before a release with a migration. Deploy the backend before the frontend
  when the API changes.

## Docs discipline

A new environment variable goes into the README table, `.env.example` (placeholder) and
`internal/config/config.go` validation when production needs it.
