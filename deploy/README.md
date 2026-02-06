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

The OME service in `deploy/docker-compose.yml` uses a `curl`-based healthcheck against the control API inside the container
(`http://localhost:${BITRIVER_OME_HTTP_PORT:-8081}/v1/health`, fallback `.../healthz`).

Header/credential fallback sequence is exact and ordered:

1. Resolve canonical probe token in this order: `${BITRIVER_OME_HEALTHCHECK_TOKEN:-${BITRIVER_OME_ACCESS_TOKEN:-$BITRIVER_OME_API_TOKEN}}`.
2. Try `AccessToken: <token>`.
3. Then try HTTP basic auth when both `BITRIVER_OME_USERNAME`/`BITRIVER_OME_PASSWORD` are non-empty.
4. Then try `Authorization: Bearer <token>`.
5. Mark container unhealthy with an explicit attempted-auth summary if all probes fail.

Expected 401 signatures during auth mismatches:

- `AccessToken` header missing or empty token: `401` with `Authorization header is required`.
- Legacy Bearer fallback mismatch (when enabled): `401` with `Authorization header is required` or `401 Unauthorized`, depending on OME version.
- Token present but does not match `<Managers><API><AccessToken>` in `ome/Server.generated.xml`: `401 Unauthorized` (often still logged with auth-required messaging, depending on OME version).
- Basic auth fallback credentials mismatch: `401 Unauthorized` from the control endpoint.

If you see `Authorization header is required`, treat it as a header/token drift issue first:

1. Confirm `.env` token values (`BITRIVER_OME_API_TOKEN`, optional `BITRIVER_OME_ACCESS_TOKEN`, optional `BITRIVER_OME_HEALTHCHECK_TOKEN`) and keep them identical when multiple are set.
2. Confirm `deploy/ome/Server.generated.xml` has the same `<Managers><API><AccessToken>` value.
3. Confirm `deploy/docker-compose.yml` has the expected `ome` environment injection and healthcheck mode order.
4. Re-render and restart: `./scripts/render-ome-config.sh --force && docker compose up -d ome`.

Copy/paste manual probe block (matches the in-container healthcheck logic):

```bash
docker compose exec ome sh -lc '
  set -eu
  health_url="http://localhost:${BITRIVER_OME_HTTP_PORT:-8081}/v1/health"
  healthz_url="http://localhost:${BITRIVER_OME_HTTP_PORT:-8081}/healthz"

  token="${BITRIVER_OME_HEALTHCHECK_TOKEN:-}"
  if [ -z "$token" ] && [ -n "${BITRIVER_OME_ACCESS_TOKEN:-}" ]; then
    token="$BITRIVER_OME_ACCESS_TOKEN"
  fi
  if [ -z "$token" ] && [ -n "${BITRIVER_OME_API_TOKEN:-}" ]; then
    token="$BITRIVER_OME_API_TOKEN"
  fi

  if [ -z "$token" ]; then
    echo "missing token: set BITRIVER_OME_ACCESS_TOKEN or BITRIVER_OME_API_TOKEN" >&2
    exit 1
  fi

  probe_with_args() {
    curl -fsS --connect-timeout 2 --max-time 4 "$@" "$health_url" || \
      curl -fsS --connect-timeout 2 --max-time 4 "$@" "$healthz_url"
  }

  probe_with_args -H "AccessToken: $token" && exit 0
  if [ -n "${BITRIVER_OME_USERNAME:-}" ] && [ -n "${BITRIVER_OME_PASSWORD:-}" ]; then
    probe_with_args -u "$BITRIVER_OME_USERNAME:$BITRIVER_OME_PASSWORD" && exit 0
  fi
  probe_with_args -H "Authorization: Bearer $token" && exit 0

  exit 1
'
```

`BITRIVER_OME_HTTP_PORT`/`BITRIVER_OME_HTTP_TLS_PORT` control `<Bind><Managers><API><Port>/<TLSPort>` in the rendered OME config (and the in-container health target), while `BITRIVER_OME_SERVER_PORT`/`BITRIVER_OME_SERVER_TLS_PORT` remain dedicated to WebRTC signalling listeners.

Example precedence resolution:

- `BITRIVER_OME_API_TOKEN=api-prod-token`
- `BITRIVER_OME_ACCESS_TOKEN=api-prod-token`
- `BITRIVER_OME_HEALTHCHECK_TOKEN=` (unset)

Result: render + `ome-health-token-check` + OME container startup + OME healthcheck all use `api-prod-token`.

The canonical OME auth element is top-level `<Managers><API><AccessToken>` in the rendered `Server.xml`; the quickstart renderer rejects deprecated `<AccessTokens>` wrappers. The renderer also enforces direct `<Application><OutputProfiles>` blocks and rejects deprecated `<Application><Outputs>` wrappers.

## Systemd installs
For bare-metal or VM installs, start with the helpers in `deploy/install/`:

- `install/wizard.sh` collects settings interactively and calls `install/ubuntu.sh`.
- `install/ubuntu.sh` provisions users/directories and installs binaries, configs, and the systemd units under `deploy/systemd/`.

After installation, edit the environment overrides in the unit files (image tags, ports, mount paths), then reload systemd and start the services. See `deploy/systemd/README.md` for a step-by-step walkthrough.
