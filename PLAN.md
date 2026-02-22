# PLAN

## Scope (current change)
- Refactor repeated `ctx == nil` guards in scoped packages: `internal/chat`, `internal/storage`, `internal/auth`, `internal/server`, and `internal/observability`.
- Add a small private helper named `normalizeContext` in each affected package that currently repeats the same fallback logic.
- Replace targeted inline nil-context checks with calls to `normalizeContext` while preserving exact behavior (`context.Background()` fallback).
- Run affected package tests to verify no regressions.

## Assumptions
- The helper remains unexported and package-local (`normalizeContext`).
- Only repeated nil checks in scoped files are replaced; other context creation patterns (timeouts/cancels) stay as-is.
- No runtime behavior, APIs, SQL, or contract files are changed.

## Risks
- Introducing duplicate helper definitions in the same package could cause build conflicts.
- Missing a nil-check replacement could leave inconsistency.
- Refactor-only change could still affect behavior if fallback semantics differ from current code.

## Test plan
- `go test ./internal/chat ./internal/storage ./internal/auth ./internal/server ./internal/observability -count=1`
