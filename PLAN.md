# PLAN

## Scope (current change)
- Add a second concurrency accessor test in `internal/api/handlers_concurrency_test.go` for preconfigured dependencies.
- Verify `sessionManager`, `mfaChallengeManager`, `logger`, `tracer`, and `srsTracker` preserve injected pointer identity under concurrent access.
- Prevent regressions where accessor lazy-init paths overwrite non-nil dependencies.

## Assumptions
- Existing accessor methods are intended to lazily initialize only when their backing field is `nil`.
- Pointer identity checks are the strongest assertion for "must not overwrite injected dependency" behavior.

## Risks
- Concurrent test assertions could become flaky if assertions rely on value semantics instead of direct pointer identity.
- Using globally shared defaults (e.g., `slog.Default()`, `tracing.Default()`) could mask overwrite behavior if not handled carefully.

## Test plan
- `go test ./internal/api -run 'TestHandlerAccessorsConcurrent' -count=1`
- `./scripts/verify.sh`
