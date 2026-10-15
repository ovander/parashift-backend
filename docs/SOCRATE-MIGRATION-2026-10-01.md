# Moving Parashift to the new Socrate: what happened and what we learned

**Date:** 2026-10-01 (one day, after Ascenda's move). **Outcome:** sign-in works end to end on
`https://socrate.vandermoten.eu`, through the backend's Backend-for-Frontend (BFF); no OAuth token
reaches the browser. In production: API **v3.0.0** (`26b706e`, Go 1.27.1, backendkit v1.15.1),
web app **v3.0.0** (`5bd8a88`, Node 24.21.0, Vite 8.3.1, Vue 3.5.43, TypeScript 5.9.3).

This is the retrospective. It does not repeat Ascenda's
(`ovander/ascenda-backend`, `docs/SOCRATE-MIGRATION-2026-10-01.md`), whose lessons this move
applied from the start; it records what was different here. The detailed records stay where
they are:

- [`SOCRATE-COMPAT-REPORT.md`](SOCRATE-COMPAT-REPORT.md): the read-only audit against Socrate
  (`go-oauth2` v1.6.0), its plan and its dated status table.
- `CHANGELOG.md` `## [3.0.0]`: what shipped, with the upgrade notes from v2.

## 1. Final shape

```
browser ── https://parashift.vandermoten.eu ── Caddy ─┬─ /api/* /bff/* /auth/* → API 127.0.0.1:8081
           (HttpOnly __Host-parashift_session cookie, │
            CSRF token in memory)                     └─ everything else → SPA (try_files → index.html)

           https://api.parashift.vandermoten.eu ───────── → API 127.0.0.1:8081   (old SPA only; to remove)

API ── OAuth (authorize, token, refresh, revoke, userinfo, JWKS) → https://socrate.vandermoten.eu
    └─ admin API (service-account calls) → http://127.0.0.1:18082
```

| Setting (API, `/opt/apps/parashift/env/.env`) | Value |
|---|---|
| `ENV` | `production` (required; replaces `APP_ENV`, which nothing read) |
| `PORT` | `8081` (the API binds `127.0.0.1`) |
| `SOCRATE_BASE_URL` | `https://socrate.vandermoten.eu` (also the issuer; no trailing slash) |
| `SOCRATE_ISSUER` | unset (defaults to `SOCRATE_BASE_URL`) |
| `SOCRATE_JWKS_URL` | `https://socrate.vandermoten.eu/.well-known/jwks.json` |
| `SOCRATE_ADMIN_URL` | `http://127.0.0.1:18082` (never derived) |
| `SOCRATE_CLIENT_ID` | `5FevumUD3pavFShhV3A8yA` |
| `SOCRATE_CLIENT_SECRET` | set on the VPS only (`QH2B…`) |
| `SOCRATE_APP_ID` | `7` |
| `BFF_REDIRECT_URL` | `https://parashift.vandermoten.eu/bff/callback` |
| `ALLOWED_ORIGINS` | `https://parashift.vandermoten.eu` |
| `SOCRATE_REDIRECT_URL` | old `/callback`; goes with PR 9 |

| Setting (Socrate, app 7) | Value |
|---|---|
| Redirect URIs | `https://parashift.vandermoten.eu/bff/callback`; the old `/callback` until PR 9 |
| Magic-link URL | none: Parashift does not use magic links (report row K5) |

The web app has no Socrate or API setting: it calls its own origin. Its only build variable left
is `VITE_FULLCALENDAR_LICENSE_KEY`.

## 2. The sequence

| Step | Backend | Web app |
|---|---|---|
| 1. Read-only audit, placeholders for the committed secret | #55 | |
| 2. Foundations: CLAUDE.md, changelog, CI (gofmt on changed files, lint, race tests) | #56 | #4, #5 |
| 3. Every Socrate call through backendkit v1.15.1 | #57 | |
| 4. Role from `app_roles[client_id]` | #58 | |
| 5. Production configuration: `ENV` required, app ID and admin URL required, bind `127.0.0.1` | #59 | |
| 6. Client attribution and rate-limit key from a loopback-trusted `X-Forwarded-For` | #60 | |
| 7. Deploy health check, Dockerfile | #61 | |
| 8. Repository kit for the public release (README, AGPL-3.0, templates, release workflow) | #62 | #6 |
| 9. `APP_ENV` guard fixed (panics in production) | #63 | |
| 10. BFF, additive: `/bff/*` next to `/auth/*` | #64 | |
| 11. Toolchain and dependencies: Go 1.27.1, four reachable vulnerabilities, tracing schema | #65, #66 | #7 |
| 12. SPA on the BFF; token code, `/callback` and `VITE_API_*`/`VITE_AUTH_*` removed | | #8 |
| 13. Release 3.0.0 | #67 | #9 |
| 14. Deploy script fixes found by the first real run | #68, #69 | |
| 15. Deploy: env, database backup, API, Caddy, web app | v3.0.0 | v3.0.0 |
| 16. Old browser token routes removed | PR 9 (to do) | |

## 3. Lessons

Each lesson is the symptom, the cause, and the rule we keep. Times are UTC.

### Before code

1. **Look at the server before planning the cut-over.** The plan assumed Caddy already sent
   `/api/*` and `/auth/*` on `parashift.vandermoten.eu` to the API. It did not: the old SPA called
   a separate host, `api.parashift.vandermoten.eu`, with CORS, and `/api/version` on the app's
   host returned the SPA's `index.html`. A BFF needs the API on the app's own origin, so the Caddy
   block had to change too. *Rule:* before the plan, paste the reverse-proxy block, the service
   unit (`systemctl show -p User -p WorkingDirectory`), the env variable names (values cut to
   four characters) and the running release.
2. **Know which release is running.** Tags v2.1.0 to v2.4.0 existed, but the VPS ran v2.0.0 from
   April: the first deploy carried five months of changes, migrations 000033 to 000036 included.
   *Rule:* read `/api/version` (and `readlink /opt/apps/<app>/current`) before choosing what to
   release.
3. **A committed secret is presumed live until the server says otherwise.** The report assumed the
   VPS secret differed from the committed `As6N…`; the env file showed it was the same one. Socrate
   had revoked it, so sign-in in production was already broken before we started. *Rule:* rotate
   at the provider and replace on the server in the same step; check the server's value by its
   first four characters.
4. **The env file can hold values from an older provider.** `SOCRATE_ISSUER` was
   `https://golfperformance.fr`. v3 refuses an issuer different from `SOCRATE_BASE_URL`; we checked
   Socrate's discovery document (`"issuer"`) before deleting the line rather than trusting either
   value. *Rule:* compare every provider URL in the env file with the provider's discovery
   document.

### Code

5. **A config name nobody sets is a silent default.** The code read `APP_ENV` in one place and
   `ENV` in another; the VPS set `APP_ENV=development`. Missing `ENV` meant development: no
   production checks, and the debug token route served (S7). The error writer's development guard
   read `APP_ENV`, so it panicked in production on any error without an i18n key (#63). *Rule:*
   one variable, required, no default outside tests.
6. **The top-level `role` claim is not Parashift's.** As in Ascenda, every Socrate global admin
   was a cross-tenant Parashift admin (S4). Here it also skipped the employee lookup. *Rule:* role
   from `app_roles[client_id]`; platform admins get the admin role on app 7 at Socrate.
7. **Linking by e-mail needs a verified e-mail.** Auto-link bound a Socrate account to the employee
   with the same address without checking `is_verified` (S8), and the invite claim checked an
   e-mail that access tokens never carry (S9). *Rule:* the verified e-mail comes from the profile
   or the BFF session, never from the token.
8. **Same-origin end-to-end tests change what "unmocked" means.** With an absolute API base, an
   unmocked call failed with a network error and views fell back to empty data. Once same-origin,
   it reached `vite preview`, which answers `index.html` with 200: 41 of 111 tests failed with
   errors like `holidays.value.find is not a function`. *Rule:* the catch-all API mock answers 404
   JSON; tests that add their own catch-all register the sign-in mocks after it.
9. **Updating the toolchain surfaces real issues.** Go 1.27.1 and `govulncheck` found four
   reachable vulnerabilities (pgx, grpc, otel ×2), and the OpenTelemetry SDK update broke
   `resource.Merge` (schema URL mismatch), which would have failed tracing start-up. *Rule:*
   `govulncheck` in CI; a unit test that builds the tracing resource.

### Deploy

10. **The deploy script had never run as written.** Its first real run found three bugs, one per
    attempt:
    - migrations ran as the account that called `sudo`, which cannot read the mode-600 env file
      (`DATABASE_URL is required …`), fixed in #68;
    - `PREVIOUS` was recorded after the migrations, so a failure there left the service stopped,
      fixed in #68;
    - the binary opens `file://migrations` relative to its working directory, the caller's home,
      fixed in #69 (`cd /opt/apps/parashift` first; the unit's `WorkingDirectory` is the same).

    *Rule:* run a new or changed deploy script against a sandbox copy of the layout with stubbed
    `sudo`, `systemctl` and `rsync` before the real deploy; migrations run as the service user
    from the service's working directory; every failure after the stop restarts a release.
11. **An env file edited for vN+1 can break vN, and with it the rollback.** We deleted
    `SOCRATE_ISSUER` for v3; v2.0.0 required it. Each rollback restarted v2 into a crash loop
    (`restart counter is at 113`), so the API was down from the first attempt (09:56) until v3 ran
    (10:06). *Rule:* keep the env file valid for both releases until the new one is confirmed, or
    restore the backed-up env file in the rollback.
12. **Binding `127.0.0.1` changes the proxy upstream.** The old Caddy block proxied
    `localhost:8081`; `localhost` can resolve to `::1`, which the IPv4 loopback bind refuses. We
    changed it in the same Caddy edit, before it showed. *Rule:* proxy to `127.0.0.1:<PORT>`.
13. **Releases need a changelog section before the tag.** Neither changelog had a version section;
    the release workflow fails without one. A one-file release PR per repository, then annotated
    tags on the merge commits. *Rule:* check the tag with `git ls-remote --tags` and the release
    workflow run before deploying.
14. **Local build files outlive the code that read them.** The Mac checkout had `.env` and
    `.env.production` with `VITE_API_BASE_URL` and `VITE_AUTH_*`; Vite loads them even though git
    ignores them. They were moved away, keeping only `VITE_FULLCALENDAR_LICENSE_KEY`.
    `VITE_DEFAULT_LOCALE` turned out to be unused. *Rule:* list `.env*` (names only) before a
    production build.
15. **Copy the deploy settings from the sibling app.** `push.sh` timed out on port 22; the VPS
    listens on 2222, as `~/.config/ascenda/deploy.env` already said.
16. **Commands for the owner's shell carry no inline comments.** zsh passed `# only …` to `grep` as
    file names. Harmless, but confusing in the middle of a deploy.

### What worked

- Ascenda's checklist and lessons applied from the start: the audit before code, roles from
  `app_roles`, the admin URL and app ID never derived, the BFF added before the SPA moved, the
  e2e fake issuer on the same origin with forwarding pages.
- Every step checked on the server before the next: env names with values cut to four characters,
  `sslmode` and `show ssl`, the service user, the discovery issuer, then four `curl` checks after
  Caddy (`/api/version`, `/bff/session`, the `/bff/login` redirect, the old host).
- Backups before each change: `pg_dump`, the env file, the Caddyfile, the old web app.
- A history scan of both repositories (all branches) before making them public: the only real
  secret was the revoked `As6N…`.

## 4. Still to do

- [x] PR 9: remove `/auth/callback|refresh|logout` and bearer-only access to `/api/v1`, with a test
      that no route returns a token; release v3.1.0.
- [ ] Then: remove the `api.parashift.vandermoten.eu` Caddy block and `SOCRATE_REDIRECT_URL`;
      remove the old `/callback` redirect URI from app 7.
- [ ] PR 10: the runbook in the README (Caddy block, env table, deploy order, checks, symptom
      index), including this deploy's script fixes.
- [ ] "Resend invite" sends a claim link instead of a magic link (report row K5).
- [ ] One web app deploy script (report row D4); coverage floor; the 16 i18n lint warnings.
- [ ] Change the database password to a random value; turn on secret scanning, private
      vulnerability reporting and branch protection once the repositories are public.

## 5. Symptom index

| Symptom | Cause | Lesson |
|---|---|---|
| `/api/version` or `/bff/session` on the app's host returns the SPA's HTML | Caddy does not send the API paths on this host to the API | 1 |
| Sign-in fails at the token exchange; the env shows `SOCRATE_CLIENT_SECRET=As6N…` | revoked secret still on the server | 3 |
| `SOCRATE_ISSUER must equal SOCRATE_BASE_URL` / `must be set in production` | issuer from an older provider, or a v2 release with the v3 env | 4, 11 |
| `ENV is required`; debug token route reachable | `APP_ENV` set instead of `ENV` | 5 |
| A Socrate operator sees every store | role read from the top-level claim | 6 |
| Many e2e failures like `….find is not a function` | unmocked same-origin call answered by `vite preview` | 8 |
| Deploy: `DATABASE_URL is required … ENV is required` during migrations | migrations not run as the service user | 10 |
| Deploy: `failed to open source, "file://migrations": stat .: permission denied` | migrations run from the wrong directory | 10 |
| Deploy fails and the API stays down (`No previous release to rollback`) | rollback did not restart the service | 10 |
| Rollback "succeeds" but systemd restarts in a loop | the old release cannot read the new env file | 11 |
| `502` from Caddy after the upgrade (prevented here) | upstream `localhost` resolved to `::1` | 12 |
| Release workflow fails on the tag | no `## [X.Y.Z]` section in `CHANGELOG.md` | 13 |
| `push.sh`: `connect to host … port 22: Operation timed out` | wrong SSH port in `deploy.env` | 15 |
