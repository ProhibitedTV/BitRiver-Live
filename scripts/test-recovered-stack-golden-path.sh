#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
  cat <<'USAGE'
Usage: ./scripts/test-recovered-stack-golden-path.sh [options]

Download one exact public Linux launcher, destroy and recover a production-
shaped host, boot the recovered immutable Compose stack, and run the complete
production golden path against its restored database.

Required:
  --release TAG                 Exact prerelease tag.
  --source-commit SHA           Full lowercase source commit.
  --release-set-sha256 SHA256   Exact public release-set.json digest.

Options:
  --artifact-dir DIR            Fresh sanitized evidence directory.
  -h, --help                    Show this help.
USAGE
}

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
repository="ProhibitedTV/BitRiver-Live"
release=""
source_commit=""
release_set_sha256=""
artifact_dir=""
namespace="ghcr.io/prohibitedtv"
wait_timeout="${BITRIVER_RECOVERED_STACK_WAIT_TIMEOUT:-300}"
project="bitriver-recovered-${RANDOM}-$$"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --release) [[ $# -ge 2 ]] || { echo "--release requires a value" >&2; exit 2; }; release=$2; shift 2 ;;
    --source-commit) [[ $# -ge 2 ]] || { echo "--source-commit requires a value" >&2; exit 2; }; source_commit=$2; shift 2 ;;
    --release-set-sha256) [[ $# -ge 2 ]] || { echo "--release-set-sha256 requires a value" >&2; exit 2; }; release_set_sha256=$2; shift 2 ;;
    --artifact-dir) [[ $# -ge 2 ]] || { echo "--artifact-dir requires a value" >&2; exit 2; }; artifact_dir=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

[[ $release =~ ^v[0-9]+\.[0-9]+\.[0-9]+-[0-9A-Za-z.-]+$ ]] || {
  echo "--release must be an exact prerelease tag" >&2
  exit 2
}
[[ $source_commit =~ ^[0-9a-f]{40}$ ]] || {
  echo "--source-commit must be 40 lowercase hexadecimal characters" >&2
  exit 2
}
[[ $release_set_sha256 =~ ^[0-9a-f]{64}$ ]] || {
  echo "--release-set-sha256 must be 64 lowercase hexadecimal characters" >&2
  exit 2
}
if [[ -z $artifact_dir ]]; then
  artifact_dir="$repo_root/.artifacts/recovered-stack-golden-path/$release"
fi
mkdir -p "$artifact_dir"
artifact_dir="$(cd "$artifact_dir" && pwd -P)"
if find "$artifact_dir" -mindepth 1 -print -quit | grep -q .; then
  echo "artifact directory must be empty: $artifact_dir" >&2
  exit 2
fi

fail() {
  echo "Recovered-stack golden-path rehearsal failed: $*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

windows_posix_compat=false
case "$(uname -s)" in
  MINGW*|MSYS*) windows_posix_compat=true ;;
esac

native_path() {
  if [[ $windows_posix_compat == true ]]; then
    cygpath -m "$1"
  else
    realpath "$1"
  fi
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

python_helper() {
  bash "$script_dir/python.sh" "$script_dir/recovered_stack.py" "$@"
}

compose() {
  local root=$1
  shift
  (
    cd "$root"
    COMPOSE_PROJECT_NAME="$project" docker compose \
      --env-file .env \
      -f deploy/docker-compose.yml \
      "$@"
  )
}

env_value() {
  local root=$1 key=$2
  sed -n "s/^${key}=//p" "$root/.env" | tail -n 1 | tr -d '\r'
}

wait_for_service() {
  local root=$1 service=$2 wanted=$3
  local deadline=$(( $(date +%s) + wait_timeout )) id="" status="missing"
  while (( $(date +%s) < deadline )); do
    id="$(compose "$root" ps -a -q "$service")"
    if [[ -n $id ]]; then
      if [[ $wanted == completed ]]; then
        status="$(docker inspect -f '{{.State.Status}}:{{.State.ExitCode}}' "$id")"
        [[ $status == exited:0 ]] && return 0
        [[ $status == exited:* ]] && fail "$service exited unsuccessfully: $status"
      else
        status="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$id")"
        [[ $status == "$wanted" ]] && return 0
        [[ $status == unhealthy ]] && fail "$service became unhealthy"
      fi
    fi
    sleep 2
  done
  fail "timed out waiting for $service to become $wanted (last status: $status)"
}

http_status() {
  curl -sS -o /dev/null -w '%{http_code}' --connect-timeout 2 --max-time 5 "$1" 2>/dev/null || true
}

wait_for_http() {
  local url=$1 expected=$2
  local deadline=$(( $(date +%s) + wait_timeout )) observed="000"
  while (( $(date +%s) < deadline )); do
    observed="$(http_status "$url")"
    [[ $observed == "$expected" ]] && return 0
    sleep 2
  done
  fail "timed out waiting for HTTP $expected at $url (last status: $observed)"
}

pull_exact_images() {
  local root=$1 image
  while IFS= read -r image; do
    [[ -n $image ]] || continue
    docker pull "$image" >/dev/null
  done < <(compose "$root" config --images | sort -u)
}

database_invariants() {
  local root=$1 postgres_id
  postgres_id="$(compose "$root" ps -q postgres)"
  [[ -n $postgres_id ]] || fail "Postgres container is not running"
  docker_exec -i \
    -e PGPASSWORD="$(env_value "$root" BITRIVER_POSTGRES_PASSWORD)" \
    "$postgres_id" psql -X -qAt -v ON_ERROR_STOP=1 \
    -U "$(env_value "$root" BITRIVER_POSTGRES_USER)" \
    -d "$(env_value "$root" BITRIVER_POSTGRES_DB)" \
    < "$repo_root/scripts/fixtures/stateful-upgrade-invariants.sql"
}

invariant_fingerprint() {
  printf '%s\n' "$1" | sed -E 's/.*"valueFingerprintSha256": "([0-9a-f]{64})".*/\1/'
}

invariant_users() {
  printf '%s\n' "$1" | sed -E 's/.*"users": ([0-9]+).*/\1/'
}

workdir="$(mktemp -d)"
workdir="$(cd "$workdir" && pwd -P)"
download_root="$workdir/download"
extract_root="$workdir/extracted"
seed_root="$workdir/seed"
seed_backup_dir="$workdir/seed-backup"
recovered_root="$workdir/recovered-host"
private_root="$workdir/private"
initial_evidence="$artifact_dir/initial-disaster-recovery"
golden_evidence="$artifact_dir/production-golden-path"
active_root=""

cleanup() {
  set +e
  if [[ -n $active_root && -f $active_root/.env ]]; then
    compose "$active_root" down -v --remove-orphans >/dev/null 2>&1
  fi
  for root in "$seed_root" "$recovered_root/opt/bitriver-live"; do
    if [[ -f $root/.env ]]; then
      compose "$root" down -v --remove-orphans >/dev/null 2>&1
    fi
  done
  docker ps -aq --filter "label=com.docker.compose.project=$project" |
    xargs -r docker rm -f >/dev/null 2>&1
  docker volume ls -q --filter "label=com.docker.compose.project=$project" |
    xargs -r docker volume rm >/dev/null 2>&1
  rm -rf -- "$workdir"
}
trap cleanup EXIT

require_command curl
require_command docker
require_command sha256sum
require_command tar
docker compose version >/dev/null 2>&1 || fail "docker compose v2 is required"

for container_name in \
  bitriver-live bitriver-viewer bitriver-postgres bitriver-postgres-host-port \
  bitriver-srs-controller bitriver-srs bitriver-srs-api bitriver-srs-config \
  bitriver-ome-health-token-check bitriver-ome bitriver-transcoder \
  bitriver-transcoder-public; do
  if docker ps -aq --filter "name=^/${container_name}$" | grep -q .; then
    fail "refusing to collide with existing container $container_name"
  fi
done

mkdir -p \
  "$download_root" "$extract_root" "$seed_root" "$seed_backup_dir" \
  "$recovered_root" "$private_root" "$initial_evidence" "$golden_evidence"
package_name="bitriver-launcher-linux-amd64.tar.gz"
release_set="$download_root/release-set.json"
package_archive="$download_root/$package_name"
base_url="https://github.com/$repository/releases/download/$release"
curl --fail --location --silent --show-error --retry 3 \
  "$base_url/release-set.json" --output "$release_set"
curl --fail --location --silent --show-error --retry 3 \
  "$base_url/$package_name" --output "$package_archive"
[[ "$(sha256sum "$release_set" | cut -d ' ' -f 1)" == "$release_set_sha256" ]] ||
  fail "downloaded release-set SHA-256 does not match the requested identity"

bash "$script_dir/python.sh" "$script_dir/host_recovery.py" \
  verify-release-package \
  --release-set "$release_set" \
  --package "$package_archive" \
  --expected-release "$release" \
  --expected-commit "$source_commit" \
  --output "$workdir/package-binding.json"

members="$workdir/archive-members.txt"
tar -tzf "$package_archive" >"$members"
while IFS= read -r member || [[ -n $member ]]; do
  member=${member%$'\r'}
  trimmed=${member%/}
  case "$trimmed" in
    ""|/*|../*|*/../*|*/..|*\\*) fail "unsafe launcher archive member: $member" ;;
  esac
done <"$members"
tar -xzf "$package_archive" -C "$extract_root"
bundle_root="$extract_root/bitriver-launcher-linux-amd64/share/bitriver-live"
[[ -d $bundle_root ]] || fail "launcher archive does not contain the expected bundle"
cp -a "$bundle_root/." "$seed_root/"

if [[ $windows_posix_compat == true ]]; then
  # Docker Desktop bind mounts do not preserve the packaged installer's POSIX
  # ownership. Keep capabilities dropped but use root-owned recovered files;
  # clean-host Linux qualification owns non-root UID acceptance.
  host_uid=0
  host_gid=0
else
  host_uid="$(id -u)"
  host_gid="$(id -g)"
fi
recovered_environment="$private_root/recovered.env"
recovered_sentinels="$private_root/recovered-sentinels"
recovered_metadata="$private_root/recovered-input.json"
python_helper prepare-environment \
  --release-set "$release_set" \
  --template "$bundle_root/deploy/.env.example" \
  --expected-release "$release" \
  --expected-commit "$source_commit" \
  --expected-release-set-sha256 "$release_set_sha256" \
  --namespace "$namespace" \
  --config-root "$(native_path "$recovered_root/etc/bitriver-live")" \
  --host-uid "$host_uid" \
  --host-gid "$host_gid" \
  --bootstrap-database bitr_bootstrap \
  --restored-database bitr_recovered \
  --output "$recovered_environment" \
  --sentinel-output "$recovered_sentinels" \
  --metadata-output "$recovered_metadata"

seed_sentinels="$private_root/seed-sentinels"
python_helper prepare-environment \
  --release-set "$release_set" \
  --template "$bundle_root/deploy/.env.example" \
  --expected-release "$release" \
  --expected-commit "$source_commit" \
  --expected-release-set-sha256 "$release_set_sha256" \
  --namespace "$namespace" \
  --config-root "$(native_path "$seed_root")" \
  --host-uid "$host_uid" \
  --host-gid "$host_gid" \
  --bootstrap-database bitr_bootstrap \
  --restored-database bitr_recovered \
  --output "$seed_root/.env" \
  --sentinel-output "$seed_sentinels" \
  --metadata-output "$private_root/seed-input.json"

pull_exact_images "$seed_root"
active_root="$seed_root"
compose "$seed_root" up -d --no-build --pull never postgres postgres-migrations
wait_for_service "$seed_root" postgres healthy
wait_for_service "$seed_root" postgres-migrations completed
seed_postgres_id="$(compose "$seed_root" ps -q postgres)"
docker_exec -i \
  -e PGPASSWORD="$(env_value "$seed_root" BITRIVER_POSTGRES_PASSWORD)" \
  "$seed_postgres_id" psql -X -q -v ON_ERROR_STOP=1 \
  -U "$(env_value "$seed_root" BITRIVER_POSTGRES_USER)" \
  -d "$(env_value "$seed_root" BITRIVER_POSTGRES_DB)" \
  < "$repo_root/scripts/fixtures/stateful-upgrade.sql"
seed_invariants="$(database_invariants "$seed_root")"
[[ "$(invariant_users "$seed_invariants")" == 4 ]] ||
  fail "production-shaped seed database does not contain four fixture users"
docker_exec -u 0 "$seed_postgres_id" apk add --no-cache bash coreutils gzip >/dev/null
docker_exec "$seed_postgres_id" mkdir -p /seed-backups
docker_cp_from_host "$bundle_root/scripts/backup-postgres.sh" \
  "$seed_postgres_id:/backup-postgres.sh" >/dev/null
docker_exec \
  -e BITRIVER_BACKUP_DIR=/seed-backups \
  -e BITRIVER_BACKUP_POSTGRES_HOST=127.0.0.1 \
  -e BITRIVER_BACKUP_POSTGRES_USER="$(env_value "$seed_root" BITRIVER_POSTGRES_USER)" \
  -e BITRIVER_BACKUP_POSTGRES_PASSWORD="$(env_value "$seed_root" BITRIVER_POSTGRES_PASSWORD)" \
  -e BITRIVER_BACKUP_POSTGRES_DB="$(env_value "$seed_root" BITRIVER_POSTGRES_DB)" \
  -e BITRIVER_BACKUP_SOURCE_RELEASE="$release" \
  -e BITRIVER_BACKUP_SOURCE_COMMIT="$source_commit" \
  -e BITRIVER_BACKUP_RUN_PRUNE=0 \
  "$seed_postgres_id" /bin/bash /backup-postgres.sh
docker_cp_to_host "$seed_postgres_id:/seed-backups/." "$seed_backup_dir" >/dev/null
postgres_backup="$(find "$seed_backup_dir" -maxdepth 1 -name 'bitriver-postgres-*.sql.gz' -type f -print -quit)"
[[ -n $postgres_backup && -f ${postgres_backup}.manifest.json && -f ${postgres_backup}.sha256 ]] ||
  fail "production-shaped backup set was not retained"
compose "$seed_root" down -v --remove-orphans
active_root=""

recovery_started="$(date +%s)"
BITRIVER_DISASTER_RECOVERY_ARTIFACT_DIR="$initial_evidence" \
  bash "$script_dir/test-disaster-recovery.sh" \
    --bundle-root "$bundle_root" \
    --release-set "$release_set" \
    --package-archive "$package_archive" \
    --prepared-environment "$recovered_environment" \
    --postgres-backup "$postgres_backup" \
    --sentinel-file "$recovered_sentinels" \
    --export-recovered-root "$recovered_root" \
    --release "$release" \
    --source-commit "$source_commit"

install_root="$recovered_root/opt/bitriver-live"
config_environment="$recovered_root/etc/bitriver-live/bitriver.env"
[[ -f $install_root/deploy/docker-compose.yml && -f $config_environment ]] ||
  fail "lost-host drill did not export a complete recovered installation"
# Absolute links in a real installed host still point at the drill's temporary
# target after export. Relocate them to this final disposable recovered root.
if [[ -L $install_root/.env ]]; then
  rm -f -- "$install_root/.env"
  ln -s "$config_environment" "$install_root/.env"
fi
for managed_link in \
  "$install_root/deploy/ome/Server.generated.xml:$recovered_root/etc/bitriver-live/deploy/ome/Server.generated.xml" \
  "$install_root/deploy/srs/conf/srs.generated.conf:$recovered_root/etc/bitriver-live/deploy/srs/conf/srs.generated.conf" \
  "$install_root/deploy/data:$recovered_root/var/lib/bitriver-live/api" \
  "$install_root/deploy/transcoder-data:$recovered_root/var/lib/bitriver-live/transcoder"; do
  link_path=${managed_link%%:*}
  link_target=${managed_link#*:}
  if [[ -L $link_path ]]; then
    rm -f -- "$link_path"
    ln -s "$link_target" "$link_path"
  fi
done
# The packaged installer also normalizes the managed host path and identity.
# Restore the prepared final-root values; the lower-level drill proved every
# non-managed byte survived unchanged.
cp "$recovered_environment" "$config_environment"
chmod 0600 "$config_environment"
if [[ ! -L $install_root/.env ]]; then
  cp "$recovered_environment" "$install_root/.env"
  chmod 0600 "$install_root/.env"
fi
cmp "$recovered_environment" "$config_environment" ||
  fail "recovered environment bytes changed before runtime activation"
recovered_environment_snapshot="$private_root/recovered-snapshot.env"
cp "$config_environment" "$recovered_environment_snapshot"

active_root="$install_root"
compose "$install_root" up -d --no-build --pull never postgres
wait_for_service "$install_root" postgres healthy
runtime_postgres_id="$(compose "$install_root" ps -q postgres)"
docker_exec -u 0 "$runtime_postgres_id" apk add --no-cache bash coreutils gzip >/dev/null
docker_exec "$runtime_postgres_id" mkdir -p /recovery
runtime_postgres_root="$recovered_root/var/backups/bitriver-live/recovery/postgres"
runtime_backup="$(find "$runtime_postgres_root" -maxdepth 1 -name 'bitriver-postgres-*.sql.gz' -type f -print -quit)"
[[ -n $runtime_backup && -f ${runtime_backup}.manifest.json && -f ${runtime_backup}.sha256 ]] ||
  fail "recovered runtime is missing its Postgres backup set"
docker_cp_from_host "$install_root/scripts/restore-postgres.sh" \
  "$runtime_postgres_id:/restore-postgres.sh" >/dev/null
for backup_member in "$runtime_backup" "${runtime_backup}.manifest.json" "${runtime_backup}.sha256"; do
  docker_cp_from_host "$backup_member" \
    "$runtime_postgres_id:/recovery/$(basename "$backup_member")" >/dev/null
done
docker_exec \
  -e BITRIVER_BACKUP_POSTGRES_HOST=127.0.0.1 \
  -e BITRIVER_BACKUP_POSTGRES_USER="$(env_value "$install_root" BITRIVER_POSTGRES_USER)" \
  -e BITRIVER_BACKUP_POSTGRES_PASSWORD="$(env_value "$install_root" BITRIVER_POSTGRES_PASSWORD)" \
  -e BITRIVER_RESTORE_REHEARSAL_DB=bitr_recovered \
  -e BITRIVER_RESTORE_KEEP_DB=1 \
  -e BITRIVER_RESTORE_EXPECT_RELEASE="$release" \
  -e BITRIVER_RESTORE_REPORT_PATH=/recovery/runtime-postgres-restore-report.json \
  "$runtime_postgres_id" /bin/bash /restore-postgres.sh \
  "/recovery/$(basename "$runtime_backup")"
runtime_postgres_report="$private_root/runtime-postgres-restore-report.json"
docker_cp_to_host \
  "$runtime_postgres_id:/recovery/runtime-postgres-restore-report.json" \
  "$runtime_postgres_report" >/dev/null

runtime_environment="$private_root/runtime.env"
python_helper activate-restored-database \
  --environment "$recovered_environment_snapshot" \
  --metadata "$recovered_metadata" \
  --output "$runtime_environment"
cp "$runtime_environment" "$config_environment"
chmod 0600 "$config_environment"
if [[ ! -L $install_root/.env ]]; then
  cp "$runtime_environment" "$install_root/.env"
  chmod 0600 "$install_root/.env"
fi
compose "$install_root" up -d --force-recreate --no-build --pull never postgres
wait_for_service "$install_root" postgres healthy
compose "$install_root" up -d --no-build --pull never \
  postgres postgres-migrations srs-config ome-config
wait_for_service "$install_root" postgres healthy
for service in postgres-migrations srs-config ome-config; do
  wait_for_service "$install_root" "$service" completed
done
# Git for Windows may emulate installer symlinks as regular copies. Sync the
# newly rendered canonical files before services mount those copies.
for generated_config in \
  "$recovered_root/etc/bitriver-live/deploy/srs/conf/srs.generated.conf:$install_root/deploy/srs/conf/srs.generated.conf" \
  "$recovered_root/etc/bitriver-live/deploy/ome/Server.generated.xml:$install_root/deploy/ome/Server.generated.xml"; do
  generated_source=${generated_config%%:*}
  generated_target=${generated_config#*:}
  if [[ ! -L $generated_target ]]; then
    cp "$generated_source" "$generated_target"
  fi
done
if ! compose "$install_root" up -d --no-build --pull never \
  postgres redis srs srs-controller ome-health-token-check ome transcoder; then
  compose "$install_root" logs --no-color --tail 120 \
    postgres-migrations srs-config ome-config ome-health-token-check >&2 || true
  fail "recovered runtime dependencies did not start"
fi
for service in postgres redis srs srs-controller ome transcoder; do
  wait_for_service "$install_root" "$service" healthy
done
wait_for_service "$install_root" ome-health-token-check completed
compose "$install_root" up -d --no-build --pull never --no-deps bitriver-live
wait_for_service "$install_root" bitriver-live healthy
compose "$install_root" up -d --no-build --pull never --no-deps viewer transcoder-public
wait_for_http http://localhost:18080/healthz 200
wait_for_http http://localhost:18080/viewer 200

observations="$private_root/observed-images.tsv"
: >"$observations"
for service in \
  bitriver-live viewer srs-controller transcoder ome-config ome-health-token-check \
  postgres postgres-migrations redis srs srs-config ome transcoder-public; do
  container_id="$(compose "$install_root" ps -a -q "$service")"
  [[ -n $container_id ]] || fail "runtime image assertion has no container for $service"
  printf '%s\t%s\n' "$service" "$(docker inspect -f '{{.Config.Image}}' "$container_id")" \
    >>"$observations"
done
observed_images="$artifact_dir/recovered-runtime-images.json"
python_helper record-observed-images \
  --metadata "$recovered_metadata" \
  --observations "$observations" \
  --output "$observed_images"

pre_golden_invariants="$(database_invariants "$install_root")"
pre_golden_fingerprint="$(invariant_fingerprint "$pre_golden_invariants")"
pre_golden_users="$(invariant_users "$pre_golden_invariants")"
[[ $pre_golden_fingerprint =~ ^[0-9a-f]{64}$ && $pre_golden_users == 4 ]] ||
  fail "recovered database invariants do not match the production-shaped backup"
recovered_fixture_count="$(
  runtime_postgres_id="$(compose "$install_root" ps -q postgres)"
  docker_exec \
    -e PGPASSWORD="$(env_value "$install_root" BITRIVER_POSTGRES_PASSWORD)" \
    "$runtime_postgres_id" psql -X -qAt -v ON_ERROR_STOP=1 \
    -U "$(env_value "$install_root" BITRIVER_POSTGRES_USER)" \
    -d "$(env_value "$install_root" BITRIVER_POSTGRES_DB)" \
    -c "SELECT count(*) FROM channels WHERE id = 'channel-1' AND title = 'Upgrade channel';"
)"
[[ $recovered_fixture_count == 1 ]] || fail "recovered channel fixture is missing"
[[ -f $recovered_root/var/lib/bitriver-live/api/recovery-fixture.json ]] ||
  fail "recovered local durable-data fixture is missing"

metrics_file="$private_root/metrics-token"
printf '%s\n' "$(env_value "$install_root" BITRIVER_LIVE_METRICS_TOKEN)" >"$metrics_file"
chmod 0600 "$metrics_file"
"$script_dir/test-production-golden-path.sh" \
  --stack running \
  --client docker \
  --artifact-dir "$golden_evidence" \
  --base-url http://localhost:18080 \
  --rtmp-base-url rtmp://localhost:1935/live \
  --viewer-path /viewer \
  --internal-api-host bitriver-live:8080 \
  --metrics-bearer-file "$metrics_file"

post_golden_invariants="$(database_invariants "$install_root")"
post_golden_fingerprint="$(invariant_fingerprint "$post_golden_invariants")"
post_golden_users="$(invariant_users "$post_golden_invariants")"
[[ $post_golden_fingerprint == "$pre_golden_fingerprint" ]] ||
  fail "production golden path changed pre-existing recovered state"
(( post_golden_users > pre_golden_users )) ||
  fail "production golden path did not persist new recovered-stack accounts"

cp "$recovered_metadata" "$artifact_dir/recovered-stack-input.json"
cp "$runtime_postgres_report" "$artifact_dir/runtime-postgres-restore-report.json"
total_rto_seconds=$(( $(date +%s) - recovery_started ))
python_helper complete-disaster-report \
  --metadata "$recovered_metadata" \
  --disaster-report "$initial_evidence/disaster-recovery-report.json" \
  --original-postgres-report "$initial_evidence/postgres-restore-report.json" \
  --runtime-postgres-report "$runtime_postgres_report" \
  --golden-report "$golden_evidence/production-golden-path.json" \
  --observed-images "$observed_images" \
  --recovered-environment "$recovered_environment_snapshot" \
  --runtime-environment "$runtime_environment" \
  --sentinel-file "$recovered_sentinels" \
  --expected-release "$release" \
  --expected-commit "$source_commit" \
  --pre-users "$pre_golden_users" \
  --post-users "$post_golden_users" \
  --recovered-fixture-count "$recovered_fixture_count" \
  --total-rto-seconds "$total_rto_seconds" \
  --output "$artifact_dir/recovered-stack-disaster-recovery-report.json"

bash "$script_dir/scan-release-evidence.sh" \
  --root "$artifact_dir" \
  --sentinel-file "$recovered_sentinels"
echo "Recovered immutable-stack golden-path evidence retained in $artifact_dir"
