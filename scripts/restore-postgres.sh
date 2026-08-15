#!/usr/bin/env bash
set -Eeuo pipefail

umask 077

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

BACKUP_DIR="${BITRIVER_BACKUP_DIR:-$REPO_ROOT/data/backups/postgres}"
BACKUP_FILE="${1:-}"

POSTGRES_HOST="${BITRIVER_BACKUP_POSTGRES_HOST:-${BITRIVER_POSTGRES_HOST:-localhost}}"
POSTGRES_PORT="${BITRIVER_BACKUP_POSTGRES_PORT:-${BITRIVER_POSTGRES_PORT:-5432}}"
POSTGRES_USER="${BITRIVER_BACKUP_POSTGRES_USER:-${BITRIVER_POSTGRES_USER:-bitriver}}"
POSTGRES_PASSWORD="${BITRIVER_BACKUP_POSTGRES_PASSWORD:-${BITRIVER_POSTGRES_PASSWORD:-}}"
POSTGRES_ADMIN_DB="${BITRIVER_BACKUP_POSTGRES_ADMIN_DB:-postgres}"

REHEARSAL_DB="${BITRIVER_RESTORE_REHEARSAL_DB:-bitr_restore_$(date -u +"%Y%m%d%H%M%S")}"
KEEP_REHEARSAL_DB="${BITRIVER_RESTORE_KEEP_DB:-0}"
EXPECTED_RELEASE="${BITRIVER_RESTORE_EXPECT_RELEASE:-}"
EXPECTED_SCHEMA_FINGERPRINT="${BITRIVER_RESTORE_EXPECT_SCHEMA_FINGERPRINT:-}"
RUN_ID="$(date -u +"%Y%m%dT%H%M%SZ")"
STARTED_AT="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
STARTED_EPOCH="$(date -u +%s)"
REPORT_PATH="${BITRIVER_RESTORE_REPORT_PATH:-}"

manifest_file=""
checksum_file=""
manifest_json=""
source_release=""
source_commit=""
source_database=""
backup_created_at=""
archive_sha=""
manifest_schema_fingerprint=""
restored_row_counts_json=""
restore_migrations_file=""
restore_row_counts_file=""
partial_report=""
database_created=false

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: required command not found: $1" >&2
    exit 1
  fi
}

latest_backup() {
  find "$BACKUP_DIR" -maxdepth 1 -type f -name 'bitriver-postgres-*.sql.gz' | sort | tail -n 1
}

validate_database_name() {
  local name="$1"
  local label="$2"
  if [[ ! "$name" =~ ^[a-z_][a-z0-9_]{0,62}$ ]]; then
    echo "error: $label must match ^[a-z_][a-z0-9_]{0,62}$" >&2
    exit 1
  fi
}

validate_expected_identity() {
  if [ -n "$EXPECTED_RELEASE" ] &&
     [[ ! "$EXPECTED_RELEASE" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$ ]]; then
    echo "error: BITRIVER_RESTORE_EXPECT_RELEASE must be an exact v-prefixed release" >&2
    exit 1
  fi
  if [ -n "$EXPECTED_SCHEMA_FINGERPRINT" ] &&
     [[ ! "$EXPECTED_SCHEMA_FINGERPRINT" =~ ^[0-9a-f]{64}$ ]]; then
    echo "error: BITRIVER_RESTORE_EXPECT_SCHEMA_FINGERPRINT must be 64 lowercase hex characters" >&2
    exit 1
  fi
}

psql_admin() {
  PGPASSWORD="$POSTGRES_PASSWORD" psql -X \
    --host "$POSTGRES_HOST" \
    --port "$POSTGRES_PORT" \
    --username "$POSTGRES_USER" \
    --dbname "$POSTGRES_ADMIN_DB" \
    -v ON_ERROR_STOP=1 \
    "$@"
}

psql_rehearsal() {
  PGPASSWORD="$POSTGRES_PASSWORD" psql -X \
    --host "$POSTGRES_HOST" \
    --port "$POSTGRES_PORT" \
    --username "$POSTGRES_USER" \
    --dbname "$REHEARSAL_DB" \
    -v ON_ERROR_STOP=1 \
    "$@"
}

manifest_value() {
  printf '%s\n' "$1" | psql_admin -qAt -v manifest_json="$manifest_json"
}

