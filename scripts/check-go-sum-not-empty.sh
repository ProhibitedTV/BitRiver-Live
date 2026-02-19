#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT_DIR"

if [[ ! -s go.sum ]]; then
  echo "error: go.sum is empty. Run ./scripts/refresh-go-sum.sh in a network-enabled environment and commit the resulting checksums." >&2
  exit 1
fi

echo "go.sum presence check passed."
