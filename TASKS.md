# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add upgrade planning CLI command
  - Acceptance criteria:
    - `bitriver upgrade-plan --to <tag>` reads current deployed version hints from `.env` image tags.
    - Command validates supported upgrade hops (N-1 minors; no major skipping) and prints actionable steps.
    - Command emits explicit warnings for major upgrades/breaking changes and optional schema checks.

- [x] Task 2 — Upgrade contract documentation
  - Acceptance criteria:
    - `docs/upgrades.md` includes Supported upgrade paths, Backup and restore checklist, and Roll back safety/unsafe guidance.
    - Doc includes a single copy-paste command sequence that operators can run.

- [x] Task 3 — Versioning + release artifact consistency
  - Acceptance criteria:
    - New dedicated versioning section/doc defines SemVer policy and breaking-change criteria.
    - Release process/docs include required upgrade notes and breaking-change callouts.
    - A reusable template exists under `.github/` for release notes consistency.

## Execution log
- ✅ Task 1 complete: added `cmd/bitriver/upgrade_plan.go`, wired `upgrade-plan` in `cmd/bitriver/main.go`, and added tests in `cmd/bitriver/upgrade_plan_test.go`.
- ✅ Task 1 checks:
  - `go test ./cmd/bitriver -count=1`
  - `go run ./cmd/bitriver upgrade-plan --current v1.2.3 --to v1.3.0 --check-schema --current-schema 0010 --expected-schema 0010`

- ✅ Task 2 complete: rewrote `docs/upgrades.md` with explicit supported hops, backup/restore checklist, migration guarantees, single command sequence, and rollback safety boundaries.
- ✅ Task 2 check: static review of rendered markdown sections/command sequence.

- ✅ Task 3 complete: added `docs/versioning.md`, updated `docs/production-release.md` with release-notes consistency gate, and added `.github/RELEASE_NOTES_TEMPLATE.md`.
- ✅ Task 3 checks:
  - static review of SemVer/breaking-change policy wording
  - `./scripts/verify.sh`
