#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/deploy/docker-compose.yml"
ENV_FILE="$ROOT_DIR/.env"

usage() {
  cat <<'USAGE'
Usage: ./scripts/require-image-digests.sh [--env-file PATH] [--compose-file PATH]

Ensures third-party image digest variables used by deploy/docker-compose.yml are
set when running in production mode.

Behavior:
- Production enforcement is enabled only when BOTH are true:
  - BITRIVER_LIVE_MODE=production
  - BITRIVER_DEPLOY_IMAGE_SOURCE=pull
- Non-production contexts exit successfully without requiring digests.
USAGE
}

while (($# > 0)); do
  case "$1" in
    --env-file)
      ENV_FILE="$2"
      shift 2
      ;;
    --compose-file)
      COMPOSE_FILE="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ ! -f "$COMPOSE_FILE" ]]; then
  echo "Compose file not found: $COMPOSE_FILE" >&2
  exit 1
fi

if [[ -f "$ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
fi

live_mode="${BITRIVER_LIVE_MODE:-}"
live_mode="$(printf '%s' "$live_mode" | tr '[:upper:]' '[:lower:]' | xargs)"
image_source="${BITRIVER_DEPLOY_IMAGE_SOURCE:-pull}"
image_source="$(printf '%s' "$image_source" | tr '[:upper:]' '[:lower:]' | xargs)"

if [[ "$live_mode" != "production" || "$image_source" != "pull" ]]; then
  echo "Skipping digest enforcement (BITRIVER_LIVE_MODE=${live_mode:-unset}, BITRIVER_DEPLOY_IMAGE_SOURCE=${image_source:-unset})."
  exit 0
fi

checks=(
  "redis:7-alpine|BITRIVER_REDIS_IMAGE_DIGEST"
  "postgres:15-alpine|BITRIVER_POSTGRES_IMAGE_DIGEST"
  "ossrs/srs:|BITRIVER_SRS_IMAGE_DIGEST"
  "airensoft/ovenmediaengine:|BITRIVER_OME_IMAGE_DIGEST"
  "nginx:alpine|BITRIVER_NGINX_IMAGE_DIGEST"
  "alpine:3|BITRIVER_ALPINE_3_IMAGE_DIGEST"
  "alpine:3.19|BITRIVER_ALPINE_3_19_IMAGE_DIGEST"
  "debian:12-slim|BITRIVER_DEBIAN_IMAGE_DIGEST"
)

missing_compose_refs=()
missing_digests=()
invalid_digests=()

for entry in "${checks[@]}"; do
  image_ref="${entry%%|*}"
  digest_var="${entry##*|}"

  if ! grep -Eq "image:[[:space:]]*${image_ref}.*\\$\\{${digest_var}" "$COMPOSE_FILE"; then
    missing_compose_refs+=("${digest_var} (expected image containing '${image_ref}')")
    continue
  fi

  digest_value="${!digest_var:-}"
  digest_value="$(printf '%s' "$digest_value" | xargs)"
  if [[ -z "$digest_value" ]]; then
    missing_digests+=("$digest_var")
    continue
  fi

  if [[ ! "$digest_value" =~ ^@sha256:[a-f0-9]{64}$ ]]; then
    invalid_digests+=("${digest_var}=${digest_value}")
  fi
done

if ((${#missing_compose_refs[@]} > 0)); then
  {
    echo "Compose contract mismatch: expected third-party digest placeholders were not found in ${COMPOSE_FILE}."
    printf ' - %s\n' "${missing_compose_refs[@]}"
    echo "Update scripts/require-image-digests.sh to match deploy/docker-compose.yml before cutting a production release."
  } >&2
  exit 1
fi

if ((${#missing_digests[@]} > 0)) || ((${#invalid_digests[@]} > 0)); then
  {
    echo "Production image digest enforcement failed."
    echo "Set all required third-party digest variables in ${ENV_FILE} (or injected environment):"
    echo "  BITRIVER_REDIS_IMAGE_DIGEST"
    echo "  BITRIVER_POSTGRES_IMAGE_DIGEST"
    echo "  BITRIVER_SRS_IMAGE_DIGEST"
    echo "  BITRIVER_OME_IMAGE_DIGEST"
    echo "  BITRIVER_NGINX_IMAGE_DIGEST"
    echo "  BITRIVER_ALPINE_3_IMAGE_DIGEST"
    echo "  BITRIVER_ALPINE_3_19_IMAGE_DIGEST"
    echo "  BITRIVER_DEBIAN_IMAGE_DIGEST"
    echo "Expected format: @sha256:<64 lowercase hex chars>."
    if ((${#missing_digests[@]} > 0)); then
      echo "Missing values:"
      printf ' - %s\n' "${missing_digests[@]}"
    fi
    if ((${#invalid_digests[@]} > 0)); then
      echo "Invalid values:"
      printf ' - %s\n' "${invalid_digests[@]}"
    fi
  } >&2
  exit 1
fi

echo "Production image digest enforcement passed for third-party images."
