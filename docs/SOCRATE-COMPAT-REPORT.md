# Socrate compatibility report: Parashift

**Date:** 2026-10-01. Read-only audit: apart from the placeholders in `.env.example` (row S1), no
code was changed in the PR that adds this report.

**Scope:** the two repositories that make up Parashift, audited together because sign-in is split
between them.

- `ovander/parashift-backend`, `main` at `43026bb`: Go API, confidential OAuth client,
  `github.com/ovander/backendkit` **v1.8.0**.
- `ovander/parashift-frontend`, `main` at `8591cad`: Vue 3 SPA; starts the login and holds the
  tokens.

Paths are prefixed `backend/` or `frontend/`. `backendkit/` is the module source at the version
named in the row. `socrate/` is the Socrate source, `ovander/go-oauth2` `main` at `cb72be4`
(v1.6.0); **the version running on `socrate.vandermoten.eu` was not checked** (see U1).

**Target:** issuer `https://socrate.vandermoten.eu`, sign-in through a Backend-for-Frontend (BFF)
so that no OAuth token reaches the browser, following Ascenda's move
(`ovander/ascenda-backend`, `docs/SOCRATE-MIGRATION-2026-10-01.md`, "lesson N" below).

**Discovery document:** not fetched. This environment's egress proxy refused both
`https://socrate.vandermoten.eu` and `https://parashift.vandermoten.eu` (`CONNECT tunnel failed,
response 403`). The contract below comes from the Socrate source and Ascenda's records.

---

## 1. Verdict

**Not ready: 3 blockers, 9 high, 10 medium, 6 low, 3 unknowns.**

The login is Authorization Code with S256 PKCE, the code is exchanged by the confidential backend,
and access tokens are verified against the JWKS with an issuer and an audience check
(`backend/cmd/server/bootstrap.go:136-138`). That part fits Socrate. What does not:

1. **A client secret is committed** (S1). It must be rotated at Socrate whatever else happens.
2. **Tokens live in the browser's `localStorage`** (S2) and the backend hands them out (S3).
3. **Any Socrate global admin is a Parashift platform admin** (S4): roles come from the token's
   top-level `role`, which Socrate sets to `admin` for a global admin on every app.

The rest is the Ascenda checklist: every Socrate call through backendkit, `SOCRATE_APP_ID` and
`SOCRATE_ADMIN_URL` required, client attribution, then the BFF.

---

## 2. Integration inventory

| Aspect | What the code does | Evidence |
|---|---|---|
| Current IdP | `golfperformance.fr` in the committed example; the live backend values are in `/opt/apps/parashift/env/.env` on the VPS (not read). The SPA's values come from a git-ignored `.env.production` that is not in the repository. | `backend/.env.example:25-36`; `backend/scripts/deploy-backend.sh:17`; `frontend/.gitignore:2-4`; `frontend/scripts/push.sh:51` |
| Client | One confidential client (ID + secret). The SPA builds the authorize URL with `VITE_AUTH_CLIENT_ID`; the backend does every token call with the secret. | `frontend/src/composables/useAuth.ts:34-42`; `backend/internal/handler/auth_handler.go:142-160` |
| Interactive flow | SPA: 128-char verifier, 32-char `state` (base-36), both in `sessionStorage`, redirect to `${VITE_AUTH_BASE_URL}/oauth/authorize`, scope `openid email profile api`. `/callback` checks `state` and posts `{code, codeVerifier, redirectUri}` to `POST /auth/callback`. | `frontend/src/composables/useAuth.ts:26-54`; `frontend/src/views/CallbackView.vue:22-30`; `frontend/src/stores/auth.ts:79-94` |
| Code exchange, refresh, revoke | Hand-written `PostForm` to `/oauth/token` and `/oauth/revoke` (`client_secret_post`). Tokens returned to the browser. | `backend/internal/handler/auth_handler.go:72-194` |
| Token storage | Access and refresh tokens and the user in `localStorage` (`auth.accessToken`, `auth.refreshToken`, `auth.user`). | `frontend/src/stores/auth.ts:10-73` |
| API calls | `Authorization: Bearer` added by an axios interceptor; on 401 the SPA refreshes once (single-flight) and retries; on failure it logs out and goes to `/login`. Base URL `VITE_API_BASE_URL` (cross-origin). | `frontend/src/composables/useApi.ts:6-99` |
| Token validation | `jwtauth.New(SOCRATE_JWKS_URL, SOCRATE_ISSUER, WithAudience(SOCRATE_CLIENT_ID), WithRevocationCheck(…))` (backendkit v1.8.0). | `backend/cmd/server/bootstrap.go:136-138` |
| Identity mapping | Employee by `auth_id = sub`; otherwise auto-link by e-mail from a hand-written `/oauth/userinfo` call; otherwise 404. A token `role == "admin"` skips the lookup entirely. | `backend/internal/middleware/tenant.go:39-145` |
| Invitations | `InviteUserAsService` (`POST /api/apps/{id}/service/users`) on employee create and resend; `SendMagicLink` (`POST /api/apps/{id}/service/magic-link`) on resend for an employee already in Socrate; otherwise a claim token and `/claim/:token`. | `backend/internal/service/employee_service.go:163-189,476-530`; `frontend/src/views/ClaimView.vue` |
| App ID | `SOCRATE_APP_ID`, or derived by decoding the service token's `sub` (`app:<id>`) by hand, twice. | `backend/internal/service/service_bundle.go:89-111,187-246`; `backend/cmd/server/bootstrap.go:373-405` |
| Start-up checks | GET of the JWKS (logs the keys), then a hand-written `client_credentials` exchange and `GET /api/apps/{id}/service/users?per_page=1`. | `backend/cmd/server/bootstrap.go:266-450` |
| Not used | Introspection, token exchange, DPoP, PEP, implicit or password grant, HS256. | code search of both repositories |

