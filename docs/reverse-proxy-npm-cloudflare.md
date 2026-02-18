# Cloudflare → Nginx Proxy Manager → BitRiver Live

Use this guide when BitRiver Live runs behind **Cloudflare** and **Nginx Proxy Manager (NPM)** instead of the bundled Caddy TLS proxy. It covers required environment values, recommended DNS/TLS posture, NPM forwarding rules, and a post-deploy validation checklist.

## 1) Required `.env` values for external domain deployments

When users access BitRiver through an external domain, these values must match that public URL so CORS, redirects, and playback URLs stay coherent.

```dotenv
# Public viewer URL shown in links and consumed by the Next.js app.
NEXT_PUBLIC_VIEWER_URL=https://stream.example.com/viewer

# Public origin that should be allowed to render the viewer.
BITRIVER_VIEWER_ORIGIN=https://stream.example.com

# Admin/control-centre browser origin allowlist (comma-separated when multiple).
BITRIVER_LIVE_ADMIN_CORS_ORIGINS=https://stream.example.com

# Viewer/API browser origin allowlist (comma-separated when multiple).
BITRIVER_LIVE_VIEWER_CORS_ORIGINS=https://stream.example.com

# Public HTTP(S) base URL for transcoder output manifests/segments.
BITRIVER_TRANSCODER_PUBLIC_BASE_URL=https://stream.example.com/live
```

Notes:

- Keep schemes explicit (`https://...`) for all origin allowlists.
- If admin and viewer run on different hostnames, include both exact origins in the corresponding CORS variables.
- After updating `.env`, re-apply with `docker compose up -d`.

## 2) Recommended DNS + TLS settings

For Cloudflare-managed domains:

1. Create DNS records for your BitRiver hostname (for example `stream.example.com`) pointing at the NPM host.
2. Keep records proxied (**orange cloud**) so Cloudflare edge shielding/WAF applies.
3. Set SSL/TLS encryption mode to **Full (strict)**.
4. Install a **Cloudflare Origin Certificate** (or another trusted cert valid for your origin hostname) on NPM so Cloudflare can verify the origin certificate chain.
5. Keep “Always Use HTTPS” enabled at Cloudflare (or enforce redirects in NPM).

Why this matters:

- `Full (strict)` prevents downgrade/invalid-cert MITM between Cloudflare and your origin.
- Origin certificates avoid exposing a publicly trusted private key on the application host.

## 3) NPM host configuration (paths + websocket support)

Create one **Proxy Host** in Nginx Proxy Manager:

- **Domain Names:** `stream.example.com`
- **Scheme / Forward Hostname / Port:** `http` → your BitRiver host/IP → `8080` (or whatever API entry port you expose)
- **Websockets Support:** enabled
- **Block Common Exploits:** enabled
- **Access List:** optional (typically public viewer; restrict admin paths via network controls as needed)

Then configure path forwarding so BitRiver routing stays intact:

### Base route (`/`)

- Forward to BitRiver API service (same upstream as above).
- This serves control-centre/API root behaviour expected by the stack.

### API route (`/api`)

- Add custom location `/api` forwarding to the same BitRiver API upstream.
- Preserve host and forwarding headers (NPM defaults are usually sufficient).

### Viewer route (`/viewer`)

- Add custom location `/viewer` forwarding to the same BitRiver API upstream.
- The BitRiver API reverse-proxies viewer assets under `/viewer` in the default deployment.

### Websocket routes

BitRiver uses websocket upgrades for live chat and related realtime flows. With NPM websocket support enabled, upgrades should pass through automatically for `/api` and `/viewer` traffic. If you use advanced custom Nginx snippets, ensure they do **not** strip:

- `Upgrade`
- `Connection`
- `Sec-WebSocket-*`

## 4) Validation checklist

After DNS and proxy changes, validate end-to-end behaviour from a browser session:

1. **Login + session**
   - Open `https://stream.example.com/`.
   - Sign in as admin.
   - Confirm authenticated navigation works after a hard refresh.
2. **Viewer loading + playback**
   - Open a channel under `/viewer/...`.
   - Start/publish a stream and verify playback starts.
   - Confirm segment/manifests load from `BITRIVER_TRANSCODER_PUBLIC_BASE_URL` without mixed-content warnings.
3. **Chat websocket behaviour**
   - Open chat on viewer and admin surfaces.
   - Post messages from one client and confirm near-realtime delivery on another.
   - In browser devtools/network, verify websocket connections complete with `101 Switching Protocols`.
4. **CORS sanity**
   - Ensure browser console has no CORS errors for admin or viewer requests.
   - If errors appear, re-check `BITRIVER_LIVE_ADMIN_CORS_ORIGINS` and `BITRIVER_LIVE_VIEWER_CORS_ORIGINS`.

## 5) Related docs

- Quickstart baseline: [`docs/quickstart.md`](quickstart.md)
- Additional production options: [`docs/advanced-deployments.md`](advanced-deployments.md)
- Compose/env source of truth: [`deploy/docker-compose.yml`](../deploy/docker-compose.yml)
