#!/usr/bin/env bash
set -euo pipefail

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

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: required command not found: $1" >&2
    exit 1
  fi
}

latest_backup() {
  find "$BACKUP_DIR" -maxdepth 1 -type f -name 'bitriver-postgres-*.sql.gz' | sort | tail -n 1
}

psql_admin() {
  PGPASSWORD="$POSTGRES_PASSWORD" psql \
    --host "$POSTGRES_HOST" \
    --port "$POSTGRES_PORT" \
    --username "$POSTGRES_USER" \
    --dbname "$POSTGRES_ADMIN_DB" \
    -v ON_ERROR_STOP=1 \
    "$@"
}

psql_rehearsal() {
  PGPASSWORD="$POSTGRES_PASSWORD" psql \
    --host "$POSTGRES_HOST" \
    --port "$POSTGRES_PORT" \
    --username "$POSTGRES_USER" \
    --dbname "$REHEARSAL_DB" \
    -v ON_ERROR_STOP=1 \
    "$@"
}

cleanup() {
  if [ "$KEEP_REHEARSAL_DB" = "1" ]; then
    echo "keeping rehearsal database: $REHEARSAL_DB" >&2
    return
  fi

  psql_admin -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '${REHEARSAL_DB}' AND pid <> pg_backend_pid();" >/dev/null
  psql_admin -c "DROP DATABASE IF EXISTS \"$REHEARSAL_DB\";" >/dev/null
}

main() {
  require_command psql
  require_command gzip
  require_command sha256sum

  if [ -z "$BACKUP_FILE" ]; then
    BACKUP_FILE="$(latest_backup)"
  fi

  if [ -z "$BACKUP_FILE" ] || [ ! -f "$BACKUP_FILE" ]; then
    echo "error: backup file not found. pass a path or store backups in $BACKUP_DIR" >&2
    exit 1
  fi

  checksum_file="${BACKUP_FILE}.sha256"
  if [ -f "$checksum_file" ]; then
    (cd "$(dirname "$BACKUP_FILE")" && sha256sum -c "$(basename "$checksum_file")")
  else
    echo "warning: checksum file missing: $checksum_file" >&2
  fi

  trap cleanup EXIT INT TERM

  echo "creating rehearsal database: $REHEARSAL_DB" >&2
  psql_admin -c "DROP DATABASE IF EXISTS \"$REHEARSAL_DB\";" >/dev/null
  psql_admin -c "CREATE DATABASE \"$REHEARSAL_DB\";" >/dev/null

  echo "restoring backup into rehearsal database" >&2
  gzip -dc "$BACKUP_FILE" | psql_rehearsal

  echo "running restore smoke queries" >&2
  psql_rehearsal -c "SELECT current_database(), current_timestamp;"
  psql_rehearsal -c "SELECT COUNT(*) AS public_tables FROM information_schema.tables WHERE table_schema = 'public';"
  psql_rehearsal -c "SELECT to_regclass('public.users') IS NOT NULL AS users_table_present;"
  psql_rehearsal -c "SELECT to_regclass('public.channels') IS NOT NULL AS channels_table_present;"

  echo "restore rehearsal succeeded for $BACKUP_FILE" >&2
}

main "$@"
