#!/usr/bin/env bash
set -Eeuo pipefail

umask 077

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

BACKUP_DIR="${BITRIVER_BACKUP_DIR:-$REPO_ROOT/data/backups/postgres}"
TIMESTAMP="$(date -u +"%Y%m%dT%H%M%SZ")"
CREATED_AT="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
BACKUP_BASENAME="bitriver-postgres-${TIMESTAMP}.sql.gz"
BACKUP_PATH="$BACKUP_DIR/$BACKUP_BASENAME"
MANIFEST_PATH="$BACKUP_PATH.manifest.json"
CHECKSUM_PATH="$BACKUP_PATH.sha256"
COLLISION_LOCK="$BACKUP_PATH.lock"

POSTGRES_HOST="${BITRIVER_BACKUP_POSTGRES_HOST:-${BITRIVER_POSTGRES_HOST:-localhost}}"
POSTGRES_PORT="${BITRIVER_BACKUP_POSTGRES_PORT:-${BITRIVER_POSTGRES_PORT:-5432}}"
POSTGRES_DB="${BITRIVER_BACKUP_POSTGRES_DB:-${BITRIVER_POSTGRES_DB:-bitriver}}"
POSTGRES_USER="${BITRIVER_BACKUP_POSTGRES_USER:-${BITRIVER_POSTGRES_USER:-bitriver}}"
POSTGRES_PASSWORD="${BITRIVER_BACKUP_POSTGRES_PASSWORD:-${BITRIVER_POSTGRES_PASSWORD:-}}"

SOURCE_RELEASE="${BITRIVER_BACKUP_SOURCE_RELEASE:-${BITRIVER_RELEASE_VERSION:-unknown}}"
SOURCE_COMMIT="${BITRIVER_BACKUP_SOURCE_COMMIT:-${BITRIVER_RELEASE_COMMIT:-unknown}}"

UPLOAD_ENABLED="${BITRIVER_BACKUP_UPLOAD_ENABLED:-0}"
UPLOAD_PROVIDER="${BITRIVER_BACKUP_UPLOAD_PROVIDER:-s3}"
UPLOAD_BUCKET="${BITRIVER_BACKUP_UPLOAD_BUCKET:-}"
UPLOAD_PREFIX="${BITRIVER_BACKUP_UPLOAD_PREFIX:-bitriver-live/postgres}"
UPLOAD_REGION="${BITRIVER_BACKUP_UPLOAD_REGION:-us-east-1}"
UPLOAD_ENDPOINT="${BITRIVER_BACKUP_UPLOAD_ENDPOINT:-}"

run_prune="${BITRIVER_BACKUP_RUN_PRUNE:-1}"
partial_suffix=".partial.$$"
partial_backup="$BACKUP_PATH$partial_suffix"
partial_manifest="$MANIFEST_PATH$partial_suffix"
partial_checksum="$CHECKSUM_PATH$partial_suffix"
partial_migrations="$BACKUP_DIR/.bitriver-migrations-${TIMESTAMP}-$$.txt"
partial_row_counts="$BACKUP_DIR/.bitriver-row-counts-${TIMESTAMP}-$$.jsonl"
snapshot_pid=""
snapshot_id=""
published=false
collision_lock_acquired=false
owns_final_assets=false

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: required command not found: $1" >&2
    exit 1
  fi
}

validate_source_identity() {
  if [ "$SOURCE_RELEASE" != "unknown" ] &&
     [[ ! "$SOURCE_RELEASE" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$ ]]; then
    echo "error: BITRIVER_BACKUP_SOURCE_RELEASE must be an exact v-prefixed release or 'unknown'" >&2
    exit 1
  fi
  if [ "$SOURCE_COMMIT" != "unknown" ] && [[ ! "$SOURCE_COMMIT" =~ ^[0-9a-f]{40}$ ]]; then
    echo "error: BITRIVER_BACKUP_SOURCE_COMMIT must be a full lowercase commit SHA or 'unknown'" >&2
    exit 1
  fi
}

psql_source() {
  PGPASSWORD="$POSTGRES_PASSWORD" psql -X \
    --host "$POSTGRES_HOST" \
    --port "$POSTGRES_PORT" \
    --username "$POSTGRES_USER" \
    --dbname "$POSTGRES_DB" \
    -v ON_ERROR_STOP=1 \
    "$@"
}

stop_snapshot_exporter() {
  if [ -n "$snapshot_pid" ]; then
    kill "$snapshot_pid" >/dev/null 2>&1 || true
    wait "$snapshot_pid" >/dev/null 2>&1 || true
    snapshot_pid=""
  fi
}

