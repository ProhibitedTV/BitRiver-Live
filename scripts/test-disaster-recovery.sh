#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
workdir="$(mktemp -d)"
bundle_root="$workdir/source-free-bundle"
binary_dir="$workdir/bin"
source_host="$workdir/source-host"
target_host="$workdir/target-host"
postgres_backup_dir="$workdir/postgres-backup"
host_backup_dir="$workdir/host-backup"
external_source="$workdir/source-objects"
external_offhost="$workdir/offhost-objects"
external_target="$workdir/target-objects"
expected_object_inventory="$workdir/expected-object-inventory.json"
observed_object_inventory="$workdir/observed-object-inventory.json"
passphrase_file="$workdir/recovery-passphrase"
host_report="$workdir/host-restore-report.json"
postgres_report="$workdir/postgres-restore-report.json"
disaster_report="$workdir/disaster-recovery-report.json"
retained_artifact_dir="${BITRIVER_DISASTER_RECOVERY_ARTIFACT_DIR:-}"
postgres_container="bitriver-disaster-recovery-${RANDOM}-$$"
source_release="v1.2.3-rc.test"
source_commit="bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
postgres_image="${BITRIVER_DISASTER_POSTGRES_IMAGE:-postgres:15-alpine@sha256:4006528dcbdd9be8c1aaa50389caea4e93c46d6f54c3533bcd3253725e526e23}"
started_at_epoch="$(date -u +%s)"
secret_sentinel="disaster-recovery-secret-e1d18f74"

cleanup() {
  docker rm -f "$postgres_container" >/dev/null 2>&1 || true
  rm -rf -- "$workdir"
}
trap cleanup EXIT

fail() {
  echo "Disaster recovery rehearsal failed: $*" >&2
  exit 1
}

docker_exec() {
  MSYS_NO_PATHCONV=1 docker exec "$@"
}

wait_for_postgres() {
  local ready=false
  for _ in {1..60}; do
    if docker_exec "$postgres_container" pg_isready -q -U postgres -d postgres; then
      ready=true
      break
    fi
    sleep 1
  done
  [[ $ready == true ]] || fail "fresh recovery Postgres did not become ready"
}

mkdir -p "$bundle_root" "$binary_dir" "$external_source/recordings"
bash "$repo_root/scripts/stage-release-assets.sh" \
  --output "$bundle_root" \
  --release-tag "$source_release"
for forbidden in .git cmd internal; do
  [[ ! -e $bundle_root/$forbidden ]] || fail "source-free bundle contains $forbidden"
done
for required in \
  scripts/backup-postgres.sh \
  scripts/restore-postgres.sh \
  scripts/backup-host-state.sh \
  scripts/restore-host-state.sh \
  scripts/host_recovery.py; do
  [[ -f $bundle_root/$required ]] || fail "source-free bundle is missing $required"
done

cat >"$binary_dir/bitriver" <<FAKE_BITRIVER
#!/usr/bin/env bash
set -euo pipefail
if [[ \${1:-} == env && \${2:-} == init ]]; then
  shift 2
  env_file=""
  while [[ \$# -gt 0 ]]; do
    case "\$1" in
      --env-file) env_file=\$2; shift 2 ;;
      --example) shift 2 ;;
      *) shift ;;
    esac
  done
  [[ -n \$env_file ]]
  cat >"\$env_file" <<'SAFE_ENV'
BITRIVER_DEPLOY_IMAGE_SOURCE=pull
BITRIVER_LIVE_MODE=production
BITRIVER_LIVE_STORAGE_DRIVER=postgres
BITRIVER_LIVE_IMAGE_TAG=$source_release
BITRIVER_VIEWER_IMAGE_TAG=$source_release
BITRIVER_SRS_CONTROLLER_IMAGE_TAG=$source_release
BITRIVER_TRANSCODER_IMAGE_TAG=$source_release
BITRIVER_OME_CONFIG_IMAGE_TAG=$source_release
BITRIVER_POSTGRES_PASSWORD=$secret_sentinel
BITRIVER_REDIS_PASSWORD=$secret_sentinel
BITRIVER_OME_PASSWORD=$secret_sentinel
BITRIVER_OME_API_TOKEN=$secret_sentinel
SAFE_ENV
  chmod 0600 "\$env_file"
  exit 0