drop_rehearsal_database() {
  psql_admin -q -v rehearsal_db="$REHEARSAL_DB" <<'SQL' >/dev/null
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE datname = :'rehearsal_db'
  AND pid <> pg_backend_pid();
SQL
  psql_admin -q -v rehearsal_db="$REHEARSAL_DB" <<'SQL' >/dev/null
SELECT format('DROP DATABASE %I', :'rehearsal_db')
\gexec
SQL
  database_created=false
}

cleanup() {
  rm -f "$restore_migrations_file" "$restore_row_counts_file" "$partial_report"
  if [ "$database_created" = "true" ] && [ "$KEEP_REHEARSAL_DB" != "1" ]; then
    drop_rehearsal_database >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

validate_checksum_set() {
  local archive_name
  local manifest_name
  local line
  local digest
  local name
  local line_count=0
  local archive_seen=0
  local manifest_seen=0

  archive_name="$(basename "$BACKUP_FILE")"
  manifest_name="$(basename "$manifest_file")"

  while IFS= read -r line || [ -n "$line" ]; do
    line_count=$((line_count + 1))
    if [[ ! "$line" =~ ^([0-9a-f]{64})[[:space:]][[:space:]]([^/]+)$ ]]; then
      echo "error: invalid checksum entry in $checksum_file" >&2
      exit 1
    fi
    digest="${BASH_REMATCH[1]}"
    name="${BASH_REMATCH[2]}"
    case "$name" in
      "$archive_name")
        archive_seen=$((archive_seen + 1))
        archive_sha="$digest"
        ;;
      "$manifest_name")
        manifest_seen=$((manifest_seen + 1))
        ;;
      *)
        echo "error: checksum set contains unexpected asset: $name" >&2
        exit 1
        ;;
    esac
  done <"$checksum_file"

  if [ "$line_count" -ne 2 ] || [ "$archive_seen" -ne 1 ] || [ "$manifest_seen" -ne 1 ]; then
    echo "error: checksum set must cover the archive and manifest exactly once" >&2
    exit 1
  fi

  (cd "$(dirname "$BACKUP_FILE")" && sha256sum -c "$(basename "$checksum_file")")
}

