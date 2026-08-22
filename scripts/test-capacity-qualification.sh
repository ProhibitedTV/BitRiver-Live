#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

usage() {
  cat <<'USAGE'
Usage: ./scripts/test-capacity-qualification.sh [options]

Required identity:
  --release TAG                    Exact prerelease tag (for example v1.2.3-rc.20).
  --release-set-sha256 SHA256      Exact signed release-set SHA-256.
  --release-set-file FILE          Exact release-set.json bytes (required live).
  --source-commit SHA              Exact 40-hex candidate source commit.

Mode and workload:
  --dry-run                        Validate/hash the scenario only (default).
  --live                           Run load against a dedicated existing stack.
  --confirm-dedicated-environment Required with --live; fixtures persist in the stack.
  --scenario FILE                  Versioned scenario JSON.
  --artifact-dir DIR               Secret-scanned evidence directory.
  --client host|docker             Load-client runtime (default: host).

Live target:
  --base-url URL                   Public API/viewer origin.
  --rtmp-base-url URL              Public RTMP application URL.
  --media-host-override HOST       Rewrite returned loopback media hosts.
  --metrics-bearer-file FILE       Protected API metrics token file.
  --http-timeout SECONDS           Per-request timeout (default: 15).
  --stage-timeout SECONDS          Publisher-live deadline (default: 120).

Optional co-located development collection (host client only):
  --collector-mode remote|co-located
  --compose-project NAME           Compose project label (default: bitriver-live).
  --data-path DIR                  Direct host data filesystem to sample.

Live mode creates ordinary test accounts/channels and is only safe for a
dedicated disposable qualification environment. Dry-run is deliberately the
default. Co-located mode includes load-generator overhead and is not formal
target-host capacity evidence.
USAGE
}

mode="dry-run"
client="${BITRIVER_CAPACITY_CLIENT:-host}"
scenario="${BITRIVER_CAPACITY_SCENARIO:-$SCRIPT_DIR/capacity-scenarios/rc-qualification-small.json}"
artifact_dir="${BITRIVER_CAPACITY_ARTIFACT_DIR:-$REPO_ROOT/.artifacts/capacity-qualification}"
release="${BITRIVER_CAPACITY_RELEASE:-}"
release_set_sha256="${BITRIVER_CAPACITY_RELEASE_SET_SHA256:-}"
release_set_file="${BITRIVER_CAPACITY_RELEASE_SET_FILE:-}"
source_commit="${BITRIVER_CAPACITY_SOURCE_COMMIT:-}"
base_url="${BITRIVER_CAPACITY_BASE_URL:-http://localhost:18080}"
rtmp_base_url="${BITRIVER_CAPACITY_RTMP_BASE_URL:-rtmp://localhost:1935/live}"
media_host_override="${BITRIVER_CAPACITY_MEDIA_HOST_OVERRIDE:-}"
metrics_bearer_file="${BITRIVER_CAPACITY_METRICS_BEARER_FILE:-}"
http_timeout="${BITRIVER_CAPACITY_HTTP_TIMEOUT:-15}"
stage_timeout="${BITRIVER_CAPACITY_STAGE_TIMEOUT:-120}"
collector_mode="${BITRIVER_CAPACITY_COLLECTOR_MODE:-remote}"
compose_project="${BITRIVER_CAPACITY_COMPOSE_PROJECT:-bitriver-live}"
data_path="${BITRIVER_CAPACITY_DATA_PATH:-}"
confirmed=false

