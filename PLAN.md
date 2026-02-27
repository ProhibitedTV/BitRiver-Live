# PLAN

## Scope (current change)
- Update `scripts/verify.sh` to include a deterministic quickstart smoke phase (`./scripts/test-quickstart.sh`) when Docker is available.
- Mirror existing compose-validation skip behavior with clear messaging when Docker is unavailable.
- Keep docs synchronized for verify gate coverage in `AGENTS.md`, `docs/testing.md`, and `docs/production-release.md`.

## Assumptions
- `scripts/verify.sh` should remain fail-fast (`set -Eeuo pipefail`) and stop immediately on smoke-test failure.
- Docker availability check should continue to be PATH-based (`command -v docker`) to mirror current compose validation behavior.
- Docs should describe the new verify ordering and Docker-dependent skip semantics without changing unrelated release policy text.

## Risks
- Running quickstart smoke before/after other phases incorrectly could make verify ordering non-deterministic versus documentation.
- If skip messaging diverges between compose validation and smoke phase, operators may misunderstand what was actually validated.
- `./scripts/test-quickstart.sh` may take longer and fail in constrained environments; this should intentionally fail verify when Docker is available.

## Test plan
- Run `bash -n scripts/verify.sh` to validate shell syntax after edits.
- Run `./scripts/verify.sh` and confirm deterministic ordering includes:
  1) Docker Compose config validation, then
  2) Quickstart smoke (`./scripts/test-quickstart.sh`) when Docker exists, or explicit skip message when not.
- Review `docs/testing.md`, `docs/production-release.md`, and `AGENTS.md` for contract alignment with verify coverage.
