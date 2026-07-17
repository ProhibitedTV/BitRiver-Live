#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
container="bitriver-migration-test-${RANDOM}-$$"
database="bitriver_migration_test"
password="migration-test-password"
workdir="$(mktemp -d)"

cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
  rm -rf "$workdir"
}
trap cleanup EXIT

fail() {
  echo "Migration integration test failed: $*" >&2
  exit 1
}

assert_contains() {
  local output="$1"
  local expected="$2"
  [[ "$output" == *"$expected"* ]] || fail "expected output to contain '$expected'"
}

docker_exec() {
  # Git Bash otherwise rewrites container-absolute paths such as /migrations.
  MSYS_NO_PATHCONV=1 docker exec "$@"
}

run_migrator() {
  docker_exec \
    -e PGPASSWORD="$password" \
    -e PGHOST=127.0.0.1 \
    -e PGUSER=postgres \
    -e PGDATABASE="$database" \
    -e BITRIVER_MIGRATIONS_DIR=/migrations \
    -e 'BITRIVER_MIGRATION_SANITY_SQL=SELECT 1;' \
    -e BITRIVER_MIGRATION_RELEASE=v-test \
    -e BITRIVER_MIGRATION_COMMIT=test-commit \
    "$container" /bin/sh /runner.sh "$@"
}

psql_test() {
  docker_exec \
    -e PGPASSWORD="$password" \
    "$container" psql -X -v ON_ERROR_STOP=1 -qAt \
    -h 127.0.0.1 -U postgres -d "$database" "$@"
}

container_checksum() {
  local line
  line="$(docker_exec "$container" sha256sum "/migrations/$1")"
  printf '%s\n' "${line%% *}"
}

mkdir -p "$workdir/migrations"
cat >"$workdir/migrations/0001_initial.sql" <<'SQL'
CREATE TABLE migration_fixture (
  id INTEGER PRIMARY KEY,
  note TEXT NOT NULL
);
SQL
cp "$workdir/migrations/0001_initial.sql" "$workdir/0001_initial.original.sql"

docker run -d --name "$container" \
  -e POSTGRES_PASSWORD="$password" \
  -e POSTGRES_DB="$database" \
  postgres:15-alpine >/dev/null

ready=false
for _ in {1..60}; do
  if docker_exec "$container" pg_isready -q -U postgres -d "$database"; then
    ready=true
    break
  fi
  sleep 1
done
[[ "$ready" == true ]] || fail "Postgres did not become ready"

docker_exec "$container" mkdir -p /migrations
docker cp "$repo_root/deploy/postgres-migrate.sh" "$container:/runner.sh" >/dev/null
docker cp "$workdir/migrations/0001_initial.sql" "$container:/migrations/0001_initial.sql" >/dev/null

plan_output="$(run_migrator plan)"
assert_contains "$plan_output" "PENDING 0001_initial.sql"
assert_contains "$plan_output" "ledger absent"
[[ "$(psql_test -c "SELECT to_regclass('public.schema_migrations') IS NULL;")" == "t" ]] || fail "read-only plan created the ledger"

fresh_output="$(run_migrator apply)"
assert_contains "$fresh_output" "Applied 0001_initial.sql"
[[ "$(psql_test -c "SELECT COUNT(*) FROM public.schema_migrations WHERE status = 'applied';")" == "1" ]] || fail "fresh apply did not record one migration"

cat >"$workdir/migrations/0002_upgrade.sql" <<'SQL'
ALTER TABLE migration_fixture ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
SQL
docker cp "$workdir/migrations/0002_upgrade.sql" "$container:/migrations/0002_upgrade.sql" >/dev/null

upgrade_plan="$(run_migrator plan)"
assert_contains "$upgrade_plan" "APPLIED 0001_initial.sql"
assert_contains "$upgrade_plan" "PENDING 0002_upgrade.sql"
upgrade_output="$(run_migrator apply)"
assert_contains "$upgrade_output" "Applied 0002_upgrade.sql"
[[ "$(psql_test -c "SELECT COUNT(*) FROM public.schema_migrations WHERE status = 'applied';")" == "2" ]] || fail "previous-schema upgrade did not apply only the pending migration"

