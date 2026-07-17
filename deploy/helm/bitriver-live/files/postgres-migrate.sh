#!/bin/sh
set -eu

export LC_ALL=C

MIGRATIONS_DIR="${BITRIVER_MIGRATIONS_DIR:-/migrations}"
RELEASE_VERSION="${BITRIVER_MIGRATION_RELEASE:-unknown}"
RELEASE_COMMIT="${BITRIVER_MIGRATION_COMMIT:-unknown}"
SANITY_SQL="${BITRIVER_MIGRATION_SANITY_SQL:-SELECT 1 FROM users LIMIT 1;}"
WAIT_TIMEOUT_SECONDS="${BITRIVER_MIGRATION_WAIT_TIMEOUT_SECONDS:-120}"

usage() {
  cat <<'USAGE'
Usage:
  postgres-migrate.sh apply
  postgres-migrate.sh plan
  postgres-migrate.sh status
  postgres-migrate.sh repair retry <filename> <sha256>
  postgres-migrate.sh repair mark-applied <filename> <sha256>

The repair commands require the exact recorded SHA-256 checksum. Use retry only
after confirming the failed transaction rolled back or partial state was cleaned
up. Use mark-applied only after manually verifying the migration's schema effects.
USAGE
}

fail() {
  echo "Migration error: $*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

psql_cmd() {
  if [ -n "${PGPORT:-}" ]; then
    psql -X -v ON_ERROR_STOP=1 -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDATABASE" "$@"
  else
    psql -X -v ON_ERROR_STOP=1 -h "$PGHOST" -U "$PGUSER" -d "$PGDATABASE" "$@"
  fi
}

psql_value() {
  psql_cmd -qAt "$@"
}

postgres_is_ready() {
  if [ -n "${PGPORT:-}" ]; then
    pg_isready -q -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDATABASE"
  else
    pg_isready -q -h "$PGHOST" -U "$PGUSER" -d "$PGDATABASE"
  fi
}

wait_for_postgres() {
  case "$WAIT_TIMEOUT_SECONDS" in
    ''|*[!0-9]*) fail "BITRIVER_MIGRATION_WAIT_TIMEOUT_SECONDS must be a positive integer" ;;
  esac
  [ "$WAIT_TIMEOUT_SECONDS" -gt 0 ] || fail "BITRIVER_MIGRATION_WAIT_TIMEOUT_SECONDS must be greater than zero"

  waited=0
  until postgres_is_ready; do
    if [ "$waited" -ge "$WAIT_TIMEOUT_SECONDS" ]; then
      fail "Postgres did not become ready within ${WAIT_TIMEOUT_SECONDS}s at $PGHOST"
    fi
    echo "Waiting for Postgres at $PGHOST (${waited}s/${WAIT_TIMEOUT_SECONDS}s)..."
    sleep 2
    waited=$((waited + 2))
  done
}

validate_filename() {
  filename="$1"
  case "$filename" in
    [0-9][0-9][0-9][0-9]_*.sql) ;;
    *) fail "invalid migration filename '$filename'; expected NNNN_name.sql" ;;
  esac
  case "$filename" in
    *[!A-Za-z0-9_.-]*) fail "invalid characters in migration filename '$filename'" ;;
  esac
}

version_for() {
  printf '%s\n' "${1%%_*}"
}

checksum_for() {
  checksum_line="$(sha256sum "$1")"
  printf '%s\n' "${checksum_line%% *}"
}

ledger_exists() {
  [ "$(psql_value -c "SELECT CASE WHEN to_regclass('public.schema_migrations') IS NULL THEN 'no' ELSE 'yes' END;")" = "yes" ]
}

