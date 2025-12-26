# BitRiver Live Quickstart

Use the Go-based quickstart command to provision the full stack with Docker in a few minutes. It performs the same checks and templating as the legacy helpers while keeping every platform on one CLI. The shell (`scripts/quickstart.sh`) and PowerShell (`scripts/quickstart.ps1`) shims simply forward to the Go binary when you cannot run `go` directly.

On the first run Docker builds local images for the Go API, the Next.js viewer app, the SRS controller, and the bundled FFmpeg job controller (located in `cmd/transcoder/`), so you can launch the stack without signing in to any container registry.

## Platform prerequisites

| Platform | Docker runtime | Notes |
| --- | --- | --- |
| macOS 12+ | Docker Desktop with Compose V2 enabled | Start Docker Desktop first and leave at least 15GB free on Docker's data root. |
| Ubuntu 22.04+ / other Linux | Docker Engine + Compose plugin | Install the Compose plugin, add yourself to the `docker` group (or prefix commands with `sudo`), and confirm `docker compose` works without root. |
| Windows 11 (WSL 2 backend) | Docker Desktop | Run the quickstart inside WSL with the WSL 2 backend enabled; check `docker-desktop` has 15GB free. |
| Windows 11 (native PowerShell) | Docker Desktop (WSL 2 backend) | Same Docker Desktop install as above; native shells call the same Go CLI. |

## Run the quickstart command

```bash
# macOS, Linux, or Windows via WSL/bash
go run ./cmd/quickstart --compose-file deploy/docker-compose.yml

# Windows PowerShell (same CLI, different shell)
pwsh -c "go run ./cmd/quickstart --compose-file deploy/docker-compose.yml"
```

Want a shim to handle shell-specific permissions? Use `./scripts/quickstart.sh` from POSIX shells or `./scripts/quickstart.ps1` from PowerShell—they call the same Go quickstart and keep the `COMPOSE_FILE` defaulted to `deploy/docker-compose.yml`.

### What the command configures

1. Verifies that both Docker and Docker Compose V2 are available and warns when disk space under the Docker data root is below 15GB.
2. Generates `.env` with the same defaults baked into `deploy/docker-compose.yml` (including placeholders for the admin email and viewer URL) and rotates the administrator password to a strong random value unless the file already exists. When a pre-existing `.env` is missing required credentials, the helper backfills them (including the OME API username/password and the `BITRIVER_OME_BIND` listener address for the control listener) so Compose can start without manual edits.
3. Launches the containers with `docker compose up --build -d` using the compose file in `deploy/`. Docker automatically builds the API, viewer, SRS controller, and transcoder images the first time, so no registry login is required, and the manifest enables `restart: unless-stopped` for each long-lived service so they come back online after crashes or host reboots.
4. Renders `deploy/ome/Server.generated.xml` from the bundled template, applying `BITRIVER_OME_BIND`, stamping the configured ports, and mirroring the credentials from `.env` so health checks see the same managers authentication the compose preflight expects. If the template or credentials drift, the generator stops Compose before OME starts.
5. Waits for the API readiness check to pass (`/readyz`), then invokes the `bootstrap-admin` helper to seed the admin account and print the credentials. The `/healthz` endpoint still reports ingest dependency status in the JSON payload and may mark the stack as `degraded` when streaming services are unavailable, but readiness will only fail when core API dependencies are down.

Quickstart defaults `BITRIVER_LIVE_MODE` to `development` so session cookies stay readable over HTTP without TLS. Keep that default for local demos, and switch to `production` only after wiring HTTPS via `BITRIVER_LIVE_TLS_CERT`/`BITRIVER_LIVE_TLS_KEY` or a reverse proxy so cookies regain the `Secure` flag.

The helper leaves `NEXT_PUBLIC_API_BASE_URL` empty so the viewer inherits the API origin when proxied through `NEXT_VIEWER_BASE_PATH` (default `/viewer`). Set `NEXT_PUBLIC_API_BASE_URL` to the publicly reachable API URL when serving the viewer from its own hostname and adjust `NEXT_PUBLIC_VIEWER_URL` to match before re-running `docker compose up -d`.