---

## 3. Socrate contract (from the source and Ascenda)

| # | Fact | Source |
|---|---|---|
| C1 | Issuer and OAuth base `https://socrate.vandermoten.eu`, no trailing slash; JWKS at `/.well-known/jwks.json`. | Ascenda migration §1 |
| C2 | Access tokens: `aud[0]` = client ID; `app_roles` maps every client ID to the user's role there. The top-level `role` is the role for the client the token was issued to, **except that a global admin or superadmin with no membership row gets `admin` on every app**. | `socrate/internal/service/oauth_service.go:394-400`; `socrate/internal/shared/auth/token.go:164-171` |
| C3 | Userinfo returns `sub`, `email`, `email_verified` (the user's `is_verified`), `name`, `role`, `app_roles`. Access tokens carry no e-mail. | `socrate/internal/service/oauth_service.go:1217-1242` |
| C4 | A service account may call `/api/apps/{id}/service/*` only; the app ID cannot be looked up by the service account (`GET /api/admin/apps` is for human global admins). Set `SOCRATE_APP_ID`. | Ascenda `docs/SOCRATE-APP-ID-2026-09-30.md` (lessons 4, 5) |
| C5 | The admin API listens on Socrate's loopback; from the apps VPS it is `http://127.0.0.1:18082` (SSH tunnel). backendkit's default (`BaseURL` with port 8081) is wrong behind TLS. | lesson 7; `backendkit/socrate/client.go:22-26` |
| C6 | Magic links: an app without a `magic_link_url` gets **409** from `POST …/service/magic-link` and no e-mail is sent. The URL must be https and share an origin with one of the app's redirect URIs; Socrate appends `token` and `client_id`. | `socrate/internal/service/magic_link_url.go:13-60`; `socrate/internal/handler/magic_link_handler.go:65-69` |
| C7 | Socrate rate-limits and audits by client address and trusts `X-Forwarded-For` from the apps VPS; backendkit ≥ v1.15.0 sends one address the app resolved (`WithClientAttribution`). | lesson 8; `backendkit/socrate/attribution.go` |
| C8 | backendkit ≥ v1.15.1 is needed for `RegisterUser` and `GetUserAsService` on the service routes. Parashift uses neither today. | `backendkit` commit `6227e76` |

---

## 4. Findings

Ranked most severe first. *Fix* names the PR in the plan (§6) that closes the row.

### Security

| # | Severity | Location | Issue | Fix |
|---|---|---|---|---|
| S1 | **BLOCKER** | `backend/.env.example:27-28` (since the first commit, `6651ac6`) | A real-looking `SOCRATE_CLIENT_ID` (`5Fev…`) and **`SOCRATE_CLIENT_SECRET` (`As6N…`)** are committed. The repository is private, but the secret is in history for anyone with read access, in every clone and fork. | **Owner: rotate the secret at Socrate now** and put the new one only in the VPS env file. PR 0 replaces both values with placeholders. Rewriting history is the owner's decision; rotation makes it unnecessary for safety. |
| S2 | **BLOCKER** | `frontend/src/stores/auth.ts:10-73`; `frontend/src/composables/useApi.ts:21-27` | Access **and refresh** tokens are kept in `localStorage` and sent as `Authorization` headers. Any XSS reads a long-lived refresh token. `index.html` has no Content-Security-Policy. | BFF (PRs 7–9): HttpOnly `__Host-parashift_session` cookie, CSRF token in memory, CSP `connect-src 'self'`, `noBrowserTokens` test. |
| S3 | HIGH | `backend/internal/handler/auth_handler.go:72-126`; routes `backend/internal/router/router.go:70-72` | `POST /auth/callback` and `/auth/refresh` return tokens to the browser. `/auth/callback` takes `redirectUri` from the request body (`:83-86`) instead of the configured value. | PR 7 adds `/bff/*`; PR 9 removes `/auth/*`, with a test that no route returns a token. |
| S4 | **BLOCKER** | `backend/internal/middleware/tenant.go:77-82`; `backend/internal/handler/me_handler.go:53-71`; `backend/internal/middleware/rbac.go:27-32` | The role is the token's top-level `role` (set by `jwtauth`). Socrate gives a global admin `role: admin` on every app (C2), and `admin` here skips the employee lookup and grants `PermPlatformAdmin`: every Socrate operator is a cross-tenant Parashift admin without being a member of the app. (Lesson 2.) | PR 3: role = `app_roles[SOCRATE_CLIENT_ID]` (Ascenda `internal/middleware/auth.go`), audience check kept. |
| S5 | HIGH | `backend/internal/router/authlimiter.go:86-100` | The rate-limit key is the **leftmost** `X-Forwarded-For` entry, which the browser writes: the per-IP limits on `/auth/*` and `/claim` are bypassed by sending a different header each time. | PR 5: one resolver, `X-Forwarded-For` trusted from a loopback peer only, rightmost non-loopback entry (Ascenda `AttributionIP`). |
| S6 | HIGH | `backend/cmd/server/bootstrap.go:171` | The server listens on `:PORT`, every interface: if the host firewall lets the port through, Caddy (TLS, headers) is bypassed and, with S5 fixed, `X-Forwarded-For` would be spoofable from a non-loopback peer that is not Caddy. | PR 4: bind `127.0.0.1` in production. |
| S7 | HIGH | `backend/internal/config/config.go:78` with `backend/internal/router/router.go:78-80` | `ENV` defaults to `development`. If it is missing from the VPS env file, every production check in `Validate` is skipped **and** `GET /api/v1/debug/token` is served (it decodes any JWT without verifying it; harmless data-wise, but it is a dev tool on a public host). | PR 4: register the debug route only when `ENV=development` explicitly, and fail start-up when `ENV` is unset outside a test. Owner: confirm `ENV=production` on the VPS. |
| S8 | MEDIUM | `backend/internal/middleware/tenant.go:98-128` | Auto-link binds the first unprovisioned `sub` to the employee with the same e-mail, without checking `email_verified`. A Socrate account that registered an employee's address without verifying it takes over that employee. | PR 2 (through `socrate.Client`) needs `email_verified`, which `backendkit/socrate.ProfileInfo` does not expose (`client.go:380-389`): **backendkit change B-K1**. Until then, PR 2 disables auto-link rather than keep it unverified. |
| S9 | MEDIUM | `backend/internal/service/employee_service.go:253-257`; `backend/internal/handler/claim_handler.go:44-46` | The invite claim checks the caller's e-mail only when the token carries one; Socrate access tokens never do (C3), so anyone signed in to Parashift with the invite link claims it. | PR 7: the BFF session holds the userinfo e-mail; pass it (verified) to `ClaimByToken`. |
| S10 | LOW | `backend/internal/middleware/cors.go:11-37` | CORS allows credentials for `ALLOWED_ORIGINS`. After the BFF every call is same-origin and the list can shrink to the site itself. | Runbook (PR 10): trim `ALLOWED_ORIGINS`. |

### Contract

| # | Severity | Location | Issue | Fix |
|---|---|---|---|---|
| K1 | HIGH | `backend/internal/handler/auth_handler.go:140-194`; `backend/internal/middleware/tenant.go:36-71`; `backend/internal/service/service_bundle.go:187-246`; `backend/cmd/server/bootstrap.go:331-450` | Token, revoke, userinfo, `client_credentials` and an admin probe are hand-written HTTP; the service token's JWT is decoded by hand in two places. None of it sends client attribution or bounds the response size. | PR 2: backendkit v1.15.1; `socrate.Client` for every call; the start-up check becomes one documented reachability GET. |
| K2 | HIGH | `backend/internal/service/service_bundle.go:93-101`; `backend/cmd/server/bootstrap.go:376-405` | `SOCRATE_APP_ID` is optional; when it is set and differs from the token's `sub`, the token wins silently (`:401`). (Lesson 4.) | PR 4: required in production; derivation removed. |
| K3 | HIGH | `backend/internal/config/config.go:124-178` | `SOCRATE_ADMIN_URL` is not validated; empty, backendkit derives `https://socrate.vandermoten.eu:8081`, which is wrong (C5). (Lesson 7.) | PR 4: required in production, never derived. |
| K4 | MEDIUM | `backend/internal/config/config.go:103,143-146` | The issuer is a separate `SOCRATE_ISSUER`; a trailing slash on `SOCRATE_BASE_URL` or a mismatch between the two is not caught. | PR 4: issuer derived from `SOCRATE_BASE_URL`, no trailing slash (or both checked equal). |
| K5 | MEDIUM | `backend/internal/service/employee_service.go:520-527` | "Resend invite" for an employee already in Socrate calls `SendMagicLink`. Socrate answers 409 until the app has a `magic_link_url` (C6), so the resend fails with a 500; and Parashift has no page to land on. | Decide (owner): add magic-link sign-in (PR 7 `/bff/magic-link/verify` + SPA `/magic-link` page, Socrate URL `https://parashift.vandermoten.eu/magic-link`, lesson 6), or resend a claim link instead. |
| K6 | LOW | `backend/cmd/server/bootstrap.go:417` | The admin probe calls `GET /api/apps/{id}/service/users?per_page=1`, a route Socrate does not list for service accounts (C4); its result is always a warning. | PR 2. |
| K7 | LOW | `backend/internal/config/config.go:53,94`; `auth_handler.go:85` | `SOCRATE_REDIRECT_URL` is only a fallback for `/auth/callback`. | PR 7 replaces it with `BFF_REDIRECT_URL` (required in production). |

### Tooling and deploy

| # | Severity | Location | Issue | Fix |
|---|---|---|---|---|
| D1 | HIGH | `backend/scripts/deploy-backend.sh:25,126-137` | The health check calls `http://localhost:8081/health`; the route is `/healthz` (`router.go:60`) on `PORT` (default 4000). Every deploy fails its check and rolls back. | PR 6. |
| D2 | MEDIUM | `backend/scripts/deploy-backend.sh:144-149` | `app version` is not a subcommand (`cmd/server/main.go:52`): it starts a second server, which fails on the port. | PR 6: use `/api/version`. |
| D3 | MEDIUM | `backend/Dockerfile:1,25`; `backend/docker-compose.yml:20-21` | `golang:1.22` cannot build a `go 1.25` module; `EXPOSE 8080` and the compose port do not match `PORT` 4000. | PR 6. |
| D4 | MEDIUM | `frontend/scripts/push.sh`, `frontend/scripts/deploy-frontend.sh` | Two conflicting deploy models: `push.sh` rsyncs into `/opt/apps/parashift/frontend/` directly; `deploy-frontend.sh` expects an upload in `/tmp/parashift-frontend` and turns that directory into a symlink. `push.sh` also builds from a `.env.production` that is git-ignored and absent. | PR 8: one script; after the BFF the build needs no Socrate value (lesson 14). |
| D5 | MEDIUM | both repositories | No `CLAUDE.md`, `CONTRIBUTING.md`, `CHANGELOG.md` or tag. Frontend: no CI, no `.nvmrc`, `eslint` and its Vue parser not installed (`npm run lint:i18n` fails on every `.vue` file). Backend CI does not check `gofmt` (57 files are unformatted today) or `govulncheck`. | PR 1 (one per repo). |
| D6 | LOW | `backend/README.md:235-260` | The env block uses placeholder hosts, `SOCRATE_APP_ID=parashift`, a `/auth/callback` redirect and `JWKS_ISSUER` (the code reads `SOCRATE_ISSUER`). | PR 4 (table) and PR 10 (runbook). |
| D7 | LOW | `backend/internal/router/router.go:62` | The version route is `/api/version`, not `/api/v1/version` as in Ascenda. | Runbook uses `/api/version`. |

### Unknowns

| # | Question | Who |
|---|---|---|
| U1 | Which Socrate version runs on `socrate.vandermoten.eu`? This report reads `go-oauth2` v1.6.0 (`main`). C6 (magic-link URL) needs ≥ the release that added `magic_link_url`. | Owner / Socrate team |
| U2 | Parashift's numeric **app ID** on the new Socrate, and the **full client ID** (`.env.example` had `5Fev…`). | Owner / Socrate team |
| U3 | Were Socrate user rows carried over with their IDs (so `employees.auth_id` = `sub` still matches)? If not, every employee re-links: by e-mail (S8, needs B-K1) or by a new claim link. | Owner / Socrate team |

### backendkit changes proposed

| # | Change | Why |
|---|---|---|
| B-K1 | Add `EmailVerified bool \`json:"email_verified"\`` to `socrate.ProfileInfo` (`backendkit/socrate/client.go:380`). Socrate already returns it (C3). | Auto-link by e-mail (S8) and the claim e-mail check (S9) must use a verified address. |

---

## 5. Status

| Row | 2026-10-01 |
|---|---|
| S1 | Placeholders merged (#55). The secret now set on the VPS differs from the committed one; **owner: confirm the old one (`As6N…`) is revoked at Socrate** |
| S8 | Fixed in PR 2: auto-link reads the profile through `socrate.Client.GetProfile` and links only when Socrate has verified the address |
| S4 | Fixed in PR 3: the role is `app_roles[SOCRATE_CLIENT_ID]` (`middleware.AppRole`), `user` without one; the audience check stays on |
| S2, S3, S5–S7, S9, S10 | open |
| K1, K6 | Fixed in PR 2: backendkit v1.15.1; every Socrate call through `socrate.Client`; the admin probe and the JWT decoding are gone; the JWKS GET is the one documented start-up check |
| K2–K5, K7 | open |
| D1–D4, D6, D7 | open |
| D5 | Fixed in #56 and ovander/parashift-frontend#4, #5 (`govulncheck` still not in CI) |
| U1, U3 | asked |
| U2 | Answered: app ID **7**, client ID `5Fev…` (the one that was in `.env.example`) |
| B-K1 | Withdrawn: `socrate.Client.GetProfile` (`GET /api/profile`, user token) already returns `is_verified` |

Each fix PR updates its row with the date and the PR.

---

## 6. Plan (one PR per line)

| PR | Repo | Change | Rows |
|---|---|---|---|
| 0 | backend | This report; placeholders in `.env.example` | S1 |
| 1 | both | `CLAUDE.md`, `CHANGELOG.md`, CI running the local gate; frontend `.nvmrc` 24 and `eslint` | D5 |
| 2 | backend | backendkit v1.15.1; every Socrate call through `socrate.Client` | K1, K6, S8 |
| 3 | backend | Role from `app_roles[client_id]`, audience check kept | S4 |
| 4 | backend | `SOCRATE_APP_ID`, `SOCRATE_ADMIN_URL` required in production; no trailing slash; bind `127.0.0.1`; `ENV` explicit | K2–K4, S6, S7, D6 |
| 5 | backend | Client attribution and rate-limit key from one loopback-trusted resolver | S5 |
| 6 | backend | Deploy health check, Dockerfile | D1–D3 |
| 7 | backend | BFF A, additive: `/bff/*`, session middleware, CSRF | S3 (part), S9, K5, K7 |
| 8 | frontend | BFF B: SPA on `/bff`, no token code, CSP, `noBrowserTokens`, e2e with fake BFF | S2, D4 |
| 9 | backend | BFF C: remove `/auth/*` and bearer-only `/api/v1` | S3 |
| 10 | backend | Owner's cut-over runbook in the README | S10, D6, D7 |
