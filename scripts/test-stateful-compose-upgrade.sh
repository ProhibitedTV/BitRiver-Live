#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source_release="v1.2.3-rc.19"
source_commit="1e14e3cf7d5f1d949b396d4f7897660575ea468e"
source_release_set_sha="374a4084d1880abab1fa980d528a47bb5e324ed85541248438015fb13f2cc204"
candidate_release="v1.2.3-rc.20"
candidate_commit="9a8516a60c584c96a46b630b55c46df33f46fbdc"
candidate_release_set_sha="dd8eabcea7cf920a6f520e3e472cf44d3e1c7b0b7ad74945904f67ea74a47873"
namespace="ghcr.io/prohibitedtv"
project="bitriver-upgrade-${RANDOM}-$$"
workdir="$(mktemp -d)"
source_root="$workdir/source"
candidate_root="$workdir/candidate"
override_file="$workdir/upgrade-volume-override.yml"
evidence_dir="$workdir/evidence"
backup_dir="$workdir/backup"
retained_artifact_dir="${BITRIVER_COMPOSE_UPGRADE_ARTIFACT_DIR:-}"
wait_timeout="${BITRIVER_COMPOSE_UPGRADE_WAIT_TIMEOUT:-300}"
active_root=""

fail() {
  echo "Stateful Compose upgrade rehearsal failed: $*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

compose() {
  local root="$1"
  shift
  (
    cd "$root"
    COMPOSE_PROJECT_NAME="$project" docker compose \
      --env-file .env \
      -f deploy/docker-compose.yml \
      -f "$override_file" \
      "$@"
  )
}

docker_exec() {
  MSYS_NO_PATHCONV=1 docker exec "$@"
}

cleanup() {
  set +e
  if [[ -n "$active_root" && -f "$active_root/.env" ]]; then
    compose "$active_root" down -v --remove-orphans >/dev/null 2>&1
  fi
  for root in "$source_root" "$candidate_root"; do
    if [[ -f "$root/.env" ]]; then
      compose "$root" down -v --remove-orphans >/dev/null 2>&1
    fi
  done
  docker ps -aq --filter "label=com.docker.compose.project=$project" |
    xargs -r docker rm -f >/dev/null 2>&1
  docker volume ls -q --filter "label=com.docker.compose.project=$project" |
    xargs -r docker volume rm >/dev/null 2>&1
  rm -rf "$workdir"
}
trap cleanup EXIT

env_value() {
  local root="$1"
  local key="$2"
  (
    set -a
    # shellcheck disable=SC1090,SC1091
    source "$root/.env"
    set +a
    printf '%s' "${!key:-}"
  )
}

sha256_path() {
  local line
  line="$(sha256sum "$1")"
  printf '%s\n' "${line%% *}"
}

wait_for_service() {
  local root="$1"
  local service="$2"
  local wanted="$3"
  local deadline=$(( $(date +%s) + wait_timeout ))
  local id=""
  local status="missing"
  while (( $(date +%s) < deadline )); do
    id="$(compose "$root" ps -a -q "$service")"
    if [[ -n "$id" ]]; then
      if [[ "$wanted" == "completed" ]]; then
        status="$(docker inspect -f '{{.State.Status}}:{{.State.ExitCode}}' "$id")"
        [[ "$status" == "exited:0" ]] && return 0
        [[ "$status" == exited:* ]] && fail "$service exited unsuccessfully: $status"
      else
        status="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$id")"
        [[ "$status" == "$wanted" ]] && return 0
        [[ "$status" == "unhealthy" ]] && fail "$service became unhealthy"
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
  local url="$1"
  local expected="$2"
  local deadline=$(( $(date +%s) + wait_timeout ))
  local observed="000"
  while (( $(date +%s) < deadline )); do
    observed="$(http_status "$url")"
    [[ "$observed" == "$expected" ]] && return 0
    sleep 2
  done
  fail "timed out waiting for HTTP $expected at $url (last status: $observed)"
}

pull_exact_images() {
  local root="$1"
  local image
  while IFS= read -r image; do
    [[ -n "$image" ]] || continue
    docker pull "$image" >/dev/null
  done < <(compose "$root" config --images | sort -u)
}

