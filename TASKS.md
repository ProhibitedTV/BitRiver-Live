# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Pin `dorny/paths-filter` in CI workflow
  - Acceptance criteria:
    - `.github/workflows/ci.yml` uses a full 40-character SHA for `dorny/paths-filter`.
    - The `uses:` line includes a trailing release comment (e.g., `# v3.0.2`).

- [x] Task 2 — Audit and align all workflow `uses:` pins
  - Acceptance criteria:
    - All external `uses:` lines under `.github/workflows/` are pinned to 40-character SHAs.
    - Any updated entries include human-readable release comments matching repo style.

- [x] Task 3 — Validate workflow pinning and CI contract
  - Acceptance criteria:
    - A command-based scan shows no remaining non-SHA external `uses:` entries.
    - `./scripts/check-ci-contract.sh` passes.

## Execution log
- ✅ Task 1 complete: pinned `dorny/paths-filter` in `.github/workflows/ci.yml` to `de90cc6fb38fc0963ad72b210f1f284cd68cea36` with `# v3.0.2` comment.
- ✅ Task 1 check: `rg -n "dorny/paths-filter" .github/workflows/ci.yml`.
- ✅ Task 2 complete: audited `.github/workflows/*.yml`; no other external non-SHA `uses:` entries remained.
- ✅ Task 2 check: Python scan for non-SHA external `uses:` refs returned `All external uses entries are SHA-pinned.`.
- ✅ Task 3 complete: CI contract validation passed.
- ✅ Task 3 check: `./scripts/check-ci-contract.sh`.
