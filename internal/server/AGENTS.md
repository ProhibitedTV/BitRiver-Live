# `internal/server` Guidance

This package constructs the HTTP server (`server.New`) used by `cmd/server`. Read the root `AGENTS.md` for global rules.

## Router + middleware
- Build routers through `server.New`. Add new routes there rather than in `cmd/server`. Keep middleware order: auth → rate limiting → metrics → audit → logging. Adding middleware requires updating tests to confirm order is preserved.
- Register routes through the shared mux so instrumentation (Prometheus metrics, audit logging, tracing) stays consistent. New routes should include metrics labels and rate-limit buckets.

## Static assets + viewer proxy
- The control-centre SPA is embedded via `web.Static()`. When assets change, rerun the documented generate/copy pipeline so embedded files stay current.
- Viewer proxying is optional; respect the config flags/environment toggles when adding handlers so the viewer can run embedded or proxied.

## Shutdown + background jobs
- `server.New` manages health checks, background refresh loops, and queue watchers. Ensure new goroutines honour the context and stop on shutdown.

## Before opening a PR
- Update `server/config.go` (or equivalent structs) when introducing new settings.
- Extend tests under `internal/server` to cover middleware/routing changes.
- Run `go test ./internal/server -count=1` plus repo-wide tests.
