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
  psql_database "$database" <<'SQL'
WITH fixture_values AS (
  SELECT concat_ws(E'\n',
    (SELECT string_agg(id || ':' || array_to_string(roles, ','), ',' ORDER BY id) FROM users),
    (SELECT user_id || ':' || bio || ':' || social_links::text FROM profiles WHERE user_id = 'creator'),
    (SELECT id || ':' || owner_id || ':' || title || ':' || schedule::text FROM channels WHERE id = 'channel-1'),
    (SELECT id || ':' || channel_id || ':' || coalesce(playback_url, '') || ':' || metadata::text FROM uploads WHERE id = 'upload-1'),
    (SELECT id || ':' || channel_id || ':' || coalesce(playback_base_url, '') || ':' || metadata::text FROM recordings WHERE id = 'recording-1'),
    (SELECT id || ':' || status FROM chat_reports WHERE id = 'report-1'),
    (SELECT id || ':' || status FROM appeals WHERE id = 'appeal-1'),
    (SELECT id || ':' || status FROM legal_dmca_cases WHERE id = 'dmca-1'),
    (SELECT id || ':' || action FROM chat_automod_actions WHERE id = 'automod-1'),
    (SELECT provider || ':' || event_id || ':' || status FROM payment_transactions WHERE id = 'payment-1')
  ) AS value
)
SELECT jsonb_build_object(
  'rowCounts', jsonb_build_object(
    'users', (SELECT count(*) FROM users),
    'profiles', (SELECT count(*) FROM profiles),
    'authSessions', (SELECT count(*) FROM auth_sessions),
    'authMfa', (SELECT count(*) FROM auth_mfa),
    'channels', (SELECT count(*) FROM channels),
    'follows', (SELECT count(*) FROM follows),
    'streamSessions', (SELECT count(*) FROM stream_sessions),
    'recordings', (SELECT count(*) FROM recordings),
    'uploads', (SELECT count(*) FROM uploads),
    'chatMessages', (SELECT count(*) FROM chat_messages),
    'chatFilters', (SELECT count(*) FROM chat_filters),
    'chatReports', (SELECT count(*) FROM chat_reports),
    'appeals', (SELECT count(*) FROM appeals),
    'legalCases', (SELECT count(*) FROM legal_dmca_cases),
    'paymentTransactions', (SELECT count(*) FROM payment_transactions)
  ),
  'valueFingerprintSha256', encode(digest(value::bytea, 'sha256'), 'hex')
)::text
FROM fixture_values;
SQL
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

source_plan="$(run_migrator "$source_database" "$source_release" "$source_commit" /migrations plan)"
assert_contains "$source_plan" "ledger absent"
source_apply="$(run_migrator "$source_database" "$source_release" "$source_commit" /migrations apply)"
assert_contains "$source_apply" "Post-migration sanity check passed"

docker_exec -i \
  -e PGPASSWORD="$password" \
  "$container" psql -X -q -v ON_ERROR_STOP=1 \
  -h 127.0.0.1 -U postgres -d "$source_database" <<'SQL'
INSERT INTO users (id, display_name, email, roles, password_hash, self_signup) VALUES
  ('admin', 'Upgrade admin', 'admin@example.invalid', ARRAY['admin'], 'hash-admin', false),
  ('creator', 'Upgrade creator', 'creator@example.invalid', ARRAY['creator'], 'hash-creator', false),
  ('moderator', 'Upgrade moderator', 'moderator@example.invalid', ARRAY['moderator'], 'hash-moderator', false),
  ('viewer', 'Upgrade viewer', 'viewer@example.invalid', ARRAY['viewer'], 'hash-viewer', true);
