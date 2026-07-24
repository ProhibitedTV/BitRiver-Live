#!/usr/bin/env bash
set -Eeuo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

export GOTOOLCHAIN="local"
export GOPROXY="off"
export GOSUMDB="off"

go test ./internal/storage -count=1 -timeout=120s -run TestIngestPipelineEndToEnd