cleanup() {
  stop_snapshot_exporter
  rm -f \
    "$partial_backup" \
    "$partial_manifest" \
    "$partial_checksum" \
    "$partial_migrations" \
    "$partial_row_counts"
  if [ "$owns_final_assets" = "true" ] && [ "$published" != "true" ]; then
    rm -f "$BACKUP_PATH" "$MANIFEST_PATH" "$CHECKSUM_PATH"
  fi
  if [ "$collision_lock_acquired" = "true" ]; then
    rmdir "$COLLISION_LOCK" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

start_snapshot_exporter() {
  coproc BITRIVER_BACKUP_SNAPSHOT {
    psql_source -qAt <<'SQL'
BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;
SELECT pg_export_snapshot();
SELECT pg_sleep(86400);
ROLLBACK;
SQL
  }
  snapshot_pid="${BITRIVER_BACKUP_SNAPSHOT_PID:-}"
  if [ -z "$snapshot_pid" ] || ! IFS= read -r snapshot_id <&"${BITRIVER_BACKUP_SNAPSHOT[0]}"; then
    echo "error: could not export a consistent Postgres snapshot" >&2
    exit 1
  fi
  if [[ ! "$snapshot_id" =~ ^[0-9A-Fa-f-]+$ ]]; then
    echo "error: Postgres returned an invalid snapshot identifier" >&2
    exit 1
  fi
}

capture_migration_fingerprint() {
  psql_source -qAt -F '|' -v backup_snapshot="$snapshot_id" >"$partial_migrations" <<'SQL'
BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET TRANSACTION SNAPSHOT :'backup_snapshot';
DO $block$
BEGIN
  IF to_regclass('public.schema_migrations') IS NULL THEN
    RAISE EXCEPTION 'BitRiver migration ledger is absent; run canonical migrations before backup';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM public.schema_migrations) THEN
    RAISE EXCEPTION 'BitRiver migration ledger contains no applied migrations';
  END IF;
  IF EXISTS (SELECT 1 FROM public.schema_migrations WHERE status <> 'applied') THEN
    RAISE EXCEPTION 'BitRiver migration ledger contains applying or failed rows';
  END IF;
END
$block$;
SELECT filename, version, checksum_sha256, status, release_version, release_commit
FROM public.schema_migrations
ORDER BY filename;
COMMIT;
SQL
}

capture_row_counts() {
  psql_source -qAt -v backup_snapshot="$snapshot_id" >"$partial_row_counts" <<'SQL'
BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET TRANSACTION SNAPSHOT :'backup_snapshot';
SELECT format(
  'SELECT jsonb_build_object(%L, count(*)::bigint) FROM %I.%I;',
  tablename,
  schemaname,
  tablename
)
FROM pg_catalog.pg_tables
WHERE schemaname = 'public'
ORDER BY tablename
\gexec
COMMIT;
SQL
}

row_counts_json() {
  local separator=""
  local line
  printf '['
  while IFS= read -r line; do
    [ -n "$line" ] || continue
    printf '%s%s' "$separator" "$line"
    separator=','
  done <"$partial_row_counts"
  printf ']'
}

write_manifest() {
  local archive_sha="$1"
  local archive_size="$2"
  local migration_fingerprint="$3"
  local pg_dump_version="$4"
  local psql_version="$5"
  local row_counts_json="$6"

  psql_source -qAt \
    -v backup_snapshot="$snapshot_id" \
    -v created_at="$CREATED_AT" \
    -v source_release="$SOURCE_RELEASE" \
    -v source_commit="$SOURCE_COMMIT" \
    -v archive_name="$BACKUP_BASENAME" \
    -v archive_sha="$archive_sha" \
    -v archive_size="$archive_size" \
    -v checksum_name="$(basename "$CHECKSUM_PATH")" \
    -v migration_fingerprint="$migration_fingerprint" \
    -v pg_dump_version="$pg_dump_version" \
    -v psql_version="$psql_version" \
    -v row_counts_json="$row_counts_json" >"$partial_manifest" <<'SQL'
BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET TRANSACTION SNAPSHOT :'backup_snapshot';

DO $block$
BEGIN
  IF to_regclass('public.schema_migrations') IS NULL THEN
    RAISE EXCEPTION 'BitRiver migration ledger is absent; run canonical migrations before backup';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM public.schema_migrations) THEN
    RAISE EXCEPTION 'BitRiver migration ledger contains no applied migrations';
  END IF;
  IF EXISTS (SELECT 1 FROM public.schema_migrations WHERE status <> 'applied') THEN
    RAISE EXCEPTION 'BitRiver migration ledger contains applying or failed rows';
  END IF;
END
$block$;

