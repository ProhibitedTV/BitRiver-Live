# BitRiver Live

![BitRiver Live banner](./bitriver-live-banner-text.png)

BitRiver Live is a self-hosted live-streaming stack for teams that want one clear deployment bundle instead of a grab bag of loosely connected services.

It packages a Go control-plane API, a Next.js viewer, ingest and transcoding services, and stateful data services into one Docker Compose deployment (`deploy/docker-compose.yml`) driven by a single root `.env` file.

## 1) What this project is

### Mental model

Think of BitRiver Live as **one deployment bundle** with multiple ways to launch it:

- Installer launcher (`bitriver-live`)
- Source launcher (`go run ./cmd/bitriver quickstart`)
- Shell wrappers (`scripts/quickstart.sh`, `scripts/quickstart.ps1`)

Each entrypoint runs the same deployment pipeline:

1. Doctor checks
2. `.env` bootstrap/validation
3. OvenMediaEngine config render
4. Postgres migrations
5. `docker compose up`
6. readiness checks
7. admin bootstrap

### What it is not

- Not a managed SaaS service.
- Not a single-process binary without external services.
- Not a Kubernetes-first deployment (Compose is the canonical path in this repo).

### Non-goals (for now)

BitRiver Live is focused on a reliable, self-hosted live stack with a clear deployment contract. To keep that scope honest, it does **not** currently aim to provide:

- A built-in global CDN footprint.
- Twitch-scale trust & safety operations or moderation staffing models.
- Fully automatic abuse detection/enforcement with no operator oversight.
- Infinite, no-planning-required horizontal scale.

## 2) Current status

### Works today

- Single-host Docker Compose deployment of:
  - API/control centre (`cmd/server`)
  - Viewer (`web/viewer`)
  - SRS ingest + controller
  - OvenMediaEngine playback
  - FFmpeg transcoder (`cmd/transcoder`)
  - Postgres + Redis
- Installer and source launchers that converge on the same Compose contract.
- Admin bootstrap and health endpoints (`/readyz`, `/healthz`, `/api/status`).

### Partial / needs operator judgment

- Advanced topologies (reverse proxies, scaling patterns, object-storage-heavy retention) are documented, but require manual infrastructure work.
- Cross-platform UX wrappers exist, but operational behavior is still defined by the shared Compose pipeline.

### Known sharp edges (documentation honesty)

- This repository separates docs into three zones: Product (stable run/deploy docs), Ops/Runbooks (operational truth), and Labs (experimental planning docs under `docs/labs/`). Labs content is non-binding and not part of shipped guarantees.
- First-time operators still need to validate host-level Docker capacity and networking; quickstart cannot fix host misconfiguration automatically.

## 3) Quick start (real)

Use this for the fastest path to a running stack.

### Assumptions

- Docker with Compose V2 is installed and running.
- You have enough disk for images/volumes (15GB+ is a practical minimum from project docs).
- You are running from either:
  - an installed launcher package, or
  - a cloned repository root.

### Option A: packaged launcher (recommended for operators)

```bash
bitriver-live
```

What this does:

- Creates `<launcher-root>/.env` from `deploy/.env.example` if missing.
- Runs the canonical deployment pipeline above.
- Starts services from `deploy/docker-compose.yml`.

### Option B: source checkout

```bash
go run ./cmd/bitriver quickstart
```

Wrapper equivalents (scripts are thin shims around the same CLI command):

```bash
./scripts/quickstart.sh
./scripts/quickstart.ps1
```

### First login / verification

- Verify the install: `bitriver smoke` (or `go run ./cmd/bitriver smoke`).
- Open the listener shown by quickstart. When the viewer proxy is configured (the default compose shape), the host root lands in the public viewer flow and the control centre lives at `/admin`.
- Use generated/admin credentials from quickstart output.
- Confirm health in the `/admin` Overview dashboard (or query `/api/status`).

### Useful day-2 operations commands

```bash
# service status
docker compose -f deploy/docker-compose.yml ps

# logs
docker compose -f deploy/docker-compose.yml logs -f

# stop without deleting data
go run ./cmd/bitriver compose down --file deploy/docker-compose.yml

# restart after .env changes
go run ./cmd/bitriver compose up --file deploy/docker-compose.yml
```

## 4) Core workflows

### A) Deploy or upgrade the stack

**Input:** `deploy/docker-compose.yml` + root `.env`  
**Action:** run quickstart or a compose wrapper  
**Output:** running containers, migrated database, rendered OME config (`deploy/ome/Server.generated.xml`)

Primary files:

- `cmd/bitriver/*` (deployment orchestration)
- `deploy/docker-compose.yml`
- `deploy/.env.example`
- `scripts/quickstart.sh`

### B) Ingest stream to viewer playback

**Input:** stream pushed to the SRS RTMP endpoint (`BITRIVER_SRS_RTMP_PORT`)  
**Action:** SRS ingest → transcoder outputs → OME playback orchestration  
**Output:** playback URLs/pages are served through the API and loaded by the viewer app

Primary files/services:

- `deploy/srs/` and `cmd/srs-controller`
- `cmd/transcoder`
- `deploy/ome/`
- `web/viewer`

### C) Operator/admin management

**Input:** admin credentials + control centre UI/API  
**Action:** authenticate, then manage channels/users/settings  
**Output:** persisted state in Postgres/Redis and updated API responses

Primary code areas:

- `cmd/server`
- `internal/api`
- `internal/service`
- `internal/storage`

