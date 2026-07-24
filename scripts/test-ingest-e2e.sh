#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Compatibility entrypoint retained for existing branch protection and
# workflow callers. This is now a real canonical-Compose product gate; the
# cheap storage/controller integration guard lives in test-ingest-storage.sh.
exec "$SCRIPT_DIR/test-production-golden-path.sh" \
  --stack quickstart \
  --client "${BITRIVER_GOLDEN_PATH_CLIENT:-docker}" \
  --artifact-dir "${BITRIVER_GOLDEN_PATH_ARTIFACT_DIR:-$SCRIPT_DIR/../.artifacts/production-golden-path}"
