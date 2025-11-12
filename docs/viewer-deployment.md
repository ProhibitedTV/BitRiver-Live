# Deploying the BitRiver Viewer bundle

The release workflow now publishes a pre-built Next.js viewer bundle alongside the
server binaries. Each GitHub Release exposes an artifact named
`bitriver-viewer-<tag>.tar.gz` that contains the output of `npm run build` from
`web/viewer`.

The archive includes:

- `.next/standalone` with the production Node.js server
- `.next/static` assets
- the `public/` directory
- `package.json`, `package-lock.json`, and `next.config.js`

This structure mirrors what `next build` produces locally, allowing you to run
the viewer behind a reverse proxy or host it from static infrastructure.

## Downloading the bundle

1. Navigate to the GitHub Release page for the version you want to deploy.
2. Download `bitriver-viewer-<tag>.tar.gz` to the target host.
3. Extract the bundle. The archive contains a single top-level directory named
   `bitriver-viewer/`.

```bash
sudo mkdir -p /opt/bitriver-viewer
sudo tar -xzvf bitriver-viewer-v1.2.3.tar.gz -C /opt/bitriver-viewer --strip-components=1
```

> Replace `v1.2.3` with the actual tag name you downloaded.

## Running the viewer behind Nginx

1. Install Node.js 20 (or later) on the host.
2. Configure the runtime base path if you plan to serve the viewer from a
   sub-path. The bundle respects the `NEXT_VIEWER_BASE_PATH` environment
   variable.
3. Launch the standalone Next.js server:

```bash
cd /opt/bitriver-viewer
NEXT_VIEWER_BASE_PATH=/viewer node .next/standalone/server.js
```

The `standalone` output bundles all production dependencies, so additional
`npm install` steps are not required.

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

## Hosting via GitHub Pages

If your organization enables the optional GitHub Pages publication step, the
workflow uploads the same build output using `actions/upload-pages-artifact` and
`actions/deploy-pages`. In that configuration the viewer is accessible at the
Pages URL announced in the Release notes.

When hosting from Pages (or any other CDN) make sure the `NEXT_VIEWER_BASE_PATH`
value matches the path segment you serve the site from so that asset URLs
resolve correctly.