expected_image_reference() {
  local root="$1"
  local image_name="$2"
  local tag_key=""
  local digest_key=""
  case "$image_name" in
    bitriver-live)
      tag_key=BITRIVER_LIVE_IMAGE_TAG
      digest_key=BITRIVER_LIVE_IMAGE_DIGEST
      ;;
    bitriver-viewer)
      tag_key=BITRIVER_VIEWER_IMAGE_TAG
      digest_key=BITRIVER_VIEWER_IMAGE_DIGEST
      ;;
    bitriver-srs-controller)
      tag_key=BITRIVER_SRS_CONTROLLER_IMAGE_TAG
      digest_key=BITRIVER_SRS_CONTROLLER_IMAGE_DIGEST
      ;;
    bitriver-transcoder)
      tag_key=BITRIVER_TRANSCODER_IMAGE_TAG
      digest_key=BITRIVER_TRANSCODER_IMAGE_DIGEST
      ;;
    bitriver-ome-config)
      tag_key=BITRIVER_OME_CONFIG_IMAGE_TAG
      digest_key=BITRIVER_OME_CONFIG_IMAGE_DIGEST
      ;;
    *)
      fail "unknown first-party image: $image_name"
      ;;
  esac
  printf '%s/%s:%s%s\n' \
    "$(env_value "$root" BITRIVER_IMAGE_NAMESPACE)" \
    "$image_name" \
    "$(env_value "$root" "$tag_key")" \
    "$(env_value "$root" "$digest_key")"
}

assert_service_image() {
  local root="$1"
  local service="$2"
  local image_name="$3"
  local id
  local actual
  local expected
  id="$(compose "$root" ps -a -q "$service")"
  [[ -n "$id" ]] || fail "no container found for image assertion: $service"
  actual="$(docker inspect -f '{{.Config.Image}}' "$id")"
  expected="$(expected_image_reference "$root" "$image_name")"
  [[ "$actual" == "$expected" ]] ||
    fail "$service image mismatch: expected $expected, observed $actual"
}

assert_first_party_images() {
  local root="$1"
  assert_service_image "$root" bitriver-live bitriver-live
  assert_service_image "$root" viewer bitriver-viewer
  assert_service_image "$root" srs-controller bitriver-srs-controller
  assert_service_image "$root" transcoder bitriver-transcoder
  assert_service_image "$root" ome-config bitriver-ome-config
}

