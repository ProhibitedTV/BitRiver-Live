#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script_root="${BITRIVER_BACKUP_TEST_SCRIPT_ROOT:-$repo_root/scripts}"
container="bitriver-backup-restore-test-${RANDOM}-$$"
source_database="bitriver_backup_source"
password="backup-restore-test-password"
source_release="${BITRIVER_BACKUP_TEST_RELEASE:-v1.2.3-rc.test}"
source_commit="${BITRIVER_BACKUP_TEST_COMMIT:-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb}"
postgres_image="${BITRIVER_BACKUP_TEST_POSTGRES_IMAGE:-postgres:15-alpine}"
workdir="$(mktemp -d)"

[[ $source_release =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || {
  echo "Backup/restore integration test requires an exact v-prefixed release" >&2
  exit 2
}
[[ $source_commit =~ ^[0-9a-f]{40}$ ]] || {
  echo "Backup/restore integration test requires a full lowercase commit" >&2
  exit 2
}
for required_script in backup-postgres.sh restore-postgres.sh; do
  [[ -f $script_root/$required_script ]] || {
    echo "Backup/restore integration test script is missing: $script_root/$required_script" >&2
    exit 2
  }
done

cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
  rm -rf "$workdir"
}
trap cleanup EXIT

fail() {
  echo "Backup/restore integration test failed: $*" >&2
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

psql_admin() {
  docker_exec \
    -e PGPASSWORD="$password" \
    "$container" psql -X -qAt -v ON_ERROR_STOP=1 \
    -h 127.0.0.1 -U postgres -d postgres "$@"
}

psql_source() {
  docker_exec \
    -e PGPASSWORD="$password" \
    "$container" psql -X -qAt -v ON_ERROR_STOP=1 \
    -h 127.0.0.1 -U postgres -d "$source_database" "$@"
}

psql_database() {
  local database="$1"
  shift
  docker_exec \
    -e PGPASSWORD="$password" \
    "$container" psql -X -qAt -v ON_ERROR_STOP=1 \
    -h 127.0.0.1 -U postgres -d "$database" "$@"
}

assert_database_absent() {
  local database="$1"
  local count
  count="$(psql_admin -c "SELECT count(*) FROM pg_database WHERE datname = '$database';")"
  [[ "$count" == "0" ]] || fail "database '$database' exists after pre-mutation refusal"
}

run_restore() {
  local rehearsal_database="$1"
  local report_path="$2"
  local archive_path="$3"
  local expected_release="${4:-}"
  local expected_schema="${5:-}"
  local keep_database="${6:-0}"
  local docker_args=(
    -e BITRIVER_BACKUP_POSTGRES_HOST=127.0.0.1 \
    -e BITRIVER_BACKUP_POSTGRES_USER=postgres \
    -e BITRIVER_BACKUP_POSTGRES_PASSWORD="$password" \
    -e BITRIVER_RESTORE_REHEARSAL_DB="$rehearsal_database" \
    -e BITRIVER_RESTORE_REPORT_PATH="$report_path"
  )
  if [ -n "$expected_release" ]; then
    docker_args+=(-e BITRIVER_RESTORE_EXPECT_RELEASE="$expected_release")
  fi
  if [ -n "$expected_schema" ]; then
    docker_args+=(-e BITRIVER_RESTORE_EXPECT_SCHEMA_FINGERPRINT="$expected_schema")
  fi
  if [ "$keep_database" = "1" ]; then
    docker_args+=(-e BITRIVER_RESTORE_KEEP_DB=1)
  fi
  docker_exec "${docker_args[@]}" \
    "$container" /bin/sh /restore-postgres.sh "$archive_path"
}

docker run -d --name "$container" \
  -e POSTGRES_PASSWORD="$password" \
  -e POSTGRES_DB="$source_database" \
  "$postgres_image" >/dev/null

ready=false
for _ in {1..60}; do
  if docker_exec "$container" pg_isready -q -U postgres -d "$source_database"; then
    ready=true
    break
  fi
  sleep 1
done
[[ "$ready" == true ]] || fail "Postgres did not become ready"

docker cp "$script_root/backup-postgres.sh" "$container:/backup-postgres.sh" >/dev/null
docker cp "$script_root/restore-postgres.sh" "$container:/restore-postgres.sh" >/dev/null

docker_exec -i \
  -e PGPASSWORD="$password" \
  "$container" psql -X -q -v ON_ERROR_STOP=1 \
  -v source_release="$source_release" \
  -v source_commit="$source_commit" \
  -h 127.0.0.1 -U postgres -d "$source_database" <<'SQL'
CREATE TABLE public.schema_migrations (
  filename TEXT PRIMARY KEY,
  version TEXT NOT NULL,
  checksum_sha256 TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  applied_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  release_version TEXT NOT NULL DEFAULT 'unknown',
  release_commit TEXT NOT NULL DEFAULT 'unknown',
  failure_reason TEXT NOT NULL DEFAULT ''
);
INSERT INTO public.schema_migrations (
  filename, version, checksum_sha256, status, applied_at,
  release_version, release_commit
) VALUES (
  '0001_initial.sql', '0001', repeat('a', 64), 'applied', now(),
  :'source_release', :'source_commit'
);

CREATE TABLE public.users (id TEXT PRIMARY KEY, email TEXT NOT NULL, role TEXT NOT NULL);
INSERT INTO public.users VALUES
  ('admin', 'admin@example.invalid', 'admin'),
  ('creator', 'creator@example.invalid', 'creator'),
  ('moderator', 'moderator@example.invalid', 'moderator'),
  ('viewer', 'viewer@example.invalid', 'viewer');
CREATE TABLE public.auth_sessions (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, mfa_verified BOOLEAN NOT NULL);
INSERT INTO public.auth_sessions VALUES ('session-1', 'admin', true);
CREATE TABLE public.mfa_factors (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, kind TEXT NOT NULL);
INSERT INTO public.mfa_factors VALUES ('mfa-1', 'admin', 'totp');
CREATE TABLE public.channels (id TEXT PRIMARY KEY, owner_id TEXT NOT NULL, title TEXT NOT NULL);
INSERT INTO public.channels VALUES ('channel-1', 'creator', 'Recovery fixture');
CREATE TABLE public.profiles (user_id TEXT PRIMARY KEY, display_name TEXT NOT NULL);
INSERT INTO public.profiles VALUES ('creator', 'Fixture creator');
CREATE TABLE public.follows (user_id TEXT NOT NULL, channel_id TEXT NOT NULL, PRIMARY KEY (user_id, channel_id));
INSERT INTO public.follows VALUES ('viewer', 'channel-1');
CREATE TABLE public.channel_schedule (id TEXT PRIMARY KEY, channel_id TEXT NOT NULL, title TEXT NOT NULL);
INSERT INTO public.channel_schedule VALUES ('schedule-1', 'channel-1', 'Fixture event');
CREATE TABLE public.moderation_records (id TEXT PRIMARY KEY, channel_id TEXT NOT NULL, action TEXT NOT NULL);
INSERT INTO public.moderation_records VALUES ('moderation-1', 'channel-1', 'timeout');
CREATE TABLE public.legal_cases (id TEXT PRIMARY KEY, status TEXT NOT NULL);
INSERT INTO public.legal_cases VALUES ('legal-1', 'open');
CREATE TABLE public.chat_filters (id TEXT PRIMARY KEY, channel_id TEXT NOT NULL, pattern TEXT NOT NULL);
INSERT INTO public.chat_filters VALUES ('filter-1', 'channel-1', 'blocked-term');
CREATE TABLE public.chat_messages (id TEXT PRIMARY KEY, channel_id TEXT NOT NULL, author_id TEXT NOT NULL);
INSERT INTO public.chat_messages VALUES ('message-1', 'channel-1', 'viewer');
CREATE TABLE public.uploads (id TEXT PRIMARY KEY, channel_id TEXT NOT NULL, object_key TEXT NOT NULL);
INSERT INTO public.uploads VALUES ('upload-1', 'channel-1', 'uploads/fixture.mp4');
CREATE TABLE public.recordings (id TEXT PRIMARY KEY, channel_id TEXT NOT NULL, manifest_key TEXT NOT NULL);
INSERT INTO public.recordings VALUES ('recording-1', 'channel-1', 'recordings/fixture.m3u8');
CREATE TABLE public.object_fixtures (object_key TEXT PRIMARY KEY, sha256 TEXT NOT NULL, size_bytes BIGINT NOT NULL);
INSERT INTO public.object_fixtures VALUES ('uploads/fixture.mp4', repeat('c', 64), 4096);
CREATE TABLE public.operator_settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
INSERT INTO public.operator_settings VALUES ('transcode_ladder', 'non-default-fixture');
SQL

docker_exec \
  -e BITRIVER_BACKUP_DIR=/backups \
  -e BITRIVER_BACKUP_POSTGRES_HOST=127.0.0.1 \
  -e BITRIVER_BACKUP_POSTGRES_USER=postgres \
  -e BITRIVER_BACKUP_POSTGRES_PASSWORD="$password" \
  -e BITRIVER_BACKUP_POSTGRES_DB="$source_database" \
  -e BITRIVER_BACKUP_SOURCE_RELEASE="$source_release" \
  -e BITRIVER_BACKUP_SOURCE_COMMIT="$source_commit" \
  -e BITRIVER_BACKUP_RUN_PRUNE=0 \
  "$container" /bin/sh /backup-postgres.sh

archive="$(docker_exec "$container" /bin/sh -c 'ls /backups/bitriver-postgres-*.sql.gz')"
[[ -n "$archive" ]] || fail "backup archive was not published"
archive_name="${archive##*/}"
manifest="${archive}.manifest.json"
checksum="${archive}.sha256"

docker_exec -w /backups "$container" sha256sum -c "${archive_name}.sha256"
[[ -z "$(docker_exec "$container" find /backups -name '*.partial.*' -print -quit)" ]] ||
  fail "partial backup artifacts remained"
docker_exec "$container" grep -Fq '"users": 4' "$manifest" || fail "user invariant missing"
docker_exec "$container" grep -Fq '"object_fixtures": 1' "$manifest" || fail "object invariant missing"
if docker_exec "$container" grep -Fq "$password" "$manifest"; then
  fail "backup manifest leaked the database password"
fi

docker_exec "$container" mkdir -p /fixed-bin /collision
docker_exec -i "$container" /bin/sh -c 'cat >/fixed-bin/date && chmod 0755 /fixed-bin/date' <<'SH'
#!/bin/sh
case "$*" in
  *+%Y%m%dT%H%M%SZ*) printf '%s\n' '20260815T010203Z' ;;
  *+%Y-%m-%dT%H:%M:%SZ*) printf '%s\n' '2026-08-15T01:02:03Z' ;;
  *) exec /bin/date "$@" ;;
