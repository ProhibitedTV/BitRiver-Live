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
   pre-existing `.env` is missing required credentials, the helper backfills them (including the OME API username/password)
   so Compose can start without manual edits.
3. Launch the containers with `docker compose up --build -d` using the compose file in `deploy/`. Docker automatically builds the API, viewer, SRS controller, and transcoder images the first time, so no registry login is required, and the manifest enables `restart: unless-stopped` for each long-lived service so they come back online after crashes or host reboots.
4. Wait for Postgres to accept connections. The compose bundle now launches a short-lived `postgres-migrations` service that walks the SQL files in `deploy/migrations/`, applies them with `psql`, and exits. If a migration fails the service stops and the API never starts, giving you a chance to correct the database state before retrying `docker compose up -d`.
5. Wait for the API health check to pass, then invoke the `bootstrap-admin` helper to seed the admin account and print the credentials.

Update the generated `.env` before inviting real users—swap in a valid admin email, capture the printed admin password (the
quickstart rotates it automatically on first run), rotate the
`BITRIVER_POSTGRES_USER`/`BITRIVER_POSTGRES_PASSWORD` pair (and update `BITRIVER_LIVE_POSTGRES_DSN` to match), change the Redis
credentials (`BITRIVER_REDIS_PASSWORD` and `BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD`), and point
`BITRIVER_TRANSCODER_PUBLIC_BASE_URL` at the HTTP origin your viewers can actually reach instead of the default
`http://localhost:9080`. Update the public viewer URL to match your domain or reverse proxy as well. The helper prints the
seeded credentials after the stack is ready; log in immediately and rotate the password from the control center settings page.

## Common follow-up commands

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
  It also refreshes the OME control credentials baked into `deploy/ome/Server.xml` so `BITRIVER_OME_USERNAME` and `BITRIVER_OME_PASSWORD` stay in sync with the mounted configuration.
- Codex CLI users: install or invoke the CLI from the repository root while Docker continues to manage the running services, and point it at this project for AI-guided edits. For example:
  ```bash
  # from the repository root
  pip install --user codex-cli  # install if needed
  codex edit .                 # target this project directory
  ```
  Rerun `docker compose up -d` after changing `.env` or editing code so the containers reload the latest configuration and binaries.

## Troubleshooting

- **`Error: Docker is required`** – Install Docker Engine from [docs.docker.com/engine/install](https://docs.docker.com/engine/install/)
  and re-run the script.
- **`Error: Docker Compose V2 is required`** – Install the compose plugin or upgrade Docker Desktop/Engine so the `docker compose`
  sub-command is available.
- **`permission denied while trying to connect to the Docker daemon socket`** – Add your account to the `docker` group with `sudo usermod -aG docker $USER` followed by `newgrp docker` (or log out and back in), then rerun the quickstart without `sudo`. You can run `sudo ./scripts/quickstart.sh` in a pinch, but expect root-owned files like `.env` until you fix the group membership.
- **Port already in use** – Stop or reconfigure any services that currently bind to ports 5432, 6379, 8080, 8081, 9000, 9001,
  1935, or 1985. Alternatively edit the corresponding `*_PORT` values in `.env` (for example, `BITRIVER_LIVE_PORT=9090`) and
  rerun `docker compose up -d`.
- **OME health check fails** – Confirm that `deploy/ome/Server.xml` declares the OME role with `<Type>origin</Type>` inside the
  root `<Server>` block. The copy in this repository is already aligned to the upstream OvenMediaEngine schema for
  `BITRIVER_OME_IMAGE_TAG` (default `0.15.10`) and mounts to `/opt/ovenmediaengine/bin/origin_conf/Server.xml` inside the
  `bitriver-ome` container, so you do not need to add custom top-level `<Bind>` or other non-schema stanzas for quickstart. The compose service pins the hostname to `ome` so the
  default `BITRIVER_OME_API=http://ome:8081` resolves correctly; keep that alias if you customize the container name. The
  quickstart script also seeds the `.env` with that value so the API always knows where to call OME regardless of the host
  system. If you deploy OME outside of Docker, update `BITRIVER_OME_API` to the reachable host/IP. If you upgrade OME and see
  schema errors (for example, an "Unknown item" message), refresh `deploy/ome/Server.xml` from the matching OME image and then
  re-apply your credential overrides.
- **Quickstart re-run pulled the wrong OME version** – When reusing an existing installation, keep `BITRIVER_OME_IMAGE_TAG`
  aligned with the version that matches your `Server.xml` schema before re-running `./scripts/quickstart.sh` or `docker compose
  up -d`. The script preserves `.env` and volumes, so a stale tag can point Docker at a newer image that no longer matches the
  persisted configuration.
- **Environment tweaks** – Edit `.env` and rerun `docker compose up -d` to apply changes. The compose stack automatically loads
  the file so you never need to touch `deploy/docker-compose.yml` directly.

For more advanced tuning (TLS, Redis-backed rate limiting, scaling) continue with [`docs/advanced-deployments.md`](advanced-deployments.md).