while (($# > 0)); do
  case "$1" in
    --dry-run) mode="dry-run" ;;
    --live) mode="live" ;;
    --confirm-dedicated-environment) confirmed=true ;;
    --client) shift; client="${1:-}" ;;
    --scenario) shift; scenario="${1:-}" ;;
    --artifact-dir) shift; artifact_dir="${1:-}" ;;
    --release) shift; release="${1:-}" ;;
    --release-set-sha256) shift; release_set_sha256="${1:-}" ;;
    --release-set-file) shift; release_set_file="${1:-}" ;;
    --source-commit) shift; source_commit="${1:-}" ;;
    --base-url) shift; base_url="${1:-}" ;;
    --rtmp-base-url) shift; rtmp_base_url="${1:-}" ;;
    --media-host-override) shift; media_host_override="${1:-}" ;;
    --metrics-bearer-file) shift; metrics_bearer_file="${1:-}" ;;
    --http-timeout) shift; http_timeout="${1:-}" ;;
    --stage-timeout) shift; stage_timeout="${1:-}" ;;
    --collector-mode) shift; collector_mode="${1:-}" ;;
    --compose-project) shift; compose_project="${1:-}" ;;
    --data-path) shift; data_path="${1:-}" ;;
    -h|--help) usage; exit 0 ;;
    *) echo "error: unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
  shift
done

case "$client" in host|docker) ;; *) echo "error: --client must be host or docker" >&2; exit 2 ;; esac
case "$collector_mode" in remote|co-located) ;; *) echo "error: --collector-mode must be remote or co-located" >&2; exit 2 ;; esac
[[ -f $scenario ]] || { echo "error: scenario file not found: $scenario" >&2; exit 2; }
[[ -n $release && -n $release_set_sha256 && -n $source_commit ]] || {
  echo "error: exact release, release-set SHA-256, and source commit are required" >&2
  exit 2
}
if [[ $mode == live ]]; then
  [[ $confirmed == true ]] || {
    echo "error: --live requires --confirm-dedicated-environment" >&2
    exit 2
  }
  [[ -n $metrics_bearer_file && -f $metrics_bearer_file ]] || {
    echo "error: --live requires an existing --metrics-bearer-file" >&2
    exit 2
  }
  [[ -n $release_set_file && -f $release_set_file ]] || {
    echo "error: --live requires an existing --release-set-file" >&2
    exit 2
  }
fi
if [[ $collector_mode == co-located ]]; then
  [[ $client == host ]] || {
    echo "error: co-located collection requires --client host" >&2
    exit 2
  }
  [[ -n $data_path && -d $data_path ]] || {
    echo "error: co-located collection requires an existing --data-path" >&2
    exit 2
  }
fi

mkdir -p "$artifact_dir"
artifact_dir="$(cd "$artifact_dir" && pwd)"
scenario="$(cd "$(dirname "$scenario")" && pwd)/$(basename "$scenario")"
if [[ $mode == live ]]; then
  metrics_bearer_file="$(cd "$(dirname "$metrics_bearer_file")" && pwd)/$(basename "$metrics_bearer_file")"
  release_set_file="$(cd "$(dirname "$release_set_file")" && pwd)/$(basename "$release_set_file")"
fi
sentinel_file="$(mktemp)"
trap 'rm -f "$sentinel_file"' EXIT

common_args=(
  --scenario "$scenario"
  --artifact-dir "$artifact_dir"
  --release "$release"
  --release-set-sha256 "$release_set_sha256"
  --source-commit "$source_commit"
  --base-url "$base_url"
  --rtmp-base-url "$rtmp_base_url"
  --http-timeout "$http_timeout"
  --stage-timeout "$stage_timeout"
  --collector-mode "$collector_mode"
  --compose-project "$compose_project"
)
[[ -n $media_host_override ]] && common_args+=(--media-host-override "$media_host_override")
if [[ $mode == dry-run ]]; then
  common_args+=(--dry-run)
else
  common_args+=(--confirm-dedicated-environment --sentinel-file "$sentinel_file")
  common_args+=(--metrics-bearer-file "$metrics_bearer_file")
  common_args+=(--release-set-file "$release_set_file")
fi
[[ -n $data_path ]] && common_args+=(--data-path "$data_path")

