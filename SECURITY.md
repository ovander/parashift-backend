# Security policy

Parashift holds staff schedules, contracts and leave for pharmacy teams, so security reports are
welcome and handled first.

## Reporting a vulnerability

Please use GitHub's **private vulnerability reporting**: the repository's **Security** tab →
**Report a vulnerability**. Do not open a public issue or pull request for a vulnerability.

Include what you found, how to reproduce it, and the version you tested (the app and API
versions are shown in the app; the API answers them at `/api/version`).

You will get an acknowledgement within a week. Fixes are released as soon as they are ready, and
the report is credited in the release notes unless you prefer otherwise.

## Scope

- In scope: the Parashift web app (`ovander/parashift-frontend`) and API
  (`ovander/parashift-backend`), including store (tenant) isolation, role checks, invitations and
  sign-in handling.
- Out of scope: the Socrate identity provider itself (report those to its maintainers),
  denial-of-service by volume, and findings that need a compromised user device.

## Supported versions

Only the latest release, the one deployed at https://parashift.vandermoten.eu, receives security
fixes.

## Known findings

The sign-in audit of 2026-10-01 and the status of each of its findings are in
[`docs/SOCRATE-COMPAT-REPORT.md`](docs/SOCRATE-COMPAT-REPORT.md). Known and being closed: the
`/auth` routes return tokens to the browser until sign-in moves to the Backend-for-Frontend.
