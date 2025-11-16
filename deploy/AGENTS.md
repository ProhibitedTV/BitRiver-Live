# `deploy/` Guidance

Docker Compose files, migrations, and install scripts live here. Read the root `AGENTS.md` first.

## Expectations
- Compose files, systemd units, and SQL migrations underpin `scripts/quickstart.sh`. When ports, env vars, image names, or component lists change, update Compose, `.env` templates, and README/docs together so the copy/paste quickstart keeps working.
- SQL migrations define the canonical schema. Each change requires a new migration file plus documentation in `docs/production-release.md` or release notes.
- Keep comments describing exposed ports and service relationships; they are used by manual installers.

## Before opening a PR
- Run `scripts/quickstart.sh` (or a targeted docker compose up) after editing Compose/migrations to ensure the stack bootstraps.
- Update docs referencing affected services (`README.md`, `docs/quickstart.md`, `docs/advanced-deployments.md`).
- Coordinate with `internal/storage` and `cmd/tools` so migrations + importers stay in sync.
