# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Add CI outcome assertions for `should_run_viewer_checks`
  - Acceptance criteria:
    - Add test coverage for CI + viewer-related workflow where checks are enabled when `web/viewer` exists and Node/npm are available.
    - Add CI non-viewer workflow coverage where checks are skipped by default and enabled with `force_ci_viewer_checks=true`.
  - Relevant checks:
    - `bash scripts/test-verify-viewer-detection.sh`

- [x] Task 2 — Add missing Node/npm CI outcome assertion
  - Acceptance criteria:
    - Add coverage showing `should_run_viewer_checks` returns skip when Node/npm support is unavailable, even for viewer-related CI workflows.
    - Preserve `BITRIVER_VERIFY_SOURCE_ONLY=1` sourcing and PASS/FAIL output style.
  - Relevant checks:
    - `bash scripts/test-verify-viewer-detection.sh`

## Execution log
- ✅ `bash scripts/test-verify-viewer-detection.sh`
- ✅ `./scripts/verify.sh` (viewer lint/test correctly skipped: no viewer changes in repo working tree)
