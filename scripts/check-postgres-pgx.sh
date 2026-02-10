#!/bin/sh
set -eu

expected_driver="${1:-${BITRIVER_LIVE_STORAGE_DRIVER:-json}}"

case "$expected_driver" in
  json|postgres)
    ;;
  *)
    echo "error: expected storage driver must be json or postgres (got '$expected_driver')" >&2
    exit 1
    ;;
esac

allow_stub_for_postgres="${BITRIVER_LIVE_ALLOW_STUB_PGX_FOR_POSTGRES:-false}"

exec go run ./cmd/tools/pgx-mode-check \
  --expected-storage-driver "$expected_driver" \
  --allow-stub-for-postgres="$allow_stub_for_postgres"
