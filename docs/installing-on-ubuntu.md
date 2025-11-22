# Installing BitRiver Live on Ubuntu

This guide walks operators through bringing up BitRiver Live on an Ubuntu Server virtual machine. It covers VM preparation, package installation, data services, ingest components, application builds, and final verification. For reference architectures and service manifests, see [`deploy/docker-compose.yml`](../deploy/docker-compose.yml), [`docs/scaling-topologies.md`](scaling-topologies.md), and the systemd units documented in [`deploy/systemd/README.md`](../deploy/systemd/README.md). The sample SRS configuration in [`deploy/srs/conf/srs.conf`](../deploy/srs/conf/srs.conf) is referenced when enabling ingest.

> **Supported releases:** Ubuntu 22.04 LTS or later. Earlier releases may require updated package repositories for Node.js and Redis.

## 1. Prepare the virtual machine

1. Provision an Ubuntu VM with at least 4 vCPUs, 8 GB RAM, and 100 GB of SSD-backed storage. Place it on a subnet that can reach your ingest and viewer networks.
2. Attach a static public IP or configure DNS for the hostname viewers will use (e.g., `stream.example.com`).
3. Harden the base OS:
   - Create a non-root sudo user and disable password SSH logins.
   - Update packages and reboot into the latest kernel.

```bash
sudo apt update
sudo apt full-upgrade -y
sudo reboot
```

4. Enable uncomplicated firewall (UFW) to expose only the required ports. Adjust for your topology if traffic terminates at a load balancer.

```bash
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 1935/tcp   # RTMP ingest to SRS
sudo ufw allow 8080/tcp   # API (adjust if reverse proxy terminates TLS)
sudo ufw allow 9080/tcp   # HLS playback mirror (transcoder-public)
sudo ufw allow 8088/tcp   # SRS WebRTC/HTTP-FLV (optional)
sudo ufw enable
```

> **TLS reminder:** Plan for HTTPS. Either terminate TLS at a reverse proxy (e.g., Nginx, Caddy) or use Certbot to issue certificates for the Go API directly. Schedule renewals before going live.

## 2. Install base packages

Install OS dependencies for the API, database clients, Node.js-based viewer, and ingest tooling.

```bash
sudo apt install -y build-essential git curl ufw pkg-config libcap2-bin \
  golang-go nodejs npm postgresql-client redis-tools docker.io docker-compose-plugin
```

`libcap2-bin` provides the `setcap` utility the Ubuntu installer uses to keep privileged ports working across manual restarts. If you plan to bind to :80 or :443 directly from the Go process, keep it installed; otherwise pass `--addr :8080` (or another high port) when terminating traffic at a reverse proxy.

If you prefer managed runtimes, replace the distribution Go and Node.js packages with upstream toolchains (e.g., via `snap`, `asdf`, or tarballs). Ensure `go version` reports 1.21+ and `node --version` reports 18+.

## 3. Configure PostgreSQL and Redis

Install server packages and enable services.

```bash
sudo apt install -y postgresql postgresql-contrib redis-server
sudo systemctl enable --now postgresql
sudo systemctl enable --now redis-server
```

### PostgreSQL

1. Switch to the `postgres` user to create the application database and credentials. Choose a role name and password that are unique to this deployment—the automation in [`deploy/check-env.sh`](../deploy/check-env.sh) rejects the historical `bitriver`/`changeme` samples so you do not accidentally reuse them.

```bash
sudo -u postgres psql <<'SQL'
-- Replace brlive_app and the sample password with your own values
CREATE ROLE brlive_app WITH LOGIN PASSWORD 'P0stgres-Example!';
CREATE DATABASE brlive_app OWNER brlive_app;
GRANT ALL PRIVILEGES ON DATABASE brlive_app TO brlive_app;
SQL
```

2. Enforce TLS between the API and Postgres in production. Update `/etc/postgresql/14/main/postgresql.conf` and `pg_hba.conf` to require `hostssl` entries, and deploy certificates managed by your secrets store (HashiCorp Vault, AWS Secrets Manager, etc.). Restart PostgreSQL after editing:

```bash
sudo systemctl restart postgresql
```

3. Verify connectivity from the application host.

When you test connectivity, use the same credentials you created above and ensure the DSN matches the `.env` file you will provide to the application (see [`deploy/.env.example`](../deploy/.env.example) for the canonical key names). The validation script flags mismatched values so the API and migrators do not fall back to the blocked defaults.

```bash
psql "postgres://brlive_app:P0stgres-Example!@localhost:5432/brlive_app?sslmode=disable" -c '\l'
```

Replace `sslmode=disable` with `require` when TLS is enabled.

If you are upgrading from the JSON datastore, run:

```bash
go run -tags postgres ./cmd/tools/migrate-json-to-postgres \
  --json /var/lib/bitriver-live/store.json \
  --postgres-dsn "postgres://brlive_app:P0stgres-Example!@localhost:5432/brlive_app?sslmode=disable"
```

The helper copies records into Postgres and verifies the row counts before exiting.

### Redis

1. Harden Redis for networked deployments:
   - Bind Redis to `127.0.0.1` or your private subnet.
   - Set a strong password in `/etc/redis/redis.conf` (`requirepass`).
   - Enable TLS if clients connect over untrusted networks (stunnel or Redis 6+ native TLS).

   The Docker Compose bundle now enables `requirepass` automatically and refuses to start until
   `BITRIVER_REDIS_PASSWORD` is populated in `.env`. The API reads the same credential via
   `BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD`, so update both entries together when you rotate the
   password. `deploy/check-env.sh` checks for empty values and blocks the placeholder samples to
   prevent accidental reuse.

2. Restart and validate:

```bash
sudo systemctl restart redis-server
redis-cli -a 'R3dis-Example!' ping
```

## 4. Download BitRiver Live release assets

