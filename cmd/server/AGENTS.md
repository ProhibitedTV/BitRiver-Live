# `cmd/server` Guidance

This directory contains the main API/control-centre binary. Read the root-level `AGENTS.md` for global rules before editing.

## Entry point expectations
- `main.go` wires more than 70 CLI flags into `internal/server.New`. Add new flags near related ones, update the usage string, and thread values through `server.Config` (including nested structs) so downstream components receive consistent settings.
- Most flags have an environment-variable twin (see `envconfig` helpers). Keep that parity for secrets (OAuth, session signing, storage DSNs) so Docker/Compose deployments stay configurable without recompiling.
- Startup config includes OAuth providers, rate limiting, chat dependencies, storage selection, and queue wiring. Maintain the sequence: parse flags → load env → validate → call `server.New` → start background health checks.
- Graceful shutdown relies on signal handling (SIGINT/SIGTERM), context cancellation, draining HTTP servers, and stopping background workers. Preserve this flow when adding goroutines or servers.

## Health + API plumbing
- The server feeds chat and rate-limiter health checks into `internal/api.Handler`. When adding new dependencies, expose health functions via `server.Config` and register them with the handler.
- Static assets come from `web.Static()` plus the optional viewer proxy; ensure new routes honour the proxy/embedding toggles.

## Before opening a PR
- Keep new flags grouped with their peers and documented in the README/docs.
- Ensure env + flag parity for secrets/config.
- `gofmt` and `go test ./cmd/server -count=1` in addition to repo-wide tests.
- Exercise graceful shutdown paths locally (Ctrl+C) when adding long-running routines.