esac
SH
collision_args=(
  -e PATH=/fixed-bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
  -e BITRIVER_BACKUP_DIR=/collision
  -e BITRIVER_BACKUP_POSTGRES_HOST=127.0.0.1
  -e BITRIVER_BACKUP_POSTGRES_USER=postgres
  -e BITRIVER_BACKUP_POSTGRES_PASSWORD="$password"
  -e BITRIVER_BACKUP_POSTGRES_DB="$source_database"
  -e BITRIVER_BACKUP_SOURCE_RELEASE="$source_release"
  -e BITRIVER_BACKUP_SOURCE_COMMIT="$source_commit"
  -e BITRIVER_BACKUP_RUN_PRUNE=0
)
docker_exec "${collision_args[@]}" "$container" /bin/sh /backup-postgres.sh
collision_archive=/collision/bitriver-postgres-20260815T010203Z.sql.gz
collision_manifest="${collision_archive}.manifest.json"
collision_checksum="${collision_archive}.sha256"
collision_before="$(
  docker_exec "$container" sha256sum \
    "$collision_archive" "$collision_manifest" "$collision_checksum"
)"
set +e
collision_output="$(
  docker_exec "${collision_args[@]}" "$container" /bin/sh /backup-postgres.sh 2>&1
)"
collision_rc=$?
set -e
[[ "$collision_rc" -ne 0 ]] || fail "same-second backup collision unexpectedly succeeded"
assert_contains "$collision_output" "refusing to replace an existing backup set"
collision_after="$(
  docker_exec "$container" sha256sum \
    "$collision_archive" "$collision_manifest" "$collision_checksum"
)"
[[ "$collision_after" == "$collision_before" ]] ||
  fail "same-second collision changed the previously published backup set"
