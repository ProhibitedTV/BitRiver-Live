# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Align OME bind-tag requirements in `scripts/test-quickstart.sh`
  - Acceptance criteria:
    - Keep guard rejecting legacy root `<Bind><IP>` / `<Bind><Address>` tags.
    - Remove `RootBind` requirement tied to the forbidden legacy path.
    - Add explicit canonical root `<IP>` validation against `BITRIVER_OME_BIND`.
  - Relevant checks:
    - `./scripts/test-quickstart.sh`
    - `bash -n scripts/test-quickstart.sh`
  - Result:
    - Logic update complete; smoke script blocked by environment (`docker` unavailable), shell syntax check passed.

## Execution log

- ⚠️ `./scripts/test-quickstart.sh` (failed: `docker` is not installed in this environment)
- ✅ `bash -n scripts/test-quickstart.sh`