Always install from a tagged release so the binaries, installer scripts, and Docker Compose manifests stay in sync. Download the archive that matches your architecture from the [GitHub Releases](https://github.com/BitRiver-Live/BitRiver-Live/releases) page.

```bash
# Replace v1.2.3 with the release tag you plan to deploy
export BITRIVER_LIVE_VERSION="v1.2.3"
export BITRIVER_LIVE_PACKAGE="bitriver-live-linux-amd64.tar.gz"  # Use linux-arm64 on Ampere/Graviton hosts

curl -LO "https://github.com/BitRiver-Live/BitRiver-Live/releases/download/${BITRIVER_LIVE_VERSION}/${BITRIVER_LIVE_PACKAGE}"
sudo mkdir -p /opt/bitriver-live
sudo tar -C /opt/bitriver-live -xzf "${BITRIVER_LIVE_PACKAGE}"
sudo chown -R $USER:$USER /opt/bitriver-live
rm "${BITRIVER_LIVE_PACKAGE}"
```

The archive expands into `/opt/bitriver-live` with the compiled binaries plus the `deploy/` directory (`docker-compose.yml`, `.env.example`, `check-env.sh`, `install/`, `nginx/`, `ome/`, `srs/`, and `systemd/`) referenced throughout this guide.

## 5. Deploy ingest services

BitRiver Live relies on SRS for ingest and OvenMediaEngine (OME) plus a transcoder for playback. Choose the approach that matches your operations model.

### Option A: Docker Compose

The compose bundle under [`deploy/docker-compose.yml`](../deploy/docker-compose.yml) wires SRS, OME, the transcoder, PostgreSQL, Redis, and the API/viewer. Adapt it for production by overriding secrets and persistent volumes.

Before starting the services, create an environment file and replace the placeholder credentials. Docker Compose refuses to launch until these values are present.

```bash
cd /opt/bitriver-live
cp deploy/.env.example .env
${EDITOR:-nano} .env
./deploy/check-env.sh
```

Update the entries for:

- `BITRIVER_POSTGRES_USER` and `BITRIVER_POSTGRES_PASSWORD`
- `BITRIVER_REDIS_PASSWORD` and `BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD`
- `BITRIVER_LIVE_ADMIN_EMAIL` and `BITRIVER_LIVE_ADMIN_PASSWORD`
- `BITRIVER_SRS_TOKEN`
- `BITRIVER_OME_USERNAME` and `BITRIVER_OME_PASSWORD`
- `BITRIVER_TRANSCODER_TOKEN`
- `BITRIVER_LIVE_IMAGE_TAG`, `BITRIVER_VIEWER_IMAGE_TAG`, `BITRIVER_SRS_CONTROLLER_IMAGE_TAG`, and `BITRIVER_TRANSCODER_IMAGE_TAG` (set all four to the release tag you extracted in [Step 4](#4-download-bitriver-live-release-assets) so every container runs the same build)
- `BITRIVER_SRS_IMAGE_TAG` (defaults to `v5.0.185` so Compose matches the systemd units; bump it only after testing a newer SRS release and update both deployments together)

The `.env` guardrails shipped with the release bundle intentionally block the original `bitriver`/`changeme` samples. [`deploy/.env.example`](../deploy/.env.example) documents every variable, while [`deploy/check-env.sh`](../deploy/check-env.sh) refuses to continue until each credential changes and `BITRIVER_LIVE_POSTGRES_DSN` matches the database user/password you selected earlier. Rerun the script after every edit so Compose and the systemd units pick up consistent DSNs.

Set `BITRIVER_TRANSCODER_TOKEN` to a strong bearer credential. The FFmpeg job controller refuses to start when `JOB_CONTROLLER_TOKEN` (the environment variable consumed inside the container) is empty, so populate it before launching the stack.

Ensure `BITRIVER_LIVE_POSTGRES_DSN` references the same Postgres user and password you configure above before bringing the stack online.

The bundled PostgreSQL container now reuses these credentials for its health probe, so the readiness check automatically honours any changes you make to `BITRIVER_POSTGRES_USER` (and `BITRIVER_POSTGRES_DB` if you override it) in `.env`.

> **New default:** The compose manifest no longer publishes PostgreSQL to the host. Containers on the internal network can still reach it at `postgres:5432`, but the port remains firewalled from the host unless you explicitly opt in.

To expose Postgres for administrative access, set `COMPOSE_PROFILES=postgres-host` in `.env` (or your shell) and rerun `docker compose`. The opt-in profile starts a small sidecar that shares the database network namespace and publishes `5432` to the host. Override `BITRIVER_POSTGRES_HOST_PORT` when you need a different host port, and tighten your firewall rules before enabling the profile. `./deploy/check-env.sh` prints a reminder about the security trade-offs whenever the profile is active.

Redis now hosts the chat queue by default (`BITRIVER_LIVE_CHAT_QUEUE_DRIVER=redis`). The compose
manifest points the API at the in-cluster service (`BITRIVER_LIVE_CHAT_QUEUE_REDIS_ADDR=redis:6379`)
with the password you set above, and creates the `bitriver-live-chat` stream/group pairing out of
the box. When you connect an external Redis deployment, update the address, stream, group, and
password fields accordingly before restarting the stack.

The API talks to SRS through the dedicated proxy. Leave `BITRIVER_SRS_API` set to `http://srs-controller:1985` when you use the compose bundle, or point it at the controller host/port (`http://localhost:1986` on the default Docker Compose network). Adjust `SRS_CONTROLLER_UPSTREAM` in `.env` when the proxy needs to reach an external SRS instance instead of the bundled container.

Rerun `./deploy/check-env.sh` until it reports the environment file is ready. The compose manifest also uses required-variable expansion, so `docker compose` fails with an explanatory error when any of the credentials are missing or unchanged from the defaults.

All long-running services in the compose file specify `restart: unless-stopped`, ensuring Docker automatically restarts containers after crashes or reboots. Override the policy per service if your operations model requires different behaviour.

The manifest now includes a short-lived `postgres-migrations` helper that waits for the bundled Postgres container to pass its health check, applies every SQL file in `deploy/migrations/` with `psql`, and exits. The `bitriver-live` API depends on that service with `condition: service_completed_successfully`, so the API will only start after migrations finish cleanly. Re-running `docker compose up -d` during upgrades triggers the helper again, applying any new migrations before the refreshed containers come online.

```bash
cd /opt/bitriver-live
sudo docker compose -f deploy/docker-compose.yml pull
sudo docker compose -f deploy/docker-compose.yml up -d srs srs-controller ome transcoder transcoder-public
```

The compose bundle now binds `./transcoder-data` on the host to `/work` inside the FFmpeg controller so HLS manifests survive container restarts. Create the directory structure once before starting production traffic:

```bash
mkdir -p /opt/bitriver-live/transcoder-data/public
```

By default the stack serves `/work/public` through the `transcoder-public` Nginx sidecar. It forwards container port `8080` to host port `9080`, which keeps playback URLs stable for both local and remote viewers. The release bundle includes `deploy/nginx/transcoder-public.conf`; copy and adjust it when you need to customise headers or upstreams. The transcoder advertises the mirror via `BITRIVER_TRANSCODER_PUBLIC_DIR=/work/public` and requires you to supply a routable value for `BITRIVER_TRANSCODER_PUBLIC_BASE_URL`. Pick the HTTP origin your viewers will actually reach—use the hostname published by your CDN, reverse proxy, or load balancer (for example, `https://cdn.example.com/hls`). When you expose the bundled Nginx sidecar directly, publish it under a public DNS record or IP that remote clients can reach and place that URL in `.env`. Leaving the variable empty or pointing it at loopback (`http://localhost`, `http://127.0.0.1`, etc.) causes `deploy/check-env.sh` and `docker compose` to fail fast, preventing you from advertising unreachable playback URLs. When a live job starts the controller drops a symlink at `/work/public/live/<jobID>` that points at the active session’s output directory and removes it once the stream stops, so the mirror never accumulates stale session folders. Nginx is configured to follow those symlinks (`disable_symlinks off;`).

If you prefer a different publication path, mount an object storage bucket or a directory served by another reverse proxy and point the transcoder at it with `BITRIVER_TRANSCODER_PUBLIC_DIR` (local staging directory) plus `BITRIVER_TRANSCODER_PUBLIC_BASE_URL` (public HTTP origin). S3-compatible storage works well with `s3fs`, `rclone mount`, or a periodic sync (`aws s3 sync /opt/bitriver-live/transcoder-data/public s3://cdn-bucket/uploads/`).

Review `deploy/srs/conf/srs.conf` for the default SRS ports and authentication settings. Mount a customised version into the container when you need stricter access control or TLS certificates for RTMP/RTMPS.

The sample config enables `http_hooks` that call the BitRiver Live ingest endpoint at `http://bitriver-live:8080/api/ingest/srs-hook`.
The hooks fire on publish/unpublish and play/stop to let the API validate stream keys and clean up sessions. Replace the `token`
query string in the hook URLs with the same value you set for `BITRIVER_SRS_TOKEN` and point the host at wherever the API
listens (for example, `localhost:8080` when running the API outside Docker).

#### Upgrading the SRS container

BitRiver Live pins SRS to `ossrs/srs:v5.0.185`, matching the systemd helpers under [`deploy/systemd/README.md`](../deploy/systemd/README.md). When you validate a newer upstream release:

1. Update `.env` with the new tag by editing `BITRIVER_SRS_IMAGE_TAG`.
2. Pull and restart the Compose services so the change takes effect:
   ```bash
   sudo docker compose -f deploy/docker-compose.yml pull srs
   sudo docker compose -f deploy/docker-compose.yml up -d srs srs-controller
   ```
3. Mirror the tag in `/opt/bitriver-srs/.env` (or the directory you keep for the systemd service) so `SRS_IMAGE` matches.
4. Restart the native unit to pick up the new image:
   ```bash
   sudo systemctl restart srs.service
   ```
5. Verify both deployments report the same version via the health endpoint before routing traffic:
   ```bash
   curl -fsS http://localhost:1985/api/v1/versions | jq '.data.srs.info.version'
   ```

If you roll back, reverse the steps: restore the previous tag in `.env` and the systemd environment file, pull the known-good image, and restart both services.

### Option B: systemd services

If you run SRS, OME, and the transcoder as native services, use [`deploy/systemd/README.md`](../deploy/systemd/README.md) for installation guidance. Copy the tracked unit files into `/etc/systemd/system/`, create the matching `/opt/bitriver-*/.env` files, and enable each service:

```bash
sudo install -d -m 0755 /opt/bitriver-srs /opt/bitriver-srs-controller /opt/bitriver-ome /opt/bitriver-transcoder
sudo install -m 0644 deploy/systemd/srs.service /etc/systemd/system/srs.service
sudo install -m 0644 deploy/systemd/srs-controller.service /etc/systemd/system/srs-controller.service
sudo install -m 0644 deploy/systemd/ome.service /etc/systemd/system/ome.service
sudo install -m 0644 deploy/systemd/bitriver-transcoder.service /etc/systemd/system/bitriver-transcoder.service
sudo systemctl daemon-reload
sudo systemctl enable --now srs.service
sudo systemctl enable --now srs-controller.service
sudo systemctl enable --now ome.service
sudo systemctl enable --now bitriver-transcoder.service
```

Populate the `.env` files with the ports, tokens, and image tags described in [`deploy/systemd/README.md`](../deploy/systemd/README.md) before starting traffic. The transcoder unit expects `TRANSCODER_TOKEN` (passed through as `JOB_CONTROLLER_TOKEN`) to be non-empty; systemd restarts will fail until you provide the credential.

Check status and logs to confirm ingest readiness.

```bash
sudo systemctl status srs.service srs-controller.service
journalctl -u srs.service -u srs-controller.service -f
```

## 6. Deploy the API service

### Guided setup

For a prompt-driven experience, run the wizard at [`deploy/install/wizard.sh`](../deploy/install/wizard.sh) from the release directory you extracted earlier:

```bash
cd /opt/bitriver-live
./deploy/install/wizard.sh
```

The wizard walks through the common inputs—install directory (default `/opt/bitriver-live`), data directory (default `/var/lib/bitriver-live`), service user (default `bitriver`), listen address, storage driver, optional hostname hint, TLS certificate/key paths, rate-limiting values, whether to allow public self-signup, and whether to redirect systemd logs. Viewer self-registration now defaults to disabled; opt in when prompted if you want to reopen public account creation. The wizard still defaults to the Postgres storage backend; be ready with a DSN and a database that has been migrated with the SQL files in [`deploy/migrations/`](../deploy/migrations). When you choose the Postgres storage backend it prompts for the DSN (required) and optionally a Postgres session-store DSN, letting you reuse the primary connection string or point to a dedicated database. The prompt rejects placeholder credentials such as `bitriver:changeme` or `bitriver:bitriver`, matching the safeguards in [`deploy/check-env.sh`](../deploy/check-env.sh); rotate a dedicated Postgres user/password before running the installer. When the wizard detects a source checkout it validates that Go 1.21+ is available; when invoked from a release tarball it skips the Go check because the binaries are already present. It still warns if a `bitriver-live.service` unit already exists before invoking the Ubuntu installer. Because the underlying helper uses `sudo` to create users, directories, and systemd units, the wizard highlights those privileged steps and asks for confirmation first.

If a run fails midway, fix the highlighted issue and start the wizard again—it is safe to rerun, and you can accept the previous defaults to regenerate the service.

### Option A: Automated installer (recommended)

The UI-generated installer script now wraps the tracked helper at [`deploy/install/ubuntu.sh`](../deploy/install/ubuntu.sh). Run it from the extracted release so it uses the binaries and migrations from the same tag:

Provide the required inputs (install directory, data directory, and service user) via flags or matching environment variables. The installer now defaults the storage backend to Postgres and refuses to continue until you provide `--postgres-dsn <DSN>` (or `BITRIVER_LIVE_POSTGRES_DSN`). Apply the SQL files in [`deploy/migrations/`](../deploy/migrations) to that database before re-running the helper so the schema is ready for the API. When Postgres is in use the session manager automatically persists to the same DSN; pass `--session-store memory` to keep ephemeral sessions or `--session-store-dsn` to point at a dedicated session database. Use `--storage-driver json` only when you intentionally opt into the legacy JSON store for development.

```bash
cd /opt/bitriver-live
./deploy/install/ubuntu.sh \
  --install-dir /opt/bitriver-live \
  --data-dir /var/lib/bitriver-live \
  --service-user bitriver \
  --mode production \
  --addr :80 \
  --postgres-dsn "postgres://stream_user:super-strong-password@localhost:5432/bitriver_live?sslmode=disable" \
  --enable-logs \
  --hostname stream.example.com
```

Run the helper from the release root—the script reuses the packaged `bitriver-live`/`bootstrap-admin` binaries or, when a checked-out module is present, rebuilds them from source.

The script builds the API binary, writes `$INSTALL_DIR/.env`, configures optional TLS and rate-limiting variables, and registers a `bitriver-live.service` systemd unit. Review the generated `.env` file to ensure storage selections (JSON or Postgres), database DSNs, session-store driver settings, and Redis credentials are present before starting traffic.

Viewer self-registration is disabled by default in the generated configuration so that only administrators can create accounts. Re-enable open signups later with `--allow-self-signup` or by setting `BITRIVER_LIVE_ALLOW_SELF_SIGNUP=true` in the environment file.

When the listen address resolves to a privileged port (<1024) the installer injects `AmbientCapabilities=CAP_NET_BIND_SERVICE`/`CapabilityBoundingSet=CAP_NET_BIND_SERVICE` into the systemd unit and runs `sudo setcap 'cap_net_bind_service=+ep' "$INSTALL_DIR/bitriver-live"` so manual restarts keep the binding. Operators fronting the service with Nginx, Caddy, or another reverse proxy should set `--addr :8080` (or a similar high port) and forward 80/443 from the proxy to avoid capabilities altogether.

Provide `--bootstrap-admin-email` (optionally pairing it with `--bootstrap-admin-password`) to seed the first control-center account automatically. When you skip the password flag the installer now generates a strong random secret, records it in `$INSTALL_DIR/.env`, and prints it exactly once so you can capture it before leaving the terminal. The installer runs the `bootstrap-admin` helper after copying the binaries so the JSON datastore or Postgres database already contains an administrator when systemd starts the service. Rotate the password from the control center after your first login.

Environment variable equivalents:

* `INSTALL_DIR`, `DATA_DIR`, `SERVICE_USER`
* `BITRIVER_LIVE_ADDR`, `BITRIVER_LIVE_MODE`
* `BITRIVER_LIVE_TLS_CERT`, `BITRIVER_LIVE_TLS_KEY`
* `BITRIVER_LIVE_RATE_GLOBAL_RPS`, `BITRIVER_LIVE_RATE_LOGIN_LIMIT`, `BITRIVER_LIVE_RATE_LOGIN_WINDOW`
* `BITRIVER_LIVE_RATE_REDIS_ADDR`, `BITRIVER_LIVE_RATE_REDIS_PASSWORD`
* `BITRIVER_LIVE_ENABLE_LOGS`, `BITRIVER_LIVE_LOG_DIR`
* `BITRIVER_LIVE_HOSTNAME_HINT`
* `BITRIVER_LIVE_ALLOW_SELF_SIGNUP`
* `BITRIVER_LIVE_POSTGRES_DSN`
* `BITRIVER_LIVE_SESSION_STORE`, `BITRIVER_LIVE_SESSION_POSTGRES_DSN` (defaults to Postgres and reuses `BITRIVER_LIVE_POSTGRES_DSN` when left unset)

### Option B: Manual install

If you prefer hand-crafted units, follow the manual process below. The release archive now exposes the API binary as `bitriver-live`, so you can install it directly without renaming.

1. Install the API binary from the release archive.

```bash
cd /opt/bitriver-live
install -d -m 755 bin
install -m 755 bitriver-live bin/bitriver-live
```

2. Install a dedicated system user and directories for configuration and data.

```bash
sudo useradd --system --home /var/lib/bitriver-live --shell /usr/sbin/nologin bitriver
sudo mkdir -p /etc/bitriver-live /var/lib/bitriver-live
sudo chown -R bitriver:bitriver /var/lib/bitriver-live
```

3. Create `/etc/bitriver-live/bitriver-live.env` with secrets and connection details. Store passwords, tokens, and API keys in a secrets manager (Vault, SOPS, AWS SSM). Distribute them at boot time via encrypted disks or templating tools (e.g., `ansible-vault`, `systemd-creds`).

```
BITRIVER_LIVE_ADDR=:8080
BITRIVER_LIVE_MODE=production
BITRIVER_LIVE_TLS_CERT=/etc/letsencrypt/live/stream.example.com/fullchain.pem
BITRIVER_LIVE_TLS_KEY=/etc/letsencrypt/live/stream.example.com/privkey.pem
BITRIVER_LIVE_STORAGE_DRIVER=postgres
BITRIVER_LIVE_POSTGRES_DSN=postgres://stream_user:super-strong-password@localhost:5432/bitriver?sslmode=require
BITRIVER_LIVE_RATE_REDIS_ADDR=127.0.0.1:6379
BITRIVER_LIVE_RATE_REDIS_PASSWORD=changeme
BITRIVER_LIVE_SESSION_STORE=postgres
# Uncomment to allow new viewers to register their own accounts once you are ready to accept signups.
# BITRIVER_LIVE_ALLOW_SELF_SIGNUP=true
# Optional: override if you want a dedicated session database.
# BITRIVER_LIVE_SESSION_POSTGRES_DSN=postgres://stream_user:super-strong-password@localhost:5432/bitriver_sessions?sslmode=require
BITRIVER_SRS_TOKEN=REPLACE_ME
BITRIVER_OME_USERNAME=REPLACE_ME
BITRIVER_OME_PASSWORD=REPLACE_ME
BITRIVER_TRANSCODER_TOKEN=REPLACE_ME
BITRIVER_TRANSCODER_PUBLIC_BASE_URL=https://cdn.example.com/hls
BITRIVER_TRANSCODER_PUBLIC_DIR=/var/lib/bitriver-transcoder/public
```

4. Install the systemd unit from `deploy/systemd/bitriver-live.service` or author a minimal unit:

```ini
[Unit]
Description=BitRiver Live API
After=network-online.target postgresql.service redis-server.service

[Service]
User=bitriver
Group=bitriver
EnvironmentFile=/etc/bitriver-live/bitriver-live.env
ExecStart=/opt/bitriver-live/bin/bitriver-live --data /var/lib/bitriver-live/store.json
Restart=on-failure
RestartSec=5s
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

5. Reload systemd and start the service.

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now bitriver-live.service
sudo systemctl status bitriver-live.service
```

## 7. Deploy the viewer

The GitHub Release for each BitRiver Live tag publishes a production-ready viewer bundle (`bitriver-viewer-<tag>.tar.gz`) and a container image (`ghcr.io/bitriver-live/bitriver-viewer:<tag>`). Use one of the options below so the API and viewer stay on the same version. Only clone the repository and build `web/viewer` from source when you intentionally need to test local changes.

### Option A: Run the standalone bundle (recommended)

1. Download the viewer archive that matches the API version and extract it alongside the API release:

   ```bash
   # Replace v1.2.3 with the release you are deploying
   export BITRIVER_LIVE_VERSION="v1.2.3"
   curl -LO "https://github.com/BitRiver-Live/BitRiver-Live/releases/download/${BITRIVER_LIVE_VERSION}/bitriver-viewer-${BITRIVER_LIVE_VERSION}.tar.gz"

   sudo mkdir -p /opt/bitriver-viewer
   sudo tar -xzvf "bitriver-viewer-${BITRIVER_LIVE_VERSION}.tar.gz" -C /opt/bitriver-viewer --strip-components=1
   rm "bitriver-viewer-${BITRIVER_LIVE_VERSION}.tar.gz"
   ```

   The archive expands into `/opt/bitriver-viewer` with `.next/standalone/`, `.next/static/`, and `public/` directories that mirror the output of `npm run build`.

2. Create `/opt/bitriver-viewer/.env` with runtime settings for the standalone server. At minimum provide the API origin and listening address:

   ```bash
   sudo tee /opt/bitriver-viewer/.env >/dev/null <<'EOF'
NEXT_PUBLIC_API_BASE_URL=https://stream.example.com
NEXT_VIEWER_BASE_PATH=/viewer
PORT=3000
HOSTNAME=0.0.0.0
EOF
   ```

3. Start the bundled server with Node.js 20+ (or wrap it in systemd as documented in [`deploy/systemd/README.md`](../deploy/systemd/README.md)):

   ```bash
   cd /opt/bitriver-viewer
   node .next/standalone/server.js
   ```

   The `standalone` output includes all production dependencies so no additional `npm install` step is required. Reverse proxies such as Nginx or Caddy can front the process; make sure `BITRIVER_VIEWER_ORIGIN` in the API `.env` points at the viewer URL.

For more deployment patterns—including systemd units and CDN hosting—see [`docs/viewer-deployment.md`](viewer-deployment.md).

### Option B: Deploy the container image

If you prefer containers, run the published image for the same release tag. Mount the environment file so configuration stays consistent with the standalone deployment:

```bash
docker run -d --name bitriver-viewer \
  --restart unless-stopped \
  -p 3000:3000 \
  --env-file /opt/bitriver-viewer/.env \
  ghcr.io/bitriver-live/bitriver-viewer:${BITRIVER_LIVE_VERSION}
```

When fronting the viewer with Nginx or another proxy, route `/viewer` requests to the container (or standalone server) and terminate TLS upstream.

> **Building from source?** Clone the repository and follow [`web/viewer/README.md`](../web/viewer/README.md) only when you intentionally want to modify the Next.js app or test development builds. Production installations should stay on the tagged release assets above.

## 8. Post-install checks

1. Validate services are running.

```bash
systemctl --failed
sudo systemctl status bitriver-live.service bitriver-viewer.service srs.service srs-controller.service ome.service bitriver-transcoder.service
```

2. Confirm database connectivity and migrations.

```bash
psql "postgres://stream_user:super-strong-password@localhost:5432/bitriver?sslmode=require" \
  --command "SELECT NOW(), current_user;"
```

3. Check Redis health.

```bash
redis-cli -a 'changeme' info server | head
```

4. Hit the API and health endpoints.

```bash
curl -k https://stream.example.com/healthz
curl -k https://stream.example.com/api/channels
```

5. Inspect logs for ingest services.

```bash
journalctl -u srs.service -u srs-controller.service -u ome.service -u bitriver-transcoder.service --since "-5 minutes"
```

6. Ensure TLS certificates renew automatically (`certbot renew --dry-run`) and firewall rules persist across reboots (`sudo ufw status`). Rotate secrets periodically and audit access logs.

With these steps complete the BitRiver Live stack should be ready to accept creators, ingest live streams via SRS, transcode them through the FFmpeg controller, and serve viewers via the Next.js frontend.
