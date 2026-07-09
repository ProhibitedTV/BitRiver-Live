#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT_DIR"

usage() {
  cat <<'USAGE'
Usage: ./scripts/release-gate-smoke.sh [bitriver release smoke-gate flags]

Runs the named golden-path release smoke gate through the Go CLI.

Common examples:
  ./scripts/release-gate-smoke.sh --tier fast
  ./scripts/release-gate-smoke.sh --tier full --target v1.2.3
  ./scripts/release-gate-smoke.sh --tier fast --artifact-dir .artifacts/release-gate-fast

Pass -h or --help to the Go command for the complete flag list.
USAGE
}

case "${1:-}" in
  -h|--help)
    usage
    echo
    GOTOOLCHAIN="${GOTOOLCHAIN:-local}" \
      GOPROXY="${GOPROXY:-off}" \
      GOSUMDB="${GOSUMDB:-off}" \
      go run ./cmd/bitriver release smoke-gate --help
    exit 0
    ;;
esac

GOTOOLCHAIN="${GOTOOLCHAIN:-local}" \
  GOPROXY="${GOPROXY:-off}" \
  GOSUMDB="${GOSUMDB:-off}" \
  go run ./cmd/bitriver release smoke-gate "$@"