fi
echo "unexpected fake bitriver invocation: \$*" >&2
exit 1
FAKE_BITRIVER
cp "$repo_root/scripts/bitriver-live-wrapper.sh" "$binary_dir/bitriver-live"
chmod 0755 "$binary_dir/bitriver" "$binary_dir/bitriver-live"

installer="$bundle_root/deploy/install/compose-host.sh"
operator_user="$(id -un)"
bash "$installer" install \
  --source-root "$bundle_root" \
  --binary-dir "$binary_dir" \
  --root-prefix "$source_host" \
  --operator-user "$operator_user" >/dev/null

printf '{"durable":true,"kind":"api"}\n' \
  >"$source_host/var/lib/bitriver-live/api/recovery-fixture.json"
mkdir -p "$source_host/var/lib/bitriver-live/transcoder/public/live"
printf '#EXTM3U\n#EXT-X-ENDLIST\n' \
  >"$source_host/var/lib/bitriver-live/transcoder/public/live/recovered.m3u8"
printf 'external-recording-fixture\n' >"$external_source/recordings/recovered.ts"
mkdir -p "$external_offhost"
cp -a "$external_source/." "$external_offhost/"
bash "$repo_root/scripts/python.sh" "$repo_root/scripts/host_recovery.py" \
  object-inventory \
  --root "$external_source" \
  --output "$expected_object_inventory"

BITRIVER_BACKUP_RETAIN_DIR="$postgres_backup_dir" \
  bash "$repo_root/scripts/test-backup-restore.sh"
postgres_name="$(tr -d '\r\n' <"$postgres_backup_dir/backup-name.txt")"
postgres_backup="$postgres_backup_dir/$postgres_name"
[[ -f $postgres_backup ]] || fail "real Postgres rehearsal did not retain its backup set"

printf '%s\n' 'correct horse battery staple disaster recovery key' >"$passphrase_file"
chmod 0600 "$passphrase_file"
backup_timestamp="$(date -u +"%Y%m%dT%H%M%SZ")"
bash "$repo_root/scripts/backup-host-state.sh" \
  --root-prefix "$source_host" \
  --output-dir "$host_backup_dir" \
  --postgres-backup "$postgres_backup" \
  --source-release "$source_release" \
  --source-commit "$source_commit" \
  --passphrase-file "$passphrase_file" \
  --object-inventory "$expected_object_inventory" \
  --timestamp "$backup_timestamp"
host_archive="$host_backup_dir/bitriver-host-${backup_timestamp}.tar.gz.enc"

rm -rf -- "$source_host" "$external_source"
[[ ! -e $source_host && ! -e $external_source ]] || fail "source runtime survived the destructive cut"

bash "$repo_root/scripts/restore-host-state.sh" \
  --archive "$host_archive" \
  --root-prefix "$target_host" \
  --expected-release "$source_release" \
  --expected-commit "$source_commit" \
  --passphrase-file "$passphrase_file" \
  --report "$host_report"

bash "$installer" install \
  --source-root "$bundle_root" \
  --binary-dir "$binary_dir" \
  --root-prefix "$target_host" \
  --operator-user "$operator_user" >/dev/null
install_root="$target_host/opt/bitriver-live"
grep -Fq "BITRIVER_POSTGRES_PASSWORD=$secret_sentinel" \
  "$target_host/etc/bitriver-live/bitriver.env" || fail "installer did not preserve recovered secrets"
[[ -L $target_host/etc/bitriver-live/Server.generated.xml ]] ||
  fail "installer did not normalize recovered OME compatibility path"
[[ -L $install_root/deploy/data && -L $install_root/deploy/transcoder-data ]] ||
  fail "installer did not reconnect recovered durable data"

