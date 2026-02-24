# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Update local viewer change detection fallback order in `scripts/verify.sh`
  - Acceptance criteria:
    - CI SHA-based detection path remains unchanged in behavior.
    - Local mode checks committed `web/viewer` changes via merge-base (`origin/main` if available, else `HEAD~1`) before `git status`.
    - `--viewer` and `--ci-viewer` behavior remains unchanged.
  - Relevant checks:
    - `bash scripts/test-verify-viewer-detection.sh`
  - Result:
    - Passed.

- [x] Task 2 — Add/update deterministic script-level test coverage
  - Acceptance criteria:
    - Test validates committed local `web/viewer` changes are detected without CI SHA env vars.
    - Existing CI diff and local uncommitted fallback coverage continue to pass.
  - Relevant checks:
    - `bash scripts/test-verify-viewer-detection.sh`
  - Result:
    - Passed.

- [x] Task 3 — Run integration verification gate
  - Acceptance criteria:
    - `./scripts/verify.sh` completes successfully for a targeted Go package run.
    - Record command outcomes in execution log.
  - Relevant checks:
    - `./scripts/verify.sh --go-packages ./cmd/bitriver`
  - Result:
    - Passed.

## Execution log

- ✅ `bash scripts/test-verify-viewer-detection.sh`
- ✅ `bash scripts/test-verify-viewer-detection.sh`
- ✅ `./scripts/verify.sh --go-packages ./cmd/bitriver`
