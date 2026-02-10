#!/bin/sh
set -eu

expected_driver="${1:-${BITRIVER_LIVE_STORAGE_DRIVER:-postgres}}"

if [ "$expected_driver" != "postgres" ]; then
  echo "error: expected storage driver must be postgres for deployment checks (got '$expected_driver')" >&2
  exit 1
fi

exec go run ./cmd/tools/pgx-mode-check --expected-storage-driver "$expected_driver"
