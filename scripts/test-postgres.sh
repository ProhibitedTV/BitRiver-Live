#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

DB_USER="${BITRIVER_TEST_POSTGRES_USER:-bitriver}"
DB_PASSWORD="${BITRIVER_TEST_POSTGRES_PASSWORD:-bitriver}"
DB_NAME="${BITRIVER_TEST_POSTGRES_DB:-bitriver_test}"
PORT="${BITRIVER_TEST_POSTGRES_PORT:-54329}"
IMAGE="${BITRIVER_TEST_POSTGRES_IMAGE:-postgres:15-alpine}"
CONTAINER_NAME="bitr-postgres-test-$$"
if [ -z "${BITRIVER_TEST_POSTGRES_DSN:-}" ]; then
  if ! command -v docker >/dev/null 2>&1; then
    echo "error: docker is required to run postgres-tagged tests when BITRIVER_TEST_POSTGRES_DSN is unset" >&2
    if [ -n "${CI:-}" ]; then
      exit 1
    fi

    echo "skipping postgres-tagged tests; set BITRIVER_TEST_POSTGRES_DSN to run without docker" >&2
    exit 0
  fi

  MIGRATIONS_DIR="$REPO_ROOT/deploy/migrations"
  if [ ! -d "$MIGRATIONS_DIR" ]; then
    echo "error: expected migrations under $MIGRATIONS_DIR" >&2
    exit 1
  fi

  cleanup() {
    docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
  }
  trap cleanup EXIT INT TERM

  export BITRIVER_TEST_POSTGRES_DSN="postgres://$DB_USER:$DB_PASSWORD@127.0.0.1:$PORT/$DB_NAME?sslmode=disable"

  # shellcheck disable=SC2086
  docker run \
    --rm \
    --detach \
    --name "$CONTAINER_NAME" \
    --publish "$PORT:5432" \
    --env "POSTGRES_USER=$DB_USER" \
    --env "POSTGRES_PASSWORD=$DB_PASSWORD" \
    --env "POSTGRES_DB=$DB_NAME" \
    --health-cmd="pg_isready -U $DB_USER -d $DB_NAME" \
    --health-interval=5s \
    --health-timeout=5s \
    --health-retries=10 \
    --volume "$MIGRATIONS_DIR:/migrations:ro" \
    $IMAGE >/dev/null

  echo "waiting for postgres to become ready..." >&2
  for attempt in $(seq 1 60); do
    status=$(docker inspect --format '{{.State.Health.Status}}' "$CONTAINER_NAME" 2>/dev/null || echo "")
    if [ "$status" = "healthy" ]; then
      break
    fi
    if [ "$status" = "unhealthy" ]; then
      echo "error: postgres container failed health checks" >&2
      docker logs "$CONTAINER_NAME" >&2 || true
      exit 1
    fi
    if ! docker ps --format '{{.Names}}' | grep -q "^$CONTAINER_NAME$"; then
      echo "error: postgres container exited early" >&2
      docker logs "$CONTAINER_NAME" >&2 || true
      exit 1
    fi
    sleep 1
    if [ "$attempt" -eq 60 ]; then
      echo "error: postgres container did not become ready" >&2
      docker logs "$CONTAINER_NAME" >&2 || true
      exit 1
    fi
  done

  echo "applying migrations from $MIGRATIONS_DIR" >&2
  mapfile -t migrations < <(find "$MIGRATIONS_DIR" -maxdepth 1 -type f -name '*.sql' | sort)
  if [ ${#migrations[@]} -eq 0 ]; then
    echo "error: no migrations found in $MIGRATIONS_DIR" >&2
    exit 1
  fi

  for migration in "${migrations[@]}"; do
    base_name="$(basename "$migration")"
    echo "  -> $base_name" >&2
    docker exec -e PGPASSWORD="$DB_PASSWORD" "$CONTAINER_NAME" \
      psql -v ON_ERROR_STOP=1 \
        -h localhost \
        -U "$DB_USER" \
        -d "$DB_NAME" \
        -f "/migrations/$base_name"
  done
else
  echo "using provided BITRIVER_TEST_POSTGRES_DSN; skipping docker setup" >&2
fi

default_packages=("./internal/storage/...")
if [ "$#" -gt 0 ]; then
  packages=("$@")
else
  packages=("${default_packages[@]}")
fi

echo "running go test -tags postgres ${packages[*]}" >&2
export GOTOOLCHAIN="${GOTOOLCHAIN:-local}"

# The postgres suite must run without contacting module proxies so vendored
# replacements remain intact; use the vendored module set and disable the
# network.
export GOPROXY="${GOPROXY:-off}"
export GOSUMDB="${GOSUMDB:-off}"
if [ -n "${GOFLAGS:-}" ]; then
  export GOFLAGS="$GOFLAGS -mod=vendor"
else
  export GOFLAGS="-mod=vendor"
fi

go test -count=1 -tags postgres "${packages[@]}"
