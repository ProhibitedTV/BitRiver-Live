# `internal/auth` Guidance

Authentication + session management lives here. See the root `AGENTS.md` for general expectations.

## Session manager
- `SessionManager` defaults to 24-hour sessions. Keep default durations + refresh behaviour unless you update README/docs and clients.
- Supports pluggable stores (memory, Postgres). When changing persistence logic, update both implementations and keep interfaces consistent.

## Security expectations
- Cryptographic operations (token comparison, signing) must remain constant-time. Use existing helpers rather than rolling new primitives.
- Secrets/keys should flow through injected config—never read from globals or environment variables directly inside helpers.

## Testing
- Update unit tests under `internal/auth` when behaviour changes.
- For Postgres-backed sessions, adjust `postgres_store_integration_test.go` and run the `postgres` tag suite (see `docs/testing.md`).

## Before opening a PR
- Document new auth flows in `docs/` and README if user-visible.
- Run `go test ./internal/auth -count=1` and the Postgres suite when storage changes.
- Ensure clients (`internal/api`, viewer) are updated if session schema or cookie names change.