database_invariants() {
  local root="$1"
  local postgres_id
  postgres_id="$(compose "$root" ps -q postgres)"
  [[ -n "$postgres_id" ]] || fail "Postgres container is not running"
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

start_dependencies() {
  local root="$1"
  compose "$root" up -d --no-build --pull never \
    postgres redis srs srs-controller ome transcoder postgres-migrations
  for service in postgres redis srs srs-controller ome transcoder; do
    wait_for_service "$root" "$service" healthy
  done
  wait_for_service "$root" postgres-migrations completed
}

start_applications() {
  local root="$1"
  compose "$root" up -d --no-build --pull never --no-deps bitriver-live
  wait_for_service "$root" bitriver-live healthy
  compose "$root" up -d --no-build --pull never --no-deps viewer transcoder-public
  wait_for_http http://localhost:18080/healthz 200
  wait_for_http http://localhost:18080/viewer 200
}

require_command curl
require_command docker
require_command git
require_command sha256sum
require_command tar
docker compose version >/dev/null 2>&1 || fail "docker compose v2 is required"

for container_name in \
  bitriver-live bitriver-viewer bitriver-postgres bitriver-srs \
  bitriver-srs-controller bitriver-srs-config bitriver-transcoder \
  bitriver-transcoder-public bitriver-ome bitriver-ome-health-token-check; do
  if docker ps -aq --filter "name=^/${container_name}$" | grep -q .; then
    fail "refusing to collide with existing container $container_name"
  fi
done

mkdir -p "$source_root" "$candidate_root" "$evidence_dir" "$backup_dir"
git -C "$repo_root" archive "$source_commit" | tar -xf - -C "$source_root"
git -C "$repo_root" archive "$candidate_commit" | tar -xf - -C "$candidate_root"
[[ "$(git -C "$repo_root" rev-parse "$source_release^{commit}")" == "$source_commit" ]] ||
  fail "$source_release does not resolve to the recorded source commit"
[[ "$(git -C "$repo_root" rev-parse "$candidate_release^{commit}")" == "$candidate_commit" ]] ||
  fail "$candidate_release does not resolve to the recorded candidate commit"

curl -fsSL --retry 3 \
  "https://github.com/ProhibitedTV/BitRiver-Live/releases/download/$source_release/release-set.json" \
  -o "$workdir/source-release-set.json"
curl -fsSL --retry 3 \
  "https://github.com/ProhibitedTV/BitRiver-Live/releases/download/$candidate_release/release-set.json" \
  -o "$workdir/candidate-release-set.json"

"$repo_root/scripts/python.sh" "$repo_root/scripts/stateful_compose_upgrade.py" \
  --source-release-set "$workdir/source-release-set.json" \
  --candidate-release-set "$workdir/candidate-release-set.json" \
  --source-template "$source_root/deploy/.env.example" \
  --source-output "$source_root/.env" \
  --candidate-output "$candidate_root/.env" \
  --sentinel-output "$workdir/environment-sentinels" \
  --metadata-output "$workdir/input-metadata.json" \
  --source-tag "$source_release" \
  --source-commit "$source_commit" \
  --source-sha256 "$source_release_set_sha" \
  --candidate-tag "$candidate_release" \
  --candidate-commit "$candidate_commit" \
  --candidate-sha256 "$candidate_release_set_sha" \
  --namespace "$namespace"

cat > "$override_file" <<'YAML'
services:
  transcoder:
    volumes:
      - bitriver-upgrade-transcoder:/work
  transcoder-public:
    volumes:
      - bitriver-upgrade-transcoder:/work:ro
volumes:
  bitriver-upgrade-transcoder:
YAML

pull_started="$(date +%s)"
pull_exact_images "$source_root"
pull_exact_images "$candidate_root"
pull_seconds=$(( $(date +%s) - pull_started ))

source_started="$(date +%s)"
active_root="$source_root"
start_dependencies "$source_root"
compose "$source_root" up -d --no-build --pull never --no-deps bitriver-live
wait_for_http http://localhost:18080/readyz 200
compose "$source_root" up -d --no-build --pull never --no-deps viewer transcoder-public
wait_for_http http://localhost:18080/viewer 200
source_health_status="$(http_status http://localhost:18080/healthz)"
[[ "$source_health_status" != "000" ]] ||
  fail "RC19 aggregate health was unreachable"
source_health_healthy=false
if [[ "$source_health_status" == 2* || "$source_health_status" == 3* ]]; then
  source_health_healthy=true
fi
assert_first_party_images "$source_root"

source_postgres_id="$(compose "$source_root" ps -q postgres)"
docker_exec -i \
  -e PGPASSWORD="$(env_value "$source_root" BITRIVER_POSTGRES_PASSWORD)" \
  "$source_postgres_id" psql -X -q -v ON_ERROR_STOP=1 \
  -U "$(env_value "$source_root" BITRIVER_POSTGRES_USER)" \
  -d "$(env_value "$source_root" BITRIVER_POSTGRES_DB)" \
  < "$repo_root/scripts/fixtures/stateful-upgrade.sql"
source_invariants="$(database_invariants "$source_root")"
source_fingerprint="$(invariant_fingerprint "$source_invariants")"
source_users="$(invariant_users "$source_invariants")"
[[ "$source_fingerprint" =~ ^[0-9a-f]{64}$ ]] || fail "source fixture fingerprint is invalid"
[[ "$source_users" =~ ^[0-9]+$ ]] || fail "source user count is invalid"
source_ome_sha="$(sha256_path "$source_root/deploy/ome/Server.generated.xml")"
source_srs_sha="$(sha256_path "$source_root/deploy/srs/conf/srs.generated.conf")"

docker cp "$candidate_root/deploy/migrations" "$source_postgres_id:/candidate-migrations" >/dev/null
docker cp "$candidate_root/deploy/postgres-migrate.sh" "$source_postgres_id:/candidate-postgres-migrate.sh" >/dev/null
candidate_plan="$(
  docker_exec \
    -e PGPASSWORD="$(env_value "$source_root" BITRIVER_POSTGRES_PASSWORD)" \
    -e PGHOST=127.0.0.1 \
    -e PGUSER="$(env_value "$source_root" BITRIVER_POSTGRES_USER)" \
    -e PGDATABASE="$(env_value "$source_root" BITRIVER_POSTGRES_DB)" \
    -e BITRIVER_MIGRATIONS_DIR=/candidate-migrations \
    -e 'BITRIVER_MIGRATION_SANITY_SQL=SELECT 1 FROM users LIMIT 1;' \
    -e BITRIVER_MIGRATION_RELEASE="$candidate_release" \
    -e BITRIVER_MIGRATION_COMMIT="$candidate_commit" \
    "$source_postgres_id" /bin/sh /candidate-postgres-migrate.sh plan
)"
[[ "$candidate_plan" == *"APPLIED 0001_initial.sql"* && "$candidate_plan" != *"PENDING"* ]] ||
  fail "candidate migration preflight was not a clean no-op plan"

