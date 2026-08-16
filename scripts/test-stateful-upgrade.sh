#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source_release="v1.2.3-rc.19"
source_commit="1e14e3cf7d5f1d949b396d4f7897660575ea468e"
source_release_set="374a4084d1880abab1fa980d528a47bb5e324ed85541248438015fb13f2cc204"
candidate_release="v1.2.3-rc.20"
candidate_commit="9a8516a60c584c96a46b630b55c46df33f46fbdc"
candidate_release_set="dd8eabcea7cf920a6f520e3e472cf44d3e1c7b0b7ad74945904f67ea74a47873"
expected_migration_tree="e3447f18a93b349dc1c1e31095b0b1169fd53920"
expected_migration_runner="a7a03f0c563047fc64144b22fed8f4ead9ae9fb8"

container="bitriver-stateful-upgrade-test-${RANDOM}-$$"
source_database="bitriver_upgrade_source"
interrupted_database="bitriver_upgrade_interrupted"
restored_database="bitriver_upgrade_restored"
password="stateful-upgrade-test-password"
workdir="$(mktemp -d)"
retained_report="${BITRIVER_UPGRADE_REPORT_PATH:-}"

cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
  rm -rf "$workdir"
}
trap cleanup EXIT

fail() {
  echo "Stateful upgrade rehearsal failed: $*" >&2
  exit 1
}

assert_contains() {
  local output="$1"
  local expected="$2"
  [[ "$output" == *"$expected"* ]] || fail "expected output to contain '$expected'"
}

docker_exec() {
  MSYS_NO_PATHCONV=1 docker exec "$@"
}

psql_database() {
  local database="$1"
  shift
  docker_exec -i \
    -e PGPASSWORD="$password" \
    "$container" psql -X -qAt -v ON_ERROR_STOP=1 \
    -h 127.0.0.1 -U postgres -d "$database" "$@"
}

psql_admin() {
  psql_database postgres "$@"
}

run_migrator() {
  local database="$1"
  local release="$2"
  local commit="$3"
  local migrations_dir="$4"
  shift 4
  docker_exec \
    -e PGPASSWORD="$password" \
    -e PGHOST=127.0.0.1 \
    -e PGUSER=postgres \
    -e PGDATABASE="$database" \
    -e BITRIVER_MIGRATIONS_DIR="$migrations_dir" \
    -e 'BITRIVER_MIGRATION_SANITY_SQL=SELECT 1 FROM users LIMIT 1;' \
    -e BITRIVER_MIGRATION_RELEASE="$release" \
    -e BITRIVER_MIGRATION_COMMIT="$commit" \
    "$container" /bin/sh /postgres-migrate.sh "$@"
}

migration_fingerprint() {
  local database="$1"
  local line
  # $PGDATABASE expands inside the container shell.
  # shellcheck disable=SC2016
  line="$(
    docker_exec \
      -e PGPASSWORD="$password" \
      -e PGDATABASE="$database" \
      "$container" /bin/sh -c \
      'psql -X -qAt -F "|" -v ON_ERROR_STOP=1 -h 127.0.0.1 -U postgres -d "$PGDATABASE" -c "SELECT filename, version, checksum_sha256, status, release_version, release_commit FROM public.schema_migrations ORDER BY filename" | sha256sum'
  )"
  printf '%s\n' "${line%% *}"
}

fixture_invariants() {
  local database="$1"
  psql_database "$database" -f /stateful-upgrade-invariants.sql
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

require_command docker
require_command git

if ! git -C "$repo_root" diff --quiet HEAD -- deploy/migrations deploy/postgres-migrate.sh; then
  fail "canonical migration files differ from HEAD; commit or remove the drift before generating release evidence"
fi
untracked_migration_files="$(
  git -C "$repo_root" ls-files --others -- \
    deploy/migrations deploy/postgres-migrate.sh
)"
[[ -z "$untracked_migration_files" ]] ||
  fail "canonical migration files include untracked paths; commit or remove the drift before generating release evidence"

actual_migration_tree="$(git -C "$repo_root" rev-parse HEAD:deploy/migrations)"
actual_migration_runner="$(git -C "$repo_root" rev-parse HEAD:deploy/postgres-migrate.sh)"
[[ "$actual_migration_tree" == "$expected_migration_tree" ]] ||
  fail "canonical migration tree changed; select and classify a new exact upgrade pair"
[[ "$actual_migration_runner" == "$expected_migration_runner" ]] ||
  fail "canonical migration runner changed; select and classify a new exact upgrade pair"

docker run -d --name "$container" \
  -e POSTGRES_PASSWORD="$password" \
  -e POSTGRES_DB="$source_database" \
  postgres:15-alpine >/dev/null

