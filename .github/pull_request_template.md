## What and why

<!-- What this changes and why. Link the issue if there is one ("Closes #…"). -->

## How it was tested

<!-- New or changed tests, and anything checked by hand. -->

- [ ] Changed Go files are gofmt-formatted; `go build ./...` and `go vet ./...` pass
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run ./...` (v2.14.0) reports no issue; `govulncheck ./...` finds no reachable vulnerability
- [ ] A line is added under `## [Unreleased]` in `CHANGELOG.md`

## Deploy notes

<!-- Delete what does not apply. -->
- Migration: <!-- number, and whether it deletes or rewrites data (back up first) -->
- New or changed environment variable: <!-- name, default; also in the README table and .env.example -->
- Order with the web app: <!-- e.g. deploy the API first -->