docker_exec -u 0 "$source_postgres_id" apk add --no-cache bash coreutils gzip >/dev/null
docker_exec "$source_postgres_id" mkdir -p /upgrade-backups
docker cp "$repo_root/scripts/backup-postgres.sh" "$source_postgres_id:/backup-postgres.sh" >/dev/null
backup_started="$(date +%s)"
docker_exec \
  -e BITRIVER_BACKUP_DIR=/upgrade-backups \
  -e BITRIVER_BACKUP_POSTGRES_HOST=127.0.0.1 \
  -e BITRIVER_BACKUP_POSTGRES_USER="$(env_value "$source_root" BITRIVER_POSTGRES_USER)" \
  -e BITRIVER_BACKUP_POSTGRES_PASSWORD="$(env_value "$source_root" BITRIVER_POSTGRES_PASSWORD)" \
  -e BITRIVER_BACKUP_POSTGRES_DB="$(env_value "$source_root" BITRIVER_POSTGRES_DB)" \
  -e BITRIVER_BACKUP_SOURCE_RELEASE="$source_release" \
  -e BITRIVER_BACKUP_SOURCE_COMMIT="$source_commit" \
  -e BITRIVER_BACKUP_RUN_PRUNE=0 \
  "$source_postgres_id" /bin/bash /backup-postgres.sh
backup_seconds=$(( $(date +%s) - backup_started ))
docker cp "$source_postgres_id:/upgrade-backups/." "$backup_dir" >/dev/null
backup_checksum="$(find "$backup_dir" -maxdepth 1 -name '*.sha256' -print -quit)"
[[ -n "$backup_checksum" ]] || fail "manifest-bound pre-upgrade backup was not copied"
(
  cd "$backup_dir"
  sha256sum -c "${backup_checksum##*/}"
)
source_seconds=$(( $(date +%s) - source_started ))

compose "$source_root" down --remove-orphans
active_root="$candidate_root"
upgrade_started="$(date +%s)"
start_dependencies "$candidate_root"
[[ -z "$(compose "$candidate_root" ps -a -q bitriver-live)" ]] ||
  fail "interrupted cut point unexpectedly contained the candidate API"
interrupted_health_status="$(http_status http://localhost:18080/healthz)"
[[ "$interrupted_health_status" == "000" ]] ||
  fail "interrupted pre-application cut point falsely exposed health: $interrupted_health_status"

start_applications "$candidate_root"
assert_first_party_images "$candidate_root"
candidate_postgres_id="$(compose "$candidate_root" ps -q postgres)"
candidate_postgres_image="$(docker inspect -f '{{.Config.Image}}' "$candidate_postgres_id")"
expected_candidate_postgres="postgres:15-alpine$(env_value "$candidate_root" BITRIVER_POSTGRES_IMAGE_DIGEST)"
[[ "$candidate_postgres_image" == "$expected_candidate_postgres" ]] ||
  fail "candidate Postgres image mismatch"
