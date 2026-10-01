# Contributing to the Parashift API

Thank you for your interest. Parashift is two repositories: this Go API and the Vue web app,
[`ovander/parashift-frontend`](https://github.com/ovander/parashift-frontend). Contributions are
accepted under the project's licence, [AGPL-3.0](LICENSE).

## Development setup

Requirements: Go 1.26 or later (the `toolchain` line in `go.mod` downloads 1.27.1, as CI uses), Docker for the local database. The tests need
neither a database nor a network service.

```bash
git clone https://github.com/ovander/parashift-backend && cd parashift-backend
cp .env.example .env     # ENV=development, DATABASE_URL, the Socrate values (see the README)
make db-up               # PostgreSQL 16 in Docker
make run                 # http://localhost:4000
```

## Design rules

The full list is in [`CLAUDE.md`](CLAUDE.md), which Claude Code follows too. In short:

- `handler` → `service` → `repo`; handlers never call repositories.
- Every repository call takes the tenant from the request context, never from the body.
- Roles come from the context the middleware sets; never from the token's top-level `role`.
- Socrate is reached only through `backendkit`; no hand-written OAuth or admin calls.
- A schema change is a new numbered migration pair in `migrations/` with a working `down`.

## Tests and checks

Run these before opening a pull request; CI runs the same:

```bash
git diff --name-only --diff-filter=AM origin/main...HEAD -- '*.go' | xargs -r gofmt -l   # prints nothing
go build ./... && go vet ./...
go test -race ./...
golangci-lint run ./...  # v2.14.0
```

- Tests sit next to the code (`*_test.go`); router-level tests are in `internal/e2e`.
- A bug fix comes with a test that fails without it.
- Never weaken a check to get green: no skipped test, no `//nolint` or `t.Skip` without a
  one-line reason.

## Pull requests

1. Branch from `main` (`feat/…`, `fix/…`, `chore/…`, `ci/…`, `docs/…`).
2. Commit with [Conventional Commits](https://www.conventionalcommits.org/).
3. Add a line under `## [Unreleased]` in [`CHANGELOG.md`](CHANGELOG.md); a new environment
   variable also goes into the README table and `.env.example`.
4. Open the PR with the template filled in, including deploy notes (migration, environment,
   order with the web app).
5. CI must be green. The maintainer reviews and merges.

## Releases

The maintainer tags `vX.Y.Z` on `main` after moving the `[Unreleased]` section under the new
version; the tag publishes a GitHub Release with those notes and Linux binaries
(`.github/workflows/release.yml`). Deploys are run by the maintainer.

## Security

Do not open a public issue for a vulnerability; see [`SECURITY.md`](SECURITY.md).
