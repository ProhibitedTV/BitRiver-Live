# Deploying the BitRiver Viewer bundle

The release workflow now publishes a pre-built Next.js viewer bundle alongside the
server binaries. Each GitHub Release exposes an artifact named
`bitriver-viewer-<tag>.tar.gz` that contains the output of `npm run build` from
`web/viewer`. A matching container image is also published at
`ghcr.io/prohibitedtv/bitriver-viewer:<tag>` for teams that prefer Docker.

The archive includes:

- `.next/standalone` with the production Node.js server
- `.next/static` assets
- the `public/` directory
- `package.json`, `package-lock.json`, and `next.config.js`

This structure mirrors what `next build` produces locally, allowing you to run
the standalone Node server behind a reverse proxy without cloning the
repository. BitRiver does not publish a static-export or GitHub Pages build.

## Downloading the bundle

1. Navigate to the GitHub Release page for the version you want to deploy.
2. Download `bitriver-viewer-<tag>.tar.gz` to the target host.
3. Extract the bundle. The archive contains a single top-level directory named
   `bitriver-viewer/`.

```bash
sudo mkdir -p /opt/bitriver-viewer
sudo tar -xzvf bitriver-viewer-v1.2.3-rc.13.tar.gz -C /opt/bitriver-viewer --strip-components=1
```

> Keep the viewer bundle, API, and first-party image tag aligned. The current
> public candidate is `v1.2.3-rc.13`; verify its signed release-set entry before
> extracting or pulling it.

The extraction creates `/opt/bitriver-viewer/.next/standalone/`,
`/opt/bitriver-viewer/.next/static/`, and `/opt/bitriver-viewer/public/`. Keep
that layout intact—systemd units and reverse proxies expect the Next.js server
to run from the `standalone` directory and serve static assets from `.next/static`. Those systemd units are Linux-only and intended for advanced operators; the default path for all platforms is to run the bundled Docker Compose stack via `go run ./cmd/bitriver compose up`.

Create `/opt/bitriver-viewer/.env` with the configuration your deployment needs.
A minimal environment looks like:

```ini
NEXT_PUBLIC_API_BASE_URL=https://stream.example.com
NEXT_VIEWER_BASE_PATH=/viewer
PORT=3000
HOSTNAME=0.0.0.0
```

The standalone server reads the `.env` file automatically when you start it via
`node .next/standalone/server.js`.

## Running the viewer behind Nginx

1. Install Node.js 24 LTS on the host. Other Node majors are outside the supported viewer baseline.
2. Configure the runtime base path if you plan to serve the viewer from a
   sub-path. The bundle respects the `NEXT_VIEWER_BASE_PATH` environment
   variable.
3. Launch the standalone Next.js server:

```bash
cd /opt/bitriver-viewer
NEXT_VIEWER_BASE_PATH=/viewer node .next/standalone/server.js
```

The `standalone` output bundles all production dependencies, so additional
`npm install` steps are not required. If you wrap the process with systemd,
point the unit's working directory at `/opt/bitriver-viewer` and load the
environment file with `EnvironmentFile=/opt/bitriver-viewer/.env`.

4. Point Nginx at the running server. A minimal reverse-proxy definition looks
   like this:

```nginx
server {
    listen 443 ssl;
    server_name viewer.example.com;

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

You can wrap the Node.js process in a systemd unit to keep it running across
reboots.

## Running the container image

When you prefer Docker or another container runtime, pull the image tagged with
the same release as the API. Mount the environment file created above and map
port 3000 (or your chosen listener):

```bash
docker run -d --name bitriver-viewer \
  --restart unless-stopped \
  -p 3000:3000 \
  --env-file /opt/bitriver-viewer/.env \
  ghcr.io/prohibitedtv/bitriver-viewer:v1.2.3-rc.13@sha256:9da7986407258fbf503d6a4f78ff812e4364815063c426d8f9060a89ba753f5f
```

Use Docker Compose or Kubernetes manifests when you need to manage replicas, but
keep the environment variables consistent so the viewer points at the correct
API origin.

> **Need to build from source?** Clone the repository and follow
> [`web/viewer/README.md`](../web/viewer/README.md) only when you are modifying
> the Next.js app. Production deployments should rely on the release bundle or
> container to stay aligned with the API binaries.
