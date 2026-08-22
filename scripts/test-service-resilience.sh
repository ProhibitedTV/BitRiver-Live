#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

usage() {
  cat <<'USAGE'
Usage: ./scripts/test-service-resilience.sh [service-resilience options]

Runs a destructive-but-isolated local-build rehearsal of API, Postgres, Redis,
SRS/controller, OvenMediaEngine, transcoder, and viewer stop/start recovery.
The command refuses any existing canonical BitRiver containers, stages a clean
tracked tree plus a private copy of the root .env, and removes its containers,
volumes, credentials, and copied environment on completion.

Common options:
  --report PATH        Retained secret-scanned JSON report path.
  --wait-seconds N     Per-degradation/recovery deadline (15-900; default 240).
  --retain-workdir     Retain the private staged tree for debugging.
  -h, --help           Show the Go runner's complete option help.
USAGE
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

cd "$REPO_ROOT"
export GOTOOLCHAIN="${GOTOOLCHAIN:-local}"
export GOPROXY="${GOPROXY:-off}"
export GOSUMDB="${GOSUMDB:-off}"
exec go run ./cmd/tools/service-resilience "$@"
