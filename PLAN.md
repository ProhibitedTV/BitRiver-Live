# PLAN

## Scope (current change)
- Identify and remove duplicated `StartStream` ingest boot logic between `internal/storage/storage.go` and `internal/storage/postgres_channels.go`.
- Extract shared helpers in `internal/storage` for:
  - ingest boot retry execution (`BootStream` attempts, timeout, retry interval), and
  - mapping ingest boot output into stream-session fields (URLs, job IDs, endpoints, rendition manifests).
- Keep datastore-specific persistence and rollback writes in place (JSON in-memory/persist flow vs Postgres SQL flow).
- Expand tests to lock behavior for default attempts (`<=0` => `1`), retry interval waiting, and rollback to `offline` + cleared `current_session_id` on boot failure.

## Assumptions
- Existing `StartStream` error strings (notably `"boot ingest: %w"`) must remain unchanged.
- Fallback/rollback semantics differ by backend only in persistence mechanics; logical outcomes must remain equivalent.
- Timing assertions for retry interval should use tolerant bounds to avoid flaky CI failures.

## Risks
- Over-centralizing could accidentally move persistence-side effects that should remain backend-specific.
- Retry helper changes could alter call counts or timeout behavior if parameters are mishandled.
- Time-based tests can be flaky if bounds are too tight.

## Test plan
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/storage -run 'Test(StartStreamRetriesBootFailures|StartStreamFailureRollsBackState|StartStreamDefaultAttemptsToOne|StartStreamAppliesRetryInterval)' -count=1 -timeout=120s`
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/storage -run 'TestRepositoryScenarios/(JSONStore|Postgres)/StreamLifecycleWithoutIngest|TestRepositoryScenarios/(JSONStore|Postgres)/StreamTimeouts' -count=1 -timeout=120s`
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/storage -count=1 -timeout=120s`