ensure_ledger() {
  psql_cmd -q <<'SQL'
CREATE TABLE IF NOT EXISTS public.schema_migrations (
    filename TEXT PRIMARY KEY,
    version TEXT NOT NULL,
    checksum_sha256 TEXT NOT NULL CHECK (checksum_sha256 ~ '^[0-9a-f]{64}$'),
    status TEXT NOT NULL CHECK (status IN ('applying', 'applied', 'failed')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    applied_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    release_version TEXT NOT NULL DEFAULT 'unknown',
    release_commit TEXT NOT NULL DEFAULT 'unknown',
    failure_reason TEXT NOT NULL DEFAULT ''
);
SQL
}

ledger_row() {
  psql_value -v migration_file="$1" <<'SQL'
SELECT checksum_sha256 || '|' || status
FROM public.schema_migrations
WHERE filename = :'migration_file';
SQL
}

check_removed_files() {
  ledger_files="$(psql_value -c "SELECT filename FROM public.schema_migrations ORDER BY filename;")"
  for filename in $ledger_files; do
    validate_filename "$filename"
    [ -f "$MIGRATIONS_DIR/$filename" ] || fail "ledger contains '$filename' but the canonical SQL file is missing; restore the release's migration set"
  done
}

print_status() {
  if ! ledger_exists; then
    echo "Migration ledger: absent"
    return 0
  fi

  echo "Migration ledger (non-sensitive):"
  echo "filename|version|checksum_sha256|status|applied_at|release|commit"
  psql_cmd -qAt -F '|' -c \
    "SELECT filename, version, checksum_sha256, status, COALESCE(applied_at::text, ''), release_version, release_commit FROM public.schema_migrations ORDER BY filename;"
}

run_plan() {
  echo "Migration plan:"
  plan_failed=0
  found=0
  has_ledger=0
  if ledger_exists; then
    has_ledger=1
    check_removed_files
  fi

  for file in "$MIGRATIONS_DIR"/*.sql; do
    [ -f "$file" ] || continue
    found=1
    filename="$(basename "$file")"
    validate_filename "$filename"
    checksum="$(checksum_for "$file")"

    if [ "$has_ledger" -eq 0 ]; then
      echo "PENDING $filename sha256=$checksum (ledger absent)"
      continue
    fi

    row="$(ledger_row "$filename")"
    if [ -z "$row" ]; then
      echo "PENDING $filename sha256=$checksum"
      continue
    fi

    recorded_checksum="${row%%|*}"
    status="${row#*|}"
    if [ "$recorded_checksum" != "$checksum" ]; then
      echo "DRIFT $filename recorded=$recorded_checksum current=$checksum" >&2
      plan_failed=1
      continue
    fi

    case "$status" in
      applied) echo "APPLIED $filename sha256=$checksum" ;;
      applying|failed)
        echo "BLOCKED $filename status=$status sha256=$checksum; inspect status and use checksum-confirmed repair" >&2
        plan_failed=1
        ;;
      *)
        echo "BLOCKED $filename has unknown ledger status '$status'" >&2
        plan_failed=1
        ;;
    esac
  done

  [ "$found" -eq 1 ] || fail "no SQL migrations found in $MIGRATIONS_DIR"
  return "$plan_failed"
}

claim_migration() {
  filename="$1"
  version="$2"
  checksum="$3"
  claimed="$(psql_value \
    -v migration_file="$filename" \
    -v migration_version="$version" \
    -v migration_checksum="$checksum" \
    -v migration_release="$RELEASE_VERSION" \
    -v migration_commit="$RELEASE_COMMIT" <<'SQL'
INSERT INTO public.schema_migrations (
    filename, version, checksum_sha256, status, release_version, release_commit
) VALUES (
    :'migration_file', :'migration_version', :'migration_checksum', 'applying',
    :'migration_release', :'migration_commit'
)
ON CONFLICT (filename) DO NOTHING
RETURNING 'claimed';
SQL
)"
  [ "$claimed" = "claimed" ] || fail "could not claim '$filename'; another runner or existing ledger state won the race"
}

set_migration_state() {
  filename="$1"
  status="$2"
  reason="$3"
  psql_cmd -q \
    -v migration_file="$filename" \
    -v migration_status="$status" \
    -v migration_reason="$reason" \
    -v migration_release="$RELEASE_VERSION" \
    -v migration_commit="$RELEASE_COMMIT" <<'SQL'
UPDATE public.schema_migrations
SET status = :'migration_status',
    applied_at = CASE WHEN :'migration_status' = 'applied' THEN NOW() ELSE NULL END,
    updated_at = NOW(),
    release_version = :'migration_release',
    release_commit = :'migration_commit',
    failure_reason = :'migration_reason'
WHERE filename = :'migration_file';
SQL
}

execute_claimed_migration() {
  file="$1"
  filename="$2"
  checksum="$3"
  echo "Applying $filename sha256=$checksum release=$RELEASE_VERSION commit=$RELEASE_COMMIT"
  if psql_cmd --single-transaction -f "$file"; then
    set_migration_state "$filename" applied ""
    echo "Applied $filename"
    return 0
  else
    migration_rc=$?
  fi

  set_migration_state "$filename" failed "psql exited with status $migration_rc"
  echo "Migration $filename failed; ledger status is failed. Fix the cause, verify rollback/partial state, then use checksum-confirmed repair." >&2
  return "$migration_rc"
}

run_apply() {
  ensure_ledger
  if ! run_plan; then
    fail "preflight found checksum drift or unresolved migration state"
  fi

  for file in "$MIGRATIONS_DIR"/*.sql; do
    [ -f "$file" ] || continue
    filename="$(basename "$file")"
    validate_filename "$filename"
    checksum="$(checksum_for "$file")"
    row="$(ledger_row "$filename")"
    if [ -n "$row" ]; then
      echo "Skipping applied migration $filename"
      continue
    fi

    version="$(version_for "$filename")"
    claim_migration "$filename" "$version" "$checksum"
    execute_claimed_migration "$file" "$filename" "$checksum" || exit $?
  done

  psql_cmd -q -c "$SANITY_SQL" >/dev/null
  echo "Post-migration sanity check passed."
  print_status
}

run_repair() {
  action="${1:-}"
  filename="${2:-}"
  confirmed_checksum="${3:-}"
  [ -n "$action" ] && [ -n "$filename" ] && [ -n "$confirmed_checksum" ] || {
    usage >&2
    exit 2
  }
  validate_filename "$filename"
  file="$MIGRATIONS_DIR/$filename"
  [ -f "$file" ] || fail "migration file not found: $filename"
  ledger_exists || fail "migration ledger does not exist"

  actual_checksum="$(checksum_for "$file")"
  row="$(ledger_row "$filename")"
  [ -n "$row" ] || fail "migration is not present in the ledger: $filename"
  recorded_checksum="${row%%|*}"
  status="${row#*|}"
  [ "$actual_checksum" = "$recorded_checksum" ] || fail "checksum drift for '$filename'; restore the canonical SQL before recovery"
  [ "$confirmed_checksum" = "$recorded_checksum" ] || fail "checksum confirmation does not match the recorded value for '$filename'"

  case "$action" in
    retry)
      [ "$status" = "failed" ] || fail "retry requires failed status; '$filename' is '$status'"
      set_migration_state "$filename" applying "operator-confirmed retry"
      execute_claimed_migration "$file" "$filename" "$actual_checksum" || exit $?
      ;;
    mark-applied)
      case "$status" in
        applying|failed) ;;
        *) fail "mark-applied requires applying or failed status; '$filename' is '$status'" ;;
      esac
      set_migration_state "$filename" applied ""
      echo "Marked $filename applied after checksum-confirmed operator verification."
      ;;
    *) fail "unknown repair action '$action'; expected retry or mark-applied" ;;
  esac

  print_status
}

require_command psql
require_command pg_isready
require_command sha256sum
require_command basename
require_command sleep
[ -n "${PGHOST:-}" ] || fail "required environment variable is empty: PGHOST"
[ -n "${PGUSER:-}" ] || fail "required environment variable is empty: PGUSER"
[ -n "${PGDATABASE:-}" ] || fail "required environment variable is empty: PGDATABASE"
[ -d "$MIGRATIONS_DIR" ] || fail "migration directory does not exist: $MIGRATIONS_DIR"
wait_for_postgres

mode="${1:-apply}"
if [ "$#" -gt 0 ]; then
  shift
fi
case "$mode" in
  apply) [ "$#" -eq 0 ] || { usage >&2; exit 2; }; run_apply ;;
  plan) [ "$#" -eq 0 ] || { usage >&2; exit 2; }; run_plan ;;
  status) [ "$#" -eq 0 ] || { usage >&2; exit 2; }; print_status ;;
  repair) run_repair "$@" ;;
  -h|--help|help) usage ;;
  *) usage >&2; fail "unknown mode '$mode'" ;;
esac
