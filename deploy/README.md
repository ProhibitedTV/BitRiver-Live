# Deployment assets

This directory contains everything used to start BitRiver Live locally (Docker Compose), via systemd units, and in production-style installs.

## Layout
- `docker-compose.yml` – Compose stack that powers `./scripts/quickstart.sh` and local development. It expects the repository root `.env` file.
- `.env.example` – Template of required environment variables. Copy to `.env` at the repo root and adjust values before running Compose or systemd units.
- `check-env.sh` – Fails fast when required variables are missing or malformed; used by quickstart and manual Compose runs.
- `ome/Server.xml` – Source OvenMediaEngine config template. `./scripts/quickstart.sh` renders it into `ome/Server.generated.xml`; edit the template, not the generated file.
- `srs/` – Stock SRS configuration template plus the generated file rendered from `.env` for Compose/systemd.
- `migrations/` – Canonical SQL migrations for the API database.
- `postgres-migrate.sh` – Canonical ledger-aware migration runner used directly by Compose.
- `helm/bitriver-live/files/postgres-migrate.sh`, `helm/bitriver-live/files/backup-postgres.sh`, `helm/bitriver-live/files/srs.conf`, and `helm/bitriver-live/migrations/*.sql` – Generated Helm copies synced from canonical deploy assets via `./scripts/sync-helm-deploy-assets.sh` (do not edit generated files directly).
- `install/` – Interactive installer and automation helpers for systemd deployments (see below).
- `systemd/` – Unit files for running the services outside of Docker; see `systemd/README.md` for installation steps.

## Docker Compose
The Compose stack provides a one-command bootstrap for development and demos:

```bash
./scripts/quickstart.sh
```

PowerShell:

```powershell
.\scripts\quickstart.ps1
```

Both wrappers are thin shims around `go run ./cmd/bitriver quickstart`, so source checkouts require Go 1.26+
available on `PATH` (they do not provide a pure-Docker startup path with no local Go toolchain). On Windows, use
PowerShell or Git Bash for repository scripts; a broken WSL `bash` can fail before the quickstart reaches Docker or
BitRiver validation.

If you're operating from an installed package rather than a source checkout, use the installed `bitriver-live` launcher flow in
[`docs/quickstart.md`](../docs/quickstart.md) instead of invoking the repository wrapper script directly.

If you invoke Compose directly from a source checkout, set the Compose file path, switch the saved root `.env` to local build mode (`BITRIVER_LIVE_MODE=development` and `BITRIVER_DEPLOY_IMAGE_SOURCE=build`), and ensure `.env` is populated:

```bash
export COMPOSE_FILE=deploy/docker-compose.yml
./deploy/check-env.sh
docker compose --env-file .env -f "$COMPOSE_FILE" up -d --build --pull never
```

Published package/release installs should stay on the pull-mode launcher flow described in [`docs/quickstart.md`](../docs/quickstart.md); the direct Compose command above is the supported local rehearsal path when you need to build first-party images from this checkout.

Compose always re-renders `ome/Server.generated.xml` via the `ome-config` helper before starting OvenMediaEngine. Update `.env`
with your OME credentials first—`ome-test-*` defaults are rejected and will cause the render step to fail. The `ome-config`
container runtime is now `scratch`, so the helper image only contains the statically linked `/usr/local/bin/bitriver`
entrypoint (no shell or Debian userland packages).
Compose also renders `srs/conf/srs.generated.conf` via the `srs-config` helper, replacing `${BITRIVER_SRS_TOKEN}` from `.env`
before starting SRS so the ingest hooks always share the same token as the API.
The `srs-config` helper is invoked via `bash` and sanitized into container tmpfs at `/tmp/` to avoid Windows CRLF issues while
preserving repo-relative path resolution used by the script; keep shell scripts checked out with LF line endings
(`.gitattributes` enforces this for `*.sh` files).

`BITRIVER_CONFIG_ROOT` is mounted at `/etc/bitriver-live` only in the SRS/OME
renderers and OME token verifier. Source checkouts use `..` (the repository
root). The packaged installer persists its absolute config directory in
`bitriver.env`, and the Compose unit/launchers supply the same value, allowing
the installed absolute `.env` and generated-config symlinks to resolve inside
those containers while keeping durable secrets out of `/opt/bitriver-live`.
Installer upgrades append the key for older env files, replace the single
managed value, and refuse ambiguous duplicates while preserving every other
operator setting.

