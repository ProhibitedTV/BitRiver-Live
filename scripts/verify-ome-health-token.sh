#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
ENV_FILE="${ENV_FILE:-$REPO_ROOT/.env}"
CONFIG_FILE="${CONFIG_FILE:-$REPO_ROOT/deploy/ome/Server.generated.xml}"

usage() {
  cat <<'USAGE'
Usage: scripts/verify-ome-health-token.sh [--env-file PATH] [--config PATH]

Checks that deploy/ome/Server.generated.xml contains a non-empty
<Managers><API><AccessToken> value and that it matches the runtime
health token source ${BITRIVER_OME_ACCESS_TOKEN:-$BITRIVER_OME_API_TOKEN}
resolved from the same .env file Docker Compose uses.
USAGE
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --env-file)
      shift
      ENV_FILE="${1:-}"
      [ -n "$ENV_FILE" ] || {
        echo "--env-file requires a path" >&2
        exit 1
      }
      ;;
    --config)
      shift
      CONFIG_FILE="${1:-}"
      [ -n "$CONFIG_FILE" ] || {
        echo "--config requires a path" >&2
        exit 1
      }
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
  shift
done

[ -f "$ENV_FILE" ] || {
  echo "OME token verification failed: env file not found at $ENV_FILE" >&2
  exit 1
}

[ -f "$CONFIG_FILE" ] || {
  echo "OME token verification failed: generated config not found at $CONFIG_FILE" >&2
  exit 1
}

read_env_value() {
  key="$1"
  awk -v key="$key" '
    /^[[:space:]]*#/ || /^[[:space:]]*$/ { next }
    {
      line = $0
      sub(/^[[:space:]]*export[[:space:]]+/, "", line)
      eq = index(line, "=")
      if (eq == 0) next
      name = substr(line, 1, eq - 1)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", name)
      if (name != key) next
      val = substr(line, eq + 1)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", val)
      if (val ~ /^".*"$/ || val ~ /^\047.*\047$/) {
        val = substr(val, 2, length(val) - 2)
      }
      out = val
    }
    END {
      if (out != "") {
        print out
      }
    }
  ' "$ENV_FILE"
}

rendered_token=$(awk '
  BEGIN {
    in_managers = 0
    in_api = 0
    seen_managers = 0
    seen_api = 0
  }
  /<Managers>/ {
    if (seen_managers == 0 && in_managers == 0) {
      in_managers = 1
      seen_managers = 1
    }
  }
  in_managers && /<API>/ {
    if (seen_api == 0 && in_api == 0) {
      in_api = 1
      seen_api = 1
    }
  }
  in_managers && in_api && /<AccessToken>/ {
    token = $0
    sub(/^.*<AccessToken>[[:space:]]*/, "", token)
    sub(/[[:space:]]*<\/AccessToken>.*$/, "", token)
    gsub(/^[[:space:]]+|[[:space:]]+$/, "", token)
    print token
    exit
  }
  in_api && /<\/API>/ { in_api = 0 }
  in_managers && /<\/Managers>/ { in_managers = 0 }
' "$CONFIG_FILE")

access_token=$(read_env_value "BITRIVER_OME_ACCESS_TOKEN" || true)
api_token=$(read_env_value "BITRIVER_OME_API_TOKEN" || true)
expected_token="$access_token"
if [ -z "$expected_token" ]; then
  expected_token="$api_token"
fi

[ -n "$rendered_token" ] || {
  echo "OME token verification failed: <Managers><API><AccessToken> is empty in $CONFIG_FILE" >&2
  exit 1
}

[ -n "$expected_token" ] || {
  echo "OME token verification failed: resolved runtime token from \${BITRIVER_OME_ACCESS_TOKEN:-\$BITRIVER_OME_API_TOKEN} is empty in $ENV_FILE" >&2
  exit 1
}

if [ "$rendered_token" != "$expected_token" ]; then
  cat >&2 <<EOF_MSG
OME token verification failed: rendered and runtime tokens differ.
  rendered (<Managers><API><AccessToken>): $rendered_token
  expected (\${BITRIVER_OME_ACCESS_TOKEN:-\$BITRIVER_OME_API_TOKEN}): $expected_token
Fix by updating $ENV_FILE and re-rendering with:
  go run ./cmd/bitriver ome render --force --env-file $ENV_FILE
EOF_MSG
  exit 1
fi

echo "OME token verification passed: rendered AccessToken matches compose runtime health token source."
