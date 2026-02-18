#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

BACKUP_DIR="${BITRIVER_BACKUP_DIR:-$REPO_ROOT/data/backups/postgres}"
TIMESTAMP="$(date -u +"%Y%m%dT%H%M%SZ")"
BACKUP_BASENAME="bitriver-postgres-${TIMESTAMP}.sql.gz"
BACKUP_PATH="$BACKUP_DIR/$BACKUP_BASENAME"
CHECKSUM_PATH="$BACKUP_PATH.sha256"

POSTGRES_HOST="${BITRIVER_BACKUP_POSTGRES_HOST:-${BITRIVER_POSTGRES_HOST:-localhost}}"
POSTGRES_PORT="${BITRIVER_BACKUP_POSTGRES_PORT:-${BITRIVER_POSTGRES_PORT:-5432}}"
POSTGRES_DB="${BITRIVER_BACKUP_POSTGRES_DB:-${BITRIVER_POSTGRES_DB:-bitriver}}"
POSTGRES_USER="${BITRIVER_BACKUP_POSTGRES_USER:-${BITRIVER_POSTGRES_USER:-bitriver}}"
POSTGRES_PASSWORD="${BITRIVER_BACKUP_POSTGRES_PASSWORD:-${BITRIVER_POSTGRES_PASSWORD:-}}"

UPLOAD_ENABLED="${BITRIVER_BACKUP_UPLOAD_ENABLED:-0}"
UPLOAD_PROVIDER="${BITRIVER_BACKUP_UPLOAD_PROVIDER:-s3}"
UPLOAD_BUCKET="${BITRIVER_BACKUP_UPLOAD_BUCKET:-}"
UPLOAD_PREFIX="${BITRIVER_BACKUP_UPLOAD_PREFIX:-bitriver-live/postgres}"
UPLOAD_REGION="${BITRIVER_BACKUP_UPLOAD_REGION:-us-east-1}"
UPLOAD_ENDPOINT="${BITRIVER_BACKUP_UPLOAD_ENDPOINT:-}"

run_prune="${BITRIVER_BACKUP_RUN_PRUNE:-1}"

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: required command not found: $1" >&2
    exit 1
  fi
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
  local checksum_destination="${destination}.sha256"
  local aws_args=("--region" "$UPLOAD_REGION")

  if [ -n "$UPLOAD_ENDPOINT" ]; then
    aws_args+=("--endpoint-url" "$UPLOAD_ENDPOINT")
  fi

  echo "uploading backup to ${destination}" >&2
  aws s3 cp "$BACKUP_PATH" "$destination" "${aws_args[@]}"
  aws s3 cp "$CHECKSUM_PATH" "$checksum_destination" "${aws_args[@]}"
}

main() {
  require_command pg_dump
  require_command gzip
  require_command sha256sum

  mkdir -p "$BACKUP_DIR"

  echo "creating postgres backup at $BACKUP_PATH" >&2
  PGPASSWORD="$POSTGRES_PASSWORD" pg_dump \
    --host "$POSTGRES_HOST" \
    --port "$POSTGRES_PORT" \
    --username "$POSTGRES_USER" \
    --dbname "$POSTGRES_DB" \
    --clean \
    --if-exists \
    --no-owner \
    --no-privileges | gzip -c > "$BACKUP_PATH"

  sha256sum "$BACKUP_PATH" > "$CHECKSUM_PATH"
  echo "backup complete: $BACKUP_PATH" >&2

  upload_backup

  if [ "$run_prune" = "1" ]; then
    "$SCRIPT_DIR/prune-backups.sh"
  fi
}

main "$@"
