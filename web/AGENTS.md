# `web/` Guidance

This directory holds both the embedded control-centre assets (`web/static`) and the standalone Next.js viewer (`web/viewer`). Read the root `AGENTS.md` before editing.

## Layers
- `web/static` assets are bundled into the Go binary via `web/embed.go`. After changing static files, run the documented copy/generate pipeline (see comments in `web/embed.go`) so `go generate` picks up the updates.
- `web/viewer` is a Next.js 13 app that can be proxied through `cmd/server` or hosted separately. Lint/test via `npm run lint`, `npm run test`, and `npm run test:playwright` from the `web/viewer` directory.
- Link protocol changes with `internal/chat` and API payloads. When message shapes or REST responses change, update the viewer client in `web/viewer/lib/viewer-api.ts` and the static assets if they embed related code.

## Manual QA
- Visual or escaping-sensitive edits must follow the checklist in `web/manual-qa.md`.

## Before opening a PR
- Rerun `go generate ./web` (or the noted copy step) after editing `web/static`.
- From `web/viewer`: `npm install` (once) then `npm run lint`, `npm run test`, and `npm run test:playwright` for UX-impacting changes.
- Take/update screenshots when modifying visible UI and attach them to the PR if required.