docker_exec -w /collision "$container" sha256sum -c "${collision_checksum##*/}"
[[ "$(docker_exec "$container" find /collision -maxdepth 1 -type f | wc -l | tr -d '[:space:]')" == "3" ]] ||
  fail "same-second collision left an incomplete or extra backup artifact"
[[ -z "$(docker_exec "$container" find /collision -name '*.partial.*' -print -quit)" ]] ||
  fail "same-second collision left a partial artifact"
if docker_exec "$container" test -d "${collision_archive}.lock"; then
  fail "same-second collision left its lock directory"
fi

fingerprint="$(
  docker_exec "$container" grep -oE '"migrationFingerprintSha256": "[0-9a-f]{64}"' "$manifest" |
    head -n 1 |
    cut -d '"' -f 4
)"
[[ "$fingerprint" =~ ^[0-9a-f]{64}$ ]] || fail "manifest fingerprint is invalid"

success_report=/backups/restore-success.json
run_restore \
  bitr_restore_success \
  "$success_report" \
  "$archive" \
  "$source_release" \
  "$fingerprint"
assert_database_absent bitr_restore_success
docker_exec "$container" grep -Fq '"result": "passed"' "$success_report" || fail "restore result missing"
docker_exec "$container" grep -Fq '"cleanup": "dropped"' "$success_report" || fail "cleanup result missing"
docker_exec "$container" grep -Fq '"rowCounts": "matched"' "$success_report" || fail "row invariant result missing"
docker_exec "$container" grep -Fq '"migrationFingerprint": "matched"' "$success_report" || fail "migration result missing"
if docker_exec "$container" grep -Fq "$password" "$success_report"; then
  fail "restore report leaked the database password"
