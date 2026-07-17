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

production_mod="$temp_dir/go.production.mod"
go run ./cmd/tools/production-module --output "$production_mod"
rm -f "$temp_dir/go.production.sum"

(
  env \
    GOTOOLCHAIN=local \
    GOPROXY="${GOPROXY:-https://proxy.golang.org,direct}" \
    GOSUMDB="${GOSUMDB:-sum.golang.org}" \
    GOFLAGS='' \
    go mod download -modfile="$production_mod" all
)

cp "$temp_dir/go.production.sum" "$ROOT_DIR/go.sum"

echo "Refreshed go.sum using networked module checksums."