noop_output="$(run_migrator apply)"
assert_contains "$noop_output" "Skipping applied migration 0001_initial.sql"
assert_contains "$noop_output" "Skipping applied migration 0002_upgrade.sql"

cat >"$workdir/migrations/0001_initial.sql" <<'SQL'
CREATE TABLE migration_fixture (id INTEGER PRIMARY KEY, note TEXT NOT NULL, edited_history BOOLEAN);
SQL
docker cp "$workdir/migrations/0001_initial.sql" "$container:/migrations/0001_initial.sql" >/dev/null
set +e
drift_output="$(run_migrator plan 2>&1)"
drift_rc=$?
set -e
[[ "$drift_rc" -ne 0 ]] || fail "checksum drift plan unexpectedly succeeded"
assert_contains "$drift_output" "DRIFT 0001_initial.sql"
docker cp "$workdir/0001_initial.original.sql" "$container:/migrations/0001_initial.sql" >/dev/null

cat >"$workdir/migrations/0003_requires_gate.sql" <<'SQL'
CREATE TABLE recovered_failure AS SELECT id FROM migration_recovery_gate;
SQL
docker cp "$workdir/migrations/0003_requires_gate.sql" "$container:/migrations/0003_requires_gate.sql" >/dev/null
set +e
failed_output="$(run_migrator apply 2>&1)"
failed_rc=$?
set -e
[[ "$failed_rc" -ne 0 ]] || fail "invalid migration unexpectedly succeeded"
assert_contains "$failed_output" "ledger status is failed"
[[ "$(psql_test -c "SELECT status FROM public.schema_migrations WHERE filename = '0003_requires_gate.sql';")" == "failed" ]] || fail "failed migration status was not recorded"
[[ "$(psql_test -c "SELECT to_regclass('public.recovered_failure') IS NULL;")" == "t" ]] || fail "failed migration left partial table state"

psql_test -c "CREATE TABLE migration_recovery_gate (id INTEGER PRIMARY KEY);" >/dev/null
failed_checksum="$(container_checksum 0003_requires_gate.sql)"
retry_output="$(run_migrator repair retry 0003_requires_gate.sql "$failed_checksum")"
assert_contains "$retry_output" "Applied 0003_requires_gate.sql"
[[ "$(psql_test -c "SELECT status FROM public.schema_migrations WHERE filename = '0003_requires_gate.sql';")" == "applied" ]] || fail "retry did not repair failed status"

cat >"$workdir/migrations/0004_interrupted.sql" <<'SQL'
CREATE TABLE recovered_interruption (id INTEGER PRIMARY KEY);
SQL
docker cp "$workdir/migrations/0004_interrupted.sql" "$container:/migrations/0004_interrupted.sql" >/dev/null
interrupted_checksum="$(container_checksum 0004_interrupted.sql)"
psql_test \
  -c "INSERT INTO public.schema_migrations (filename, version, checksum_sha256, status, release_version, release_commit) VALUES ('0004_interrupted.sql', '0004', '$interrupted_checksum', 'applying', 'v-test', 'test-commit');" >/dev/null
psql_test --single-transaction -f /migrations/0004_interrupted.sql >/dev/null
set +e
interrupted_plan="$(run_migrator plan 2>&1)"
interrupted_rc=$?
set -e
[[ "$interrupted_rc" -ne 0 ]] || fail "ambiguous applying state unexpectedly passed preflight"
assert_contains "$interrupted_plan" "BLOCKED 0004_interrupted.sql status=applying"
mark_output="$(run_migrator repair mark-applied 0004_interrupted.sql "$interrupted_checksum")"
assert_contains "$mark_output" "Marked 0004_interrupted.sql applied"

status_output="$(run_migrator status)"
assert_contains "$status_output" "filename|version|checksum_sha256|status|applied_at|release|commit"
assert_contains "$status_output" "0004_interrupted.sql|0004|$interrupted_checksum|applied"
[[ "$status_output" != *"$password"* ]] || fail "status output leaked the database password"
[[ "$status_output" != *"migration-test-password"* ]] || fail "status output leaked a credential value"

echo "Postgres migration lifecycle tests passed."
