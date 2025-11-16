# `cmd/tools` Guidance

Helper binaries here (for example `bootstrap-admin`, `migrate-json-to-postgres`) are CI-grade utilities. Follow the root `AGENTS.md` plus the notes below.

## Expectations
- Validate input thoroughly (flags + env). Fail fast with actionable errors.
- Prefer using the shared storage snapshot helpers under `internal/storage` to ensure consistency across migrations/imports.
- Migration helpers must verify record counts and critical invariants after running. Whenever the schema evolves, extend verification to cover new tables/fields so data moves safely.
- Keep tools composable so scripts/CI can call them non-interactively (no prompts, exit non-zero on failure).

## Before opening a PR
- Add/refresh usage docs in `docs/` when adding or changing a tool.
- Ensure new helpers integrate with `scripts/` or `deploy/` workflows when relevant.
- Cover critical logic with unit tests and, when applicable, integration tests behind the `postgres` tag.