On managed Linux installs, `BITRIVER_HOST_UID` and `BITRIVER_HOST_GID` are
persisted beside the config root and supplied by systemd. They select the
non-root owner only for services that write the operator's config/data bind
mounts; image-specific users remain the fallback elsewhere. Unix source and
archive launch wrappers derive the current numeric IDs when the variables are
empty. All three config helpers mount `/workspace` read-only, render/verify the
generated files through `/etc/bitriver-live/deploy`, and keep the SRS temporary
script on tmpfs. Only the two renderer config-root mounts are writable. Do not
solve permission failures by making `bitriver.env` world readable or granting
renderer containers new capabilities.

Managed Ubuntu installs persist those outputs at
`/etc/bitriver-live/deploy/ome/Server.generated.xml` and
`/etc/bitriver-live/deploy/srs/conf/srs.generated.conf`. The installer migrates
the former flat paths without changing their bytes and rejects divergent dual
copies before it stages new program assets.
The generated files are operator-owned mode `0640`; OME and SRS receive only
that operator GID as a supplementary read group. The environment stays mode
`0600`, runtime mounts stay read-only, and no Linux capability is added.

Viewer self-registration is disabled by default so only administrators can add users. Toggle `BITRIVER_LIVE_ALLOW_SELF_SIGNUP`
in `.env` and rerun `./deploy/check-env.sh` followed by `docker compose up -d` to reopen or close public signups.


## Release note: removed legacy OME custom compose override

`deploy/docker-compose.ome-custom.yml` has been removed because the base `deploy/docker-compose.yml` already mounts the generated OME config and is the only supported Compose path.

If you have local automation that still references the removed file or `BITRIVER_OME_CUSTOM_CONFIG`, update it to run the standard flow instead:

```bash
export COMPOSE_FILE=deploy/docker-compose.yml
./deploy/check-env.sh
docker compose --env-file .env -f "$COMPOSE_FILE" up -d --build --pull never
```

## Syncing canonical deploy assets into Helm
Helm keeps generated copies of selected deploy artifacts so charts can be packaged self-contained, but the authoritative sources live outside the chart:

- `deploy/srs/conf/srs.conf`
- `deploy/postgres-migrate.sh`
- `deploy/migrations/*.sql`
- `scripts/backup-postgres.sh`

To refresh generated Helm copies after editing canonical files, run:

```bash
./scripts/sync-helm-deploy-assets.sh
```

To enforce drift detection in CI (or locally before commits), run:

```bash
./scripts/check-helm-deploy-assets-drift.sh
```

If drift is reported, re-run the sync command and commit the regenerated Helm files. Do not hand-edit `deploy/helm/bitriver-live/files/postgres-migrate.sh`, `deploy/helm/bitriver-live/files/backup-postgres.sh`, `deploy/helm/bitriver-live/files/srs.conf`, or `deploy/helm/bitriver-live/migrations/*.sql`. Migration runner, backup runner, and SQL copies are byte-identical to canonical sources so each deployment shape uses the same behavior and migration SHA-256 history.

## Backup scheduling examples

Backup automation examples are provided for operators who want scheduled Postgres dumps and retention pruning:

- `deploy/docker-compose.backups.yml` – Compose override with a cron sidecar that executes `scripts/backup-postgres.sh` and `scripts/prune-backups.sh`.
- `deploy/kubernetes/postgres-backup-cronjob.yaml` – Kubernetes CronJob examples for backup + prune workflows.
- `deploy/helm/bitriver-live/templates/cronjob-postgres-backup.yaml` + `deploy/helm/bitriver-live/values.yaml` (`backups.*`) – Helm-native backup scheduling hook.

These are opt-in examples; validate credentials, object-storage lifecycle, and retention against your compliance policy before enabling them in production. The Helm scheduler requires object storage when enabled so its archive, manifest, and checksum survive Job cleanup.


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

For source-checkout local build rehearsals, leave the first-party digest variables empty and rely on `docker compose --env-file .env -f deploy/docker-compose.yml up -d --build --pull never` to build the API, viewer, SRS controller, and transcoder images from the working tree.

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

The release path for an Ubuntu bare-metal/XOA VM is `deploy/install/compose-host.sh` (packaged as `install.sh` and `/usr/local/sbin/bitriver-host`). It wraps the canonical Compose graph with one bounded systemd unit instead of installing a divergent set of native per-service units:

```bash
sudo ./install.sh install --operator-user "$USER"
sudo bitriver-host configure
sudo bitriver-host activate
```

Program assets live under `/opt/bitriver-live`, secrets/configuration under `/etc/bitriver-live`, and durable state under `/var/lib/bitriver-live`. Ordinary uninstall preserves configuration/data; permanent purge requires both `--purge-data` and `--yes-really-purge`. See [`docs/installing-on-ubuntu.md`](../docs/installing-on-ubuntu.md).

The older `install/wizard.sh`, `install/ubuntu.sh`, and native service units remain historical/advanced helpers and are not equivalent to the release Compose contract.