Running `deploy/check-env.sh` against the quickstart `.env` will now warn (instead of failing) when loopback values remain for the viewer URL, OME bind/IP, or the transcoder public base URL while `BITRIVER_LIVE_MODE` stays in development. Production deployments should still replace those placeholders with routable hosts before re-running the validator.

The health payload still expects the ingest services to be reachable from the API container:

- **SRS controller:** `BITRIVER_SRS_API` defaults to `http://srs-controller:1985` inside the Compose network. If you move SRS elsewhere, point this URL at a reachable host and keep the API token aligned with the controller's configuration.
- **OvenMediaEngine:** `BITRIVER_OME_API` defaults to `http://ome:8081` and expects the username/password plus `BITRIVER_OME_API_TOKEN` from `.env` (the health probe forwards `BITRIVER_OME_ACCESS_TOKEN`, which defaults to the same value). A short-lived `ome-config` helper in the compose file renders `deploy/ome/Server.generated.xml` from `deploy/ome/Server.xml` before OME starts, keeping the control credentials aligned with `.env` so a 401 surfaces as `unhealthy` instead of silently failing. When running OME outside Compose, keep this URL reachable from the API container so `/healthz` reports the correct status even though the HTTP status code remains 200 during degraded states, and mirror the same credentials in your OME configuration. The template rewrites the control listener bind/IP from `BITRIVER_OME_BIND` (default `0.0.0.0`), stamps the server-level `<Bind><IP>`, `<Bind><Port>`, and `<Bind><TLSPort>` entries with `BITRIVER_OME_BIND`, `BITRIVER_OME_SERVER_PORT`, and `BITRIVER_OME_SERVER_TLS_PORT` to match the WebRTC signalling and TLS ports, and fills the top-level `<Server><IP>` with `BITRIVER_OME_IP` (defaulting to `BITRIVER_OME_BIND`)—update both the template and `BITRIVER_OME_API` together if you customize the control port. Leave `BITRIVER_OME_SIGNALLING_PORT` empty to reuse `BITRIVER_OME_SERVER_PORT` for the host binding; set it explicitly only when the host-facing WebRTC port must differ from the value rendered into `Server.xml` so the API and browsers connect to the expected port.
  Edit `BITRIVER_OME_USERNAME`, `BITRIVER_OME_PASSWORD`, `BITRIVER_OME_API_TOKEN`, `BITRIVER_OME_BIND`, `BITRIVER_OME_IP`, `BITRIVER_OME_SERVER_PORT`, or `BITRIVER_OME_SERVER_TLS_PORT` in `.env`? Re-render `deploy/ome/Server.generated.xml` with `./scripts/render-ome-config.sh` (it overwrites the generated file on every run) before running `docker compose up -d` when you operate OME outside Compose so the control credentials and bind address stay aligned with the health check. Override `BITRIVER_OME_ACCESS_TOKEN` only if your deployment needs a different health probe token; by default it mirrors `BITRIVER_OME_API_TOKEN`. The quickstart helper reruns the renderer automatically so template changes picked up via `git pull` land in the generated config before Compose starts.
- **Transcoder:** `BITRIVER_TRANSCODER_API` defaults to `http://transcoder:9000`; ensure the host and port resolve from the API container and that the token matches `BITRIVER_TRANSCODER_TOKEN`.

