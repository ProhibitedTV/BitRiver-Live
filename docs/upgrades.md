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
go run ./cmd/bitriver upgrade-plan --compose-file deploy/docker-compose.yml --env-file .env --target vX.Y.Z
```

The planner prints a checklist with current-image detection, migration expectations, backup guidance, and rollback caveats.

The command is best-effort: if Docker is unavailable or the stack is stopped, it warns and falls back to `.env` tags when possible.

### Current automated rehearsal boundary: RC19 to RC20

The repository-owned stateful data-plane rehearsal uses the immediate immutable
published pair `v1.2.3-rc.19` (`1e14e3cf7d5f1d949b396d4f7897660575ea468e`)
to `v1.2.3-rc.20` (`9a8516a60c584c96a46b630b55c46df33f46fbdc`).
It binds evidence to the public `release-set.json` SHA-256 values
`374a4084d1880abab1fa980d528a47bb5e324ed85541248438015fb13f2cc204`
and `dd8eabcea7cf920a6f520e3e472cf44d3e1c7b0b7ad74945904f67ea74a47873`.
RC19 is used only as the immediate populated-state source; this does not turn
the rejected candidate into an approved release.

Run the focused rehearsal against disposable Postgres 15:

```bash
./scripts/test-stateful-upgrade.sh

# Retain the secret-scanned machine-readable report when collecting evidence.
BITRIVER_UPGRADE_REPORT_PATH=.artifacts/stateful-upgrade-report.json \
  ./scripts/test-stateful-upgrade.sh
```

The test uses the real canonical schema and representative non-empty account,
auth/MFA/session, channel/profile/follow/schedule, moderation/legal/chat,
stream/upload/recording/object-reference, and payment state. It requires a
manifest-bound pre-upgrade backup, proves the candidate migration path preserves
exact ledger and value/count fingerprints, exercises in-place rollback plus a
verified fresh-database restore, and requires an ambiguous `applying` migration
to block preflight. Its retained report schema is
`bitriver.stateful-upgrade-report/v1`.

RC19 and RC20 have byte-identical migration trees and runners, so this exact hop
is classified **in-place compatible at the database/migration layer**. It is not
full upgrade approval: exact-image Compose upgrade/image rollback, packaged
configuration and generated-config rollback, interrupted deploy cut points,
and post-upgrade ingest/playback/chat/admin/VOD golden-path evidence remain
required. A future schema change must be classified again; never carry this
in-place result forward by assumption.

### Ubuntu artifact installations

Boot-managed Ubuntu installs use the same Compose contract with these host paths:

- program/workspace: `/opt/bitriver-live`
- environment: `/etc/bitriver-live/bitriver.env`
- generated OME config: `/etc/bitriver-live/deploy/ome/Server.generated.xml`
- generated SRS config: `/etc/bitriver-live/deploy/srs/conf/srs.generated.conf`
- durable state: `/var/lib/bitriver-live`

The host manager migrates the older flat OME/SRS paths into this source-shaped
tree and retains compatibility links. A sole legacy file is moved without
changing its bytes. If both legacy and canonical files exist with different
content, the upgrade fails before staging program assets so the operator can
reconcile the conflict without data loss.

After completing the backup checklist, install/extract the new published artifact and stage it through the host manager:

```bash
# From a newly extracted launcher archive:
sudo ./install.sh upgrade --operator-user "$USER"

# Or after installing the new .deb:
sudo bitriver-host upgrade --operator-user "$USER"
```

Keep the package, launcher, five first-party image tags, and resolved digests on
one exact release. In particular, do not replace an RC tag with the unqualified
stable `v1.2.3` tag until that stable release actually exists.

The host manager preserves configuration/data and restarts only when the unit was already active. Run `sudo bitriver-host status`, authenticated OME control, real ingest, and playback checks after every upgrade. Follow [`docs/installing-on-ubuntu.md`](installing-on-ubuntu.md) for reboot and Nginx Proxy Manager acceptance.

Before changing images, query the database-backed plan. This command is read-only and does not create the ledger on an uninitialized database:

```bash
go run ./cmd/bitriver migrations --mode plan --compose-file deploy/docker-compose.yml --env-file .env
```

Example output:

```text
BitRiver Live upgrade plan
Planner version: dev (dev, unknown)
Compose file: deploy/docker-compose.yml
Env file: .env
Target tag: v1.4.0

Current image tags (best-effort):
- bitriver-live: v1.3.2 (tag=v1.3.2, source=env-file)

