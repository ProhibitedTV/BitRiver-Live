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
FORCE_LEGACY_OUTPUTS="${BITRIVER_OME_FORCE_LEGACY_OUTPUTS:-0}"

usage() {
  cat <<'USAGE'
Usage: scripts/render-ome-config.sh [--check] [--force] [--env-file PATH] [--quiet]

Options:
  --check       Only verify that deploy/ome/Server.generated.xml exists.
  --force       Re-render even if the generated file already exists.
  --env-file    Path to the .env file to source (defaults to ./../.env).
  --quiet       Suppress informational output.
  --force-legacy-outputs
                Render without <Outputs>/<OutputStreams> regardless of BITRIVER_OME_IMAGE_TAG.
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
    --force-legacy-outputs)
      FORCE_LEGACY_OUTPUTS=1
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
OME_IMAGE_TAG="${BITRIVER_OME_IMAGE_TAG:-0.15.0}"
OME_ICE_PORT_RANGE="${BITRIVER_OME_ICE_PORT_RANGE:-10000-10009}"
OME_TCP_RELAY="${BITRIVER_OME_TCP_RELAY:-${BITRIVER_OME_RELAY_PORT:-3478}}"
if [[ "$OME_TCP_RELAY" != *:* ]]; then
  OME_TCP_RELAY="*:$(echo "$OME_TCP_RELAY" | tr -d '*:')"
fi
OME_ICE_CANDIDATE="${BITRIVER_OME_ICE_CANDIDATE:-}"
if [[ -z "$OME_ICE_CANDIDATE" ]]; then
  OME_ICE_CANDIDATE="*:${OME_ICE_PORT_RANGE}/udp"
fi

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

render_args=(
  python3 "$SCRIPT_DIR/render_ome_config.py"
  --template "$TEMPLATE"
  --output "$OUTPUT"
  --bind "$OME_BIND"
  --server-ip "$OME_IP"
  --port "$OME_PORT"
  --tls-port "$OME_TLS_PORT"
  --image-tag "$OME_IMAGE_TAG"
  --tcp-relay "$OME_TCP_RELAY"
  --ice-candidate "$OME_ICE_CANDIDATE"
)

if [[ "$FORCE_LEGACY_OUTPUTS" == "1" ]]; then
  render_args+=(--force-legacy-outputs)
fi

if ! render_output=$("${render_args[@]}" 2>&1); then
  echo "Failed to render deploy/ome/Server.generated.xml. Check BITRIVER_OME_* values in $ENV_FILE and the template at $TEMPLATE." >&2
  echo "$render_output" >&2
  exit 1
fi

if [[ $QUIET -eq 0 ]]; then
  echo "Rendered OME configuration to $OUTPUT"
fi
