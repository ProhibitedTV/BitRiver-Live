# Deployment assets

This directory contains everything used to start BitRiver Live locally (Docker Compose), via systemd units, and in production-style installs.

## Layout
- `docker-compose.yml` – Compose stack that powers `./scripts/quickstart.sh` and local development. It expects the repository root `.env` file.
- `.env.example` – Template of required environment variables. Copy to `.env` at the repo root and adjust values before running Compose or systemd units.
- `check-env.sh` – Fails fast when required variables are missing or malformed; used by quickstart and manual Compose runs.
- `ome/Server.xml` – Source OvenMediaEngine config template. `./scripts/quickstart.sh` renders it into `ome/Server.generated.xml`; edit the template, not the generated file.
- `srs/` – Stock SRS configuration used by the Compose stack and systemd unit.
- `migrations/` – Canonical SQL migrations for the API database.
- `install/` – Interactive installer and automation helpers for systemd deployments (see below).
- `systemd/` – Unit files for running the services outside of Docker; see `systemd/README.md` for installation steps.

## Docker Compose
The Compose stack provides a one-command bootstrap for development and demos:

```bash
./scripts/quickstart.sh
```

If you invoke Compose directly, set the Compose file path and ensure `.env` is populated:

```bash
export COMPOSE_FILE=deploy/docker-compose.yml
./deploy/check-env.sh
docker compose up --build
```

## Systemd installs
For bare-metal or VM installs, start with the helpers in `deploy/install/`:

- `install/wizard.sh` collects settings interactively and calls `install/ubuntu.sh`.
- `install/ubuntu.sh` provisions users/directories and installs binaries, configs, and the systemd units under `deploy/systemd/`.

After installation, edit the environment overrides in the unit files (image tags, ports, mount paths), then reload systemd and start the services. See `deploy/systemd/README.md` for a step-by-step walkthrough.