candidate_invariants="$(database_invariants "$candidate_root")"
[[ "$candidate_invariants" == "$source_invariants" ]] ||
  fail "candidate upgrade changed pre-existing state before the golden path"
candidate_ome_sha="$(sha256_path "$candidate_root/deploy/ome/Server.generated.xml")"
candidate_srs_sha="$(sha256_path "$candidate_root/deploy/srs/conf/srs.generated.conf")"
upgrade_seconds=$(( $(date +%s) - upgrade_started ))

metrics_file="$workdir/metrics-token"
printf '%s\n' "$(env_value "$candidate_root" BITRIVER_LIVE_METRICS_TOKEN)" > "$metrics_file"
chmod 0600 "$metrics_file"
golden_path_started="$(date +%s)"
"$repo_root/scripts/test-production-golden-path.sh" \
  --stack running \
  --client docker \
  --artifact-dir "$evidence_dir/golden-path" \
  --base-url http://localhost:18080 \
  --rtmp-base-url rtmp://localhost:1935/live \
  --viewer-path /viewer \
  --internal-api-host bitriver-live:8080 \
  --metrics-bearer-file "$metrics_file"
golden_path_seconds=$(( $(date +%s) - golden_path_started ))
golden_path_sha="$(sha256_path "$evidence_dir/golden-path/production-golden-path.json")"
post_golden_invariants="$(database_invariants "$candidate_root")"
post_golden_fingerprint="$(invariant_fingerprint "$post_golden_invariants")"
post_golden_users="$(invariant_users "$post_golden_invariants")"
[[ "$post_golden_fingerprint" == "$source_fingerprint" ]] ||
  fail "candidate golden path changed the source fixture fingerprint"
(( post_golden_users > source_users )) || fail "candidate golden path did not add persisted accounts"

compose "$candidate_root" down --remove-orphans
active_root="$source_root"
rollback_started="$(date +%s)"
start_dependencies "$source_root"
compose "$source_root" up -d --no-build --pull never --no-deps bitriver-live
wait_for_http http://localhost:18080/readyz 200
compose "$source_root" up -d --no-build --pull never --no-deps viewer transcoder-public
wait_for_http http://localhost:18080/viewer 200
rollback_health_status="$(http_status http://localhost:18080/healthz)"
[[ "$rollback_health_status" != "000" ]] ||
  fail "RC19 rollback aggregate health was unreachable"
rollback_health_healthy=false
if [[ "$rollback_health_status" == 2* || "$rollback_health_status" == 3* ]]; then
  rollback_health_healthy=true
fi
assert_first_party_images "$source_root"
[[ "$(sha256_path "$source_root/deploy/ome/Server.generated.xml")" == "$source_ome_sha" ]] ||
  fail "OME configuration rollback did not recover the exact source bytes"
[[ "$(sha256_path "$source_root/deploy/srs/conf/srs.generated.conf")" == "$source_srs_sha" ]] ||
  fail "SRS configuration rollback did not recover the exact source bytes"
rollback_invariants="$(database_invariants "$source_root")"
[[ "$rollback_invariants" == "$post_golden_invariants" ]] ||
  fail "image/config rollback changed persisted source or candidate state"
rollback_seconds=$(( $(date +%s) - rollback_started ))

