# Upgrading BitRiver Live

Use this guide when you are moving an existing deployment to a newer release. It assumes you are running the supported Docker Compose bundle (`deploy/docker-compose.yml`) with the repository-root `.env` file and the generated OvenMediaEngine config in `deploy/ome/Server.generated.xml`.

## Upgrade essentials: migrations, `.env` updates, and OME re-render

Use this section as the concise checklist when you need to ensure schema, environment, and OvenMediaEngine configuration stay aligned during upgrades. The full, step-by-step flow follows in the numbered sections below.

### Schema migrations

- **Compose path (default):** `docker compose up` always runs the `postgres-migrations` helper before the API starts, applying every SQL file in `deploy/migrations/`.
- **External Postgres:** apply the migrations in `deploy/migrations/` with your database tooling before starting the API, and keep the schema in sync with the release notes under `docs/releases/`.

### Safe upgrade flow (summary)

1. Stop the stack (keep volumes): `docker compose -f deploy/docker-compose.yml down`.
2. Refresh `.env` from `deploy/.env.example` and validate it with `deploy/check-env.sh`.
3. Re-render OME config: `./scripts/render-ome-config.sh --check || ./scripts/render-ome-config.sh --force`.
4. Optionally run migrations explicitly: `docker compose -f deploy/docker-compose.yml run --rm postgres-migrations` (add `-T` on Windows shells without a TTY).
5. Start everything again: `docker compose -f deploy/docker-compose.yml up -d`.

### `.env` changes

> **Upgrade callout (OME healthcheck auth contract):** The OME healthcheck in both Compose and Helm sends `AccessToken: <token>` first (token `${BITRIVER_OME_HEALTHCHECK_TOKEN:-${BITRIVER_OME_ACCESS_TOKEN:-$BITRIVER_OME_API_TOKEN}}`), then automatically retries with `Authorization: Bearer <token>`. Keep `<Managers><API><AccessToken>` in `deploy/ome/Server.generated.xml` aligned with that value after any token change. Compose now probes in this order: AccessToken header, basic auth (when username/password are present), then Authorization Bearer header. Helm keeps its existing compatibility toggle behavior.

- Compare your existing `.env` against `deploy/.env.example` whenever you upgrade, then add new keys or defaults before restarting.
- Run `deploy/check-env.sh` to confirm there are no sample credentials or missing required variables.

### OME re-render steps

- When you change `BITRIVER_OME_*` variables or update `deploy/ome/Server.xml`, regenerate `deploy/ome/Server.generated.xml` with:
  ```bash
  ./scripts/render-ome-config.sh --check || ./scripts/render-ome-config.sh --force
  ```
- The Compose `ome-config` preflight runs on every `docker compose up`, but a manual render is still recommended after `.env` edits so you can catch config issues before bringing the stack online.

## 1. Review release notes and incompatibilities

Before you pull new images or rebuild containers, read the release notes under `docs/releases/` (for example, [`docs/releases/v1.0.0.md`](releases/v1.0.0.md)). Any breaking changes, new required environment variables, or schema changes are called out there so you can plan the upgrade safely.

If a release introduces incompatibilities, document them in `docs/releases/` and follow the mitigation steps before proceeding.

## 2. Database migrations (when/where they run)

The Compose bundle ships a dedicated `postgres-migrations` helper that applies every SQL file in `deploy/migrations/` against the Postgres container before the API starts. The main `bitriver-live` service depends on this helper (and on a healthy Postgres container), so migrations always run during `docker compose up` when you use the standard Compose bundle.

If you run Postgres outside the bundle, apply the SQL files in `deploy/migrations/` using your preferred tooling **before** starting the API. The API expects the schema to match the latest migration set.

## 3. Safe upgrade flow for Docker Compose

Follow this flow to avoid partial upgrades:

1. **Pull or unpack the new release** (git pull, new release tarball, or updated installer bundle).
2. **Stop the running services (keep data volumes):**
   ```bash
   docker compose -f deploy/docker-compose.yml down
   ```
3. **Refresh the `.env` file:**
   - Compare your existing `.env` to `deploy/.env.example` and add any new variables or defaults.
   - Run the environment guard script:
     ```bash
     deploy/check-env.sh
     ```
4. **Re-render the OvenMediaEngine config** if the `.env` or OME template changed:
   ```bash
   ./scripts/render-ome-config.sh --check || ./scripts/render-ome-config.sh --force
   ```
   Compose also runs the `ome-config` helper on every start, so a clean `docker compose up` will regenerate `deploy/ome/Server.generated.xml` from the current `.env` as needed.
5. **Run migrations explicitly (optional but safe):**
   ```bash
   docker compose -f deploy/docker-compose.yml up -d postgres
   docker compose -f deploy/docker-compose.yml run --rm postgres-migrations
   ```
   On Windows shells without a TTY, add `-T` to the `docker compose run` command to disable pseudo-TTY allocation.
6. **Restart the stack:**
   ```bash
   docker compose -f deploy/docker-compose.yml up -d
   ```

The `postgres-migrations` helper also runs on step 6, so skipping step 5 is acceptable if you prefer the single `up -d` flow; the explicit run is helpful if you want to confirm migrations before the API comes back online.

## 4. Handling `.env` changes and OME template re-render

- Update `.env` any time new release variables appear in `deploy/.env.example` or release notes. Keep credentials and tags current before bringing containers back up.
- If you touch `BITRIVER_OME_*` values or update the `deploy/ome/Server.xml` template, re-render `deploy/ome/Server.generated.xml` with the script above or rely on the `ome-config` preflight in Compose to regenerate it at startup.

## 5. Post-upgrade validation

After the stack is running:

- Confirm containers are healthy (`docker compose ps`).
- Verify the API `/readyz` and `/healthz` endpoints return success.
- Stream a short test channel end-to-end to confirm ingest and playback.

If issues appear, re-check the release notes for incompatibilities and confirm migrations completed cleanly before rollback.
