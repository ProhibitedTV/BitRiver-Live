# BitRiver Live Quickstart

The `scripts/quickstart.sh` helper script provisions the full BitRiver Live stack with Docker in a few minutes. It handles tool
checks, creates a `.env` file with sensible defaults, and brings the compose services online.

On the first run Docker builds local images for the Go API, the Next.js viewer app, the SRS controller, and the bundled FFmpeg job controller (located in `cmd/transcoder/`), so you can launch the stack without signing in to any container registry.

## Run the stack

From the repository root, execute:

> **Linux tip:** Add yourself to the `docker` group so you can talk to the daemon without `sudo`:
> 
> ```bash
> sudo usermod -aG docker $USER
> newgrp docker  # or log out and back in
> ```
> 
> You can also run the quickstart with `sudo ./scripts/quickstart.sh`, but that will create root-owned files such as `.env`, so fixing the group membership first is the better long-term solution.

```bash
./scripts/quickstart.sh
```

The script will:

1. Verify that both Docker and Docker Compose V2 are available.
2. Generate `.env` with the same defaults baked into `deploy/docker-compose.yml` (including placeholders for the admin email
   and viewer URL) and rotate the administrator password to a strong random value unless the file already exists. When a
   pre-existing `.env` is missing required credentials, the helper backfills them (including the OME API username/password
   and the `BITRIVER_OME_BIND` listener address for the `<Bind>`/`<IP>` tags) so Compose can start without manual edits.
3. Launch the containers with `docker compose up --build -d` using the compose file in `deploy/`. Docker automatically builds the API, viewer, SRS controller, and transcoder images the first time, so no registry login is required, and the manifest enables `restart: unless-stopped` for each long-lived service so they come back online after crashes or host reboots.
4. Wait for Postgres to accept connections. The compose bundle now launches a short-lived `postgres-migrations` service that walks the SQL files in `deploy/migrations/`, applies them with `psql`, and exits. If a migration fails the service stops and the API never starts, giving you a chance to correct the database state before retrying `docker compose up -d`.
5. Wait for the API readiness check to pass (`/readyz`), then invoke the `bootstrap-admin` helper to seed the admin account and print the credentials. The `/healthz` endpoint still reports ingest dependency status in the JSON payload and may mark the stack as `degraded` when streaming services are unavailable, but readiness will only fail when core API dependencies are down.

The health payload still expects the ingest services to be reachable from the API container:

- **SRS controller:** `BITRIVER_SRS_API` defaults to `http://srs-controller:1985` inside the Compose network. If you move SRS elsewhere, point this URL at a reachable host and keep the API token aligned with the controller's configuration.
- **OvenMediaEngine:** `BITRIVER_OME_API` defaults to `http://ome:8081` and requires the username/password set in `.env`. The quickstart renders `deploy/ome/Server.generated.xml` from `deploy/ome/Server.xml` on every run and the compose bundle mounts it into the container, keeping the control credentials aligned with `.env` so a 401 surfaces as `unhealthy` instead of silently failing. When running OME outside Compose, keep this URL reachable from the API container so `/healthz` reports the correct status even though the HTTP status code remains 200 during degraded states, and mirror the same credentials in your OME configuration. Current OME images expect `<Bind>` alongside `<IP>`, so the template rewrites both values from `BITRIVER_OME_BIND` (default `0.0.0.0`) while preserving the `<Listeners><TCP><Bind>0.0.0.0</Bind><IP>0.0.0.0</IP><Port>8081</Port></TCP></Listeners>` schema—update both the template and `BITRIVER_OME_API` together if you customize the control port.
- **Transcoder:** `BITRIVER_TRANSCODER_API` defaults to `http://transcoder:9000`; ensure the host and port resolve from the API container and that the token matches `BITRIVER_TRANSCODER_TOKEN`.

Update the generated `.env` before inviting real users—swap in a valid admin email, capture the printed admin password (the
quickstart rotates it automatically on first run), rotate the
`BITRIVER_POSTGRES_USER`/`BITRIVER_POSTGRES_PASSWORD` pair (and update `BITRIVER_LIVE_POSTGRES_DSN` to match), change the Redis
credentials (`BITRIVER_REDIS_PASSWORD` and `BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD`), and point
`BITRIVER_TRANSCODER_PUBLIC_BASE_URL` at the HTTP origin your viewers can actually reach instead of the default
`http://localhost:9080`. Update the public viewer URL to match your domain or reverse proxy as well. The helper prints the
seeded credentials after the stack is ready; log in immediately and rotate the password from the control center settings page.

