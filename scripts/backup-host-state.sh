#!/usr/bin/env bash
set -Eeuo pipefail

umask 077

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
helper="$script_dir/host_recovery.py"
root_prefix="${BITRIVER_HOST_BACKUP_ROOT_PREFIX:-/}"
output_dir="${BITRIVER_HOST_BACKUP_DIR:-/var/backups/bitriver-live}"
postgres_backup="${BITRIVER_HOST_BACKUP_POSTGRES_FILE:-}"
source_release="${BITRIVER_BACKUP_SOURCE_RELEASE:-}"
source_commit="${BITRIVER_BACKUP_SOURCE_COMMIT:-}"
passphrase_file="${BITRIVER_HOST_BACKUP_PASSPHRASE_FILE:-}"
object_inventory="${BITRIVER_HOST_BACKUP_OBJECT_INVENTORY:-}"
timestamp="${BITRIVER_HOST_BACKUP_TIMESTAMP:-$(date -u +"%Y%m%dT%H%M%SZ")}"
created_at="${BITRIVER_HOST_BACKUP_CREATED_AT:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"
iterations="${BITRIVER_HOST_BACKUP_PBKDF2_ITERATIONS:-200000}"

usage() {
  cat <<'USAGE'
Usage: ./scripts/backup-host-state.sh [options]

Required:
  --postgres-backup FILE  Complete bitriver-postgres-*.sql.gz backup set.
  --source-release TAG    Exact v-prefixed release identity.
  --source-commit SHA     Exact 40-character release commit.
  --passphrase-file FILE  Restricted non-empty OpenSSL passphrase file.

Options:
  --root-prefix DIR       Prefix containing etc/ and var/ (default /).
  --output-dir DIR        Recovery-set destination.
  --object-inventory FILE Optional bitriver.object-inventory/v1 document.
  --timestamp VALUE       UTC filename timestamp for deterministic scheduling.
  -h, --help

The command atomically publishes an encrypted archive, a secret-safe manifest,
and a checksum file. It never writes a plaintext host-state archive.
USAGE
}