## 5) Configuration (minimal)

Start with the defaults in `deploy/.env.example`.

Change values only when needed:

- `BITRIVER_LIVE_PORT`: move API/control-centre host port.
- `NEXT_PUBLIC_API_BASE_URL`, `NEXT_PUBLIC_VIEWER_URL`, `BITRIVER_VIEWER_ORIGIN`: required when using real domains or reverse proxies.
- `BITRIVER_LIVE_ADMIN_EMAIL`, `BITRIVER_LIVE_ADMIN_PASSWORD`: production-safe admin credentials.
- `BITRIVER_POSTGRES_*`, `BITRIVER_REDIS_PASSWORD`: database/cache credentials.
- `BITRIVER_DEPLOY_IMAGE_SOURCE`: choose image source mode (`pull` recommended for production deployments).

If you edit OME-related env keys, re-render config before restarting:

```bash
go run ./cmd/bitriver ome render --force --env-file ./.env
```

## 6) Failure modes and gotchas

- **Docker not available**  
  Symptom: quickstart doctor fails early.  
  Fix: start Docker Desktop or Docker Engine; confirm `docker compose` works.

- **Default/sample secrets left in `.env`**  
  Symptom: env validation/deploy checks fail or insecure deployment.  
  Fix: rotate required credentials and rerun quickstart.

- **OME config drift after env edits**  
  Symptom: ingest/playback auth mismatch or OME startup failures.  
  Fix: rerender `deploy/ome/Server.generated.xml` then restart Compose.

- **Host port conflicts (5432/6379/8080/1935/etc.)**  
  Symptom: Compose service start failures.  
  Fix: change `*_PORT` values in `.env` or free occupied ports.

- **Reverse proxy websocket/CORS mistakes**  
  Symptom: viewer/chat/login failures behind external domains.  
  Fix: follow `docs/reverse-proxy-npm-cloudflare.md` and `docs/advanced-deployments.md` exactly.

- **Unsupported expectation: “no Docker dependencies”**  
  The current design intentionally centers on Docker Compose; bare-metal custom layouts require manual adaptation.

## 7) Design notes (brief)

- Backend architecture is intentionally layered (`cmd -> internal/app -> internal/{api,service,domain} -> adapters`) to keep business rules reusable and testable. See `docs/architecture.md`.
- Deployment is intentionally standardized to **one Compose + one `.env` contract** to avoid divergent runbooks across platforms.
- Operational tradeoff: easier onboarding and repeatability, with less flexibility than fully custom multi-cluster setups.

## Production status

BitRiver Live is **production-capable today for operator-managed, single-host deployments** using the documented Docker Compose contract.

- **Production-capable now (current scope):** single-host operator-managed deployments, with operators owning host hardening, backups/restores, and upgrade execution. See [`docs/production-single-host.md`](docs/production-single-host.md), [`docs/upgrades.md`](docs/upgrades.md), and [`docs/security.md`](docs/security.md).
- **Recommended/optional overlays:** monitoring and alerting overlays (`deploy/compose.monitoring.yml`) plus runtime limit overlays (`deploy/compose.limits.yml`) are recommended for production operations, but optional depending on your environment maturity. See [`docs/monitoring.md`](docs/monitoring.md).
- **Roadmap (not current GA scope):** HA/multi-host topologies are not claimed as generally available in this repo's deployment contract yet; treat them as planned evolution rather than present-day guarantees.

- Production release process: [`docs/production-release.md`](docs/production-release.md)
- Lifecycle and guarantee policy: [`docs/production-status.md`](docs/production-status.md)

## Additional documentation

- Quickstart details: [`docs/quickstart.md`](docs/quickstart.md)
- Smoke test command: [`docs/smoke-test.md`](docs/smoke-test.md)
- Advanced deployments and reverse proxies: [`docs/advanced-deployments.md`](docs/advanced-deployments.md), [`docs/reverse-proxy-npm-cloudflare.md`](docs/reverse-proxy-npm-cloudflare.md)
- Operations/backups/restores: [`docs/operations.md`](docs/operations.md)
- Security hardening guide (operator entrypoint): [`docs/security.md`](docs/security.md)
- Secrets hardening: [`docs/secrets-hardening.md`](docs/secrets-hardening.md)
- Security guardrails: [`docs/security.md`](docs/security.md)
- Upgrades: [`docs/upgrades.md`](docs/upgrades.md)
- Testing: [`docs/testing.md`](docs/testing.md), [`docs/testing-status.md`](docs/testing-status.md)
- Architecture contract: [`docs/architecture.md`](docs/architecture.md)
- Manual frontend QA checklist: [`web/manual-qa.md`](web/manual-qa.md)
- Labs (planning / non-binding): [`docs/labs/README.md`](docs/labs/README.md), [`docs/labs/product-roadmap.md`](docs/labs/product-roadmap.md), [`docs/labs/cross-platform-plan.md`](docs/labs/cross-platform-plan.md)

## Development and test commands

Go (offline module policy in this repository):

```bash
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./... -count=1 -timeout=120s
```

Viewer:

```bash
cd web/viewer
npm install
npm run lint
npm run test
npm run test:playwright
```

## NOTE

If you find behavior in code that conflicts with docs, treat code as the immediate source of truth for runtime behavior and open a docs follow-up issue/PR. This repository has Product/Ops docs and Labs planning docs; when in doubt, follow operational runbooks in `docs/` and treat `docs/labs/` as planning/non-binding context.
