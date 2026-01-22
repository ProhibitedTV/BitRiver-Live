# Production release runbook

This checklist keeps production releases consistent across the API, viewer, and
supporting services. Follow each section in order before publishing a new tag or
rolling out the artefacts to your infrastructure.

**Canonical deployment path:** Production rollouts must flow through the
repository-root `.env`, `deploy/docker-compose.yml`, and their guardrails
(`deploy/check-env.sh`, `scripts/render-ome-config.sh`). Go CLI shims and CI
workflows wrap these artefacts rather than inventing alternate deployment
paths. Set `BITRIVER_LIVE_MODE=production` in the release `.env` before
starting the API; the binary now fails fast when the mode is missing so
production-only protections (including `/metrics` guardrails) stay enabled.
Production also refuses to start without a non-zero login throttle
(`BITRIVER_LIVE_RATE_LOGIN_LIMIT`/`BITRIVER_LIVE_RATE_LOGIN_WINDOW`) so
password spray protection is always enabled in release builds.

Recent schema changes to account for:

- `0006_profile_social_links.sql` adds a `social_links` JSONB column to
  `profiles` so broadcasters can surface their external accounts. Ensure this
  migration is applied during rollout.

For the upgrade mechanics around schema migrations, safe Compose sequencing, `.env` changes, and OvenMediaEngine config regeneration, follow the upgrade essentials in [`docs/upgrades.md`](upgrades.md#upgrade-essentials-migrations-env-updates-and-ome-re-render).

## 1. Pre-release verification

Run every test suite locally (or on a staging CI run) so the GitHub release
workflow does not discover failures after the tag is pushed.

### Go unit tests

```bash
GOTOOLCHAIN=local \
  GOPROXY=off \
  GOSUMDB=off \
  go test ./... -count=1 -timeout=120s
```

### Postgres storage tests

Point `BITRIVER_TEST_POSTGRES_DSN` at an empty, migrated database and execute
the tagged suite. The helper spins up a disposable Postgres container, applies
the tracked migrations, and runs the integration tests:

```bash
./scripts/test-postgres.sh
```

### Viewer lint and integration tests

Run the viewer quality gates from the repository root. The first invocation will
install dependencies; subsequent releases can reuse the cached `node_modules`
directory.

```bash
cd web/viewer
npm install
npm run lint
npm run test:integration
```

## 2. Tag the release and trigger the workflow

1. Ensure `CHANGELOG.md` (when present) and version references are up to date.
2. Create an annotated tag that follows the `vMAJOR.MINOR.PATCH` pattern:
   ```bash
   git tag -a vX.Y.Z -m "Release vX.Y.Z"
   git push origin vX.Y.Z
   ```
3. The push triggers [`.github/workflows/release.yml`](../.github/workflows/release.yml),
   which rebuilds the Go binaries for every platform, packages the viewer
   bundle, and publishes the artefacts to the GitHub Release. Monitor the
   workflow until every job completes successfully.

### Repository secrets for the release workflow

The `verify-env` job in the release workflow renders a production-ready `.env`
file and validates it with `deploy/check-env.sh`. Configure the following
repository secrets (mirroring [`deploy/.env.example`](../deploy/.env.example))
so the job can populate every required variable and image tag:

- `BITRIVER_POSTGRES_USER`
- `BITRIVER_POSTGRES_PASSWORD`
- `BITRIVER_REDIS_PASSWORD`
- `BITRIVER_OME_API`
- `BITRIVER_LIVE_ADMIN_EMAIL`
- `BITRIVER_LIVE_ADMIN_PASSWORD`
- `BITRIVER_LIVE_SESSION_TTL` (use the same duration as `deploy/.env.example`,
  currently `168h`, unless your session policy requires a shorter window)
- `BITRIVER_LIVE_ALLOW_SELF_SIGNUP` (set to `false` in most production deploys)
- `BITRIVER_LIVE_METRICS_TOKEN` (or `BITRIVER_LIVE_METRICS_ALLOW_NETWORKS` when
  you enforce a scrape allowlist) so `/metrics` remains protected
- `BITRIVER_LIVE_RATE_LOGIN_LIMIT` (set to a non-zero value, such as `10`, and
  pair with `BITRIVER_LIVE_RATE_LOGIN_WINDOW` to cap password spray attempts)
- `BITRIVER_SRS_TOKEN`
- `BITRIVER_OME_USERNAME`
- `BITRIVER_OME_PASSWORD`
- `BITRIVER_OME_API_TOKEN` (override `BITRIVER_OME_ACCESS_TOKEN` only if the health probe must use a different token)
- `BITRIVER_OME_BIND`, `BITRIVER_OME_IP`, `BITRIVER_OME_SERVER_PORT`, and `BITRIVER_OME_SERVER_TLS_PORT`
- `BITRIVER_TRANSCODER_TOKEN`
- `BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD`
- `BITRIVER_TRANSCODER_PUBLIC_BASE_URL`
- `BITRIVER_LIVE_IMAGE_TAG`
- `BITRIVER_VIEWER_IMAGE_TAG`
- `BITRIVER_SRS_CONTROLLER_IMAGE_TAG`
- `BITRIVER_TRANSCODER_IMAGE_TAG`
- `BITRIVER_SRS_IMAGE_TAG`
- `BITRIVER_OME_IMAGE_TAG`
- `NEXT_PUBLIC_API_BASE_URL`
- `NEXT_PUBLIC_VIEWER_URL`

The release workflow persists the verified `.env` from this job and reuses it
to render the production OvenMediaEngine config. The `build` matrix now fails
if `deploy/ome/Server.generated.xml` would change when rendered for the
tagged release, preventing stale placeholders from landing in the packaged
artefacts.

## 3. Rotate credentials and validate environment files

Every deployment environment must own unique secrets. Before rolling the new
build out:

1. Copy the updated `deploy/.env.example` into the release directory or target
   host and fill in the values for Postgres, Redis, SRS, OvenMediaEngine, and
   transcoder credentials. Run `cmd/bitriver env init` when you need the tool to
   regenerate fresh secrets so none of the sample credentials from
   `deploy/.env.example` survive in production. Ensure `NEXT_PUBLIC_API_BASE_URL`
   and `NEXT_PUBLIC_VIEWER_URL` point at the public API and viewer endpoints
   users will reach (not localhost or example.com placeholders).
2. Run the guard script to confirm defaults are gone and service URLs match the
   target environment. The validator now fails when any sample credential from
   `deploy/.env.example` remains in the release `.env`:
   ```bash
   deploy/check-env.sh
   ```
   The release workflow surfaces this output in the deploy logs and fails when
   any OvenMediaEngine URLs, bind addresses, or ports point at loopback
   addresses, placeholders, or are missing.
   The same preflight now errors when `/metrics` lacks
   `BITRIVER_LIVE_METRICS_TOKEN`/`BITRIVER_LIVE_METRICS_ALLOW_NETWORKS` or the
   login throttling floor (`BITRIVER_LIVE_RATE_LOGIN_LIMIT`) is missing, so
   release deployments match the protections enforced by the runtime binary.
   Postgres DSNs in production must use TLS; keep `sslmode` at `require` or
   `verify-full` on both `BITRIVER_LIVE_POSTGRES_DSN` and
   `BITRIVER_LIVE_SESSION_POSTGRES_DSN`, and append
   `sslrootcert=/certs/postgres-ca.pem` (or a similar mounted path) when the
   database presents a private CA. The validator rejects `sslmode=disable` for
   any host other than the local Compose `postgres` service so plaintext
   connections cannot slip into a release.
3. Verify the rendered OvenMediaEngine config matches the image tag in `.env`
   before cutting the release tag or starting Compose:
   ```bash
   ./scripts/render-ome-config.sh --check
   ```
   The guard fails when `deploy/ome/Server.generated.xml` was rendered for a
   different `BITRIVER_OME_IMAGE_TAG`, ensuring the preflight stays in sync
   with the tag baked into the container image.
3. For systemd-based installs, refresh the `.env` files under `/opt/bitriver-*`
   and restart the services only after the script reports success. Ensure any
   container image tags (`BITRIVER_LIVE_IMAGE_TAG`, `BITRIVER_VIEWER_IMAGE_TAG`,
   etc.) match the newly published release.

## 4. Confirm ingest and object storage configuration

Review [`docs/advanced-deployments.md`](advanced-deployments.md) and verify the
following before rollout:

- SRS, OvenMediaEngine, and transcoder configuration directories point at the
  release you are deploying, and image tags match `vX.Y.Z`.
- Object storage variables (`BITRIVER_LIVE_OBJECT_*`) reference the intended
  endpoint, credentials, bucket, and lifecycle policies.
- Recording retention windows (`BITRIVER_LIVE_RECORDING_RETENTION_*`) align with
  the business requirements for VOD publishing and archival.

## 5. Post-release smoke checks

Once the artefacts are live:

1. Verify the API reports the new version and serves the admin UI without
   console errors.
2. Load the viewer at `/viewer`, confirm linted assets are present, and stream a
   test channel end-to-end (RTMP ingest → HLS playback).
3. Inspect the database to ensure migrations completed and new tables/columns
   exist.
4. Upload a short VOD to confirm object storage credentials, prefixes, and
   retention windows are honoured.
5. Rotate any temporary credentials created during testing and archive the
   release artefacts in your asset registry for rollback.
