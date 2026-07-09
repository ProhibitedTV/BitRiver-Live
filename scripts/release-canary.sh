#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT_DIR"

GOTOOLCHAIN="${GOTOOLCHAIN:-local}" \
GOPROXY="${GOPROXY:-off}" \
GOSUMDB="${GOSUMDB:-off}" \
go run ./cmd/bitriver release canary "$@"
