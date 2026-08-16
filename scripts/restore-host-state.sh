#!/usr/bin/env bash
set -Eeuo pipefail

umask 077

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
helper="$script_dir/host_recovery.py"
archive="${BITRIVER_HOST_RESTORE_ARCHIVE:-}"
root_prefix="${BITRIVER_HOST_RESTORE_ROOT_PREFIX:-/}"
expected_release="${BITRIVER_HOST_RESTORE_EXPECT_RELEASE:-}"
expected_commit="${BITRIVER_HOST_RESTORE_EXPECT_COMMIT:-}"
passphrase_file="${BITRIVER_HOST_RESTORE_PASSPHRASE_FILE:-}"
report_path="${BITRIVER_HOST_RESTORE_REPORT_PATH:-}"

usage() {
  cat <<'USAGE'
Usage: ./scripts/restore-host-state.sh [options]

Required:
  --archive FILE          bitriver-host-*.tar.gz.enc recovery archive.
  --expected-release TAG  Exact v-prefixed release identity.
  --expected-commit SHA   Exact 40-character release commit.
  --passphrase-file FILE  Restricted OpenSSL passphrase file.

Options:
  --root-prefix DIR       Fresh prefix receiving etc/ and var/ (default /).
  --report FILE           Secret-safe measured restore report path.
  -h, --help

Restore verifies the complete encrypted set and every archive member before it
creates packaged-host state. The target etc/bitriver-live and
var/lib/bitriver-live paths must not already exist.
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
    --archive) [[ $# -ge 2 ]] || fail "--archive requires a value"; archive=$2; shift 2 ;;
    --root-prefix) [[ $# -ge 2 ]] || fail "--root-prefix requires a value"; root_prefix=$2; shift 2 ;;
    --expected-release) [[ $# -ge 2 ]] || fail "--expected-release requires a value"; expected_release=$2; shift 2 ;;
    --expected-commit) [[ $# -ge 2 ]] || fail "--expected-commit requires a value"; expected_commit=$2; shift 2 ;;
    --passphrase-file) [[ $# -ge 2 ]] || fail "--passphrase-file requires a value"; passphrase_file=$2; shift 2 ;;
    --report) [[ $# -ge 2 ]] || fail "--report requires a value"; report_path=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) fail "unknown argument: $1" ;;
  esac
done

require_command openssl
require_command realpath
require_command tar
[[ -f $helper ]] || fail "host recovery helper is missing: $helper"
[[ -n $archive ]] || fail "--archive is required"
[[ -n $expected_release ]] || fail "--expected-release is required"
[[ -n $expected_commit ]] || fail "--expected-commit is required"
[[ -n $passphrase_file ]] || fail "--passphrase-file is required"
archive="$(realpath "$archive")"
[[ -f $archive ]] || fail "host recovery archive is missing"
[[ -f ${archive}.manifest.json ]] || fail "host recovery manifest is missing"
[[ -f ${archive}.sha256 ]] || fail "host recovery checksum is missing"
[[ -f $passphrase_file && ! -L $passphrase_file ]] || fail "passphrase file must be a regular non-symlink file"
passphrase_file="$(realpath "$passphrase_file")"
passphrase_mode="$(stat -c '%a' "$passphrase_file")"
(( (8#$passphrase_mode & 077) == 0 )) || fail "passphrase file must not be accessible by group or others"

mkdir -p "$root_prefix"
root_prefix="$(realpath "$root_prefix")"
for relative in etc/bitriver-live var/lib/bitriver-live var/backups/bitriver-live/recovery; do
  [[ ! -e $root_prefix/$relative && ! -L $root_prefix/$relative ]] ||
    fail "restore target is not fresh: $root_prefix/$relative already exists"
done
if [[ -z $report_path ]]; then
  report_path="${archive}.restore-report.json"
fi
started_at_epoch="$(date -u +%s)"
iterations="$(
  bash "$script_dir/python.sh" "$helper" verify-backup \
    --archive "$archive" \
    --expected-release "$expected_release" \
    --expected-commit "$expected_commit"
)"
[[ $iterations =~ ^[0-9]+$ ]] || fail "validated backup did not expose a PBKDF2 iteration count"

decrypt=(
  openssl enc -d -aes-256-cbc -pbkdf2 -iter "$iterations"
  -pass "file:$passphrase_file"
  -in "$archive"
)

"${decrypt[@]}" |
  bash "$script_dir/python.sh" "$helper" validate-archive \
    --manifest "${archive}.manifest.json"

"${decrypt[@]}" | tar -xzf - -C "$root_prefix"

bash "$script_dir/python.sh" "$helper" restore-report \
  --archive "$archive" \
  --root-prefix "$root_prefix" \
  --expected-release "$expected_release" \
  --expected-commit "$expected_commit" \
  --started-at-epoch "$started_at_epoch" \
  --output "$report_path"

echo "host state restored and verified: $report_path"
echo "verified Postgres set: $root_prefix/var/backups/bitriver-live/recovery/postgres"
