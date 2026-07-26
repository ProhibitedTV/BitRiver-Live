#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/render-alertmanager-config.sh [--env-file PATH] [--output PATH]

Renders deploy/monitoring/alertmanager.yml.tmpl to an Alertmanager config file by
substituting monitoring receiver environment variables.
USAGE
}

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEMPLATE_FILE="$ROOT_DIR/deploy/monitoring/alertmanager.yml.tmpl"
ENV_FILE="$ROOT_DIR/deploy/monitoring/alertmanager.env"
OUTPUT_FILE="$ROOT_DIR/deploy/monitoring/alertmanager.yml"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env-file)
      [[ $# -ge 2 ]] || { echo "error: --env-file requires a path" >&2; exit 1; }
      ENV_FILE="$2"
      shift 2
      ;;
    --output)
      [[ $# -ge 2 ]] || { echo "error: --output requires a path" >&2; exit 1; }
      OUTPUT_FILE="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "error: unknown argument '$1'" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -f "$ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
fi

: "${BITRIVER_ALERTMANAGER_DEFAULT_WEBHOOK_URL:=http://example.invalid/default}"
: "${BITRIVER_ALERTMANAGER_DEFAULT_WEBHOOK_TOKEN:=replace-default-token}"
: "${BITRIVER_ALERTMANAGER_CRITICAL_WEBHOOK_URL:=http://example.invalid/critical}"
: "${BITRIVER_ALERTMANAGER_CRITICAL_WEBHOOK_TOKEN:=replace-critical-token}"
: "${BITRIVER_ALERTMANAGER_AUTH_WEBHOOK_URL:=http://example.invalid/auth}"
: "${BITRIVER_ALERTMANAGER_AUTH_WEBHOOK_TOKEN:=replace-auth-token}"
export \
  BITRIVER_ALERTMANAGER_DEFAULT_WEBHOOK_URL \
  BITRIVER_ALERTMANAGER_DEFAULT_WEBHOOK_TOKEN \
  BITRIVER_ALERTMANAGER_CRITICAL_WEBHOOK_URL \
  BITRIVER_ALERTMANAGER_CRITICAL_WEBHOOK_TOKEN \
  BITRIVER_ALERTMANAGER_AUTH_WEBHOOK_URL \
  BITRIVER_ALERTMANAGER_AUTH_WEBHOOK_TOKEN

if command -v envsubst >/dev/null 2>&1; then
  envsubst <"$TEMPLATE_FILE" >"$OUTPUT_FILE"
else
  python3 - "$TEMPLATE_FILE" "$OUTPUT_FILE" <<'PY'
import os
import re
import sys
from pathlib import Path

template = Path(sys.argv[1]).read_text()
pattern = re.compile(r"\$\{([A-Z0-9_]+)\}")


def repl(match: re.Match[str]) -> str:
    return os.environ.get(match.group(1), "")

Path(sys.argv[2]).write_text(pattern.sub(repl, template))
PY
fi

echo "Rendered Alertmanager config: $OUTPUT_FILE"
