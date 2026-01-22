# Operations runbook

This guide focuses on day-two operations for the default Docker Compose stack in `deploy/docker-compose.yml`. It references the
Compose service names (`postgres`, `redis`, `transcoder`) and the host-mounted volumes so you can lift and restore state safely
without breaking the one-command deployment flow.

## Postgres logical backups and restores

Postgres is the system of record for channels, users, recordings metadata, and audit history. The Compose service name is
`postgres`, and the bundled `.env` defines `BITRIVER_POSTGRES_DB`, `BITRIVER_POSTGRES_USER`, and
`BITRIVER_POSTGRES_PASSWORD`.

### Backup with `pg_dump`

1. Ensure the stack is running so `postgres` is healthy.
2. Run `pg_dump` inside the container and copy the archive to the host:

```bash
mkdir -p ./backups

docker compose exec -T postgres \
  pg_dump -U "$BITRIVER_POSTGRES_USER" \
  -d "$BITRIVER_POSTGRES_DB" \
  -F c \
  -f /tmp/bitriver-live.backup

docker compose cp postgres:/tmp/bitriver-live.backup \
  ./backups/bitriver-live-$(date +%F).backup
```

If you prefer to run `pg_dump` from a workstation or a job runner, point it at the same DSN the API uses (`postgres:5432`
inside Compose, or the host port if you enable the `postgres-host` profile).

### Restore with `pg_restore`

1. Stop the API so it does not write to Postgres during the restore:

```bash
docker compose stop bitriver-live
```

2. Copy the backup archive into the `postgres` container and restore it:

```bash
docker compose cp ./backups/bitriver-live-2024-01-01.backup postgres:/tmp/bitriver-live.backup

docker compose exec -T postgres \
  pg_restore --clean --if-exists --create \
  -U "$BITRIVER_POSTGRES_USER" \
  -d postgres \
  /tmp/bitriver-live.backup
```

3. Start the API again and confirm it reconnects:

```bash
docker compose start bitriver-live
```

If you restore into a new database name or host, update `BITRIVER_LIVE_POSTGRES_DSN` in `.env` before restarting the stack.

## Redis persistence and recovery

The Compose `redis` service is configured as cache-only by default (`--save ""` and `--appendonly no`), so chat queue history
and rate-limit counters are not persisted across restarts.

If you want Redis persistence:

1. Update the `redis` command in `deploy/docker-compose.yml` to enable either RDB snapshots or AOF.
2. Restart the service so Redis writes data under the `redis-data` volume.
3. Trigger a snapshot when needed:

```bash
docker compose exec -T redis redis-cli --no-auth-warning -a "$BITRIVER_REDIS_PASSWORD" save
```

4. Back up the `redis-data` volume alongside Postgres:

```bash
mkdir -p ./backups

docker run --rm \
  -v bitriver-live_redis-data:/data \
  -v "$PWD/backups":/backup \
  alpine \
  tar czf /backup/redis-data-$(date +%F).tgz -C /data .
```

To restore, stop the stack, replace the contents of the `redis-data` volume with the archive, then start `redis` again. When
running cache-only (default), you can skip Redis backups entirely.

## Media assets: transcoder output + recordings

The transcoder writes HLS manifests and segments under `/work/public` inside the `transcoder` container. The Compose stack
mounts that directory to `./transcoder-data` on the host so media survives container restarts. Back up the host directory with
standard filesystem tooling:

```bash
rsync -av --delete ./transcoder-data/ /mnt/backup/bitriver-transcoder-data/
```

Recordings metadata lives in Postgres, but the media files live wherever you configured the recording storage:

- **Default Compose stack:** recordings and HLS assets live under `./transcoder-data/public` (from `/work/public`). Back up the
  directory alongside the other transcoder data.
- **Object storage configured:** follow your provider’s snapshot/replication runbook, and make sure bucket lifecycle rules
  align with the retention settings configured in the API.
- **Mounted storage (NFS/S3FS/etc.):** treat the mount point as the source of truth and back it up using your normal
  filesystem snapshots.

Keep backups of Postgres and media assets in the same recovery window so channel metadata and playback artefacts stay in
sync after a restore.
