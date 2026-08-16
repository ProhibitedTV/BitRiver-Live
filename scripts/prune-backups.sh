#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

BACKUP_DIR="${BITRIVER_BACKUP_DIR:-$REPO_ROOT/data/backups/postgres}"
RETENTION_DAYS="${BITRIVER_BACKUP_RETENTION_DAYS:-14}"
KEEP_MIN="${BITRIVER_BACKUP_KEEP_MIN:-3}"

if [ ! -d "$BACKUP_DIR" ]; then
  echo "backup directory does not exist, skipping prune: $BACKUP_DIR" >&2
  exit 0
fi

mapfile -t backups < <(find "$BACKUP_DIR" -maxdepth 1 -type f -name 'bitriver-postgres-*.sql.gz' | sort)

if [ "${#backups[@]}" -le "$KEEP_MIN" ]; then
  echo "found ${#backups[@]} backups, keep_min=$KEEP_MIN; nothing to prune" >&2
  exit 0
fi

mapfile -t expired < <(find "$BACKUP_DIR" -maxdepth 1 -type f -name 'bitriver-postgres-*.sql.gz' -mtime "+$RETENTION_DAYS" | sort)

if [ "${#expired[@]}" -eq 0 ]; then
  echo "no backups older than ${RETENTION_DAYS} days" >&2
  exit 0
fi

remaining="${#backups[@]}"
for file in "${expired[@]}"; do
  if [ "$remaining" -le "$KEEP_MIN" ]; then
    echo "reached keep_min=$KEEP_MIN, stopping prune" >&2
    break
  fi

  echo "pruning backup $file" >&2
  rm -f "$file" "$file.manifest.json" "$file.sha256"
  remaining=$((remaining - 1))
done