fi

retained_report=/backups/restore-retained.json
run_restore \
  bitr_restore_retained \
  "$retained_report" \
  "$archive" \
  "$source_release" \
  "$fingerprint" \
  1
[[ "$(psql_database bitr_restore_retained -c "SELECT string_agg(id || ':' || role, ',' ORDER BY id) FROM public.users;")" == \
  "admin:admin,creator:creator,moderator:moderator,viewer:viewer" ]] ||
  fail "restored role fixtures differ from the source"
[[ "$(psql_database bitr_restore_retained -c "SELECT owner_id || '|' || title FROM public.channels WHERE id = 'channel-1';")" == \
  "creator|Recovery fixture" ]] || fail "restored channel fixture differs from the source"
[[ "$(psql_database bitr_restore_retained -c "SELECT object_key || '|' || size_bytes FROM public.object_fixtures;")" == \
  "uploads/fixture.mp4|4096" ]] || fail "restored object fixture differs from the source"
[[ "$(psql_database bitr_restore_retained -c "SELECT value FROM public.operator_settings WHERE key = 'transcode_ladder';")" == \
  "non-default-fixture" ]] || fail "restored operator setting differs from the source"
docker_exec "$container" grep -Fq '"cleanup": "retained"' "$retained_report" ||
  fail "retained restore report is incorrect"
psql_admin -c 'DROP DATABASE bitr_restore_retained;' >/dev/null
assert_database_absent bitr_restore_retained

set +e
wrong_release_output="$(
  run_restore \
    bitr_restore_wrong_release \
    /backups/wrong-release.json \
    "$archive" \
    v1.2.3-rc.wrong 2>&1
)"
wrong_release_rc=$?
set -e
[[ "$wrong_release_rc" -ne 0 ]] || fail "release mismatch unexpectedly restored"
assert_contains "$wrong_release_output" "backup release mismatch"
assert_database_absent bitr_restore_wrong_release

set +e
wrong_schema_output="$(
  run_restore \
    bitr_restore_wrong_schema \
    /backups/wrong-schema.json \
    "$archive" \
    '' \
    0000000000000000000000000000000000000000000000000000000000000000 2>&1
)"
wrong_schema_rc=$?
set -e
[[ "$wrong_schema_rc" -ne 0 ]] || fail "schema mismatch unexpectedly restored"
assert_contains "$wrong_schema_output" "schema fingerprint does not match"
assert_database_absent bitr_restore_wrong_schema

