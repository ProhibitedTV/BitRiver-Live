# BitRiver Live – Repository Guidance

## Architecture overview
- **Stack:** Go 1.21 backend (see `go.mod`) plus a Next.js 13 viewer under `web/viewer`. Commands under `cmd/` (API server, transcoder, RTMP controller, tools) share packages under `internal/`, rely on vendored helpers in `third_party/`, and bundle static assets from `web/static`. Docker, Compose, and scripts in `deploy/` and `scripts/` orchestrate the complete stack described in the README.
- **Service story:** The README promises a one-command (`./scripts/quickstart.sh`) deployment that boots `cmd/server`, the proxied viewer, RTMP ingest, the transcoder, chat, Postgres, and Redis together. Keep these flows working so contributors can keep following the documented quickstart and architecture guarantees.

## Toolchain + dependency policy
- Go code targets Go 1.21. Always set `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off` so builds never touch the network, and run `gofmt` plus `go mod tidy` before committing. When editing vendored replacements under `third_party/`, keep them in sync with `go.mod` and avoid external fetches.
- Canonical Go test command (from `docs/testing.md`):
  ```bash
  GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./... -count=1 -timeout=120s
  ```
- Web viewer tooling (from `web/viewer/package.json`): `npm run lint`, `npm run test`, and `npm run test:playwright`. Install deps via `npm install` before running.

## Definition of done
1. **Tests:** Update or add tests beside behavioural changes. Follow `docs/testing.md` for unit suites, the Postgres-tagged storage suite, and helper scripts such as `scripts/test-postgres.sh`.
2. **Docs:** Refresh `docs/` and `README.md` whenever behaviour, commands, or architecture promises change. Document new operational steps in the relevant guide.
3. **Automation:** Keep `scripts/quickstart.sh`, `scripts/test-postgres.sh`, and similar helpers passing. Never land a change that leaves integration scripts or Docker Compose broken.
4. **Manual QA:** When visual or escaping-sensitive flows change, run the checklists in `web/manual-qa.md`. For deployment workflows, exercise `scripts/quickstart.sh` when touching Docker Compose, migrations, or env plumbing.

### Manual QA / deployment table
| When to run | Reference |
| --- | --- |
| Control centre escaping or viewer UX changes | `web/manual-qa.md` |
| Deploy/Compose/script alterations | `scripts/quickstart.sh`, `docs/quickstart.md`, `docs/advanced-deployments.md` |

## Breadcrumbs
- Subdirectories add their own `AGENTS.md` for package-specific rules. Always read the guide nearest to the files you edit.
- Relevant docs: `docs/testing.md`, `docs/quickstart.md`, `docs/advanced-deployments.md`, `docs/production-release.md`.

## Before opening a PR
- `gofmt` + `go mod tidy` (Go) / `npm run lint` (web).
- `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./... -count=1 -timeout=120s`.
- Extra suites from `docs/testing.md` (Postgres, viewer tests, Playwright) as applicable.
- Update docs/README and rerun manual QA or `scripts/quickstart.sh` when relevant.