Update the generated `.env` before inviting real users—swap in a valid admin email, capture the printed admin password (the quickstart rotates it automatically on first run), rotate the `BITRIVER_POSTGRES_USER`/`BITRIVER_POSTGRES_PASSWORD` pair (and update `BITRIVER_LIVE_POSTGRES_DSN` to match), change the Redis credentials (`BITRIVER_REDIS_PASSWORD` and `BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD`), and point `BITRIVER_TRANSCODER_PUBLIC_BASE_URL` at the HTTP origin your viewers can actually reach instead of the default `http://localhost:9080`. Update the public viewer URL to match your domain or reverse proxy as well. The helper prints the seeded credentials after the stack is ready; log in immediately and rotate the password from the control center settings page. Viewer self-registration stays disabled by default to keep new accounts admin-controlled; set `BITRIVER_LIVE_ALLOW_SELF_SIGNUP=true` in `.env` and rerun `docker compose up -d` to reopen public signups, or leave it false to require manual invites.

## Common follow-up commands

Compose lives in `deploy/docker-compose.yml`. Set the file path once from the repository root to avoid `no configuration file p
rovided` errors, then use the standard Compose subcommands:

```bash
export COMPOSE_FILE=deploy/docker-compose.yml
```

- Inspect service health (Compose rerenders `deploy/ome/Server.generated.xml` via the `ome-config` preflight before OME starts, so stale configs block startup instead of leaving the container unhealthy):
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
  go run ./cmd/quickstart --compose-file deploy/docker-compose.yml
  ```
  The Go command reuses your existing `.env` and Docker volumes, so configuration, database data, and media files persist across updates. Prefer this entry point even on Windows (with `pwsh -c`), and fall back to `./scripts/quickstart.sh` or `./scripts/quickstart.ps1` only when your shell cannot run `go` directly.
`docker compose up` (including the quickstart wrapper) reruns the `ome-config` helper so OME consumes the credentials from `.env` and the current `BITRIVER_OME_BIND` value in both the root `<Bind><IP>` entry and the control listener `<Bind>`/`<IP>` without requiring an extra compose override.
- Running `git pull` followed by the quickstart keeps OME in a predictable state:
  - The helper preserves your `.env` while backfilling any new variables introduced upstream so you avoid silent crashes from missing credentials, including the OME managers token.
  - `deploy/ome/Server.generated.xml` is always re-rendered from `deploy/ome/Server.xml` and the refreshed `.env`, eliminating drift between the template and the live config mounted into the container.
  - Docker images rebuild and database migrations run automatically before the stack restarts, giving you a clean, rerun-safe deploy loop whenever templates or env keys change.
- Codex CLI users: follow the [Codex CLI guide](codex-cli.md) for installation, authentication, and edit workflows tailored to this repository. Rerun `docker compose up -d` after applying Codex patches so containers reload configuration and binaries.

## Troubleshooting

- **`Error: Docker is required`** – Install Docker Engine from [docs.docker.com/engine/install](https://docs.docker.com/engine/install/)
  and re-run `go run ./cmd/quickstart --compose-file deploy/docker-compose.yml` (or the shim for your shell).
- **`Error: Docker Compose V2 is required`** – Install the compose plugin or upgrade Docker Desktop/Engine so the `docker compose`
  sub-command is available for the Go quickstart across all platforms.
- **`permission denied while trying to connect to the Docker daemon socket`** – Add your account to the `docker` group with `sudo usermod -aG docker $USER` followed by `newgrp docker` (or log out and back in), then rerun the quickstart without `sudo`. You can run `sudo go run ./cmd/quickstart --compose-file deploy/docker-compose.yml` or the shell/PowerShell shims in a pinch, but expect root-owned files like `.env` until you fix the group membership.
- **Port already in use** – Stop or reconfigure any services that currently bind to ports 5432, 6379, 8080, 8081, 9000, 9001, 1935, or 1985. Alternatively edit the corresponding `*_PORT` values in `.env` (for example, `BITRIVER_LIVE_PORT=9090`) and rerun `docker compose up -d`.
- **`Empty <AccessToken> is not allowed`** – The OvenMediaEngine template detected a missing `BITRIVER_OME_API_TOKEN` in `.env`. Set a non-empty value in `.env`, mirror it into `BITRIVER_OME_ACCESS_TOKEN` if you want the health probe to use a distinct header, rerun `./scripts/render-ome-config.sh --force`, and restart the stack with `docker compose up -d` so `deploy/ome/Server.generated.xml` is regenerated with the token.
- **Still seeing `AccessTokens` errors after rendering?** – Verify the stamp and contents of the generated config before restarting OME:
  1. Ensure `.env` sets `BITRIVER_OME_IMAGE_TAG` to the version you actually run and includes a non-empty `BITRIVER_OME_API_TOKEN`.
  2. Force regeneration to align the schema with that tag:
     ```bash
     ./scripts/render-ome-config.sh --force
     ```
  3. Confirm the rendered file contains the managers auth block and matches the tag marker:
     ```bash
     grep -n "BITRIVER_OME_IMAGE_TAG" deploy/ome/Server.generated.xml
     rg --heading "AccessTokens|Authentication" deploy/ome/Server.generated.xml
     ```
     Only restart `docker compose up -d bitriver-ome` after both checks pass.
- **Reading OME startup logs after template fixes** – Quickstart reruns should show the standard OME banner, FFmpeg and version
  lines, STUN public IP resolution, a burst of ICE candidate logs, and the `All modules are initialized successfully` marker
  followed by `Create HostMetrics...` and `Create ApplicationMetrics(#default#live...)`. You should no longer see
  `Application.type is not optional` or `Empty <AccessToken> is not allowed`. Messages such as `WebRTC publisher is disabled in
  #default#live application, so it was not created` simply reflect the `<Publishers>`/`<Providers>` switches in your
  `<Application>` block being set to `Off`; toggle them to match whether BitRiver Live should push or pull streams via WebRTC,
  LLHLS, and related outputs.
- **OME health check fails** – The compose service pins the hostname to `ome` so the default `BITRIVER_OME_API=http://ome:8081` resolves correctly; keep that alias if you customize the container name. The health probe forwards the `BITRIVER_OME_API_TOKEN` header when the rendered config includes `<AccessToken>` (otherwise it falls back to an unauthenticated probe with optional basic auth), so a 401 response will mark the container as unhealthy—the compose preflight reruns `./scripts/render-ome-config.sh` automatically before OME starts, mounting the regenerated `deploy/ome/Server.generated.xml` into `/opt/ovenmediaengine/bin/origin_conf/Server.xml` and `/opt/ovenmediaengine/bin/edge_conf/Server.xml`, but you can still verify the credentials landed in the rendered file:
  ```bash
  ./scripts/render-ome-config.sh --check || ./scripts/render-ome-config.sh --force
  grep -E '<(ID|Password|AccessToken)>' deploy/ome/Server.generated.xml
  ```
  If you deploy OME outside of Docker, update `BITRIVER_OME_API` to the reachable host/IP and ensure the configured username/password/access token match the container's baked credentials and your copied `Server.xml` before bringing the stack back up.
- **Quickstart re-run pulled the wrong OME version** – When reusing an existing installation, keep `BITRIVER_OME_IMAGE_TAG`
  aligned with the version that matches your `Server.xml` schema before re-running the Go quickstart (`go run ./cmd/quickstart --compose-file deploy/docker-compose.yml`) or `docker compose up -d`. The quickstart and `scripts/render-ome-config.sh --check` both compare the tag in `.env` with the marker stamped inside
  `deploy/ome/Server.generated.xml` and force a regeneration before Compose starts if they diverge. The default `0.16.0` tag
  always renders the managers authentication block and requires a non-empty `BITRIVER_OME_API_TOKEN`; non-semver image tags are
  rejected by the renderer to avoid generating configs with unsupported authentication fields.
- **Environment tweaks** – Edit `.env` and rerun `docker compose up -d` to apply changes. The compose stack automatically loads
  the file so you never need to touch `deploy/docker-compose.yml` directly.

For more advanced tuning (TLS, Redis-backed rate limiting, scaling) continue with [`docs/advanced-deployments.md`](advanced-deployments.md).