Migrations: EXPECTED
- compose file includes postgres-migrations service; migrations are expected before API startup in the default deployment contract.

Warnings:
- WARN: unable to read running image tags from docker compose ps: docker compose ps failed: ...
- WARN: using env-file tag values because running compose service tags were unavailable.

Operator checklist:
[ ] 1) Review upgrade notes: docs/upgrades.md and docs/production-release.md
[ ] 2) Complete backups before maintenance (docs/upgrades.md#backup-and-restore-checklist-required)
...

Rollback caveats:
- Safe rollback usually requires that irreversible migrations have NOT run.
```

## Backup and restore checklist (required)

Complete all items before stopping production traffic:

- [ ] Run `./scripts/backup-postgres.sh` with the exact source release and full
  commit, then verify and durably store the archive, manifest, and checksum as
  one set. Release evidence must not use `unknown` provenance.
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

If you cannot produce a complete manifest-bound database backup set and config
backup, **do not start the upgrade**. Follow
[`docs/operations.md`](operations.md#postgres-logical-backups-and-restores) for
the isolated restore rehearsal and full durable recovery inventory.

## Single copy-paste upgrade sequence

Run from repo root (replace `vX.Y.Z`):

```bash
go run ./cmd/bitriver upgrade-plan --compose-file deploy/docker-compose.yml --env-file .env --target vX.Y.Z
go run ./cmd/bitriver migrations --mode plan --compose-file deploy/docker-compose.yml --env-file .env
docker compose -f deploy/docker-compose.yml down
cp .env .env.backup.$(date +%Y%m%d%H%M%S)
deploy/check-env.sh
./scripts/render-ome-config.sh --check || ./scripts/render-ome-config.sh --force
go run ./cmd/bitriver migrations --mode apply --compose-file deploy/docker-compose.yml --env-file .env
docker compose -f deploy/docker-compose.yml up -d
go run ./cmd/bitriver verify --compose-file deploy/docker-compose.yml --env-file .env
```

## Migration behavior guarantees

For the supported Compose deployment:

- `postgres-migrations` runs before API startup during `docker compose up`.
- Migration identity is the complete filename, ordered byte-for-byte (the historical set contains two distinct `0002_*` files).
- The `schema_migrations` ledger records filename, version prefix, raw SHA-256, `applying`/`applied`/`failed` status, timestamps, and release/commit provenance.
- Only pending SQL files are applied. Applied files are skipped; changed or removed history and unresolved `applying`/`failed` rows stop startup.
- Each SQL file runs with `psql --single-transaction`. The final non-sensitive ledger is printed into the migration job log collected by release diagnostics.
- Compose and Helm consume the same canonical runner and byte-identical SQL generated from `deploy/migrations/`.
- The API expects the schema to match the release's migration set before serving traffic.

Important limitations:

- Migrations are forward-only by default; automatic down migrations are not provided.
- Destructive, data-transforming, or rollback-incompatible changes require explicit release notes describing compatibility, backup/restore, validation, and rollback impact.
- Never edit or rename an applied migration. Add a new forward migration instead.
- Schema compatibility is only guaranteed for the supported upgrade hops above.

## Failed or interrupted migration recovery

Inspect the sanitized history first:

```bash
go run ./cmd/bitriver migrations --mode status --compose-file deploy/docker-compose.yml --env-file .env
```

A `failed` row means PostgreSQL reported an error and the release remains blocked. Fix the external cause, confirm the transaction rolled back (or clean up documented partial state), copy the exact checksum from status, then explicitly retry:

```bash
go run ./cmd/bitriver migrations --mode repair --repair-action retry \
  --file 0012_example.sql --checksum <64-character-sha256> \
  --compose-file deploy/docker-compose.yml --env-file .env
```

An `applying` row is deliberately treated as ambiguous: the process may have stopped after rollback, or after the schema commit but before the ledger update. Inspect the migration SQL and database schema. If the SQL did not commit, restore/clean partial state and use the failed retry path after recording the incident. If every intended schema effect is already present and validated, acknowledge only that exact file/checksum:

```bash
go run ./cmd/bitriver migrations --mode repair --repair-action mark-applied \
  --file 0012_example.sql --checksum <64-character-sha256> \
  --compose-file deploy/docker-compose.yml --env-file .env
```

Do not use `mark-applied` to bypass a migration error. If the schema result is uncertain, restore the pre-upgrade Postgres backup and repeat the supported upgrade instead.

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