docker_exec "$container" mkdir -p /backups/corrupt
docker_exec "$container" cp "$archive" "$manifest" "$checksum" /backups/corrupt/
# $1 expands in the container's /bin/sh.
# shellcheck disable=SC2016
docker_exec "$container" /bin/sh -c 'printf corrupt >>"$1"' sh "/backups/corrupt/$archive_name"
set +e
corrupt_output="$(
  run_restore \
    bitr_restore_corrupt \
    /backups/corrupt-report.json \
    "/backups/corrupt/$archive_name" 2>&1
)"
corrupt_rc=$?
set -e
[[ "$corrupt_rc" -ne 0 ]] || fail "corrupt archive unexpectedly restored"
assert_contains "$corrupt_output" "FAILED"
assert_database_absent bitr_restore_corrupt

docker_exec "$container" mkdir -p /backups/missing-checksum
docker_exec "$container" cp "$archive" "$manifest" /backups/missing-checksum/
set +e
missing_checksum_output="$(
  run_restore \
    bitr_restore_missing_checksum \
    /backups/missing-checksum-report.json \
    "/backups/missing-checksum/$archive_name" 2>&1
)"
missing_checksum_rc=$?
set -e
[[ "$missing_checksum_rc" -ne 0 ]] || fail "missing checksum unexpectedly restored"
assert_contains "$missing_checksum_output" "backup checksum set is required"
assert_database_absent bitr_restore_missing_checksum

docker_exec "$container" mkdir -p /backups/missing-manifest
docker_exec "$container" cp "$archive" "$checksum" /backups/missing-manifest/
set +e
missing_manifest_output="$(
  run_restore \
    bitr_restore_missing_manifest \
    /backups/missing-manifest-report.json \
    "/backups/missing-manifest/$archive_name" 2>&1
)"
missing_manifest_rc=$?
set -e
[[ "$missing_manifest_rc" -ne 0 ]] || fail "missing manifest unexpectedly restored"
assert_contains "$missing_manifest_output" "backup manifest is required"
assert_database_absent bitr_restore_missing_manifest

set +e
source_target_output="$(
  run_restore \
    "$source_database" \
    /backups/source-target.json \
    "$archive" 2>&1
)"
source_target_rc=$?
set -e
[[ "$source_target_rc" -ne 0 ]] || fail "source database target unexpectedly restored"
assert_contains "$source_target_output" "must not equal the backup source database"
[[ "$(psql_source -c 'SELECT count(*) FROM public.users;')" == "4" ]] ||
  fail "source database changed during refusal"

psql_source -c "UPDATE public.schema_migrations SET status = 'failed';" >/dev/null
sleep 1
set +e
blocked_backup_output="$(
  docker_exec \
    -e BITRIVER_BACKUP_DIR=/backups \
    -e BITRIVER_BACKUP_POSTGRES_HOST=127.0.0.1 \
    -e BITRIVER_BACKUP_POSTGRES_USER=postgres \
    -e BITRIVER_BACKUP_POSTGRES_PASSWORD="$password" \
    -e BITRIVER_BACKUP_POSTGRES_DB="$source_database" \
    -e BITRIVER_BACKUP_SOURCE_RELEASE="$source_release" \
    -e BITRIVER_BACKUP_SOURCE_COMMIT="$source_commit" \
    -e BITRIVER_BACKUP_RUN_PRUNE=0 \
    "$container" /bin/sh /backup-postgres.sh 2>&1
)"
blocked_backup_rc=$?
set -e
[[ "$blocked_backup_rc" -ne 0 ]] || fail "non-applied migration ledger unexpectedly backed up"
assert_contains "$blocked_backup_output" "migration ledger contains applying or failed rows"
[[ "$(docker_exec "$container" find /backups -maxdepth 1 -name 'bitriver-postgres-*.sql.gz' | wc -l | tr -d '[:space:]')" == "1" ]] ||
  fail "failed backup left an apparently valid archive"
[[ -z "$(docker_exec "$container" find /backups -name '*.partial.*' -print -quit)" ]] ||
  fail "failed backup left a partial artifact"

if [[ -n ${BITRIVER_BACKUP_RETAIN_DIR:-} ]]; then
  mkdir -p "$BITRIVER_BACKUP_RETAIN_DIR"
  retained_dir="$(cd "$BITRIVER_BACKUP_RETAIN_DIR" && pwd -P)"
  for retained in "$archive" "$manifest" "$checksum"; do
    docker cp "$container:$retained" "$retained_dir/$(basename "$retained")" >/dev/null
  done
  printf '%s\n' "$archive_name" >"$retained_dir/backup-name.txt"
fi

echo "Postgres backup/restore rehearsal tests passed."
