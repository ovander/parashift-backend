# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Fixed

- An API error without an i18n key no longer panics (and answers 500) in production: the
  development guard read `APP_ENV`, which nothing sets, instead of `ENV`.
- `scripts/deploy-backend.sh` health-checks `http://127.0.0.1:$PORT/healthz` (PORT from the
  env file, default 4000) instead of `localhost:8081/health`, which made every deploy roll back,
  and reads the deployed version from `/api/version` instead of starting a second server.
- Dockerfile: Go 1.26.4 (was 1.22, which cannot build the module), version ldflags, port 4000.

### Changed

- `scripts/push.sh` reads the VPS address (`SSH_USER`, `SSH_HOST`, `SSH_PORT`) from the
  environment or `~/.config/parashift/deploy.env` instead of the repository.

- backendkit v1.15.1 (from v1.8.0). Every call to Socrate goes through `socrate.Client`: the
  `/auth` code exchange, refresh and revocation, and the profile read for auto-link. The app ID
  is `SOCRATE_APP_ID` only (no longer decoded from a service token), and the start-up admin
  probe is replaced by a log line.

### Security

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
