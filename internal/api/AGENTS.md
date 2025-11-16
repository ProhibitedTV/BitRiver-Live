# `internal/api` Guidance

`Handler` centralises HTTP endpoints for the Go API. Review the root `AGENTS.md` for repo-wide rules.

## Design notes
- `Handler` glues `storage.Repository`, `auth.SessionManager`, queue/health probes, and feature toggles together. When adding routes, extend the handler constructor so dependencies are injected (never reach out to globals).
- Helper functions (`writeJSON`, `decodeJSON`, `setSessionCookie`, error mappers) must stay consistent. Touching one usually means updating every handler; keep behaviour uniform.
- Respect middleware expectations from `internal/server` (auth → rate limiting → metrics → audit → logging). New handlers should assume headers/cookies are already validated.

## Testing
- Add/extend tests in `handlers_test.go` (or nearby files) whenever adding routes or changing behaviour. Cover both success and error paths.
- For session or storage changes, use the fakes from `internal/testsupport` rather than duplicating mocks.

## Before opening a PR
- Document new endpoints in `docs/` and update the README if public behaviour changes.
- Keep response schemas backward compatible when possible; update `web/static` and viewer clients if payloads change.
- Run `go test ./internal/api -count=1` plus the repo suite.