validate_manifest() {
  local shape
  local manifest_archive
  local manifest_archive_sha
  local manifest_archive_size
  local manifest_checksum
  local actual_archive_size

  manifest_json="$(cat "$manifest_file")"
  shape="$(manifest_value "
WITH manifest AS (SELECT :'manifest_json'::jsonb AS doc)
SELECT CASE WHEN
  doc->>'schemaVersion' = 'bitriver.postgres-backup/v1'
  AND jsonb_typeof(doc->'createdAt') = 'string'
  AND jsonb_typeof(doc->'source') = 'object'
  AND jsonb_typeof(doc->'source'->'release') = 'string'
  AND jsonb_typeof(doc->'source'->'commit') = 'string'
  AND jsonb_typeof(doc->'archive') = 'object'
  AND doc #>> '{archive,format}' = 'postgresql-plain-sql+gzip'
  AND doc #>> '{archive,sha256}' ~ '^[0-9a-f]{64}$'
  AND jsonb_typeof(doc->'archive'->'sizeBytes') = 'number'
  AND (doc #>> '{archive,sizeBytes}') ~ '^[1-9][0-9]*$'
  AND jsonb_typeof(doc->'archive'->'checksumAsset') = 'string'
  AND jsonb_typeof(doc->'database') = 'object'
  AND length(doc #>> '{database,name}') > 0
  AND length(doc #>> '{database,serverVersion}') > 0
  AND (doc #>> '{database,serverVersionNum}') ~ '^[1-9][0-9]*$'
  AND doc #>> '{database,migrationFingerprintSha256}' ~ '^[0-9a-f]{64}$'
  AND jsonb_typeof(doc->'database'->'migrations') = 'array'
  AND jsonb_array_length(doc->'database'->'migrations') > 0
  AND NOT EXISTS (
    SELECT 1
    FROM jsonb_array_elements(doc->'database'->'migrations') AS migration
    WHERE migration->>'status' IS DISTINCT FROM 'applied'
       OR COALESCE(migration->>'filename', '') = ''
       OR COALESCE(migration->>'checksumSha256', '') !~ '^[0-9a-f]{64}$'
  )
  AND jsonb_typeof(doc->'database'->'rowCounts') = 'object'
  AND EXISTS (
    SELECT 1
    FROM jsonb_each(doc->'database'->'rowCounts')
  )
  AND NOT EXISTS (
    SELECT 1
    FROM jsonb_each(doc->'database'->'rowCounts') AS row_count
    WHERE row_count.key = ''
       OR jsonb_typeof(row_count.value) IS DISTINCT FROM 'number'
       OR row_count.value::text !~ '^[0-9]+$'
  )
  AND jsonb_typeof(doc->'tools') = 'object'
  AND length(doc #>> '{tools,pgDump}') > 0
  AND length(doc #>> '{tools,psql}') > 0
  AND doc #>> '{consistency,snapshot}' = 'postgres-exported-snapshot'
  AND doc #>> '{consistency,tableRowCounts}' = 'exact'
THEN 'valid' ELSE 'invalid' END
FROM manifest;")"
  if [ "$shape" != "valid" ]; then
    echo "error: backup manifest structure or migration/row-count evidence is invalid" >&2
    exit 1
  fi

  backup_created_at="$(manifest_value "SELECT (:'manifest_json'::jsonb->>'createdAt')::timestamptz;")"
  source_release="$(manifest_value "SELECT :'manifest_json'::jsonb #>> '{source,release}';")"
  source_commit="$(manifest_value "SELECT :'manifest_json'::jsonb #>> '{source,commit}';")"
  source_database="$(manifest_value "SELECT :'manifest_json'::jsonb #>> '{database,name}';")"
  manifest_archive="$(manifest_value "SELECT :'manifest_json'::jsonb #>> '{archive,name}';")"
  manifest_archive_sha="$(manifest_value "SELECT :'manifest_json'::jsonb #>> '{archive,sha256}';")"
  manifest_archive_size="$(manifest_value "SELECT :'manifest_json'::jsonb #>> '{archive,sizeBytes}';")"
  manifest_checksum="$(manifest_value "SELECT :'manifest_json'::jsonb #>> '{archive,checksumAsset}';")"
  manifest_schema_fingerprint="$(manifest_value "SELECT :'manifest_json'::jsonb #>> '{database,migrationFingerprintSha256}';")"

  if [ "$manifest_archive" != "$(basename "$BACKUP_FILE")" ]; then
    echo "error: backup manifest archive identity does not match the selected file" >&2
    exit 1
  fi
  if [ "$manifest_archive_sha" != "$archive_sha" ]; then
    echo "error: backup manifest archive checksum disagrees with the verified checksum set" >&2
    exit 1
  fi
  actual_archive_size="$(wc -c <"$BACKUP_FILE" | tr -d '[:space:]')"
  if [ "$manifest_archive_size" != "$actual_archive_size" ]; then
    echo "error: backup manifest archive size does not match the selected file" >&2
    exit 1
  fi
  if [ "$manifest_checksum" != "$(basename "$checksum_file")" ]; then
    echo "error: backup manifest checksum asset identity is invalid" >&2
    exit 1
  fi
  if [[ ! "$manifest_schema_fingerprint" =~ ^[0-9a-f]{64}$ ]]; then
    echo "error: backup manifest migration fingerprint is invalid" >&2
    exit 1
  fi
  if [ "$source_release" != "unknown" ] &&
     [[ ! "$source_release" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$ ]]; then
    echo "error: backup manifest source release is invalid" >&2
    exit 1
  fi
  if [ "$source_commit" != "unknown" ] && [[ ! "$source_commit" =~ ^[0-9a-f]{40}$ ]]; then
    echo "error: backup manifest source commit is invalid" >&2
    exit 1
  fi
  if [ -n "$EXPECTED_RELEASE" ] && [ "$source_release" != "$EXPECTED_RELEASE" ]; then
    echo "error: backup release mismatch: expected $EXPECTED_RELEASE, manifest records $source_release" >&2
    exit 1
  fi
  if [ -n "$EXPECTED_SCHEMA_FINGERPRINT" ] &&
     [ "$manifest_schema_fingerprint" != "$EXPECTED_SCHEMA_FINGERPRINT" ]; then
    echo "error: backup schema fingerprint does not match the expected release schema" >&2
    exit 1
  fi
  if [ "$REHEARSAL_DB" = "$source_database" ]; then
    echo "error: rehearsal database must not equal the backup source database" >&2
    exit 1
  fi
}

capture_restored_invariants() {
  psql_rehearsal -qAt -F '|' >"$restore_migrations_file" <<'SQL'
SELECT filename, version, checksum_sha256, status, release_version, release_commit
FROM public.schema_migrations
ORDER BY filename;
SQL

  restored_migration_line="$(sha256sum "$restore_migrations_file")"
  restored_migration_fingerprint="${restored_migration_line%% *}"
  if [ "$restored_migration_fingerprint" != "$manifest_schema_fingerprint" ]; then
    echo "error: restored migration fingerprint does not match backup manifest" >&2
    exit 1
  fi

  psql_rehearsal -qAt >"$restore_row_counts_file" <<'SQL'
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
SQL

  local separator=""
  local line
  local row_counts_array="["
  while IFS= read -r line; do
    [ -n "$line" ] || continue
    row_counts_array+="${separator}${line}"
    separator=','
  done <"$restore_row_counts_file"
  row_counts_array+="]"

  restored_row_counts_json="$(psql_rehearsal -qAt -v row_counts_json="$row_counts_array" <<'SQL'
SELECT COALESCE(
  jsonb_object_agg(count_entry.key, count_entry.value ORDER BY count_entry.key),
  '{}'::jsonb
)::text
FROM jsonb_array_elements(:'row_counts_json'::jsonb) AS count_object(value)
CROSS JOIN LATERAL jsonb_each(count_object.value) AS count_entry;
SQL
)"

  row_counts_match="$(
    psql_admin -qAt \
      -v manifest_json="$manifest_json" \
      -v restored_row_counts_json="$restored_row_counts_json" <<'SQL'
SELECT CASE WHEN
  :'manifest_json'::jsonb #> '{database,rowCounts}' = :'restored_row_counts_json'::jsonb
  THEN 'yes' ELSE 'no' END;
SQL
  )"
  if [ "$row_counts_match" != "yes" ]; then
    echo "error: restored public-table row counts do not match backup manifest" >&2
    exit 1
  fi
}

write_report() {
  local cleanup_state="$1"
  local completed_at
  local completed_epoch
  local duration_seconds
  local manifest_sha_line
  local manifest_sha
  local release_check="not-requested"
  local schema_check="not-requested"

  completed_at="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  completed_epoch="$(date -u +%s)"
  duration_seconds=$((completed_epoch - STARTED_EPOCH))
  manifest_sha_line="$(sha256sum "$manifest_file")"
  manifest_sha="${manifest_sha_line%% *}"
  if [ -n "$EXPECTED_RELEASE" ]; then
    release_check="matched"
  fi
  if [ -n "$EXPECTED_SCHEMA_FINGERPRINT" ]; then
    schema_check="matched"
  fi

  psql_admin -qAt \
    -v completed_at="$completed_at" \
    -v started_at="$STARTED_AT" \
    -v duration_seconds="$duration_seconds" \
    -v backup_created_at="$backup_created_at" \
    -v archive_name="$(basename "$BACKUP_FILE")" \
    -v archive_sha="$archive_sha" \
    -v manifest_name="$(basename "$manifest_file")" \
    -v manifest_sha="$manifest_sha" \
    -v source_release="$source_release" \
    -v source_commit="$source_commit" \
    -v migration_fingerprint="$manifest_schema_fingerprint" \
    -v row_counts_json="$restored_row_counts_json" \
    -v rehearsal_db="$REHEARSAL_DB" \
    -v cleanup_state="$cleanup_state" \
    -v release_check="$release_check" \
    -v schema_check="$schema_check" >"$partial_report" <<'SQL'
SELECT jsonb_pretty(jsonb_build_object(
  'schemaVersion', 'bitriver.postgres-restore-report/v1',
  'result', 'passed',
  'startedAt', :'started_at',
  'completedAt', :'completed_at',
  'backup', jsonb_build_object(
    'archive', :'archive_name',
    'archiveSha256', :'archive_sha',
    'manifest', :'manifest_name',
    'manifestSha256', :'manifest_sha',
    'createdAt', :'backup_created_at',
    'sourceRelease', :'source_release',
    'sourceCommit', :'source_commit',
    'observedRpoSeconds', GREATEST(
      0,
      floor(extract(epoch FROM now() - :'backup_created_at'::timestamptz))::bigint
    )
  ),
  'restore', jsonb_build_object(
    'isolatedDatabase', :'rehearsal_db',
    'observedRtoSeconds', :'duration_seconds'::bigint,
    'cleanup', :'cleanup_state'
  ),
  'compatibility', jsonb_build_object(
    'release', :'release_check',
    'schemaFingerprint', :'schema_check'
  ),
  'invariants', jsonb_build_object(
    'migrationFingerprint', 'matched',
    'migrationFingerprintSha256', :'migration_fingerprint',
    'rowCounts', 'matched',
    'publicTableCount', (
      SELECT count(*)
      FROM jsonb_object_keys(:'row_counts_json'::jsonb)
    )
  )
));
SQL
  mv "$partial_report" "$REPORT_PATH"
}

main() {
  require_command psql
  require_command gzip
  require_command sha256sum
  validate_database_name "$REHEARSAL_DB" "BITRIVER_RESTORE_REHEARSAL_DB"
  validate_database_name "$POSTGRES_ADMIN_DB" "BITRIVER_BACKUP_POSTGRES_ADMIN_DB"
  validate_expected_identity
  if [ "$KEEP_REHEARSAL_DB" != "0" ] && [ "$KEEP_REHEARSAL_DB" != "1" ]; then
    echo "error: BITRIVER_RESTORE_KEEP_DB must be 0 or 1" >&2
    exit 1
  fi
  case "$REHEARSAL_DB" in
    postgres|template0|template1|"$POSTGRES_ADMIN_DB")
      echo "error: refusing protected rehearsal database name: $REHEARSAL_DB" >&2
      exit 1
      ;;
  esac

  if [ -z "$BACKUP_FILE" ]; then
    BACKUP_FILE="$(latest_backup)"
  fi
  if [ -z "$BACKUP_FILE" ] || [ ! -f "$BACKUP_FILE" ]; then
    echo "error: backup file not found. pass a path or store backups in $BACKUP_DIR" >&2
    exit 1
  fi

  manifest_file="${BACKUP_FILE}.manifest.json"
  checksum_file="${BACKUP_FILE}.sha256"
  if [ ! -f "$manifest_file" ]; then
    echo "error: backup manifest is required: $manifest_file" >&2
    exit 1
  fi
  if [ ! -f "$checksum_file" ]; then
    echo "error: backup checksum set is required: $checksum_file" >&2
    exit 1
  fi
  if [ -z "$REPORT_PATH" ]; then
    REPORT_PATH="${BACKUP_FILE}.restore-report-${RUN_ID}.json"
  fi
  if [ -e "$REPORT_PATH" ]; then
    echo "error: refusing to overwrite existing restore report: $REPORT_PATH" >&2
    exit 1
  fi
  partial_report="${REPORT_PATH}.partial.$$"
  restore_migrations_file="${REPORT_PATH}.migrations.partial.$$"
  restore_row_counts_file="${REPORT_PATH}.row-counts.partial.$$"

  validate_checksum_set
  validate_manifest

  existing_database="$(psql_admin -qAt -v rehearsal_db="$REHEARSAL_DB" <<'SQL'
SELECT count(*) FROM pg_database WHERE datname = :'rehearsal_db';
SQL
)"
  if [ "$existing_database" != "0" ]; then
    echo "error: rehearsal database already exists; choose a fresh isolated name" >&2
    exit 1
  fi

  echo "creating isolated rehearsal database: $REHEARSAL_DB" >&2
  psql_admin -q -v rehearsal_db="$REHEARSAL_DB" <<'SQL' >/dev/null
SELECT format('CREATE DATABASE %I', :'rehearsal_db')
\gexec
SQL
  database_created=true

  echo "restoring verified backup into the isolated database" >&2
  gzip -dc "$BACKUP_FILE" | psql_rehearsal -q >/dev/null
  capture_restored_invariants

  if [ "$KEEP_REHEARSAL_DB" = "1" ]; then
    cleanup_state="retained"
  else
    drop_rehearsal_database
    cleanup_state="dropped"
  fi
  write_report "$cleanup_state"

  echo "restore rehearsal succeeded; report: $REPORT_PATH" >&2
}

main "$@"
