# TASKS

## Scoped change: deterministic PostgreSQL migration ledger (#1296)

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 - Inventory migration paths and establish the safety design
  - Acceptance criteria:
    - `SPEC.md` and `PLAN.md` describe the ledger, checksum, transaction, recovery, parity, documentation, and test contract.
    - Compose, Helm, CLI, release diagnostics, docs, tests, and existing migration files are inventoried before implementation.
    - Existing unrelated working-tree changes and downstream installer/OME scope are recorded as boundaries.
  - Check:
    - Read-only inventory found Compose and Helm blindly replay every SQL file, no ledger exists, duplicate `0002_*` prefixes require filename identity, and Helm is missing canonical migrations `0008` through `0011`.
    - Existing `schema_migrations` exclusion in PostgreSQL test cleanup confirms the intended ledger table name.
    - `docs/contract.md` and generated OME config differences are line-ending-only; untracked deployment helpers remain out of scope.

- [x] Task 2 - Implement the canonical ledger-aware migration runner
  - Acceptance criteria:
    - Runner validates inputs/tools, creates the ledger, orders filenames deterministically, records provenance/status, and applies only pending SQL.
    - Applied checksum drift and `applying`/`failed` state stop with actionable, non-sensitive errors.
    - Plan, status, retry, and mark-applied modes are explicit and checksum-confirmed.
    - Every migration is executed transactionally and final ledger history is printed without credentials or SQL payloads.
  - Check:
    - Added `deploy/postgres-migrate.sh` with ledger creation, deterministic raw SHA-256 checksums, pending-only apply, transaction execution, drift/ambiguous-state refusal, sanitized history, and checksum-confirmed recovery.
    - `C:\Program Files\Git\bin\bash.exe -n deploy/postgres-migrate.sh` passed.
    - `git diff --check -- PLAN.md SPEC.md TASKS.md deploy/postgres-migrate.sh` passed.

- [x] Task 3 - Wire Compose, Helm, and CLI to one canonical mechanism
  - Acceptance criteria:
    - Compose and Helm use the shared runner and pass release/commit metadata.
    - Helm migration SQL and runner copies are generated from canonical assets and SQL parity is byte-for-byte.
    - `bitriver migrations` exposes plan/status/apply/recovery without printing credentials.
    - Upgrade planning points operators to the read-only migration preflight.
  - Check:
    - Compose config renders quietly with the shared runner mounted read-only, explicit connection/provenance env, and `apply` as the default command.
    - The Helm job invokes the generated runner; all 12 SQL files and the runner match canonical SHA-256 bytes, and `./scripts/sync-helm-deploy-assets.sh --check` passed.
    - Focused Go 1.26.5 container tests passed for CLI validation, repair invocation, quickstart no-TTY behavior, and upgrade-plan output.
    - `git diff --check` passed for the CLI, Compose, Helm, env example, and sync changes.

- [x] Task 4 - Add migration lifecycle and recovery evidence
  - Acceptance criteria:
    - Tests cover fresh database, representative previous schema, no-op rerun, checksum drift, failed retry, interrupted mark-applied recovery, and sanitized history.
    - CLI invocation/validation and generated Helm parity are covered.
    - Relevant focused checks pass before documentation work proceeds.
  - Check:
    - `./scripts/test-postgres-migrations.sh` passed against disposable `postgres:15-alpine` and is now part of the Docker-enabled `./scripts/verify.sh` gate.
    - Fresh apply recorded one migration; adding a second file produced one pending upgrade and then a two-row applied history; the next apply was a no-op.
    - Editing the first applied SQL caused read-only plan failure with both recorded and current SHA-256 values.
    - A transactional missing-table failure recorded `failed` with no partial table; checksum-confirmed retry succeeded after the dependency was restored.
    - A simulated post-commit/pre-ledger-update interruption remained `applying`, blocked plan, and required checksum-confirmed mark-applied after schema verification.
    - Sanitized status included filename/version/checksum/status/timestamp/release/commit and did not contain the test database credential.
    - Focused Go 1.26.5 CLI/upgrade tests and generated Helm parity check passed after the final runner fix.

- [x] Task 5 - Update deployment, upgrade, rollback, and testing docs
  - Acceptance criteria:
    - Contract and operator docs describe ledger behavior, preflight, forward-only policy, destructive-change release notes, backups, and recovery.
    - Compose/Helm parity and first ledger-aware upgrade behavior are explicit.
    - Downstream Ubuntu/XOA/Nginx Proxy Manager and OME readiness goals stay assigned to #1297/#1300/#1304.
  - Check:
    - Updated the deployment contract, upgrade/recovery runbook, advanced deployment and Ubuntu guidance, release/versioning policy, deployment asset map, and testing guide.
    - `scripts/generate-contract-doc.sh --check` passed with `BITRIVER_RELEASE_COMMIT` in the generated environment index.
    - `scripts/check-env-example-placeholders.sh` passed.
    - Documentation `git diff --check` passed; the pre-existing OME generated config remains untouched and line-ending-only.

