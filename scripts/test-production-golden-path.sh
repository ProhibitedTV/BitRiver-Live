#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

usage() {
  cat <<'USAGE'
Usage: ./scripts/test-production-golden-path.sh [options]

Options:
  --stack running|quickstart   Validate an existing stack or own quickstart lifecycle.
  --client host|docker         Run dependencies on the host or in a Docker client image.
  --artifact-dir DIR           Sanitized evidence destination.
  --base-url URL               Public API/viewer origin for running mode.
  --rtmp-base-url URL          Public RTMP application URL for running mode.
  --viewer-path PATH           Viewer base path.
  --internal-api-host HOST     Container-reachable API Host for VOD source URLs.
  --media-host-override HOST   Rewrite returned loopback media hosts for a client container.
  --metrics-bearer-file FILE   Read protected metrics authorization from a file.
  --ffmpeg PATH                ffmpeg executable or command name.
  --ffprobe PATH               ffprobe executable or command name.
  --stage-timeout SECONDS      Per-stage live/readiness deadline.
  --media-timeout SECONDS      Media decode/probe deadline.
  --vod-timeout SECONDS        VOD processing deadline.
  -h, --help                   Show this help.

The retained report is scanned against per-run account/password/stream-key
sentinels. The sentinel file is temporary and is never stored with evidence.
USAGE
}

stack_mode="running"
client_mode="${BITRIVER_GOLDEN_PATH_CLIENT:-host}"
artifact_dir="${BITRIVER_GOLDEN_PATH_ARTIFACT_DIR:-$REPO_ROOT/.artifacts/production-golden-path}"
base_url="${BITRIVER_GOLDEN_PATH_BASE_URL:-http://localhost:18080}"
rtmp_base_url="${BITRIVER_GOLDEN_PATH_RTMP_BASE_URL:-rtmp://localhost:1935/live}"
viewer_path="${BITRIVER_GOLDEN_PATH_VIEWER_PATH:-/viewer}"
internal_api_host="${BITRIVER_GOLDEN_PATH_INTERNAL_API_HOST:-bitriver-live:8080}"
media_host_override="${BITRIVER_GOLDEN_PATH_MEDIA_HOST_OVERRIDE:-}"
metrics_bearer_file="${BITRIVER_GOLDEN_PATH_METRICS_BEARER_FILE:-}"
ffmpeg="${BITRIVER_GOLDEN_PATH_FFMPEG:-ffmpeg}"
ffprobe="${BITRIVER_GOLDEN_PATH_FFPROBE:-ffprobe}"
stage_timeout="${BITRIVER_GOLDEN_PATH_STAGE_TIMEOUT:-120}"
media_timeout="${BITRIVER_GOLDEN_PATH_MEDIA_TIMEOUT:-45}"
vod_timeout="${BITRIVER_GOLDEN_PATH_VOD_TIMEOUT:-240}"

while (($# > 0)); do
  case "$1" in
    --stack)
      shift
      stack_mode="${1:-}"
      ;;
    --client)
      shift
      client_mode="${1:-}"
      ;;
    --artifact-dir)
      shift
      artifact_dir="${1:-}"
      ;;
    --base-url)
      shift
      base_url="${1:-}"
      ;;
    --rtmp-base-url)
      shift
      rtmp_base_url="${1:-}"
      ;;
    --viewer-path)
      shift
      viewer_path="${1:-}"
      ;;
    --internal-api-host)
      shift
      internal_api_host="${1:-}"
      ;;
    --media-host-override)
      shift
      media_host_override="${1:-}"
      ;;
    --metrics-bearer-file)
      shift
      metrics_bearer_file="${1:-}"
      ;;
    --ffmpeg)
      shift
      ffmpeg="${1:-}"
      ;;
    --ffprobe)
      shift
      ffprobe="${1:-}"
      ;;
    --stage-timeout)
      shift
      stage_timeout="${1:-}"
      ;;
    --media-timeout)
      shift
      media_timeout="${1:-}"
      ;;
    --vod-timeout)
      shift
      vod_timeout="${1:-}"
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "error: unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

case "$client_mode" in
  host|docker)
    ;;
  *)
    echo "error: --client must be host or docker" >&2
    exit 2
    ;;
esac

case "$stack_mode" in
  quickstart)
    export BITRIVER_TEST_GOLDEN_PATH=1
    export BITRIVER_GOLDEN_PATH_ARTIFACT_DIR="$artifact_dir"
    export BITRIVER_GOLDEN_PATH_CLIENT="$client_mode"
    export BITRIVER_GOLDEN_PATH_INTERNAL_API_HOST="$internal_api_host"
    export BITRIVER_GOLDEN_PATH_MEDIA_HOST_OVERRIDE="$media_host_override"
    export BITRIVER_GOLDEN_PATH_METRICS_BEARER_FILE="$metrics_bearer_file"
    export BITRIVER_GOLDEN_PATH_FFMPEG="$ffmpeg"
    export BITRIVER_GOLDEN_PATH_FFPROBE="$ffprobe"
    export BITRIVER_GOLDEN_PATH_STAGE_TIMEOUT="$stage_timeout"
    export BITRIVER_GOLDEN_PATH_MEDIA_TIMEOUT="$media_timeout"
    export BITRIVER_GOLDEN_PATH_VOD_TIMEOUT="$vod_timeout"
    exec "$SCRIPT_DIR/test-quickstart.sh"
    ;;
  running)
    ;;
  *)
    echo "error: --stack must be running or quickstart" >&2
    exit 2
    ;;