mkdir -p "$evidence_dir"
cp "$workdir/input-metadata.json" "$evidence_dir/upgrade-input-metadata.json"
input_metadata_sha="$(sha256_path "$evidence_dir/upgrade-input-metadata.json")"
cat > "$evidence_dir/stateful-compose-upgrade-report.json" <<JSON
{
  "schemaVersion": "bitriver.stateful-compose-upgrade-report/v1",
  "result": "passed",
  "source": {
    "release": "$source_release",
    "commit": "$source_commit",
    "releaseSetSha256": "$source_release_set_sha",
    "aggregateHealthHttpStatus": $source_health_status,
    "aggregateHealthHealthy": $source_health_healthy,
    "ready": true
  },
  "candidate": {
    "release": "$candidate_release",
    "commit": "$candidate_commit",
    "releaseSetSha256": "$candidate_release_set_sha",
    "aggregateHealthHttpStatus": 200,
    "goldenPathReportSha256": "$golden_path_sha"
  },
  "interruptedCutPoint": {
    "name": "candidate-migration-and-config-before-application",
    "publicHealthHttpStatus": "$interrupted_health_status",
    "falselyHealthy": false
  },
  "state": {
    "sourceFixtureFingerprintSha256": "$source_fingerprint",
    "afterUpgradeMatched": true,
    "afterGoldenPathMatched": true,
    "afterRollbackMatched": true,
    "sourceUserCount": $source_users,
    "postGoldenPathUserCount": $post_golden_users,
    "manifestBoundBackupVerified": true
  },
  "configuration": {
    "sourceOmeSha256": "$source_ome_sha",
    "candidateOmeSha256": "$candidate_ome_sha",
    "sourceSrsSha256": "$source_srs_sha",
    "candidateSrsSha256": "$candidate_srs_sha",
    "rollbackMatchedSourceBytes": true
  },
  "images": {
    "validatedInputMetadataSha256": "$input_metadata_sha",
    "sourceFirstPartyMatched": true,
    "candidateFirstPartyMatched": true,
    "rollbackFirstPartyMatched": true,
    "candidatePostgres": "$candidate_postgres_image",
    "candidatePostgresMatched": true
  },
  "rollback": {
    "classification": "in-place-compatible-database-layer",
    "imageAndConfigRollback": "passed",
    "aggregateHealthHttpStatus": $rollback_health_status,
    "aggregateHealthHealthy": $rollback_health_healthy,
    "sourceReleaseApproved": false,
    "limitation": "RC19 is a rejected source release; this rehearsal records its observed health without qualifying it for production"
  },
  "timingSeconds": {
    "pull": $pull_seconds,
    "sourcePreparation": $source_seconds,
    "backup": $backup_seconds,
    "upgrade": $upgrade_seconds,
    "goldenPath": $golden_path_seconds,
    "rollback": $rollback_seconds
  },
  "remainingAcceptance": [
    "healthy rollback to an approved previous release",
    "clean-host package upgrade and reboot evidence"
  ]
}
JSON

cat > "$evidence_dir/stateful-compose-upgrade-summary.md" <<SUMMARY
# Stateful Compose upgrade rehearsal

- Pair: \`$source_release\` to \`$candidate_release\`
- Result: passed; RC19 source/rollback health observations were \`$source_health_status\`/\`$rollback_health_status\`
- Exact source/candidate/rollback first-party image references: matched
- Validated source/candidate image metadata SHA-256: \`$input_metadata_sha\`
- Candidate Postgres dependency image: \`$candidate_postgres_image\`
- Manifest-bound backup, pre-upgrade plan, fixed-state invariants: passed
- Interrupted candidate cut point exposed public health: no
- RC20 post-upgrade production golden path: passed
- Source image and generated OME/SRS config rollback: passed
- Persisted source plus candidate-created state after rollback: matched
- Remaining: healthy rollback to an approved prior release and clean-host package/reboot proof
SUMMARY

"$repo_root/scripts/scan-release-evidence.sh" \
  --root "$evidence_dir" \
  --sentinel-file "$workdir/environment-sentinels"

if [[ -n "$retained_artifact_dir" ]]; then
  [[ ! -e "$retained_artifact_dir" ]] || fail "refusing to overwrite retained artifact directory: $retained_artifact_dir"
  mkdir -p "$(dirname "$retained_artifact_dir")"
  cp -a "$evidence_dir" "$retained_artifact_dir"
  echo "Retained stateful Compose upgrade evidence: $retained_artifact_dir"
fi

echo "Exact-image RC19 to RC20 Compose upgrade/rollback rehearsal passed; RC19 source/rollback health was $source_health_status/$rollback_health_status."