ready=false
for _ in {1..60}; do
  if docker_exec "$container" pg_isready -q -U postgres -d "$source_database"; then
    ready=true
    break
  fi
  sleep 1
done
[[ "$ready" == true ]] || fail "Postgres did not become ready"

docker_exec "$container" apk add --no-cache bash coreutils gzip >/dev/null
docker cp "$repo_root/deploy/migrations" "$container:/migrations" >/dev/null
docker cp "$repo_root/deploy/postgres-migrate.sh" "$container:/postgres-migrate.sh" >/dev/null
docker cp "$repo_root/scripts/backup-postgres.sh" "$container:/backup-postgres.sh" >/dev/null
docker cp "$repo_root/scripts/restore-postgres.sh" "$container:/restore-postgres.sh" >/dev/null
docker cp "$repo_root/scripts/fixtures/stateful-upgrade.sql" "$container:/stateful-upgrade.sql" >/dev/null
docker cp "$repo_root/scripts/fixtures/stateful-upgrade-invariants.sql" "$container:/stateful-upgrade-invariants.sql" >/dev/null

source_plan="$(run_migrator "$source_database" "$source_release" "$source_commit" /migrations plan)"
assert_contains "$source_plan" "ledger absent"
source_apply="$(run_migrator "$source_database" "$source_release" "$source_commit" /migrations apply)"
assert_contains "$source_apply" "Post-migration sanity check passed"

psql_database "$source_database" -f /stateful-upgrade.sql

docker_exec "$container" mkdir -p /upgrade-config /backups
docker_exec -i "$container" /bin/sh -c 'cat >/upgrade-config/source.env' <<'ENV'
BITRIVER_TRANSCODER_LADDER=480p,720p
BITRIVER_LIVE_OBJECT_PREFIX=upgrade-source
ENV
docker_exec -i "$container" /bin/sh -c 'cat >/upgrade-config/candidate.env' <<'ENV'
BITRIVER_TRANSCODER_LADDER=480p,720p,1080p
BITRIVER_LIVE_OBJECT_PREFIX=upgrade-candidate
ENV
docker_exec "$container" cp /upgrade-config/source.env /upgrade-config/active.env
source_config_line="$(docker_exec "$container" sha256sum /upgrade-config/source.env)"
source_config_sha="${source_config_line%% *}"
candidate_config_line="$(docker_exec "$container" sha256sum /upgrade-config/candidate.env)"
candidate_config_sha="${candidate_config_line%% *}"

source_invariants="$(fixture_invariants "$source_database")"
source_schema_fingerprint="$(migration_fingerprint "$source_database")"

backup_started="$(date +%s)"
docker_exec \
  -e BITRIVER_BACKUP_DIR=/backups \
  -e BITRIVER_BACKUP_POSTGRES_HOST=127.0.0.1 \
  -e BITRIVER_BACKUP_POSTGRES_USER=postgres \
  -e BITRIVER_BACKUP_POSTGRES_PASSWORD="$password" \
  -e BITRIVER_BACKUP_POSTGRES_DB="$source_database" \
  -e BITRIVER_BACKUP_SOURCE_RELEASE="$source_release" \
  -e BITRIVER_BACKUP_SOURCE_COMMIT="$source_commit" \
  -e BITRIVER_BACKUP_RUN_PRUNE=0 \
  "$container" /bin/bash /backup-postgres.sh
backup_seconds=$(( $(date +%s) - backup_started ))
archive="$(docker_exec "$container" find /backups -maxdepth 1 -name 'bitriver-postgres-*.sql.gz' -print -quit)"
[[ -n "$archive" ]] || fail "pre-upgrade backup was not published"
archive_name="${archive##*/}"
docker_exec -w /backups "$container" sha256sum -c "${archive_name}.sha256"

upgrade_started="$(date +%s)"
candidate_plan="$(run_migrator "$source_database" "$candidate_release" "$candidate_commit" /migrations plan)"
assert_contains "$candidate_plan" "APPLIED 0001_initial.sql"
[[ "$candidate_plan" != *"PENDING"* ]] || fail "exact RC19 to RC20 hop unexpectedly contains a pending migration"
candidate_apply="$(run_migrator "$source_database" "$candidate_release" "$candidate_commit" /migrations apply)"
assert_contains "$candidate_apply" "Post-migration sanity check passed"
docker_exec "$container" cp /upgrade-config/candidate.env /upgrade-config/active.env
upgrade_seconds=$(( $(date +%s) - upgrade_started ))

candidate_invariants="$(fixture_invariants "$source_database")"
candidate_schema_fingerprint="$(migration_fingerprint "$source_database")"
[[ "$candidate_invariants" == "$source_invariants" ]] || fail "candidate migration changed representative state"
[[ "$candidate_schema_fingerprint" == "$source_schema_fingerprint" ]] || fail "candidate migration changed the exact schema ledger"

