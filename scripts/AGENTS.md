# `scripts/` Guidance

Automation and CI helpers live here. Read the root `AGENTS.md` for global policy.

## Expectations
- Scripts run in CI, so keep them POSIX-compliant (`/bin/sh`) unless explicitly marked otherwise. Start files with `set -euo pipefail` (or `set -Eeuo pipefail` for bash) to fail fast.
- Key entrypoints include `scripts/test-postgres.sh` and `scripts/test-wizard-release.sh`. They must remain idempotent and safe to re-run.
- Document new scripts in `README.md` or `docs/` when they affect developer workflows.

## Before opening a PR
- Test scripts locally (or in a container) after changes.
- Update any GitHub Actions / CI references if filenames change.
- Ensure scripts integrate with the quickstart/deploy instructions where relevant.
