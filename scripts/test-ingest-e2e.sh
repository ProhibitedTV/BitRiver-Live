#!/usr/bin/env bash
set -Eeuo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# Respect existing GOFLAGS while forcing vendored modules for reproducibility.
GOFLAGS_VALUE="${GOFLAGS:-}"
if [[ "$GOFLAGS_VALUE" != *"-mod=vendor"* ]]; then
  GOFLAGS_VALUE="-mod=vendor ${GOFLAGS_VALUE}"
fi
GOFLAGS_VALUE="${GOFLAGS_VALUE# }"

export GOTOOLCHAIN="local"
export GOPROXY="off"
export GOSUMDB="off"
export GOFLAGS="$GOFLAGS_VALUE"

go test ./internal/storage -count=1 -timeout=120s -run TestIngestPipelineEndToEnd
