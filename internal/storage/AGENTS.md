# `internal/storage` Guidance

Persistence lives here. Read the root `AGENTS.md` before editing.

## Repository surface
- `Repository` covers users, channels, recordings, chat moderation, monetisation, and more. When adding methods, update the interface, both implementations (JSON + Postgres), and all call sites.
- JSON store targets local/dev flows; Postgres is production. Keep behaviour parity (validation, errors) between them.

## Schema + migrations
- Schema changes require a new SQL migration under `deploy/migrations/` plus docs for upgrading. Update `ImportSnapshotToPostgres` and relevant `cmd/tools` verifications so snapshots/migrations stay in sync.
- Respect Postgres connection pool tuning knobs exposed in constructors (max conns, idle). Avoid hard-coded defaults that override config.
- Ingest/transcoder hooks rely on specific tables/columns; review the constructors before renaming fields.

## Testing
- Unit tests cover shared logic; integration tests live behind the `postgres` tag. Run `./scripts/test-postgres.sh` when changing SQL or behaviour.

## Before opening a PR
- Update README/docs when schema or storage commands change.
- Regenerate snapshots or fixtures if record shapes change.
- Run `go test ./internal/storage -count=1` and the Postgres suite.
