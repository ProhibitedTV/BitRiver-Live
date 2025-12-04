#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="${ENV_FILE:-$REPO_ROOT/.env}"
TEMPLATE="$REPO_ROOT/deploy/ome/Server.xml"
OUTPUT="$REPO_ROOT/deploy/ome/Server.generated.xml"
MODE="render"
QUIET=0
FORCE=0

usage() {
  cat <<'USAGE'
Usage: scripts/render-ome-config.sh [--check] [--force] [--env-file PATH] [--quiet]

Options:
  --check       Only verify that deploy/ome/Server.generated.xml exists.
  --force       Re-render even if the generated file already exists.
  --env-file    Path to the .env file to source (defaults to ./../.env).
  --quiet       Suppress informational output.
USAGE
}

while (($# > 0)); do
  case "$1" in
    --check)
      MODE="check"
      ;;
    --force)
      FORCE=1
      ;;
    --env-file)
      shift
      ENV_FILE="${1:-}"
      if [[ -z "$ENV_FILE" ]]; then
        echo "--env-file requires a path" >&2
        exit 1
      fi
      ;;
    --quiet)
      QUIET=1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage
      exit 1
      ;;
  esac
  shift
done

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Environment file not found at $ENV_FILE." >&2
  echo "Copy deploy/.env.example to .env and populate BITRIVER_OME_* variables before rendering." >&2
  exit 1
fi

if [[ ! -f "$TEMPLATE" ]]; then
  echo "OME template missing at $TEMPLATE" >&2
  exit 1
fi

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

OME_BIND="${BITRIVER_OME_BIND:-0.0.0.0}"
OME_PORT="${BITRIVER_OME_SERVER_PORT:-9000}"
OME_TLS_PORT="${BITRIVER_OME_SERVER_TLS_PORT:-9443}"
OME_IP="${BITRIVER_OME_IP:-$OME_BIND}"
OME_USERNAME="${BITRIVER_OME_USERNAME:-}"
OME_PASSWORD="${BITRIVER_OME_PASSWORD:-}"
OME_API_TOKEN="${BITRIVER_OME_API_TOKEN:-${BITRIVER_OME_ACCESS_TOKEN:-}}"
OME_IMAGE_TAG="${BITRIVER_OME_IMAGE_TAG:-}"

if [[ -z "$OME_IMAGE_TAG" ]]; then
  echo "BITRIVER_OME_IMAGE_TAG must be set in $ENV_FILE before rendering." >&2
  exit 1
fi

supports_access_token=1
if [[ "$OME_IMAGE_TAG" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
  major="${BASH_REMATCH[1]}"
  minor="${BASH_REMATCH[2]}"
  if (( major == 0 && minor < 16 )); then
    supports_access_token=0
  fi
fi

if [[ -z "$OME_USERNAME" || -z "$OME_PASSWORD" ]]; then
  echo "BITRIVER_OME_USERNAME and BITRIVER_OME_PASSWORD must be set in $ENV_FILE before rendering." >&2
  exit 1
fi

render_api_token="$OME_API_TOKEN"
if [[ $supports_access_token -eq 1 && -z "$OME_API_TOKEN" ]]; then
  cat <<EOF >&2
BITRIVER_OME_API_TOKEN must be set in $ENV_FILE before rendering when BITRIVER_OME_IMAGE_TAG=$OME_IMAGE_TAG (managers <AccessToken> is supported starting with 0.16.0). If you are migrating from BITRIVER_OME_ACCESS_TOKEN, copy that value into BITRIVER_OME_API_TOKEN.
EOF
  exit 1
fi

if [[ $supports_access_token -eq 0 ]]; then
  if [[ -n "$OME_API_TOKEN" && $QUIET -eq 0 ]]; then
    echo "BITRIVER_OME_IMAGE_TAG=$OME_IMAGE_TAG does not advertise managers <AccessToken>; dropping BITRIVER_OME_API_TOKEN from the rendered config." >&2
  fi
  render_api_token=""
fi

OME_MARKER_PREFIX="<!-- Rendered for BITRIVER_OME_IMAGE_TAG="

if [[ "$MODE" == "check" ]]; then
  if [[ ! -f "$OUTPUT" ]]; then
    echo "OME config missing at $OUTPUT. Run ./scripts/render-ome-config.sh to generate it." >&2
    exit 1
  fi
  if [[ $QUIET -eq 0 ]]; then
    echo "OME config found at $OUTPUT."
  fi
  exit 0
fi

if [[ $QUIET -eq 0 ]]; then
  if [[ $FORCE -eq 1 ]]; then
    echo "Rendering OME config (--force requested)..."
  else
    echo "Rendering OME config..."
  fi
fi

if ! render_output=$(python3 "$SCRIPT_DIR/render_ome_config.py" \
  --template "$TEMPLATE" \
  --output "$OUTPUT" \
  --bind "$OME_BIND" \
  --server-ip "$OME_IP" \
  --port "$OME_PORT" \
  --tls-port "$OME_TLS_PORT" \
  --username "$OME_USERNAME" \
  --password "$OME_PASSWORD" \
  --api-token "$render_api_token" 2>&1); then
  echo "Failed to render deploy/ome/Server.generated.xml. Check BITRIVER_OME_* values in $ENV_FILE and the template at $TEMPLATE." >&2
  echo "$render_output" >&2
  exit 1
fi

marker="${OME_MARKER_PREFIX}${OME_IMAGE_TAG} -->"
if grep -q "$OME_MARKER_PREFIX" "$OUTPUT"; then
  perl -0pi -e "s/${OME_MARKER_PREFIX}.* -->/${marker}/" "$OUTPUT"
else
  perl -0pi -e "s#(<Server[^>]*>\\s*)#\\1    ${marker}\\n#" "$OUTPUT"
fi

if [[ $QUIET -eq 0 ]]; then
  echo "Rendered OME configuration to $OUTPUT"
fi
