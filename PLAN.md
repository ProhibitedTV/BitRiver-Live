# PLAN

## Scope (current change)
- Add `_FILE` secret resolution support to `cmd/bitriver env validate` so required secret-like env keys can be supplied via mounted files.
- Ensure validation reports distinguish missing secret values vs `_FILE` path/readability failures.
- Document `_FILE` operator usage in `deploy/.env.example`, `docs/secrets-hardening.md`, and a new `docs/security.md`.
- Add focused tests in `cmd/bitriver/env_validation_test.go` for direct values, `_FILE` values, precedence, unreadable/missing file, and empty file content.

## Assumptions
- `_FILE` support is validation-time only in this change; runtime service env consumption remains unchanged.
- Deterministic precedence will be: explicit `<KEY>` value wins over `<KEY>_FILE`, with a warning when both are set.
- Secret file content should be trimmed for surrounding whitespace/newlines before use.

## Risks
- If precedence is unclear to operators, mixed `<KEY>` + `<KEY>_FILE` configuration could mask expected updates.
- File-permission behavior can vary by platform; tests should avoid brittle permission assumptions and still cover unreadable/missing paths.
- Missing one secret-like key could produce inconsistent validation behavior.

## Test plan
- `go test ./cmd/bitriver -count=1`
- `./scripts/verify.sh`
