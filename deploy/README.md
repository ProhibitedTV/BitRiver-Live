# Deployment assets

This directory contains everything used to start BitRiver Live locally (Docker Compose), via systemd units, and in production-style installs.

## Layout
- `docker-compose.yml` – Compose stack that powers `./scripts/quickstart.sh` and local development. It expects the repository root `.env` file.
- `.env.example` – Template of required environment variables. Copy to `.env` at the repo root and adjust values before running Compose or systemd units.
- `check-env.sh` – Fails fast when required variables are missing or malformed; used by quickstart and manual Compose runs.
- `ome/Server.xml` – Source OvenMediaEngine config template. `./scripts/quickstart.sh` renders it into `ome/Server.generated.xml`; edit the template, not the generated file.
- `srs/` – Stock SRS configuration template plus the generated file rendered from `.env` for Compose/systemd.
- `migrations/` – Canonical SQL migrations for the API database.
- `helm/bitriver-live/files/srs.conf` and `helm/bitriver-live/migrations/*.sql` – Generated Helm copies synced from canonical `deploy/srs/conf/srs.conf` and `deploy/migrations/` via `./scripts/sync-helm-deploy-assets.sh` (do not edit generated files directly).
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
with your OME credentials first—`ome-test-*` defaults are rejected and will cause the render step to fail. The `ome-config`
container runtime is now `scratch`, so the helper image only contains the statically linked `/usr/local/bin/bitriver`
entrypoint (no shell or Debian userland packages).
Compose also renders `srs/conf/srs.generated.conf` via the `srs-config` helper, replacing `${BITRIVER_SRS_TOKEN}` from `.env`
before starting SRS so the ingest hooks always share the same token as the API.
The `srs-config` helper is invoked via `bash` and sanitized into `/workspace/.tmp/` to avoid Windows CRLF issues while
preserving repo-relative path resolution used by the script; keep shell scripts checked out with LF line endings
(`.gitattributes` enforces this for `*.sh` files).

Viewer self-registration is disabled by default so only administrators can add users. Toggle `BITRIVER_LIVE_ALLOW_SELF_SIGNUP`
in `.env` and rerun `./deploy/check-env.sh` followed by `docker compose up -d` to reopen or close public signups.

## Syncing canonical deploy assets into Helm
Helm keeps generated copies of selected deploy artifacts so charts can be packaged self-contained, but the authoritative sources live outside the chart:

- `deploy/srs/conf/srs.conf`
- `deploy/migrations/*.sql`

To refresh generated Helm copies after editing canonical files, run:

```bash
./scripts/sync-helm-deploy-assets.sh
```

To enforce drift detection in CI (or locally before commits), run:

```bash
./scripts/check-helm-deploy-assets-drift.sh
```

If drift is reported, re-run the sync command and commit the regenerated Helm files. Do not hand-edit `deploy/helm/bitriver-live/files/srs.conf` or `deploy/helm/bitriver-live/migrations/*.sql`.

### Image tags and digests

Compose reads all image tags from `.env` so you can update versions without editing `deploy/docker-compose.yml`. For
production deployments, pin images to digests to guarantee the exact bytes you tested:

```bash
BITRIVER_LIVE_IMAGE_TAG=v1.2.3
BITRIVER_LIVE_IMAGE_DIGEST=@sha256:...
BITRIVER_VIEWER_IMAGE_TAG=v1.2.3
BITRIVER_VIEWER_IMAGE_DIGEST=@sha256:...
```

Keep each digest paired with its matching tag (never mix a new tag with an old digest). When you need to override
third-party images, use the corresponding `*_IMAGE_DIGEST` fields in `deploy/.env.example` and rerun
`./deploy/check-env.sh` before restarting Compose.

### OME healthcheck

The OME service in `deploy/docker-compose.yml` and the Helm deployment now use an unauthenticated, stable liveness/readiness probe against the local root endpoint (`http://localhost:${BITRIVER_OME_HTTP_PORT:-8081}/` for Compose and `http://localhost:8081/` in Helm templates).

Probe sequence is intentionally simple to avoid auth drift breaking container health:

1. Call the local root endpoint without auth headers.
2. Treat any non-`000` status below `500` as healthy (so auth redirects/`401`/`404` from public endpoints do not cause restart loops).
3. Fail only on transport errors or server-side `5xx` responses.

OME control-plane credentials (`BITRIVER_OME_HEALTHCHECK_TOKEN`, `BITRIVER_OME_ACCESS_TOKEN`, `BITRIVER_OME_API_TOKEN`, optional basic credentials) are still required for API calls and config rendering, but they are no longer part of the liveness/readiness probe contract.
Re-render and restart OME after credential/config changes: `./scripts/render-ome-config.sh --force && docker compose up -d ome`.

Copy/paste manual probe block (matches the in-container healthcheck logic):

```bash
docker compose exec ome sh -lc '
  set -eu
  health_url="http://localhost:${BITRIVER_OME_HTTP_PORT:-8081}/"
  http_status="$(curl -sS -o /dev/null -w "%{http_code}" --connect-timeout 2 --max-time 4 "$health_url")"
  [ -n "$http_status" ] && [ "$http_status" != "000" ] && [ "$http_status" -lt 500 ]
'
```

`BITRIVER_OME_HTTP_PORT`/`BITRIVER_OME_HTTP_TLS_PORT` control `<Bind><Managers><API><Port>/<TLSPort>` in the rendered OME config (and the in-container health target), while `BITRIVER_OME_SERVER_PORT`/`BITRIVER_OME_SERVER_TLS_PORT` remain dedicated to WebRTC signalling listeners.

Example precedence resolution:

- `BITRIVER_OME_API_TOKEN=api-prod-token`
- `BITRIVER_OME_ACCESS_TOKEN=api-prod-token`
- `BITRIVER_OME_HEALTHCHECK_TOKEN=` (unset)

Result: render + `ome-health-token-check` + OME container startup all use `api-prod-token` for control-plane auth, while liveness/readiness stays on the unauthenticated root probe.

The canonical OME auth element is top-level `<Managers><API><AccessToken>` in the rendered `Server.xml`; the quickstart renderer rejects deprecated `<AccessTokens>` wrappers. The renderer also enforces direct `<Application><OutputProfiles>` blocks and rejects deprecated `<Application><Outputs>` wrappers.

## Systemd installs
For bare-metal or VM installs, start with the helpers in `deploy/install/`:

- `install/wizard.sh` collects settings interactively and calls `install/ubuntu.sh`.
- `install/ubuntu.sh` provisions users/directories and installs binaries, configs, and the systemd units under `deploy/systemd/`.

After installation, edit the environment overrides in the unit files (image tags, ports, mount paths), then reload systemd and start the services. See `deploy/systemd/README.md` for a step-by-step walkthrough.