SELECT jsonb_pretty(jsonb_build_object(
  'schemaVersion', 'bitriver.postgres-backup/v1',
  'createdAt', :'created_at',
  'source', jsonb_build_object(
    'release', :'source_release',
    'commit', :'source_commit'
  ),
  'archive', jsonb_build_object(
    'name', :'archive_name',
    'format', 'postgresql-plain-sql+gzip',
    'sha256', :'archive_sha',
    'sizeBytes', :'archive_size'::bigint,
    'checksumAsset', :'checksum_name'
  ),
  'database', jsonb_build_object(
    'name', current_database(),
    'serverVersion', current_setting('server_version'),
    'serverVersionNum', current_setting('server_version_num')::integer,
    'migrationFingerprintSha256', :'migration_fingerprint',
    'migrations', (
      SELECT jsonb_agg(jsonb_build_object(
        'filename', filename,
        'version', version,
        'checksumSha256', checksum_sha256,
        'status', status,
        'release', release_version,
        'commit', release_commit
      ) ORDER BY filename)
      FROM public.schema_migrations
    ),
    'rowCounts', (
      SELECT COALESCE(
        jsonb_object_agg(count_entry.key, count_entry.value ORDER BY count_entry.key),
        '{}'::jsonb
      )
      FROM jsonb_array_elements(:'row_counts_json'::jsonb) AS count_object(value)
      CROSS JOIN LATERAL jsonb_each(count_object.value) AS count_entry
    )
  ),
  'tools', jsonb_build_object(
    'pgDump', :'pg_dump_version',
    'psql', :'psql_version'
  ),
  'consistency', jsonb_build_object(
    'snapshot', 'postgres-exported-snapshot',
    'tableRowCounts', 'exact'
  )
));
COMMIT;
SQL
}

upload_backup() {
  if [ "$UPLOAD_ENABLED" != "1" ]; then
    return
  fi

  if [ "$UPLOAD_PROVIDER" != "s3" ]; then
    echo "error: unsupported BITRIVER_BACKUP_UPLOAD_PROVIDER: $UPLOAD_PROVIDER" >&2
    exit 1
  fi

  if [ -z "$UPLOAD_BUCKET" ]; then
    echo "error: BITRIVER_BACKUP_UPLOAD_BUCKET is required when uploads are enabled" >&2
    exit 1
  fi

  require_command aws

  local destination="s3://${UPLOAD_BUCKET}/${UPLOAD_PREFIX}/${BACKUP_BASENAME}"
  local aws_args=("--region" "$UPLOAD_REGION")

  if [ -n "$UPLOAD_ENDPOINT" ]; then
    aws_args+=("--endpoint-url" "$UPLOAD_ENDPOINT")
  fi

  echo "uploading backup set to ${destination}" >&2
  aws s3 cp "$BACKUP_PATH" "$destination" "${aws_args[@]}"
  aws s3 cp "$MANIFEST_PATH" "${destination}.manifest.json" "${aws_args[@]}"
  aws s3 cp "$CHECKSUM_PATH" "${destination}.sha256" "${aws_args[@]}"
}

main() {
  require_command pg_dump
  require_command psql
  require_command gzip
  require_command sha256sum
  validate_source_identity

  mkdir -p "$BACKUP_DIR"
  if ! mkdir "$COLLISION_LOCK" 2>/dev/null; then
    echo "error: refusing concurrent backup set for timestamp $TIMESTAMP" >&2
    exit 1
  fi
  collision_lock_acquired=true
  if [ -e "$BACKUP_PATH" ] || [ -e "$MANIFEST_PATH" ] || [ -e "$CHECKSUM_PATH" ]; then
    echo "error: refusing to replace an existing backup set for timestamp $TIMESTAMP" >&2
    exit 1
  fi
  owns_final_assets=true

  start_snapshot_exporter

  echo "creating consistent Postgres backup at $BACKUP_PATH" >&2
  PGPASSWORD="$POSTGRES_PASSWORD" pg_dump \
    --host "$POSTGRES_HOST" \
    --port "$POSTGRES_PORT" \
    --username "$POSTGRES_USER" \
    --dbname "$POSTGRES_DB" \
    --snapshot "$snapshot_id" \
    --clean \
    --if-exists \
    --no-owner \
    --no-privileges | gzip -c >"$partial_backup"

  capture_migration_fingerprint
  capture_row_counts
  migration_fingerprint_line="$(sha256sum "$partial_migrations")"
  migration_fingerprint="${migration_fingerprint_line%% *}"
  archive_sha_line="$(sha256sum "$partial_backup")"
  archive_sha="${archive_sha_line%% *}"
  archive_size="$(wc -c <"$partial_backup" | tr -d '[:space:]')"

  write_manifest \
    "$archive_sha" \
    "$archive_size" \
    "$migration_fingerprint" \
    "$(pg_dump --version)" \
    "$(psql --version)" \
    "$(row_counts_json)"
  stop_snapshot_exporter

  manifest_sha_line="$(sha256sum "$partial_manifest")"
  manifest_sha="${manifest_sha_line%% *}"
  {
    printf '%s  %s\n' "$archive_sha" "$BACKUP_BASENAME"
    printf '%s  %s\n' "$manifest_sha" "$(basename "$MANIFEST_PATH")"
  } >"$partial_checksum"

  mv "$partial_backup" "$BACKUP_PATH"
  mv "$partial_manifest" "$MANIFEST_PATH"
  mv "$partial_checksum" "$CHECKSUM_PATH"
  published=true
  echo "backup set complete: $BACKUP_PATH" >&2

  upload_backup

  if [ "$run_prune" = "1" ]; then
    "$SCRIPT_DIR/prune-backups.sh"
  fi
}

main "$@"