set +e
if [[ $client == host ]]; then
  "$SCRIPT_DIR/python.sh" "$SCRIPT_DIR/capacity_qualification.py" "${common_args[@]}"
  harness_status=$?
else
  command -v docker >/dev/null 2>&1 || {
    echo "error: Docker client mode requires Docker" >&2
    exit 2
  }
  docker build \
    --file "$SCRIPT_DIR/golden-path-client.Dockerfile" \
    --tag bitriver-golden-path-client:local \
    "$REPO_ROOT"
  build_status=$?
  if ((build_status != 0)); then
    echo "error: failed to build the capacity client image" >&2
    exit "$build_status"
  fi

  docker_artifact_dir="$artifact_dir"
  docker_scenario="$scenario"
  docker_sentinel_file="$sentinel_file"
  docker_metrics_bearer_file="$metrics_bearer_file"
  docker_release_set_file="$release_set_file"
  if [[ ${OSTYPE:-} == msys* || ${OSTYPE:-} == cygwin* ]]; then
    docker_artifact_dir="$(cygpath -w "$artifact_dir")"
    docker_scenario="$(cygpath -w "$scenario")"
    docker_sentinel_file="$(cygpath -w "$sentinel_file")"
    [[ -n $metrics_bearer_file ]] && docker_metrics_bearer_file="$(cygpath -w "$metrics_bearer_file")"
    [[ -n $release_set_file ]] && docker_release_set_file="$(cygpath -w "$release_set_file")"
  fi
  docker_base_url="${base_url//localhost/host.docker.internal}"
  docker_base_url="${docker_base_url//127.0.0.1/host.docker.internal}"
  docker_rtmp_base_url="${rtmp_base_url//localhost/host.docker.internal}"
  docker_rtmp_base_url="${docker_rtmp_base_url//127.0.0.1/host.docker.internal}"
  docker_args=(
    run --rm
    --add-host host.docker.internal:host-gateway
    --volume "$docker_artifact_dir:/evidence"
    --volume "$docker_scenario:/run/capacity-scenario.json:ro"
    --entrypoint python3
  )
  capacity_args=(
    /harness/capacity_qualification.py
    --scenario /run/capacity-scenario.json
    --artifact-dir /evidence
    --release "$release"
    --release-set-sha256 "$release_set_sha256"
    --source-commit "$source_commit"
    --base-url "$docker_base_url"
    --rtmp-base-url "$docker_rtmp_base_url"
    --http-timeout "$http_timeout"
    --stage-timeout "$stage_timeout"
    --collector-mode remote
  )
  capacity_args+=(--media-host-override host.docker.internal)
  if [[ $mode == dry-run ]]; then
    capacity_args+=(--dry-run)
  else
    docker_args+=(--volume "$docker_sentinel_file:/run/capacity-sentinels")
    docker_args+=(--volume "$docker_metrics_bearer_file:/run/metrics-bearer:ro")
    docker_args+=(--volume "$docker_release_set_file:/run/release-set.json:ro")
    capacity_args+=(
      --confirm-dedicated-environment
      --sentinel-file /run/capacity-sentinels
      --metrics-bearer-file /run/metrics-bearer
      --release-set-file /run/release-set.json
    )
  fi
  MSYS_NO_PATHCONV=1 docker "${docker_args[@]}" \
    bitriver-golden-path-client:local "${capacity_args[@]}"
  harness_status=$?
fi
set -e

set +e
"$SCRIPT_DIR/scan-release-evidence.sh" \
  --root "$artifact_dir" \
  --sentinel-file "$sentinel_file"
scan_status=$?
set -e
if ((scan_status != 0)); then
  echo "error: retained capacity evidence failed secret scanning" >&2
  exit "$scan_status"
fi
if ((harness_status != 0)); then
  echo "error: capacity qualification failed" >&2
  exit "$harness_status"
fi

echo "Capacity qualification evidence scan passed: $artifact_dir"
