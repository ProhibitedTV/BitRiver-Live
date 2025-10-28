# BitRiver Live Viewer

This directory hosts the public-facing Next.js application that lets viewers browse channels and watch streams.

## Build command

Install dependencies and produce a production build before packaging or deploying the viewer:

```bash
npm ci
npm run build
```

Setting `NEXT_VIEWER_BASE_PATH=/viewer` ensures the generated assets expect to live under `/viewer` when proxied through the Go API. The build respects `NEXT_PUBLIC_API_BASE_URL`; leave it empty to target the same origin as the admin API or set it to a fully-qualified URL during multi-origin deployments.

## Development

Run `npm run dev` to start the local development server on port 3000. Pair it with the Go API running on `http://localhost:8080` and export `NEXT_PUBLIC_API_BASE_URL=http://localhost:8080` so the client uses the correct backend during development.