## Common follow-up commands

Compose lives in `deploy/docker-compose.yml`. Set the file path once from the repository root to avoid `no configuration file p
rovided` errors, then use the standard Compose subcommands:

```bash
export COMPOSE_FILE=deploy/docker-compose.yml
```

- Inspect service health:
  ```bash
  docker compose ps
  ```
- Follow logs for every container:
  ```bash
  docker compose logs -f
  ```
- Stop the stack and keep data volumes intact:
  ```bash
  docker compose down
  ```
- Stop the stack and remove data volumes (destructive):
  ```bash
  docker compose down -v
  ```

All commands assume you are still in the repository root (where `.env` lives) so Docker Compose can locate the project name and
compose file.

## Updating your stack

- Pull upstream changes before restarting your containers so you pick up fixes and migrations:
  ```bash
  git pull --ff-only
  ```
- Re-run the quickstart to rebuild images when Dockerfiles or dependencies change and to ensure services restart with the latest code and environment values:
  ```bash
  ./scripts/quickstart.sh
  ```
  The script reuses your existing `.env` and Docker volumes, so configuration, database data, and media files persist across updates.
The helper re-renders `deploy/ome/Server.generated.xml` from `deploy/ome/Server.xml` on each run so OME consumes the credentials from `.env` and the current `BITRIVER_OME_BIND` value in both `<Bind>` and `<IP>` without requiring an extra compose override.
- Codex CLI users: follow the [Codex CLI guide](codex-cli.md) for installation, authentication, and edit workflows tailored to this repository. Rerun `docker compose up -d` after applying Codex patches so containers reload configuration and binaries.

## Troubleshooting

- **`Error: Docker is required`** – Install Docker Engine from [docs.docker.com/engine/install](https://docs.docker.com/engine/install/)
  and re-run the script.
- **`Error: Docker Compose V2 is required`** – Install the compose plugin or upgrade Docker Desktop/Engine so the `docker compose`
  sub-command is available.
- **`permission denied while trying to connect to the Docker daemon socket`** – Add your account to the `docker` group with `sudo usermod -aG docker $USER` followed by `newgrp docker` (or log out and back in), then rerun the quickstart without `sudo`. You can run `sudo ./scripts/quickstart.sh` in a pinch, but expect root-owned files like `.env` until you fix the group membership.
- **Port already in use** – Stop or reconfigure any services that currently bind to ports 5432, 6379, 8080, 8081, 9000, 9001,
  1935, or 1985. Alternatively edit the corresponding `*_PORT` values in `.env` (for example, `BITRIVER_LIVE_PORT=9090`) and
  rerun `docker compose up -d`.
- **OME health check fails** – The compose service pins the hostname to `ome` so the default `BITRIVER_OME_API=http://ome:8081` resolves correctly; keep that alias if you customize the container name. The health probe uses the configured `BITRIVER_OME_USERNAME`/`BITRIVER_OME_PASSWORD`, so a 401 response will mark the container as unhealthy—verify that the credentials in `.env` match the rendered `deploy/ome/Server.generated.xml` (rerun the quickstart to refresh it). If you deploy OME outside of Docker, update `BITRIVER_OME_API` to the reachable host/IP and ensure the configured username/password match the container's baked credentials and your copied `Server.xml` before bringing the stack back up.
- **Quickstart re-run pulled the wrong OME version** – When reusing an existing installation, keep `BITRIVER_OME_IMAGE_TAG`
  aligned with the version that matches your `Server.xml` schema before re-running `./scripts/quickstart.sh` or `docker compose
  up -d`. The script preserves `.env` and volumes, so a stale tag can point Docker at a newer image that no longer matches the
  persisted configuration. The default `0.15.10` tag remains compatible with the bundled configuration; if you bump the tag,
  confirm the `<Bind>`/`<IP>` requirements against the upstream schema and adjust `deploy/ome/Server.xml` accordingly.
- **Environment tweaks** – Edit `.env` and rerun `docker compose up -d` to apply changes. The compose stack automatically loads
  the file so you never need to touch `deploy/docker-compose.yml` directly.

For more advanced tuning (TLS, Redis-backed rate limiting, scaling) continue with [`docs/advanced-deployments.md`](advanced-deployments.md).
