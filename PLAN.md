# PLAN

## Scope
Improve `cmd/bitriver quickstart` preflight checks before deployment starts, without changing deployment pipeline behavior.

## Assumptions
- Existing quickstart stage order remains intact; we only add output/preflight validation before migrations/compose-up.
- Host-port checks should use env-driven host ports used by default quickstart services.
- Failures should be actionable with one-line next steps.

## Implementation approach
1. Add a quickstart deployment-preflight stage in `cmd/bitriver/commands_env_compose.go` after env validation and before deployment actions.
2. Implement checks for:
   - Docker binary exists and daemon responds (`docker version`)
   - Docker Compose v2 command responds (`docker compose version`)
   - Required host ports from env are free (TCP/UDP as needed), with clear conflict list.
3. Keep errors output-only/actionable and return through existing `quickstartStageFailure` flow.
4. Add focused unit tests in `cmd/bitriver/main_test.go` for preflight failures and conflict reporting.

## Risks
- Port-check logic may flag ports already reserved by an existing local stack; message must clearly direct remediation.
- Env-derived port fallback logic must match compose defaults to avoid false positives/negatives.

## Test plan
- Run targeted quickstart tests in `cmd/bitriver` covering added preflight behavior.
- Run full repo verification script before finalizing, if environment supports it.