mkdir -p "$external_target"
cp -a "$external_offhost/." "$external_target/"
bash "$repo_root/scripts/python.sh" "$repo_root/scripts/host_recovery.py" \
  object-inventory \
  --root "$external_target" \
  --output "$observed_object_inventory"
cmp "$expected_object_inventory" "$observed_object_inventory"

docker run -d --name "$postgres_container" \
  -e POSTGRES_PASSWORD=disaster-recovery-postgres \
  "$postgres_image" >/dev/null
wait_for_postgres
docker_exec "$postgres_container" apk add --no-cache bash coreutils >/dev/null
docker_exec "$postgres_container" mkdir -p /recovery
docker cp "$install_root/scripts/restore-postgres.sh" \
  "$postgres_container:/restore-postgres.sh" >/dev/null
docker cp "$target_host/var/backups/bitriver-live/recovery/postgres/." \
  "$postgres_container:/recovery" >/dev/null
docker_exec \
  -e BITRIVER_BACKUP_POSTGRES_HOST=127.0.0.1 \
  -e BITRIVER_BACKUP_POSTGRES_USER=postgres \
  -e BITRIVER_BACKUP_POSTGRES_PASSWORD=disaster-recovery-postgres \
  -e BITRIVER_RESTORE_REHEARSAL_DB=bitr_disaster_recovered \
  -e BITRIVER_RESTORE_KEEP_DB=1 \
  -e BITRIVER_RESTORE_EXPECT_RELEASE="$source_release" \
  -e BITRIVER_RESTORE_REPORT_PATH=/recovery/postgres-restore-report.json \
  "$postgres_container" /bin/bash /restore-postgres.sh "/recovery/$postgres_name"

users="$(
  docker_exec -e PGPASSWORD=disaster-recovery-postgres \
    "$postgres_container" psql -X -qAt -v ON_ERROR_STOP=1 \
    -h 127.0.0.1 -U postgres -d bitr_disaster_recovered \
    -c 'SELECT count(*) FROM public.users;'
)"
[[ $users == 4 ]] || fail "fresh-host Postgres restore lost user fixtures"
object_fixture="$(
  docker_exec -e PGPASSWORD=disaster-recovery-postgres \
    "$postgres_container" psql -X -qAt -v ON_ERROR_STOP=1 \
    -h 127.0.0.1 -U postgres -d bitr_disaster_recovered \
    -c "SELECT object_key || '|' || size_bytes FROM public.object_fixtures;"
)"
[[ $object_fixture == 'uploads/fixture.mp4|4096' ]] ||
  fail "fresh-host Postgres restore lost object metadata"
docker cp "$postgres_container:/recovery/postgres-restore-report.json" \
  "$postgres_report" >/dev/null

bash "$repo_root/scripts/python.sh" "$repo_root/scripts/host_recovery.py" \
  disaster-report \
  --host-report "$host_report" \
  --postgres-report "$postgres_report" \
  --expected-object-inventory "$expected_object_inventory" \
  --observed-object-inventory "$observed_object_inventory" \
  --bundle-root "$bundle_root" \
  --installed-root "$install_root" \
  --destroyed-source-root "$source_host" \
  --source-release "$source_release" \
  --source-commit "$source_commit" \
  --started-at-epoch "$started_at_epoch" \
  --output "$disaster_report"

for evidence in "$host_report" "$postgres_report" "$disaster_report"; do
  if grep -Fq "$secret_sentinel" "$evidence"; then
    fail "sanitized disaster-recovery evidence exposed a host secret"
  fi
done
grep -Fq '"schemaVersion": "bitriver.disaster-recovery/v1"' "$disaster_report"
grep -Fq '"status": "passed"' "$disaster_report"

if [[ -n $retained_artifact_dir ]]; then
  mkdir -p "$retained_artifact_dir"
  cp "$host_report" "$postgres_report" "$disaster_report" "$retained_artifact_dir/"
  bash "$repo_root/scripts/scan-release-evidence.sh" --root "$retained_artifact_dir"
fi

echo "Source-free packaged-host disaster recovery rehearsal passed."
