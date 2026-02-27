# Upgrading BitRiver Live

This guide defines the **supported upgrade/rollback contract** for BitRiver Live v1.0+ when running the default deployment contract:

- `deploy/docker-compose.yml`
- repository `.env`
- `deploy/ome/Server.generated.xml`

## Supported upgrade paths

BitRiver Live follows these rules:

1. **No downgrades through the upgrade flow.** `X -> Y` must have `Y > X`.
2. **Minor upgrades are N-1 only.** You may move one minor at a time within a major line.
   - ✅ `v1.2.x -> v1.3.x`
   - ❌ `v1.1.x -> v1.3.x`
3. **No skipped majors.** You may move only one major at a time.
   - ✅ `v1.x -> v2.0+`
   - ❌ `v1.x -> v3.0+`
4. **Major upgrades are high-risk by default.** Treat them as breaking until release notes prove otherwise.

Use the planner before every maintenance window:

```bash
go run ./cmd/bitriver upgrade-plan --env-file .env --to vX.Y.Z --check-schema --current-schema <current_schema_version>
```

The planner reads `BITRIVER_LIVE_IMAGE_TAG` from `.env`, validates the hop, prints required steps, and warns when the target may include breaking changes.

## Backup and restore checklist (required)

Complete all items before stopping production traffic:

- [ ] Export Postgres backup from the running system (for example `pg_dump`/`pg_dumpall`) and verify the dump is readable.
- [ ] Snapshot/backup runtime volumes used by the release:
  - `deploy/data/`
  - `deploy/transcoder-data/`
  - `deploy/ome/`
- [ ] Copy deployment configuration artifacts:
  - `.env`
  - `deploy/docker-compose.yml` (and overlays you use)
  - any reverse proxy / ingress config that routes to BitRiver
- [ ] Record the currently deployed image tags/digests from `.env`.
- [ ] Record the current schema version (migration metadata) before migration.
- [ ] Confirm restore drill ownership (who can run restore, where backups are stored, and expected RTO).

If you cannot produce both a DB dump and config backup, **do not start the upgrade**.

## Single copy-paste upgrade sequence

Run from repo root (replace `vX.Y.Z`):

```bash
go run ./cmd/bitriver upgrade-plan --env-file .env --to vX.Y.Z --check-schema --current-schema <current_schema_version>
docker compose -f deploy/docker-compose.yml down
cp .env .env.backup.$(date +%Y%m%d%H%M%S)
deploy/check-env.sh
./scripts/render-ome-config.sh --check || ./scripts/render-ome-config.sh --force
docker compose -f deploy/docker-compose.yml run --rm postgres-migrations
docker compose -f deploy/docker-compose.yml up -d
go run ./cmd/bitriver verify --compose-file deploy/docker-compose.yml --env-file .env
```

## Migration behavior guarantees

For the supported Compose deployment:

- `postgres-migrations` runs before API startup during `docker compose up`.
- All SQL files in `deploy/migrations/` are applied in version order expected by the migration engine.
- The API expects the schema to match the release's migration set before serving traffic.

Important limitations:

- Not every migration is reversible.
- Release notes may require operator-managed data transforms before/after SQL migrations.
- Schema compatibility is only guaranteed for the supported upgrade hops above.

## Roll back

Rollback safety depends on whether migrations changed data in non-reversible ways.

### Safe rollback (usually possible)

Use this only when **no irreversible migration has been applied** (or when migrations were backward compatible and validated for rollback in release notes):

1. Stop services: `docker compose -f deploy/docker-compose.yml down`
2. Restore previous `.env` and previous image tags/digests.
3. Restore previous generated OME config (`deploy/ome/Server.generated.xml`) if needed.
4. Start previous release: `docker compose -f deploy/docker-compose.yml up -d`

### Unsafe rollback (common after schema/data changes)

If irreversible migrations ran, rolling binaries back without restoring DB state can corrupt runtime behavior. In this case:

1. Stop services.
2. Restore Postgres from pre-upgrade backup.
3. Restore related volumes/config snapshots.
4. Bring the previous release back online.

When in doubt, assume rollback is unsafe until verified in release notes.

## Post-upgrade validation

After restart:

- `docker compose -f deploy/docker-compose.yml ps`
- `go run ./cmd/bitriver verify --compose-file deploy/docker-compose.yml --env-file .env`
- Confirm API `/healthz`, viewer playback, ingest path, and any payment/chat flows relevant to your deployment.

Also update your internal runbook with:

- previous version
- target version
- migration status
- rollback decision and backup location
