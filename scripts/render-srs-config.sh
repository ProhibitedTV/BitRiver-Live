#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEFAULT_REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="${REPO_ROOT:-$DEFAULT_REPO_ROOT}"
ENV_FILE="${ENV_FILE:-$REPO_ROOT/.env}"
TEMPLATE_FILE="$REPO_ROOT/deploy/srs/conf/srs.conf"
OUTPUT_FILE="$REPO_ROOT/deploy/srs/conf/srs.generated.conf"

usage() {
  cat <<'USAGE'
Usage: scripts/render-srs-config.sh [--check] [--force] [--env-file PATH] [--quiet]

Options:
  --check       Only verify that deploy/srs/conf/srs.generated.conf exists.
  --force       Re-render even if the generated file already exists.
  --env-file    Path to the .env file to source (defaults to ./../.env).
  --quiet       Suppress informational output.
USAGE
}

check_only=0
force=0
quiet=0

while (($# > 0)); do
  case "$1" in
    --check)
      check_only=1
      ;;
    --force)
      force=1
      ;;
    --quiet)
      quiet=1
      ;;
    --env-file)
      shift
      ENV_FILE="${1:-}"
      if [[ -z "$ENV_FILE" ]]; then
        echo "--env-file requires a path" >&2
        exit 1
      fi
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

if [[ $check_only -eq 1 ]]; then
  if [[ -f "$OUTPUT_FILE" ]]; then
    exit 0
  fi
  echo "Missing rendered SRS config at $OUTPUT_FILE" >&2
  exit 1
fi

if [[ $force -eq 0 && -f "$OUTPUT_FILE" ]]; then
  if [[ $quiet -eq 0 ]]; then
    echo "SRS config already rendered at $OUTPUT_FILE"
  fi
  exit 0
fi

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Env file not found at $ENV_FILE" >&2
  exit 1
fi

if [[ ! -f "$TEMPLATE_FILE" ]]; then
  echo "SRS template not found at $TEMPLATE_FILE" >&2
  exit 1
fi

if [[ -d "$OUTPUT_FILE" ]]; then
  rm -rf "$OUTPUT_FILE"
fi

set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

if [[ -z "${BITRIVER_SRS_TOKEN:-}" ]]; then
  echo "BITRIVER_SRS_TOKEN must be set in $ENV_FILE" >&2
  exit 1
fi

rendered="$(<"$TEMPLATE_FILE")"
rendered="${rendered//\$\{BITRIVER_SRS_TOKEN\}/${BITRIVER_SRS_TOKEN}}"

if [[ "$rendered" == *\$\{BITRIVER_SRS_TOKEN\}* ]]; then
  echo "Failed to render BITRIVER_SRS_TOKEN in $TEMPLATE_FILE" >&2
  exit 1
fi

mkdir -p "$(dirname "$OUTPUT_FILE")"
printf '%s\n' "$rendered" > "$OUTPUT_FILE"

if [[ $quiet -eq 0 ]]; then
  echo "Rendered SRS config to $OUTPUT_FILE"
fi
