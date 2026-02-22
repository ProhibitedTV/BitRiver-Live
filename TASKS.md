# TASKS

Status legend: `[ ]` not started, `[-]` in progress, `[x]` done

- [x] Task 1 — Identify and rewrite high-noise function comments in `internal/storage/storage.go`
  - Acceptance criteria:
    - Generic template comment blocks are removed or collapsed.
    - Immediate comments remain for functions in this file, written as concise behavior-specific descriptions.
    - No non-comment logic changes are introduced.
  - Relevant checks:
    - ⚠️ `python3 scripts/check-function-comments.py --strict-unexported` (repository-wide baseline currently fails outside this change scope)
    - ✅ `python3 scripts/check-function-comments.py --strict-unexported --git-base HEAD~1`
  - Result:
    - Replaced verbose template comments with concise, behavior-specific comments for targeted functions in `internal/storage/storage.go`.

- [x] Task 2 — Validate storage package behavior remains intact after comment-only edits
  - Acceptance criteria:
    - `internal/storage` tests pass.
  - Relevant checks:
    - ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/storage -count=1 -timeout=120s`
  - Result:
    - Storage package tests pass with comment-only changes.

## Execution log
- ⚠️ `python3 scripts/check-function-comments.py --strict-unexported` (fails due to pre-existing repository-wide comment coverage deficits unrelated to this edit).
- ✅ `python3 scripts/check-function-comments.py --strict-unexported --git-base HEAD~1`.
- ✅ `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/storage -count=1 -timeout=120s`.