INSERT INTO profiles (user_id, bio, featured_channel_id, created_at, updated_at, social_links)
VALUES ('creator', 'upgrade fixture', 'channel-1', now(), now(), '[{"platform":"website","url":"https://example.invalid/creator"}]');
INSERT INTO oauth_accounts (provider, subject, user_id, email, display_name)
VALUES ('fixture', 'creator-subject', 'creator', 'creator@example.invalid', 'Upgrade creator');
INSERT INTO auth_sessions (token, user_id, expires_at, hashed_token, absolute_expires_at)
VALUES (repeat('a', 64), 'admin', now() + interval '1 hour', repeat('a', 64), now() + interval '8 hours');
INSERT INTO auth_mfa (user_id, secret, recovery_codes, enabled, enabled_at)
VALUES ('admin', 'JBSWY3DPEHPK3PXP', ARRAY['fixture-recovery-hash'], true, now());
INSERT INTO channels (id, owner_id, stream_key, title, category, tags, schedule, live_state)
VALUES ('channel-1', 'creator', 'fixture-stream-key', 'Upgrade channel', 'technology', ARRAY['upgrade','fixture'], '[{"title":"Upgrade event","startsAt":"2026-08-16T02:00:00Z"}]', 'offline');
INSERT INTO follows (user_id, channel_id) VALUES ('viewer', 'channel-1');
INSERT INTO stream_sessions (id, channel_id, started_at, ended_at, renditions, peak_concurrent, origin_url, playback_url, ingest_endpoints, ingest_job_ids)
VALUES ('session-1', 'channel-1', now() - interval '10 minutes', now() - interval '5 minutes', ARRAY['720p'], 7, 'rtmp://origin.invalid/live/fixture', 'https://media.invalid/live/fixture/index.m3u8', ARRAY['rtmp://ingest.invalid/live'], ARRAY['job-1']);
INSERT INTO stream_session_manifests (session_id, name, manifest_url, bitrate)
VALUES ('session-1', '720p', 'https://media.invalid/live/fixture/720p/index.m3u8', 2800000);
INSERT INTO recordings (id, channel_id, session_id, title, duration_seconds, playback_base_url, metadata, published_at, created_at)
VALUES ('recording-1', 'channel-1', 'session-1', 'Upgrade recording', 300, 'https://objects.invalid/recordings/fixture', '{"objectKey":"recordings/fixture/index.m3u8","sha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}', now(), now());
INSERT INTO recording_renditions (recording_id, name, manifest_url, bitrate)
VALUES ('recording-1', '720p', 'https://objects.invalid/recordings/fixture/720p.m3u8', 2800000);
INSERT INTO recording_thumbnails (id, recording_id, url, width, height, created_at)
VALUES ('thumbnail-1', 'recording-1', 'https://objects.invalid/recordings/fixture/thumb.jpg', 1280, 720, now());
INSERT INTO uploads (id, channel_id, title, filename, size_bytes, status, progress, recording_id, playback_url, metadata, completed_at)
VALUES ('upload-1', 'channel-1', 'Upgrade upload', 'fixture.mp4', 4096, 'completed', 100, 'recording-1', 'https://objects.invalid/uploads/fixture.mp4', '{"objectKey":"uploads/fixture.mp4","sha256":"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"}', now());
INSERT INTO chat_messages (id, channel_id, user_id, content, created_at)
VALUES ('message-1', 'channel-1', 'viewer', 'upgrade fixture message', now());
INSERT INTO chat_bans (channel_id, user_id, actor_id, reason)
VALUES ('channel-1', 'moderator', 'admin', 'upgrade fixture ban');
INSERT INTO chat_timeouts (channel_id, user_id, actor_id, reason, expires_at)
VALUES ('channel-1', 'viewer', 'moderator', 'upgrade fixture timeout', now() + interval '5 minutes');
INSERT INTO chat_filters (id, channel_id, kind, pattern)
VALUES ('filter-1', 'channel-1', 'literal', 'fixture-blocked-term');
INSERT INTO chat_automod_actions (id, channel_id, user_id, filter_id, filter_kind, filter_pattern, message, action)
VALUES ('automod-1', 'channel-1', 'viewer', 'filter-1', 'literal', 'fixture-blocked-term', 'blocked fixture', 'blocked');
INSERT INTO chat_reports (id, channel_id, reporter_id, target_id, reason, message_id)
VALUES ('report-1', 'channel-1', 'viewer', 'moderator', 'upgrade fixture report', 'message-1');
INSERT INTO appeals (id, report_id, channel_id, reporter_id, reason)
VALUES ('appeal-1', 'report-1', 'channel-1', 'moderator', 'upgrade fixture appeal');
INSERT INTO appeal_events (id, appeal_id, actor_id, action, note)
VALUES ('appeal-event-1', 'appeal-1', 'admin', 'opened', 'upgrade fixture event');
INSERT INTO legal_dmca_cases (id, reporter_name, reporter_email, content_url, description, status)
VALUES ('dmca-1', 'Fixture reporter', 'reporter@example.invalid', 'https://example.invalid/content', 'upgrade fixture case', 'open');
INSERT INTO legal_data_subject_requests (id, subject_email, request_type, status)
VALUES ('dsr-1', 'viewer@example.invalid', 'export', 'open');
INSERT INTO legal_data_subject_audit_events (id, request_id, actor_user_id, action, details, evidence_ref)
VALUES ('dsr-event-1', 'dsr-1', 'admin', 'created', 'upgrade fixture audit', 'ticket-1');
INSERT INTO legal_state_history (id, entity_type, entity_id, to_state, actor_user_id, reason)
VALUES ('legal-history-1', 'dmca', 'dmca-1', 'open', 'admin', 'upgrade fixture history');
INSERT INTO tips (id, channel_id, from_user_id, amount, currency, provider, reference, message, status, idempotency_key)
VALUES ('tip-1', 'channel-1', 'viewer', 1.25, 'USD', 'fixture', 'tip-reference-1', 'upgrade fixture tip', 'succeeded', 'tip-key-1');
INSERT INTO subscriptions (id, channel_id, user_id, tier, provider, reference, amount, currency, started_at, expires_at, auto_renew, status, idempotency_key)
VALUES ('subscription-1', 'channel-1', 'viewer', 'supporter', 'fixture', 'subscription-reference-1', 5, 'USD', now(), now() + interval '30 days', true, 'active', 'subscription-key-1');
INSERT INTO payment_transactions (id, provider, event_id, entity_type, entity_id, reference, status, idempotency_key)
VALUES ('payment-1', 'fixture', 'event-1', 'tip', 'tip-1', 'payment-reference-1', 'succeeded', 'payment-key-1');
SQL

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
