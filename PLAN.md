# PLAN

## Scope (current change)
- Reduce comment noise in `internal/storage/storage.go` by removing template/generic comments that only restate function names.
- Keep immediate function comments where repository comment checks require coverage, but rewrite them to concise behavior-specific descriptions.
- Limit this change to comments only (no runtime logic changes).

## Assumptions
- CI/go comment coverage checks require an immediate comment before each function but do not require long template blocks.
- Reworded comments that are concise and accurate will satisfy repository policy.
- No deployment contract files are impacted.

## Risks
- Removing too much could break strict function comment coverage checks.
- Editing large comment blocks may accidentally touch logic lines if done carelessly.

## Test plan
- `python3 scripts/check-function-comments.py --strict-unexported`
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/storage -count=1 -timeout=120s`
