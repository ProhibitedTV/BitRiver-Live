# Deployment assets

This directory contains everything used to start BitRiver Live locally (Docker Compose), via systemd units, and in production-style installs.

## Layout
- `docker-compose.yml` – Compose stack that powers `./scripts/quickstart.sh` and local development. It expects the repository root `.env` file.
- `.env.example` – Template of required environment variables. Copy to `.env` at the repo root and adjust values before running Compose or systemd units.
- `check-env.sh` – Fails fast when required variables are missing or malformed; used by quickstart and manual Compose runs.
- `ome/Server.xml` – Source OvenMediaEngine config template. `./scripts/quickstart.sh` renders it into `ome/Server.generated.xml`; edit the template, not the generated file.
- `srs/` – Stock SRS configuration template plus the generated file rendered from `.env` for Compose/systemd.
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

Compose always re-renders `ome/Server.generated.xml` via the `ome-config` helper before starting OvenMediaEngine. Update `.env`
with your OME credentials first—`ome-test-*` defaults are rejected and will cause the render step to fail.
Compose also renders `srs/conf/srs.generated.conf` via the `srs-config` helper, replacing `${BITRIVER_SRS_TOKEN}` from `.env`
before starting SRS so the ingest hooks always share the same token as the API.

Viewer self-registration is disabled by default so only administrators can add users. Toggle `BITRIVER_LIVE_ALLOW_SELF_SIGNUP`
in `.env` and rerun `./deploy/check-env.sh` followed by `docker compose up -d` to reopen or close public signups.

### OME healthcheck

The OME service in `deploy/docker-compose.yml` uses a `curl`-based healthcheck that hits the control API inside the container
(`http://localhost:8081/v1/health` with a fallback to `/healthz`), optionally adding the `AccessToken` header and basic auth
based on `BITRIVER_OME_ACCESS_TOKEN` (falling back to `BITRIVER_OME_API_TOKEN` when the access token is unset) and
`BITRIVER_OME_USERNAME`/`BITRIVER_OME_PASSWORD`. To run the same probe manually,
execute it inside the container so it reuses the environment variables already injected by Compose:

```bash
docker compose exec ome sh -c 'curl -fsS http://localhost:8081/v1/health || curl -fsS http://localhost:8081/healthz'
docker compose exec ome sh -c 'curl -fsS -H "AccessToken: $BITRIVER_OME_ACCESS_TOKEN" -u "$BITRIVER_OME_USERNAME:$BITRIVER_OME_PASSWORD" http://localhost:8081/v1/health'
```

If either command returns 401, re-check the credentials rendered into `ome/Server.generated.xml` and the values in `.env`.

## Systemd installs
For bare-metal or VM installs, start with the helpers in `deploy/install/`:

- `install/wizard.sh` collects settings interactively and calls `install/ubuntu.sh`.
- `install/ubuntu.sh` provisions users/directories and installs binaries, configs, and the systemd units under `deploy/systemd/`.

After installation, edit the environment overrides in the unit files (image tags, ports, mount paths), then reload systemd and start the services. See `deploy/systemd/README.md` for a step-by-step walkthrough.
