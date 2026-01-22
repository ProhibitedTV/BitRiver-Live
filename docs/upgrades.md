# Upgrading BitRiver Live

Use this guide when you are moving an existing deployment to a newer release. It assumes you are running the supported Docker Compose bundle (`deploy/docker-compose.yml`) with the repository-root `.env` file and the generated OvenMediaEngine config in `deploy/ome/Server.generated.xml`.

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