esac

if [[ -z "$artifact_dir" || -z "$base_url" || -z "$rtmp_base_url" ]]; then
  echo "error: artifact, HTTP, and RTMP locations must not be empty" >&2
  exit 2
fi

mkdir -p "$artifact_dir"
sentinel_file="$(mktemp)"
trap 'rm -f "$sentinel_file"' EXIT

product_args=(
  --artifact-dir "$artifact_dir"
  --sentinel-file "$sentinel_file"
  --base-url "$base_url"
  --rtmp-base-url "$rtmp_base_url"
  --viewer-path "$viewer_path"
  --internal-api-host "$internal_api_host"
  --ffmpeg "$ffmpeg"
  --ffprobe "$ffprobe"
  --stage-timeout "$stage_timeout"
  --media-timeout "$media_timeout"
  --vod-timeout "$vod_timeout"
)

if [[ -n "$media_host_override" ]]; then
  product_args+=(--media-host-override "$media_host_override")
fi
if [[ -n "$metrics_bearer_file" ]]; then
  product_args+=(--metrics-bearer-file "$metrics_bearer_file")
fi

set +e
if [[ "$client_mode" == "docker" ]]; then
  if ! command -v docker >/dev/null 2>&1; then
    echo "error: docker client mode requires Docker" >&2
    exit 2
  fi
  docker build \
    --file "$SCRIPT_DIR/golden-path-client.Dockerfile" \
    --tag bitriver-golden-path-client:local \
    "$REPO_ROOT"
  build_status=$?
  if ((build_status != 0)); then
    echo "error: failed to build the production golden-path client image" >&2
    exit "$build_status"
  fi

  artifact_dir="$(cd "$artifact_dir" && pwd)"
  docker_artifact_dir="$artifact_dir"
  docker_sentinel_file="$sentinel_file"
  docker_metrics_bearer_file="$metrics_bearer_file"
  if [[ "${OSTYPE:-}" == msys* || "${OSTYPE:-}" == cygwin* ]]; then
    docker_artifact_dir="$(cygpath -w "$artifact_dir")"
    docker_sentinel_file="$(cygpath -w "$sentinel_file")"
    if [[ -n "$metrics_bearer_file" ]]; then
      docker_metrics_bearer_file="$(cygpath -w "$metrics_bearer_file")"
    fi
  fi
  docker_base_url="${base_url//localhost/host.docker.internal}"
  docker_base_url="${docker_base_url//127.0.0.1/host.docker.internal}"
  docker_rtmp_base_url="${rtmp_base_url//localhost/host.docker.internal}"
  docker_rtmp_base_url="${docker_rtmp_base_url//127.0.0.1/host.docker.internal}"

  docker_run_args=(
    run --rm
    --add-host host.docker.internal:host-gateway
    --volume "$docker_artifact_dir:/evidence"
    --volume "$docker_sentinel_file:/run/golden-sentinels"
  )
  docker_product_args=(
    --artifact-dir /evidence
    --sentinel-file /run/golden-sentinels
    --base-url "$docker_base_url"
    --rtmp-base-url "$docker_rtmp_base_url"
    --viewer-path "$viewer_path"
    --internal-api-host "$internal_api_host"
    --media-host-override host.docker.internal
    --stage-timeout "$stage_timeout"
    --media-timeout "$media_timeout"
    --vod-timeout "$vod_timeout"
  )
  if [[ -n "$metrics_bearer_file" ]]; then
    docker_run_args+=(
      --volume "$docker_metrics_bearer_file:/run/metrics-bearer:ro"
    )
    docker_product_args+=(
      --metrics-bearer-file /run/metrics-bearer
    )
  fi
  MSYS_NO_PATHCONV=1 docker "${docker_run_args[@]}" \
    bitriver-golden-path-client:local \
    "${docker_product_args[@]}"
else
  "$SCRIPT_DIR/python.sh" \
    "$SCRIPT_DIR/production_golden_path.py" \
    "${product_args[@]}"
fi
harness_status=$?
set -e

set +e
"$SCRIPT_DIR/scan-release-evidence.sh" \
  --root "$artifact_dir" \
  --sentinel-file "$sentinel_file"
scan_status=$?
set -e

if ((scan_status != 0)); then
  echo "error: retained production golden-path evidence failed secret scanning" >&2
  exit "$scan_status"
fi
if ((harness_status != 0)); then
  echo "error: production golden-path product assertions failed" >&2
  exit "$harness_status"
fi

echo "Production golden-path evidence scan passed: $artifact_dir"