- [x] Task 6 - Run full verification and prepare the issue for publication
  - Acceptance criteria:
    - Full repository verification, Compose rendering, quickstart smoke, and migration integration evidence pass or exact blockers are recorded.
    - Diff review confirms no credentials, generated runtime output, or unrelated deployment helpers are included.
    - `PLAN.md` and `TASKS.md` contain final evidence and remaining downstream work.
  - Check:
    - Pinned Go 1.26.5 passed all module package zones: `./cmd/... ./internal/... ./scripts/... ./web`; focused CLI, architecture, and script packages also passed after the final harness edits.
    - `./scripts/test-postgres-migrations.sh`, Docker Compose config rendering, contract snapshot generation (12 migrations), contract invariants, env placeholder hygiene, Helm generated-asset parity, shell syntax, and `git diff --check` passed.
    - `./scripts/test-quickstart.sh` built the production dependency graph and all first-party images, then reported Postgres, Redis, SRS, SRS controller, OME, transcoder, and API healthy; the migration job completed and API/viewer endpoints were reachable.
    - The quickstart harness restored `deploy/ome/Server.generated.xml` to the exact pre-run SHA-256 and removed all Compose containers/volumes.
    - The literal `./scripts/verify.sh` wrapper passed its first three checks then stopped at Go because host Go 1.25.6 cannot satisfy the repository's Go 1.26 local-toolchain contract; its remaining constituents passed with pinned Go 1.26.5 and host Docker.
    - Viewer sources were unchanged, so viewer lint/Jest were not required; the quickstart image build still completed the Next.js 16 production build and TypeScript/static generation.
    - Diff scope review excludes the user's OME line-ending change, deployment assurance/guide files, diagnostics/startup/validation helpers, and transcoder runtime data.

### Execution log
- Task 1 analysis:
  - Confirmed the current Compose and Helm jobs execute every SQL file on every run with no durable audit state.
  - Confirmed canonical ordering must use complete filenames because two historical migrations share version prefix `0002`.
  - Confirmed Helm generated SQL is stale at `0007` while canonical migrations continue through `0011`.
  - Selected raw SHA-256 checksums plus byte-for-byte generated SQL parity so Compose and Helm cannot record different history for the same migration.
  - Selected explicit `applying`, `applied`, and `failed` states with checksum-confirmed retry/mark-applied recovery for the commit-to-ledger interruption window.
- Task 2 implementation:
  - Added a POSIX runner suitable for the pinned Postgres Alpine image; it validates required tools and connection inputs without echoing credentials.
  - The runner records a claim before SQL execution, forces a single transaction, records success/failure afterward, and leaves interruption state visible rather than silently retrying.
  - Read-only plan/status modes do not create the ledger; repair requires the exact current and recorded checksum.
- Task 3 implementation:
  - Replaced Compose and Helm's inline replay loops with the same generated runner and bounded Postgres readiness wait.
  - Added `bitriver migrations` with safe plan default plus status, apply, retry, and mark-applied modes; quickstart now delegates to the same apply path.
  - Changed generated Helm SQL to exact canonical copies and regenerated the previously missing `0008` through `0011` migrations.
  - Upgrade planning now requires a read-only migration preflight before image changes and uses the focused CLI for application.
- Task 4 implementation:
  - Added an isolated real-Postgres lifecycle test and wired it into the Docker-enabled repository verification sequence.
  - Made the test portable across Linux and Git Bash by disabling MSYS path conversion only for container-side `docker exec` paths.
  - Replaced the runner's only `psql -c` variable-substitution call with stdin SQL after real PostgreSQL proved `-c` does not expand psql variables.
- Task 5 implementation:
  - Defined complete-filename identity, raw SHA-256 history, pending-only application, and migration failure as a deployment-health blocker in the canonical contract.
  - Added exact plan/status/retry/mark-applied commands with the validation required before either recovery action.
  - Established forward-only migrations, immutable applied files, and mandatory release notes plus restore evidence for destructive or rollback-incompatible schema changes.
  - Documented byte-identical Compose/Helm migration assets and the new real-Postgres verification gate without claiming the downstream Ubuntu installer or OME readiness work is complete.
- Task 6 verification and hardening:
  - Tightened the architecture import test and check script to their actual Go package zones so large untracked deployment runtime data cannot time out an unrelated source contract.
  - Made quickstart smoke render OME through the canonical Compose helper image instead of requiring a host Go toolchain, then added byte-for-byte backup/restore of the tracked generated OME config to prevent local credentials or smoke values from leaking into a diff.
  - A full clean Compose smoke proved the new ledger job against all 12 canonical migrations; the second dependency evaluation completed as a no-op and did not block API health.