rollback_started="$(date +%s)"
rollback_plan="$(run_migrator "$source_database" "$source_release" "$source_commit" /migrations plan)"
assert_contains "$rollback_plan" "APPLIED 0001_initial.sql"
[[ "$rollback_plan" != *"PENDING"* ]] || fail "source rollback unexpectedly requires a migration"
docker_exec "$container" cp /upgrade-config/source.env /upgrade-config/active.env
active_config_line="$(docker_exec "$container" sha256sum /upgrade-config/active.env)"
active_config_sha="${active_config_line%% *}"
rollback_invariants="$(fixture_invariants "$source_database")"
rollback_seconds=$(( $(date +%s) - rollback_started ))
[[ "$rollback_invariants" == "$source_invariants" ]] || fail "in-place rollback changed representative state"
[[ "$active_config_sha" == "$source_config_sha" ]] || fail "in-place rollback did not restore the source config fixture"

docker_exec "$container" cp -a /migrations /interrupted-migrations
docker_exec -i "$container" /bin/sh -c 'cat >/interrupted-migrations/0012_interrupted_probe.sql' <<'SQL'
CREATE TABLE upgrade_interrupted_probe (id INTEGER PRIMARY KEY);
SQL
probe_line="$(docker_exec "$container" sha256sum /interrupted-migrations/0012_interrupted_probe.sql)"
probe_checksum="${probe_line%% *}"
psql_admin -c "CREATE DATABASE $interrupted_database;" >/dev/null
interrupted_baseline="$(
  run_migrator "$interrupted_database" "$candidate_release" "$candidate_commit" /migrations apply
)"
assert_contains "$interrupted_baseline" "Post-migration sanity check passed"
psql_database "$interrupted_database" \
  -v probe_checksum="$probe_checksum" \
  -v candidate_release="$candidate_release" \
  -v candidate_commit="$candidate_commit" <<'SQL'
INSERT INTO public.schema_migrations (
  filename, version, checksum_sha256, status, release_version, release_commit
) VALUES (
  '0012_interrupted_probe.sql', '0012', :'probe_checksum', 'applying',
  :'candidate_release', :'candidate_commit'
);
SQL
[[ "$(psql_database "$interrupted_database" -c "SELECT status FROM public.schema_migrations WHERE filename = '0012_interrupted_probe.sql';")" == "applying" ]] ||
  fail "interrupted migration ledger fixture was not created"
set +e
interrupted_output="$(
  run_migrator "$interrupted_database" "$candidate_release" "$candidate_commit" /interrupted-migrations plan 2>&1
)"
interrupted_rc=$?
set -e
[[ "$interrupted_rc" -ne 0 ]] ||
  fail "ambiguous applying state unexpectedly passed candidate preflight: $interrupted_output"
assert_contains "$interrupted_output" "BLOCKED 0012_interrupted_probe.sql status=applying"
psql_admin -c "DROP DATABASE $interrupted_database;" >/dev/null

psql_database "$source_database" -c "UPDATE channels SET title = 'candidate-only mutation' WHERE id = 'channel-1';" >/dev/null
mutated_invariants="$(fixture_invariants "$source_database")"
[[ "$mutated_invariants" != "$source_invariants" ]] || fail "restore-required fixture mutation was not observable"

restore_started="$(date +%s)"
docker_exec \
  -e BITRIVER_BACKUP_POSTGRES_HOST=127.0.0.1 \
  -e BITRIVER_BACKUP_POSTGRES_USER=postgres \
  -e BITRIVER_BACKUP_POSTGRES_PASSWORD="$password" \
  -e BITRIVER_RESTORE_REHEARSAL_DB="$restored_database" \
  -e BITRIVER_RESTORE_EXPECT_RELEASE="$source_release" \
  -e BITRIVER_RESTORE_EXPECT_SCHEMA_FINGERPRINT="$source_schema_fingerprint" \
  -e BITRIVER_RESTORE_KEEP_DB=1 \
  -e BITRIVER_RESTORE_REPORT_PATH=/backups/upgrade-restore-report.json \
  "$container" /bin/bash /restore-postgres.sh "$archive"
restore_seconds=$(( $(date +%s) - restore_started ))
restored_invariants="$(fixture_invariants "$restored_database")"
restored_schema_fingerprint="$(migration_fingerprint "$restored_database")"
[[ "$restored_invariants" == "$source_invariants" ]] || fail "verified restore did not recover the pre-upgrade values"
[[ "$restored_schema_fingerprint" == "$source_schema_fingerprint" ]] || fail "verified restore did not recover the pre-upgrade schema ledger"
docker_exec "$container" grep -Fq '"result": "passed"' /backups/upgrade-restore-report.json ||
  fail "database restore report did not pass"
