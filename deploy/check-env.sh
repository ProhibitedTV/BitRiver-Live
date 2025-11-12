#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="${1:-$REPO_ROOT/.env}"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Environment file not found at $ENV_FILE." >&2
  echo "Copy deploy/.env.example to .env and populate it before continuing." >&2
  exit 1
fi

# shellcheck disable=SC1090
source "$ENV_FILE"

missing=()
blocked=()

required_vars=(
  BITRIVER_POSTGRES_USER
  BITRIVER_POSTGRES_PASSWORD
  BITRIVER_LIVE_ADMIN_EMAIL
  BITRIVER_LIVE_ADMIN_PASSWORD
  BITRIVER_SRS_TOKEN
  BITRIVER_OME_USERNAME
  BITRIVER_OME_PASSWORD
  BITRIVER_TRANSCODER_TOKEN
)

declare -A forbidden_values=(
  [BITRIVER_POSTGRES_USER]="bitriver"
  [BITRIVER_POSTGRES_PASSWORD]="bitriver"
  [BITRIVER_LIVE_ADMIN_EMAIL]="admin@example.com"
  [BITRIVER_LIVE_ADMIN_PASSWORD]="change-me-now"
  [BITRIVER_SRS_TOKEN]="local-dev-token"
  [BITRIVER_OME_USERNAME]="admin"
  [BITRIVER_OME_PASSWORD]="local-dev-password"
  [BITRIVER_TRANSCODER_TOKEN]="local-dev-token"
)

for var in "${required_vars[@]}"; do
  value="${!var-}"
  if [[ -z "$value" ]]; then
    missing+=("$var")
    continue
  fi
  if [[ -n "${forbidden_values[$var]:-}" && "$value" == "${forbidden_values[$var]}" ]]; then
    blocked+=("$var")
  fi
done

if (( ${#missing[@]} > 0 )); then
  echo "The following required variables are unset or empty in $ENV_FILE:" >&2
  for var in "${missing[@]}"; do
    echo "  - $var" >&2
  done
fi

if (( ${#blocked[@]} > 0 )); then
  echo "Replace the placeholder values for these credentials before continuing:" >&2
  for var in "${blocked[@]}"; do
    echo "  - $var" >&2
  done
fi

if [[ -n "${BITRIVER_LIVE_POSTGRES_DSN:-}" && "$BITRIVER_LIVE_POSTGRES_DSN" == *"bitriver:bitriver"* ]]; then
  echo "Warning: BITRIVER_LIVE_POSTGRES_DSN still references bitriver:bitriver. Update or unset it to match the Postgres credentials." >&2
fi

if (( ${#missing[@]} > 0 || ${#blocked[@]} > 0 )); then
  exit 1
fi

echo "Environment file $ENV_FILE looks ready."
