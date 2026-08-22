#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
  cat <<'USAGE'
Usage: ./scripts/test-disaster-recovery.sh [options]

Run the disposable packaged-host lost-host recovery rehearsal.

Options:
  --bundle-root DIR        Extracted source-free launcher share/bitriver-live root.
  --release-set FILE       Exact downloaded release-set.json (published mode).
  --package-archive FILE   Exact downloaded launcher archive (published mode).
  --prepared-environment FILE
                            Private production-shaped environment to recover.
  --postgres-backup FILE   Complete product-schema backup set to recover.
  --sentinel-file FILE      Private sentinel list for prepared-environment mode.
  --export-recovered-root DIR
                            Existing empty temporary directory for the restored host.
  --release TAG            Exact release identity (default v1.2.3-rc.test).
  --source-commit SHA      Full source commit (default test fixture commit).
  -h, --help               Show this help.

Published mode requires all three package inputs. With no package inputs, the
test stages the current checkout's source-free release assets as before.
USAGE
}

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
input_bundle_root=""
release_set=""
package_archive=""
prepared_environment=""
input_postgres_backup=""
input_sentinel_file=""
export_recovered_root=""
source_release="${BITRIVER_DISASTER_SOURCE_RELEASE:-v1.2.3-rc.test}"
source_commit="${BITRIVER_DISASTER_SOURCE_COMMIT:-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --bundle-root) [[ $# -ge 2 ]] || { echo "--bundle-root requires a value" >&2; exit 2; }; input_bundle_root=$2; shift 2 ;;
    --release-set) [[ $# -ge 2 ]] || { echo "--release-set requires a value" >&2; exit 2; }; release_set=$2; shift 2 ;;
    --package-archive) [[ $# -ge 2 ]] || { echo "--package-archive requires a value" >&2; exit 2; }; package_archive=$2; shift 2 ;;
    --prepared-environment) [[ $# -ge 2 ]] || { echo "--prepared-environment requires a value" >&2; exit 2; }; prepared_environment=$2; shift 2 ;;
    --postgres-backup) [[ $# -ge 2 ]] || { echo "--postgres-backup requires a value" >&2; exit 2; }; input_postgres_backup=$2; shift 2 ;;
    --sentinel-file) [[ $# -ge 2 ]] || { echo "--sentinel-file requires a value" >&2; exit 2; }; input_sentinel_file=$2; shift 2 ;;
    --export-recovered-root) [[ $# -ge 2 ]] || { echo "--export-recovered-root requires a value" >&2; exit 2; }; export_recovered_root=$2; shift 2 ;;
    --release) [[ $# -ge 2 ]] || { echo "--release requires a value" >&2; exit 2; }; source_release=$2; shift 2 ;;
    --source-commit) [[ $# -ge 2 ]] || { echo "--source-commit requires a value" >&2; exit 2; }; source_commit=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

[[ $source_release =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || {
  echo "disaster recovery release must be an exact v-prefixed release" >&2
  exit 2
}
[[ $source_commit =~ ^[0-9a-f]{40}$ ]] || {
  echo "disaster recovery source commit must be 40 lowercase hexadecimal characters" >&2
  exit 2
}
published_mode=false
if [[ -n $input_bundle_root || -n $release_set || -n $package_archive ]]; then
  [[ -n $input_bundle_root && -n $release_set && -n $package_archive ]] || {
    echo "published mode requires --bundle-root, --release-set, and --package-archive together" >&2
    exit 2
  }
  published_mode=true
fi
if [[ -n $prepared_environment || -n $input_postgres_backup || -n $input_sentinel_file || -n $export_recovered_root ]]; then
  [[ $published_mode == true ]] || {
    echo "recovered-root export requires published mode" >&2
    exit 2
  }
  [[ -n $prepared_environment && -n $input_postgres_backup && -n $input_sentinel_file && -n $export_recovered_root ]] || {
    echo "--prepared-environment, --postgres-backup, --sentinel-file, and --export-recovered-root are required together" >&2
    exit 2
  }
  [[ -f $prepared_environment && -f $input_postgres_backup && -f ${input_postgres_backup}.manifest.json && -f ${input_postgres_backup}.sha256 && -f $input_sentinel_file && -d $export_recovered_root ]] || {
    echo "prepared recovery inputs are missing" >&2
    exit 2
  }
  if find "$export_recovered_root" -mindepth 1 -print -quit | grep -q .; then
    echo "exported recovered-root directory must be empty" >&2
    exit 2
  fi
  prepared_environment="$(cd "$(dirname "$prepared_environment")" && pwd -P)/$(basename "$prepared_environment")"
  input_postgres_backup="$(cd "$(dirname "$input_postgres_backup")" && pwd -P)/$(basename "$input_postgres_backup")"
  input_sentinel_file="$(cd "$(dirname "$input_sentinel_file")" && pwd -P)/$(basename "$input_sentinel_file")"
  export_recovered_root="$(cd "$export_recovered_root" && pwd -P)"
fi
windows_posix_compat=false
case "$(uname -s)" in
  MINGW*|MSYS*) windows_posix_compat=true ;;
esac

workdir="$(mktemp -d)"
workdir="$(cd "$workdir" && pwd -P)"
bundle_root="$workdir/source-free-bundle"
binary_dir="$workdir/bin"
fake_bin="$workdir/fake-bin"
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

docker_cp_from_host() {
  local source=$1 destination=$2
  if [[ $windows_posix_compat == true ]]; then
    source="$(cygpath -w "$source")"
  fi
  MSYS_NO_PATHCONV=1 docker cp "$source" "$destination"
}

docker_cp_to_host() {
  local source=$1 destination=$2
  if [[ $windows_posix_compat == true ]]; then
    destination="$(cygpath -w "$destination")"
  fi
  MSYS_NO_PATHCONV=1 docker cp "$source" "$destination"
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

mkdir -p "$binary_dir" "$external_source/recordings"
if [[ $published_mode == true ]]; then
  [[ -d $input_bundle_root ]] || fail "published bundle root is missing"
  [[ -f $release_set ]] || fail "published release-set is missing"
  [[ -f $package_archive ]] || fail "published package archive is missing"
  bundle_root="$(cd "$input_bundle_root" && pwd -P)"
  release_set="$(cd "$(dirname "$release_set")" && pwd -P)/$(basename "$release_set")"
  package_archive="$(cd "$(dirname "$package_archive")" && pwd -P)/$(basename "$package_archive")"
  bash "$repo_root/scripts/python.sh" "$repo_root/scripts/host_recovery.py" \
    verify-release-package \
    --release-set "$release_set" \
    --package "$package_archive" \
    --expected-release "$source_release" \
    --expected-commit "$source_commit" \
    --bundle-root "$bundle_root" \
    --output "$workdir/published-package-binding.json"
else
  mkdir -p "$bundle_root"
  bash "$repo_root/scripts/stage-release-assets.sh" \
    --output "$bundle_root" \
    --release-tag "$source_release"
fi
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
packaged_scripts="$bundle_root/scripts"

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
cp "$packaged_scripts/bitriver-live-wrapper.sh" "$binary_dir/bitriver-live"
chmod 0755 "$binary_dir/bitriver" "$binary_dir/bitriver-live"

# Git for Windows can expose a numeric primary group without a resolvable
# group name. Keep the packaged installer unchanged and provide a test-local
# `id -gn` fallback to that numeric group; all other id queries remain real.
real_id="$(command -v id)"
mkdir -p "$fake_bin"
cat >"$fake_bin/id" <<'ID_WRAPPER'
#!/usr/bin/env bash
set -Eeuo pipefail
if output="$("$BITRIVER_TEST_REAL_ID" "$@" 2>/dev/null)"; then
  printf '%s\n' "$output"
elif [[ ${1:-} == -gn && $# -eq 2 ]]; then
  "$BITRIVER_TEST_REAL_ID" -g "$2"
else
  "$BITRIVER_TEST_REAL_ID" "$@"
fi
ID_WRAPPER
chmod 0755 "$fake_bin/id"

# OpenSSL's Git-for-Windows binary does not path-convert a filename after the
# `file:` passphrase prefix. Keep the published recovery scripts unchanged and
# translate only those test-local passphrase arguments to native paths.
real_openssl="$(command -v openssl)"
if [[ $windows_posix_compat == true ]]; then
  cat >"$fake_bin/openssl" <<'OPENSSL_WRAPPER'
#!/usr/bin/env bash
set -Eeuo pipefail
translated=()
for argument in "$@"; do
  case "$argument" in
    file:/[A-Za-z]/*) argument="file:$(cygpath -m "${argument#file:}")" ;;
  esac
  translated+=("$argument")
done
exec "$BITRIVER_TEST_REAL_OPENSSL" "${translated[@]}"
OPENSSL_WRAPPER
  chmod 0755 "$fake_bin/openssl"
fi

# The Windows test filesystem may permit writes while refusing POSIX chmod.
# The clean-host Linux gate owns permission-mode acceptance; this disposable
# recovery driver only needs equivalent copy/directory behavior on that host.
install_probe="$workdir/install-mode-probe"
if ! install -d -m 0750 "$install_probe" 2>/dev/null; then
  rm -rf -- "$install_probe"
  cat >"$fake_bin/install" <<'INSTALL_WRAPPER'
#!/usr/bin/env bash
set -Eeuo pipefail
directory=false
parents=false
operands=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    -d) directory=true; shift ;;
    -D) parents=true; shift ;;
    -m|-o|-g) [[ $# -ge 2 ]] || exit 2; shift 2 ;;
    --) shift; operands+=("$@"); break ;;
    -*) echo "unsupported test-local install option: $1" >&2; exit 2 ;;
    *) operands+=("$1"); shift ;;
  esac
done
if [[ $directory == true ]]; then
  (( ${#operands[@]} > 0 )) || exit 2
  exec mkdir -p "${operands[@]}"
fi
(( ${#operands[@]} == 2 )) || exit 2
source=${operands[0]}
destination=${operands[1]}
if [[ $parents == true ]]; then
  mkdir -p "$(dirname "$destination")"
fi
exec cp "$source" "$destination"
INSTALL_WRAPPER
  chmod 0755 "$fake_bin/install"
else
  rm -rf -- "$install_probe"
fi

installer="$bundle_root/deploy/install/compose-host.sh"
operator_user="$(id -un)"
operator_uid="$(id -u "$operator_user")"
operator_gid="$(id -g "$operator_user")"
PATH="$fake_bin:$PATH" BITRIVER_TEST_REAL_ID="$real_id" \
  bash "$installer" install \
  --source-root "$bundle_root" \
  --binary-dir "$binary_dir" \
  --root-prefix "$source_host" \
  --operator-user "$operator_user" >/dev/null
if [[ -n $prepared_environment ]]; then
  source_environment="$source_host/etc/bitriver-live/bitriver.env"
  source_install_environment="$source_host/opt/bitriver-live/.env"
  cp "$prepared_environment" "$source_environment"
  chmod 0600 "$source_environment"
  if [[ ! -L $source_install_environment ]]; then
    cp "$prepared_environment" "$source_install_environment"
    chmod 0600 "$source_install_environment"
  fi
fi

printf '{"durable":true,"kind":"api"}\n' \
  >"$source_host/var/lib/bitriver-live/api/recovery-fixture.json"
mkdir -p "$source_host/var/lib/bitriver-live/transcoder/public/live"
printf '#EXTM3U\n#EXT-X-ENDLIST\n' \
  >"$source_host/var/lib/bitriver-live/transcoder/public/live/recovered.m3u8"
printf 'external-recording-fixture\n' >"$external_source/recordings/recovered.ts"
mkdir -p "$external_offhost"
cp -a "$external_source/." "$external_offhost/"
bash "$packaged_scripts/python.sh" "$packaged_scripts/host_recovery.py" \
  object-inventory \
  --root "$external_source" \
  --output "$expected_object_inventory"

if [[ -n $input_postgres_backup ]]; then
  postgres_backup="$input_postgres_backup"
  postgres_name="$(basename "$postgres_backup")"
else
  BITRIVER_BACKUP_RETAIN_DIR="$postgres_backup_dir" \
  BITRIVER_BACKUP_TEST_SCRIPT_ROOT="$packaged_scripts" \
  BITRIVER_BACKUP_TEST_RELEASE="$source_release" \
  BITRIVER_BACKUP_TEST_COMMIT="$source_commit" \
    bash "$repo_root/scripts/test-backup-restore.sh"
  postgres_name="$(tr -d '\r\n' <"$postgres_backup_dir/backup-name.txt")"
  postgres_backup="$postgres_backup_dir/$postgres_name"
fi
[[ -f $postgres_backup ]] || fail "real Postgres rehearsal did not retain its backup set"

printf '%s\n' 'correct horse battery staple disaster recovery key' >"$passphrase_file"
chmod 0600 "$passphrase_file"
backup_timestamp="$(date -u +"%Y%m%dT%H%M%SZ")"
PATH="$fake_bin:$PATH" BITRIVER_TEST_REAL_OPENSSL="$real_openssl" \
  bash "$packaged_scripts/backup-host-state.sh" \
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

PATH="$fake_bin:$PATH" BITRIVER_TEST_REAL_OPENSSL="$real_openssl" \
  bash "$packaged_scripts/restore-host-state.sh" \
  --archive "$host_archive" \
  --root-prefix "$target_host" \
  --expected-release "$source_release" \
  --expected-commit "$source_commit" \
  --passphrase-file "$passphrase_file" \
  --report "$host_report"

PATH="$fake_bin:$PATH" BITRIVER_TEST_REAL_ID="$real_id" \
  bash "$installer" install \
  --source-root "$bundle_root" \
  --binary-dir "$binary_dir" \
  --root-prefix "$target_host" \
  --operator-user "$operator_user" >/dev/null
install_root="$target_host/opt/bitriver-live"
if [[ -z $prepared_environment ]]; then
  grep -Fq "BITRIVER_POSTGRES_PASSWORD=$secret_sentinel" \
    "$target_host/etc/bitriver-live/bitriver.env" || fail "installer did not preserve recovered secrets"
fi
if [[ ! -L $target_host/etc/bitriver-live/Server.generated.xml ]]; then
  if [[ $windows_posix_compat != true ]] ||
    ! cmp "$target_host/etc/bitriver-live/Server.generated.xml" \
      "$target_host/etc/bitriver-live/deploy/ome/Server.generated.xml"; then
    fail "installer did not normalize recovered OME compatibility path"
  fi
fi
if [[ -n $prepared_environment ]]; then
  normalized_prepared_environment="$workdir/normalized-prepared.env"
  awk \
    -v config_root="$target_host/etc/bitriver-live" \
    -v host_uid="$operator_uid" \
    -v host_gid="$operator_gid" '
    /^BITRIVER_CONFIG_ROOT=/ { print "BITRIVER_CONFIG_ROOT=" config_root; found = 1; next }
    /^BITRIVER_HOST_UID=/ { print "BITRIVER_HOST_UID=" host_uid; uid_found = 1; next }
    /^BITRIVER_HOST_GID=/ { print "BITRIVER_HOST_GID=" host_gid; gid_found = 1; next }
    { print }
    END { if (!found || !uid_found || !gid_found) exit 2 }
  ' "$prepared_environment" >"$normalized_prepared_environment"
  cmp "$normalized_prepared_environment" "$target_host/etc/bitriver-live/bitriver.env" ||
    fail "prepared environment changed beyond installer-managed host normalization"
  if [[ ! -L $install_root/.env ]]; then
    cmp "$normalized_prepared_environment" "$install_root/.env" ||
      fail "installed environment copy does not match recovered configuration"
  fi
fi
if [[ ! -L $install_root/deploy/data || ! -L $install_root/deploy/transcoder-data ]]; then
  if [[ $windows_posix_compat != true ]] ||
    [[ ! -f $install_root/deploy/data/recovery-fixture.json ]] ||
    [[ ! -f $install_root/deploy/transcoder-data/public/live/recovered.m3u8 ]]; then
    fail "installer did not reconnect recovered durable data"
  fi
fi

mkdir -p "$external_target"
cp -a "$external_offhost/." "$external_target/"
bash "$packaged_scripts/python.sh" "$packaged_scripts/host_recovery.py" \
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
docker_cp_from_host "$install_root/scripts/restore-postgres.sh" \
  "$postgres_container:/restore-postgres.sh" >/dev/null
recovered_postgres_root="$target_host/var/backups/bitriver-live/recovery/postgres"
for recovered_postgres_file in \
  "$recovered_postgres_root/$postgres_name" \
  "$recovered_postgres_root/${postgres_name}.manifest.json" \
  "$recovered_postgres_root/${postgres_name}.sha256"; do
  [[ -f $recovered_postgres_file ]] || fail "recovered Postgres set is incomplete"
  docker_cp_from_host "$recovered_postgres_file" \
    "$postgres_container:/recovery/$(basename "$recovered_postgres_file")" >/dev/null
done
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
if [[ -n $input_postgres_backup ]]; then
  channel_fixture="$(
    docker_exec -e PGPASSWORD=disaster-recovery-postgres \
      "$postgres_container" psql -X -qAt -v ON_ERROR_STOP=1 \
      -h 127.0.0.1 -U postgres -d bitr_disaster_recovered \
      -c "SELECT title FROM public.channels WHERE id = 'channel-1';"
  )"
  [[ $channel_fixture == 'Upgrade channel' ]] ||
    fail "fresh-host Postgres restore lost product-schema channel state"
else
  object_fixture="$(
    docker_exec -e PGPASSWORD=disaster-recovery-postgres \
      "$postgres_container" psql -X -qAt -v ON_ERROR_STOP=1 \
      -h 127.0.0.1 -U postgres -d bitr_disaster_recovered \
      -c "SELECT object_key || '|' || size_bytes FROM public.object_fixtures;"
  )"
  [[ $object_fixture == 'uploads/fixture.mp4|4096' ]] ||
    fail "fresh-host Postgres restore lost object metadata"
fi
docker_cp_to_host "$postgres_container:/recovery/postgres-restore-report.json" \
  "$postgres_report" >/dev/null

disaster_report_args=(
  disaster-report
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
)
if [[ $published_mode == true ]]; then
  disaster_report_args+=(
    --release-set "$release_set"
    --package-archive "$package_archive"
  )
fi
bash "$repo_root/scripts/python.sh" "$repo_root/scripts/host_recovery.py" \
  "${disaster_report_args[@]}"

for evidence in "$host_report" "$postgres_report" "$disaster_report"; do
  if grep -Fq "$secret_sentinel" "$evidence"; then
    fail "sanitized disaster-recovery evidence exposed a host secret"
  fi
done
grep -Fq '"schemaVersion": "bitriver.disaster-recovery/v1"' "$disaster_report"
grep -Fq '"status": "passed"' "$disaster_report"
if [[ $published_mode == true ]]; then
  grep -Fq '"verified": true' "$disaster_report"
fi

if [[ -n $retained_artifact_dir ]]; then
  mkdir -p "$retained_artifact_dir"
  cp "$host_report" "$postgres_report" "$disaster_report" "$retained_artifact_dir/"
  if [[ $published_mode == true ]]; then
    cp "$release_set" "$workdir/published-package-binding.json" "$retained_artifact_dir/"
  fi
  scan_args=(--root "$retained_artifact_dir")
  if [[ -n $input_sentinel_file ]]; then
    scan_args+=(--sentinel-file "$input_sentinel_file")
  fi
  bash "$repo_root/scripts/scan-release-evidence.sh" "${scan_args[@]}"
fi

if [[ -n $export_recovered_root ]]; then
  cp -a "$target_host/." "$export_recovered_root/"
fi

if [[ $published_mode == true ]]; then
  echo "Published-package lost-host disaster recovery rehearsal passed."
else
  echo "Source-free packaged-host disaster recovery rehearsal passed."
fi
