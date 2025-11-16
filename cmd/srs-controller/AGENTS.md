# `cmd/srs-controller` Guidance

This binary proxies SRS APIs so the Go server can manage channels securely. Read the root `AGENTS.md` first.

## Contract
- Routes under `/v1/channels` must remain stable. Keep existing methods, payloads, and response codes intact when adding functionality.
- All proxied requests copy whitelisted headers to SRS. Preserve header forwarding rules so auth/session data stays consistent.
- Enforce strict bearer auth with constant-time token comparison before sending traffic to SRS.
- Shutdown must close HTTP servers promptly and respect context cancellation (mirrors `cmd/server`).

## Before opening a PR
- Validate token handling with unit tests.
- Confirm the proxy still starts/stops cleanly (Ctrl+C locally).
- Update deployment docs/scripts if env vars, ports, or routes change.
- Run targeted tests (`go test ./cmd/srs-controller -count=1`) plus the repo-wide suite.
