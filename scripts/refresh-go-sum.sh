#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT_DIR"

if [[ -n "${GOPROXY:-}" && "${GOPROXY}" == "off" ]]; then
  echo "error: GOPROXY=off blocks checksum refresh. Use a network-enabled GOPROXY (for example https://proxy.golang.org,direct)." >&2
  exit 1
fi

temp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$temp_dir"
}
trap cleanup EXIT

cat >"$temp_dir/go.mod" <<'MOD'
module bitriver-live-go-sum-sync

go 1.21

require (
	github.com/jackc/pgpassfile v1.0.0
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a
	github.com/jackc/pgx/v5 v5.7.4
	github.com/jackc/puddle/v2 v2.2.2
	github.com/redis/go-redis/v9 v9.5.1
	golang.org/x/crypto v0.27.0
	golang.org/x/sync v0.7.0
	golang.org/x/text v0.18.0
)
MOD

(
  cd "$temp_dir"
  env \
    GOTOOLCHAIN=local \
    GOPROXY="${GOPROXY:-https://proxy.golang.org,direct}" \
    GOSUMDB="${GOSUMDB:-sum.golang.org}" \
    GOFLAGS='' \
    go mod download all
)

cp "$temp_dir/go.sum" "$ROOT_DIR/go.sum"

echo "Refreshed go.sum using networked module checksums."
