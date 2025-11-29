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
blocked_messages=()
unset_image_tags=()
errors=()

required_vars=(
  BITRIVER_POSTGRES_USER
  BITRIVER_POSTGRES_PASSWORD
  BITRIVER_REDIS_PASSWORD
  BITRIVER_OME_API
  BITRIVER_LIVE_ADMIN_EMAIL
  BITRIVER_LIVE_ADMIN_PASSWORD
  BITRIVER_SRS_TOKEN
  BITRIVER_OME_USERNAME
  BITRIVER_OME_PASSWORD
  BITRIVER_TRANSCODER_TOKEN
  BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD
  BITRIVER_TRANSCODER_PUBLIC_BASE_URL
)

declare -A forbidden_values=(
  [BITRIVER_POSTGRES_PASSWORD]="P0stgres-Example!"
  [BITRIVER_REDIS_PASSWORD]="R3dis-Example!"
  [BITRIVER_LIVE_ADMIN_EMAIL]="admin@stream.example.com"
  [BITRIVER_LIVE_ADMIN_PASSWORD]="Sup3rSecureAdmin!"
  [BITRIVER_SRS_TOKEN]="srs-secure-token-example"
  [BITRIVER_OME_PASSWORD]="OME-Example-Pass!"
  [BITRIVER_TRANSCODER_TOKEN]="transcoder-secure-token-example"
  [BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD]="R3dis-Example!"
)

if [[ -n "${BITRIVER_REDIS_PASSWORD:-}" && -n "${BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD:-}" && \
      "$BITRIVER_REDIS_PASSWORD" != "$BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD" ]]; then
  echo "Warning: BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD does not match BITRIVER_REDIS_PASSWORD." >&2
  echo "Ensure the API and Redis share the same credential unless you intentionally override it." >&2
fi

if [[ -n "${COMPOSE_PROFILES:-}" ]]; then
  IFS=",:" read -ra __profiles <<< "$COMPOSE_PROFILES"
  for __profile in "${__profiles[@]}"; do
    if [[ "$__profile" == "postgres-host" ]]; then
      echo "Warning: COMPOSE_PROFILES includes postgres-host, which publishes PostgreSQL to the host." >&2
      echo "Confirm the host firewall and trust boundaries before enabling this profile." >&2
      break
    fi
  done
fi

for var in "${required_vars[@]}"; do
  value="${!var-}"
  if [[ -z "$value" ]]; then
    missing+=("$var")
    continue
  fi
  if [[ -n "${forbidden_values[$var]:-}" && "$value" == "${forbidden_values[$var]}" ]]; then
    blocked+=("$var")
    blocked_messages+=("$var is still set to the deploy/.env.example placeholder (${forbidden_values[$var]}). Replace it with a unique deployment secret.")
  fi
done

image_tag_vars=(
  BITRIVER_LIVE_IMAGE_TAG
  BITRIVER_VIEWER_IMAGE_TAG
  BITRIVER_SRS_CONTROLLER_IMAGE_TAG
  BITRIVER_TRANSCODER_IMAGE_TAG
  BITRIVER_SRS_IMAGE_TAG
  BITRIVER_OME_IMAGE_TAG
)

for var in "${image_tag_vars[@]}"; do
  if [[ -z "${!var-}" ]]; then
    unset_image_tags+=("$var")
  fi
done

if (( ${#unset_image_tags[@]} > 0 )); then
  echo "Populate these required image tags with the release version you extracted earlier:" >&2
  for var in "${unset_image_tags[@]}"; do
    echo "  - $var" >&2
  done
  missing+=("${unset_image_tags[@]}")
fi

if (( ${#missing[@]} > 0 )); then
  echo "The following required variables are unset or empty in $ENV_FILE:" >&2
  for var in "${missing[@]}"; do
    echo "  - $var" >&2
  done
fi

if [[ -n "${BITRIVER_SRS_IMAGE_TAG:-}" && "$BITRIVER_SRS_IMAGE_TAG" != "v5.0.185" ]]; then
  echo "Reminder: BITRIVER_SRS_IMAGE_TAG is set to $BITRIVER_SRS_IMAGE_TAG." >&2
  echo "Update the SRS tag in deploy/systemd/README.md and any running systemd units to match before upgrading." >&2
fi

if (( ${#blocked[@]} > 0 )); then
  for msg in "${blocked_messages[@]}"; do
    echo "$msg" >&2
  done
fi

if [[ -n "${BITRIVER_LIVE_POSTGRES_DSN:-}" && "$BITRIVER_LIVE_POSTGRES_DSN" == *"bitriver:bitriver"* ]]; then
  echo "Warning: BITRIVER_LIVE_POSTGRES_DSN still references bitriver:bitriver. Update or unset it to match the Postgres credentials." >&2
fi

if [[ -n "${BITRIVER_TRANSCODER_PUBLIC_BASE_URL:-}" ]]; then
  if [[ "$BITRIVER_TRANSCODER_PUBLIC_BASE_URL" == "https://cdn.example.com/hls" ]]; then
    errors+=("BITRIVER_TRANSCODER_PUBLIC_BASE_URL still uses the sample CDN URL (https://cdn.example.com/hls) from deploy/.env.example. Point this at the public origin end users can reach instead.")
  elif [[ "$BITRIVER_TRANSCODER_PUBLIC_BASE_URL" =~ ^https?://(localhost|127\.[0-9.]*|0\.0\.0\.0|::1|\[::1\])([:/]|$) ]]; then
    errors+=("BITRIVER_TRANSCODER_PUBLIC_BASE_URL points at loopback ($BITRIVER_TRANSCODER_PUBLIC_BASE_URL). Configure a CDN, reverse proxy, or other routable origin instead.")
  fi
fi

if [[ -n "${BITRIVER_OME_API:-}" ]]; then
  if [[ "$BITRIVER_OME_API" =~ ^https?://(localhost|127\.[0-9.]*|0\.0\.0\.0|::1|\[::1\])([:/]|$) ]]; then
    errors+=("BITRIVER_OME_API points at loopback ($BITRIVER_OME_API). Use the ome hostname from docker-compose.yml or another reachable host/IP.")
  fi
fi

if (( ${#errors[@]} > 0 )); then
  for msg in "${errors[@]}"; do
    echo "$msg" >&2
  done
fi

if (( ${#missing[@]} > 0 || ${#blocked[@]} > 0 || ${#errors[@]} > 0 )); then
  exit 1
fi

echo "Environment file $ENV_FILE looks ready."
