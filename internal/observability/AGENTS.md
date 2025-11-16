# `internal/observability` Guidance

Instrumentation helpers live here. Review the root `AGENTS.md` first.

## Expectations
- Reuse `metrics.Default()` and existing tracing/logging helpers. New packages should expose constructors rather than globals so other code can inject dependencies.
- Keep adapters thin: collect metrics, forward to exporters, and avoid importing application packages to prevent cycles.
- When adding metrics, document names/labels in `docs/` (for example `docs/observability.md` if added) and update dashboards/scripts that scrape them.

## Testing
- Add unit tests for any helper that transforms data. Use fakes to avoid relying on Prometheus exporters.

## Before opening a PR
- Ensure new metrics are referenced in `internal/server` middleware if needed.
- Run `go test ./internal/observability -count=1` plus repo suites.
