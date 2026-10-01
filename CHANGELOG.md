# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [3.0.0] - 2026-10-01

Sign-in moves to Socrate through a Backend-for-Frontend. **Upgrading from v2.x** needs `.env`
changes before the release starts (the server refuses to start otherwise): `ENV=production`
replaces `APP_ENV`; `SOCRATE_APP_ID`, `SOCRATE_ADMIN_URL` and `BFF_REDIRECT_URL` are required;
the server listens on `127.0.0.1:$PORT`, so Caddy must proxy to `127.0.0.1`, not `localhost`;
Caddy sends `/api/*`, `/bff/*` and `/auth/*` on the app's own host to the API. Migrations
000033 to 000036 run on deploy. Deploy before web app v3.0.0.

### Fixed

- An API error without an i18n key no longer panics (and answers 500) in production: the
  development guard read `APP_ENV`, which nothing sets, instead of `ENV`.
- `scripts/deploy-backend.sh` health-checks `http://127.0.0.1:$PORT/healthz` (PORT from the
  env file, default 4000) instead of `localhost:8081/health`, which made every deploy roll back,
  and reads the deployed version from `/api/version` instead of starting a second server.
- Dockerfile: Go 1.26.4 (was 1.22, which cannot build the module), version ldflags, port 4000.

### Changed

- Go modules updated to their latest versions: gorm 1.31.2 (from 1.25.12) with
  driver/postgres 1.6.3 and datatypes 1.2.7, chi 5.3.2, sentry-go 0.49.0, golang-migrate
  4.20.1, logrus 1.10.2, lib/pq 1.12.3 and the rest of `go.sum`. golang.org/x/time now needs
  Go 1.26, so `go.mod`'s language version is 1.26 (the toolchain stays 1.27.1).

- Toolchain: Go 1.27.1, pinned by a `toolchain` line in `go.mod` and used by CI, the Dockerfile
  and the release workflow (CI fails if they drift); golangci-lint v2.14.0 (from v2.5.0), built
  with that toolchain. CI now also runs `govulncheck` and checks that `go.mod`/`go.sum` are tidy.
- GitHub Actions on Node 24: `actions/checkout` v5, `actions/setup-go` v6.

- `scripts/push.sh` reads the VPS address (`SSH_USER`, `SSH_HOST`, `SSH_PORT`) from the
  environment or `~/.config/parashift/deploy.env` instead of the repository.

- backendkit v1.15.1 (from v1.8.0). Every call to Socrate goes through `socrate.Client`: the
  `/auth` code exchange, refresh and revocation, and the profile read for auto-link. The app ID
  is `SOCRATE_APP_ID` only (no longer decoded from a service token), and the start-up admin
  probe is replaced by a log line.

### Security

- Four vulnerabilities reachable from Parashift's code, found by the new `govulncheck` job:
  pgx 5.11.0 (SQL injection via placeholder confusion, GO-2026-5004), grpc 1.84.0 (HTTP/2 memory
  exhaustion, GO-2026-6348), OpenTelemetry 1.46.0 (OTLP response memory exhaustion,
  GO-2026-4985; SDK PATH hijacking, GO-2026-4394). The tracer resource now uses semconv
  v1.43.0, the SDK's schema: with the old v1.26.0 the merge fails and traces would lose
  `service.name`.

- The role comes from the token's `app_roles[SOCRATE_CLIENT_ID]`, never its top-level `role`:
  a Socrate global admin is no longer a Parashift platform admin unless Socrate makes them an
  admin of Parashift's app (report S4).
- Production configuration is checked at start-up: `ENV` must be set explicitly;
  `SOCRATE_APP_ID` and `SOCRATE_ADMIN_URL` are required; Socrate URLs are absolute, without a
  trailing slash, https where public; `SOCRATE_ISSUER` (now optional) must equal
  `SOCRATE_BASE_URL`. The server listens on `127.0.0.1` in production (`BIND_ADDR`), and
  `/api/v1/debug/token` is served only with `ENV=development` (report K2–K4, S6, S7).
- The per-IP rate limits on `/auth/*` and `/claim` key on one address resolved by the server
  (`X-Forwarded-For` trusted from a loopback peer only, rightmost entry), so a browser can no
  longer pick its own bucket; the same address is sent to Socrate on the calls made on a
  user's behalf (client attribution, report S5).
- Auto-link by e-mail on first sign-in only uses an address Socrate has verified (report S8).

### Added

- Backend-for-Frontend sign-in, additive: `/bff/login`, `/bff/callback`, `/bff/session` and
  `/bff/logout` run the authorization-code flow with PKCE on the server and keep the tokens in
  an in-memory session; the browser gets an HttpOnly `__Host-parashift_session` cookie and a
  CSRF token. `/api/v1` takes the session (CSRF on unsafe methods, one refresh per session) or,
  during the transition, a bearer. New settings `BFF_REDIRECT_URL` (required in production),
  `BFF_COOKIE_NAME`, `BFF_SESSION_IDLE_TTL`, `BFF_SESSION_ABSOLUTE_TTL`,
  `BFF_INSECURE_COOKIE`. The invite claim checks the session's verified e-mail (report S9).

- Repository kit for the public release: README rewritten (badges, an environment table checked
  against the code, accurate architecture, API, security and deployment sections), AGPL-3.0
  licence, `CONTRIBUTING.md`, `SECURITY.md`, `CODEOWNERS`, pull-request and issue templates,
  and a release workflow that publishes Linux binaries and the changelog section for a
  `vX.Y.Z` tag. The notes from the first build moved to `docs/history/`.

- `CLAUDE.md` (sources of truth, hard rules, local gate, git workflow) and this changelog.
- CI fails a pull request that adds or changes a Go file that is not gofmt-formatted.
