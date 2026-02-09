#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
  cat <<'USAGE'
Usage: ./scripts/sync-helm-deploy-assets.sh [--check]

Copies canonical deploy assets into Helm chart generated copies.

  --check   Verify generated Helm files match canonical sources without writing files.
USAGE
}

check_mode=0
if [[ $# -gt 1 ]]; then
  usage >&2
  exit 2
fi
if [[ $# -eq 1 ]]; then
  case "$1" in
    --check)
      check_mode=1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      usage >&2
      exit 2
      ;;
  esac
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
canonical_migrations_dir="$repo_root/deploy/migrations"
canonical_srs_conf="$repo_root/deploy/srs/conf/srs.conf"
helm_chart_dir="$repo_root/deploy/helm/bitriver-live"
helm_migrations_dir="$helm_chart_dir/migrations"
helm_srs_conf="$helm_chart_dir/files/srs.conf"

if [[ ! -d "$canonical_migrations_dir" ]]; then
  echo "Missing canonical migrations directory: $canonical_migrations_dir" >&2
  exit 1
fi
if [[ ! -f "$canonical_srs_conf" ]]; then
  echo "Missing canonical SRS config: $canonical_srs_conf" >&2
  exit 1
fi

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

mkdir -p "$workdir/migrations"

{
  echo "# GENERATED FILE: DO NOT EDIT DIRECTLY"
  echo "# Canonical source: deploy/srs/conf/srs.conf"
  echo "# Regenerate with: ./scripts/sync-helm-deploy-assets.sh"
  echo
  cat "$canonical_srs_conf"
} > "$workdir/srs.conf"

shopt -s nullglob
migration_files=("$canonical_migrations_dir"/*.sql)
if [[ ${#migration_files[@]} -eq 0 ]]; then
  echo "No canonical migration files found in $canonical_migrations_dir" >&2
  exit 1
fi
for src in "${migration_files[@]}"; do
  base="$(basename "$src")"
  {
    echo "-- GENERATED FILE: DO NOT EDIT DIRECTLY"
    echo "-- Canonical source: deploy/migrations/$base"
    echo "-- Regenerate with: ./scripts/sync-helm-deploy-assets.sh"
    echo
    cat "$src"
  } > "$workdir/migrations/$base"
done
shopt -u nullglob

if [[ $check_mode -eq 1 ]]; then
  if ! cmp -s "$workdir/srs.conf" "$helm_srs_conf"; then
    echo "Drift detected: $helm_srs_conf does not match canonical source." >&2
    echo "Run ./scripts/sync-helm-deploy-assets.sh" >&2
    exit 1
  fi

  expected_set="$(printf '%s\n' "${migration_files[@]}" | xargs -n1 basename | sort)"
  shopt -s nullglob
  actual_files=("$helm_migrations_dir"/*.sql)
  shopt -u nullglob
  actual_set="$(printf '%s\n' "${actual_files[@]##*/}" | sed '/^$/d' | sort)"
  if [[ "$expected_set" != "$actual_set" ]]; then
    echo "Drift detected: Helm migration file set differs from canonical migrations." >&2
    echo "Run ./scripts/sync-helm-deploy-assets.sh" >&2
    exit 1
  fi

  while IFS= read -r base; do
    [[ -z "$base" ]] && continue
    if ! cmp -s "$workdir/migrations/$base" "$helm_migrations_dir/$base"; then
      echo "Drift detected: $helm_migrations_dir/$base does not match canonical source." >&2
      echo "Run ./scripts/sync-helm-deploy-assets.sh" >&2
      exit 1
    fi
  done < <(printf '%s\n' "$expected_set")

  echo "Helm generated deploy assets are in sync."
  exit 0
fi

mkdir -p "$helm_chart_dir/files" "$helm_migrations_dir"
cp "$workdir/srs.conf" "$helm_srs_conf"

find "$helm_migrations_dir" -maxdepth 1 -type f -name '*.sql' -delete
cp "$workdir/migrations"/*.sql "$helm_migrations_dir/"

echo "Synced Helm generated deploy assets from canonical deploy sources."
