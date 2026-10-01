# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Fixed

- `scripts/deploy-backend.sh` health-checks `http://127.0.0.1:$PORT/healthz` (PORT from the
  env file, default 4000) instead of `localhost:8081/health`, which made every deploy roll back,
  and reads the deployed version from `/api/version` instead of starting a second server.
- Dockerfile: Go 1.26.4 (was 1.22, which cannot build the module), version ldflags, port 4000.

### Changed

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
- Auto-link by e-mail on first sign-in only uses an address Socrate has verified (report S8).

### Added

- `CLAUDE.md` (sources of truth, hard rules, local gate, git workflow) and this changelog.
- CI fails a pull request that adds or changes a Go file that is not gofmt-formatted.
