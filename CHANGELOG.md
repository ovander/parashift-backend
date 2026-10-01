# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Changed

- backendkit v1.15.1 (from v1.8.0). Every call to Socrate goes through `socrate.Client`: the
  `/auth` code exchange, refresh and revocation, and the profile read for auto-link. The app ID
  is `SOCRATE_APP_ID` only (no longer decoded from a service token), and the start-up admin
  probe is replaced by a log line.

### Security

- Auto-link by e-mail on first sign-in only uses an address Socrate has verified (report S8).

### Added

- `CLAUDE.md` (sources of truth, hard rules, local gate, git workflow) and this changelog.
- CI fails a pull request that adds or changes a Go file that is not gofmt-formatted.
