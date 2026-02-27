# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Update workflow setup-go validation logic
  - Acceptance criteria:
    - `scripts/check-go-workflow-config.sh` accepts direct SHA-pinned `actions/setup-go@<40-hex>` references.
    - Script accepts `./.github/actions/setup-go` usage only when `.github/actions/setup-go/action.yml` pins `actions/setup-go` with a 40-hex SHA.
    - Existing checks for `go-version-file: go.mod`, `GOTOOLCHAIN=local`, `GOPROXY=off`, and `GOSUMDB=off` remain enforced.

- [x] Task 2 — Update testing docs note for SHA-based enforcement
  - Acceptance criteria:
    - `docs/testing.md` no longer states literal `actions/setup-go@v5` enforcement.
    - Note reflects SHA-based direct pinning or approved local composite action behavior.

- [x] Task 3 — Run validation checks and record outcomes
  - Acceptance criteria:
    - Relevant command(s) executed after changes.
    - Results captured in execution log with pass/fail status.

## Execution log
- ✅ `bash scripts/check-go-workflow-config.sh` (pass).
- ✅ `./scripts/verify.sh` (pass; docker-related steps were skipped by script design because docker is unavailable).