fail() {
  echo "error: $*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

while (($# > 0)); do
  case "$1" in
    --root-prefix) [[ $# -ge 2 ]] || fail "--root-prefix requires a value"; root_prefix=$2; shift 2 ;;
    --output-dir) [[ $# -ge 2 ]] || fail "--output-dir requires a value"; output_dir=$2; shift 2 ;;
    --postgres-backup) [[ $# -ge 2 ]] || fail "--postgres-backup requires a value"; postgres_backup=$2; shift 2 ;;
    --source-release) [[ $# -ge 2 ]] || fail "--source-release requires a value"; source_release=$2; shift 2 ;;
    --source-commit) [[ $# -ge 2 ]] || fail "--source-commit requires a value"; source_commit=$2; shift 2 ;;
    --passphrase-file) [[ $# -ge 2 ]] || fail "--passphrase-file requires a value"; passphrase_file=$2; shift 2 ;;
    --object-inventory) [[ $# -ge 2 ]] || fail "--object-inventory requires a value"; object_inventory=$2; shift 2 ;;
    --timestamp) [[ $# -ge 2 ]] || fail "--timestamp requires a value"; timestamp=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) fail "unknown argument: $1" ;;
  esac
done

require_command openssl
require_command cp
require_command realpath
require_command sha256sum
require_command tar
[[ -f $helper ]] || fail "host recovery helper is missing: $helper"
[[ -n $postgres_backup ]] || fail "--postgres-backup is required"
[[ -n $source_release ]] || fail "--source-release is required"
[[ -n $source_commit ]] || fail "--source-commit is required"
[[ -n $passphrase_file ]] || fail "--passphrase-file is required"
[[ $timestamp =~ ^[0-9]{8}T[0-9]{6}Z$ ]] || fail "timestamp must match YYYYMMDDTHHMMSSZ"
if [[ ! $iterations =~ ^[0-9]+$ ]] || ((iterations < 100000)); then
  fail "PBKDF2 iterations must be an integer of at least 100000"
fi

root_prefix="$(realpath "$root_prefix")"
[[ -d $root_prefix/etc/bitriver-live ]] || fail "missing packaged-host configuration directory"
[[ -d $root_prefix/var/lib/bitriver-live ]] || fail "missing packaged-host data directory"
postgres_backup="$(realpath "$postgres_backup")"
[[ -f $postgres_backup ]] || fail "Postgres backup archive is missing"
[[ -f ${postgres_backup}.manifest.json ]] || fail "Postgres backup manifest is missing"
[[ -f ${postgres_backup}.sha256 ]] || fail "Postgres backup checksum is missing"
[[ -f $passphrase_file && ! -L $passphrase_file ]] || fail "passphrase file must be a regular non-symlink file"
passphrase_file="$(realpath "$passphrase_file")"
passphrase_size="$(wc -c <"$passphrase_file" | tr -d '[:space:]')"
if [[ ! $passphrase_size =~ ^[0-9]+$ ]] || ((passphrase_size < 20)); then
  fail "passphrase file must contain at least 20 bytes"
fi
passphrase_mode="$(stat -c '%a' "$passphrase_file")"
(( (8#$passphrase_mode & 077) == 0 )) || fail "passphrase file must not be accessible by group or others"

if [[ -n $object_inventory ]]; then
  object_inventory="$(realpath "$object_inventory")"
  [[ -f $object_inventory ]] || fail "object inventory is missing"
fi

mkdir -p "$output_dir"
output_dir="$(realpath "$output_dir")"
case "$output_dir/" in
  "$root_prefix/etc/bitriver-live/"*|"$root_prefix/var/lib/bitriver-live/"*)
    fail "backup output must be outside the protected host-state roots"
    ;;
esac

basename="bitriver-host-${timestamp}.tar.gz.enc"
archive_path="$output_dir/$basename"
manifest_path="$archive_path.manifest.json"
checksum_path="$archive_path.sha256"
lock_path="$output_dir/.${basename}.lock"
workdir="$output_dir/.${basename}.work.$$"
published=false
owns_final_assets=false

cleanup() {
  rm -rf -- "$workdir"
  rmdir -- "$lock_path" 2>/dev/null || true
  if [[ $owns_final_assets == true && $published != true ]]; then
    rm -f -- "$archive_path" "$manifest_path" "$checksum_path"
  fi
}
trap cleanup EXIT

mkdir "$lock_path" 2>/dev/null || fail "refusing concurrent host backup set for timestamp $timestamp"
if [[ -e $archive_path || -e $manifest_path || -e $checksum_path ]]; then
  fail "refusing to replace an existing host backup set for timestamp $timestamp"
fi
owns_final_assets=true
mkdir "$workdir"

work_archive="$workdir/$basename"
work_manifest="$workdir/$basename.manifest.json"
work_checksum="$workdir/$basename.sha256"
postgres_name="$(basename "$postgres_backup")"
postgres_manifest_name="$(basename "${postgres_backup}.manifest.json")"
postgres_checksum_name="$(basename "${postgres_backup}.sha256")"
snapshot_root="$workdir/snapshot"
snapshot_postgres_dir="$snapshot_root/var/backups/bitriver-live/recovery/postgres"
snapshot_postgres="$snapshot_postgres_dir/$postgres_name"
postgres_relative="var/backups/bitriver-live/recovery/postgres/$postgres_name"
postgres_manifest_relative="var/backups/bitriver-live/recovery/postgres/$postgres_manifest_name"
postgres_checksum_relative="var/backups/bitriver-live/recovery/postgres/$postgres_checksum_name"

bash "$script_dir/python.sh" "$helper" preflight-host \
  --root-prefix "$root_prefix"
mkdir -p \
  "$snapshot_root/etc" \
  "$snapshot_root/var/lib" \
  "$snapshot_postgres_dir"
cp -aL -- "$root_prefix/etc/bitriver-live" "$snapshot_root/etc/"
cp -aL -- "$root_prefix/var/lib/bitriver-live" "$snapshot_root/var/lib/"
cp -p -- \
  "$postgres_backup" \
  "${postgres_backup}.manifest.json" \
  "${postgres_backup}.sha256" \
  "$snapshot_postgres_dir/"

tar_args=(
  --dereference
  --hard-dereference
  --format=pax
  -czf -
  -C "$snapshot_root"
  etc/bitriver-live
  var/lib/bitriver-live
  "$postgres_relative"
  "$postgres_manifest_relative"
  "$postgres_checksum_relative"
)

object_args=()
if [[ -n $object_inventory ]]; then
  snapshot_object="$snapshot_root/var/backups/bitriver-live/recovery/object-inventory.json"
  cp -p -- "$object_inventory" "$snapshot_object"
  tar_args+=(var/backups/bitriver-live/recovery/object-inventory.json)
  object_args=(--object-inventory "$snapshot_object")
fi

tar "${tar_args[@]}" |
  openssl enc -aes-256-cbc -salt -pbkdf2 -iter "$iterations" \
    -pass "file:$passphrase_file" -out "$work_archive"

bash "$script_dir/python.sh" "$helper" backup-manifest \
  --root-prefix "$snapshot_root" \
  --archive "$work_archive" \
  --postgres-backup "$snapshot_postgres" \
  --source-release "$source_release" \
  --source-commit "$source_commit" \
  --created-at "$created_at" \
  --iterations "$iterations" \
  "${object_args[@]}" \
  --output "$work_manifest"

archive_sha="$(sha256sum "$work_archive")"
manifest_sha="$(sha256sum "$work_manifest")"
{
  printf '%s  %s\n' "${archive_sha%% *}" "$basename"
  printf '%s  %s\n' "${manifest_sha%% *}" "$basename.manifest.json"
} >"$work_checksum"

mv -- "$work_archive" "$archive_path"
mv -- "$work_manifest" "$manifest_path"
mv -- "$work_checksum" "$checksum_path"
bash "$script_dir/python.sh" "$helper" verify-backup \
  --archive "$archive_path" \
  --expected-release "$source_release" \
  --expected-commit "$source_commit" >/dev/null
published=true
echo "encrypted host backup set complete: $archive_path"
