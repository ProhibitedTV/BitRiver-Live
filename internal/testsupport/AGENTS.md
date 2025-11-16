# `internal/testsupport` Guidance

Shared fakes/stubs for tests live here. Read the root `AGENTS.md` first.

## Usage
- Prefer reusing helpers here (Redis queue stubs, HTTP fake servers, storage fixtures) instead of redefining mocks in test packages.
- Keep helpers deterministic and side-effect free so they can run under `-count=1` reliably.
- When adding new helpers, document them in the file header and keep APIs small.

## Maintenance
- Changes must be backward compatible—tests across the repo import these helpers. Introduce new constructors instead of mutating signatures when possible.
- Update README/docs if helpers become part of developer workflow (for example, scripts referencing them).

## Before opening a PR
- Cover new helpers with tests where practical.
- Run at least `go test ./internal/testsupport -count=1` and dependent packages to confirm compatibility.
