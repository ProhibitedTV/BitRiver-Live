# PLAN

## Scope (current change)
- Extend `scripts/test-verify-viewer-detection.sh` with explicit outcome tests for `should_run_viewer_checks` CI logic.
- Cover CI viewer workflow enablement, CI non-viewer workflow skip/force behavior, and missing Node/npm skip behavior.
- Keep the existing source-only loading approach (`BITRIVER_VERIFY_SOURCE_ONLY=1`) and PASS/FAIL assertion style.

## Assumptions
- `should_run_viewer_checks` relies on `is_ci_environment`, `is_viewer_related_workflow`, and `viewer_checks_supported` and should be exercised through sourced `scripts/verify.sh`.
- Test determinism is best achieved by controlling `PATH` with temporary fake `node`/`npm` binaries instead of depending on host tools.

## Risks
- Inline shell snippets may become hard to read if environment setup is duplicated.
- PATH overrides could accidentally hide required shell tools if not scoped carefully.

## Test plan
- `bash scripts/test-verify-viewer-detection.sh`