psql_admin -c "DROP DATABASE $restored_database;" >/dev/null

report_json="$(
  psql_admin \
    -v source_release="$source_release" \
    -v source_commit="$source_commit" \
    -v source_release_set="$source_release_set" \
    -v candidate_release="$candidate_release" \
    -v candidate_commit="$candidate_commit" \
    -v candidate_release_set="$candidate_release_set" \
    -v migration_tree="$expected_migration_tree" \
    -v migration_runner="$expected_migration_runner" \
    -v source_schema="$source_schema_fingerprint" \
    -v candidate_schema="$candidate_schema_fingerprint" \
    -v source_invariants="$source_invariants" \
    -v source_config_sha="$source_config_sha" \
    -v candidate_config_sha="$candidate_config_sha" \
    -v backup_seconds="$backup_seconds" \
    -v upgrade_seconds="$upgrade_seconds" \
    -v rollback_seconds="$rollback_seconds" \
    -v restore_seconds="$restore_seconds" <<'SQL'
SELECT jsonb_pretty(jsonb_build_object(
  'schemaVersion', 'bitriver.stateful-upgrade-report/v1',
  'result', 'passed',
  'scope', 'database-migration-layer',
  'source', jsonb_build_object(
    'release', :'source_release',
    'commit', :'source_commit',
    'releaseSetSha256', :'source_release_set'
  ),
  'candidate', jsonb_build_object(
    'release', :'candidate_release',
    'commit', :'candidate_commit',
    'releaseSetSha256', :'candidate_release_set'
  ),
  'migration', jsonb_build_object(
    'treeObject', :'migration_tree',
    'runnerObject', :'migration_runner',
    'delta', 'none',
    'sourceFingerprintSha256', :'source_schema',
    'candidateFingerprintSha256', :'candidate_schema',
    'ambiguousApplyingBlocked', true
  ),
  'invariants', jsonb_build_object(
    'source', :'source_invariants'::jsonb,
    'afterUpgradeMatched', true,
    'afterInPlaceRollbackMatched', true,
    'afterVerifiedRestoreMatched', true
  ),
  'configurationFixture', jsonb_build_object(
    'sourceSha256', :'source_config_sha',
    'candidateSha256', :'candidate_config_sha',
    'rollbackMatched', true
  ),
  'rollback', jsonb_build_object(
    'classification', 'in-place-compatible',
    'classificationScope', 'RC19-to-RC20 database and migration layer',
    'verifiedRestorePath', 'passed'
  ),
  'timingSeconds', jsonb_build_object(
    'backup', :'backup_seconds'::integer,
    'upgrade', :'upgrade_seconds'::integer,
    'inPlaceRollback', :'rollback_seconds'::integer,
    'verifiedRestore', :'restore_seconds'::integer
  ),
  'remainingAcceptance', jsonb_build_array(
    'exact-image Compose upgrade and image rollback',
    'operator configuration and generated media config rollback',
    'interrupted deployment cut points beyond the migration ledger',
    'post-upgrade ingest, playback, chat, admin, VOD, and production golden path'
  )
));
SQL
)"

docker_exec "$container" mkdir -p /evidence
# The report body is secret-safe JSON generated by Postgres; no fixture values are retained.
# shellcheck disable=SC2016
printf '%s\n' "$report_json" | docker_exec -i "$container" /bin/sh -c 'cat >/evidence/stateful-upgrade-report.json'
mkdir -p "$workdir/evidence"
docker cp "$container:/evidence/stateful-upgrade-report.json" "$workdir/evidence/stateful-upgrade-report.json" >/dev/null

grep -Fq '"schemaVersion": "bitriver.stateful-upgrade-report/v1"' "$workdir/evidence/stateful-upgrade-report.json" ||
  fail "stateful upgrade report schema is missing"
grep -Fq '"classification": "in-place-compatible"' "$workdir/evidence/stateful-upgrade-report.json" ||
  fail "stateful upgrade report rollback classification is missing"
grep -Fq '"ambiguousApplyingBlocked": true' "$workdir/evidence/stateful-upgrade-report.json" ||
  fail "stateful upgrade report interruption result is missing"
"$repo_root/scripts/scan-release-evidence.sh" --root "$workdir/evidence"

if [[ -n "$retained_report" ]]; then
  mkdir -p "$(dirname "$retained_report")"
  cp "$workdir/evidence/stateful-upgrade-report.json" "$retained_report"
  chmod 0600 "$retained_report"
  echo "Retained stateful upgrade report: $retained_report"
fi

echo "Stateful RC19 to RC20 data-plane upgrade/rollback rehearsal passed."
