# PLAN

## Scope
- Resolve production blocker #1296 with one canonical PostgreSQL migration runner shared by Docker Compose and Helm.
- Add a durable `schema_migrations` ledger containing filename, numeric version prefix, SHA-256 checksum, lifecycle status, timestamps, and release/commit metadata.
- Apply only pending migrations in byte-sorted filename order; refuse checksum drift and ambiguous `applying`/`failed` states.
- Provide read-only plan/status output plus checksum-confirmed retry and mark-applied recovery commands through the `bitriver` CLI.
- Include non-sensitive migration history in the one-shot job logs already captured by release diagnostics.
- Document forward-only schema policy, rollback boundaries, backups, and recovery.

## Design Decisions
- Filename is the ledger identity because the canonical set intentionally contains both `0002_auth_sessions.sql` and `0002_chat_filters.sql`; the numeric prefix remains queryable metadata.
- Canonical checksums are raw SHA-256 digests. Generated Helm SQL files must therefore be byte-for-byte copies of `deploy/migrations/*.sql`.
- Each migration runs through `psql --single-transaction`. A durable `applying` row is written first, then changed to `applied` only after SQL success; failures become `failed` and stop startup.
- A process interruption leaves an explicit `applying` record. Recovery requires the exact recorded checksum and an operator choice to retry a known-rolled-back migration or mark a manually verified migration applied.
- The Compose job derives release metadata from the API image tag and accepts optional `BITRIVER_RELEASE_COMMIT`; Helm uses its API tag and optional release commit value.
- The migration runner prints the final sanitized ledger, so existing release log collection captures applied history without credentials or SQL payloads.

## Assumptions
- Existing installations have run the historical idempotent SQL set but have no ledger. The first ledger-aware run safely replays the canonical set once, records it, and future runs become no-ops.
- PostgreSQL 15 Alpine provides `psql` and `sha256sum`; the runner verifies its required tools before touching the ledger.
- Migration filenames remain restricted and deterministic. Duplicate numeric prefixes are allowed, but duplicate filenames are impossible.
- The user's Ubuntu/XOA/Nginx Proxy Manager target remains assigned to clean-host installer issue #1297 after migration safety lands.
- Real OvenMediaEngine playback proof and restart/readiness recovery remain scoped to #1300 and #1304; this change must not claim OME deployment readiness.

## Risks
- A crash after schema commit but before the ledger update is inherently ambiguous; preserve `applying` and require checksum-confirmed manual verification instead of guessing.
- A failed non-transactional migration could leave partial state; force the runner's single transaction and document that future migrations must not opt out or embed irreversible work without release notes.
- Editing generated Helm SQL headers would create topology-specific checksums; remove migration headers and verify exact byte parity automatically.
- The existing Helm generated set is missing migrations `0008` through `0011`; regenerate it from canonical sources and test the complete set.
- Compose, Helm, CLI, docs, and verification all consume the migration contract; keep the implementation focused on this behavior and avoid unrelated installer, proxy, or OME changes.
- `docs/contract.md` and `deploy/ome/Server.generated.xml` already have line-ending-only working-tree changes; modify only the necessary contract lines and leave the OME file untouched.

## Test Plan
- Shell syntax and focused Go tests for CLI argument validation and Docker Compose invocation.
- Isolated PostgreSQL integration evidence for fresh apply, previous-schema upgrade, no-op rerun, checksum-drift refusal, failed migration retry, interrupted-state mark-applied recovery, and sanitized status output.
- Helm asset sync `--check`, Helm template/lint when available, generated contract invariants, and Docker Compose config rendering.
- Canonical quickstart smoke proving a failed/ambiguous migration blocks API startup and a healthy no-op migration job completes.
- Full `./scripts/verify.sh`, `git diff --check`, and pull-request CI before merge.

## Boundaries
- The user explicitly authorized roadmap and deployment-contract work for the Ubuntu home-hosting target; modify Compose and its matching contract docs only as required by #1296.
- Do not edit root `.env`, generated OME credentials/config, or the user's untracked deployment helper files.
- Do not build the Ubuntu installer, Nginx Proxy Manager integration, or OME readiness changes in this PR; carry those requirements into #1297, #1300, and #1304.
- Do not provide automatic down migrations or claim arbitrary destructive downgrades are supported.

## Completion
- Implemented and locally verified on 2026-07-17. The exact `./scripts/verify.sh` wrapper reached its Go step and stopped because the Windows host has Go 1.25.6 while the repository requires 1.26 with `GOTOOLCHAIN=local`; every constituent gate was then run with the pinned Go 1.26.5 container plus host Docker/Git Bash.
- Real PostgreSQL lifecycle evidence, all Go package zones, architecture/dependency checks, generated contract/Helm parity, Compose rendering, and the full quickstart smoke passed. OME reported healthy in the smoke, and the pre-existing generated OME file was restored byte-for-byte afterward.
- This proves the migration safety change and basic OME process health only. Clean Ubuntu/XOA/Nginx Proxy Manager install proof remains #1297; real OME playback remains #1300; OME restart/unavailable recovery remains #1304.
