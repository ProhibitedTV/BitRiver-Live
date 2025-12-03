# Production release runbook

This checklist keeps production releases consistent across the API, viewer, and
supporting services. Follow each section in order before publishing a new tag or
rolling out the artefacts to your infrastructure.

Recent schema changes to account for:

- `0006_profile_social_links.sql` adds a `social_links` JSONB column to
  `profiles` so broadcasters can surface their external accounts. Ensure this
  migration is applied during rollout.

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
- `BITRIVER_SRS_TOKEN`
- `BITRIVER_OME_USERNAME`
- `BITRIVER_OME_PASSWORD`
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

## 3. Rotate credentials and validate environment files

Every deployment environment must own unique secrets. Before rolling the new
build out:

1. Copy the updated `deploy/.env.example` into the release directory or target
   host and fill in the values for Postgres, Redis, SRS, OvenMediaEngine, and
   transcoder credentials. Ensure `NEXT_PUBLIC_API_BASE_URL` and
   `NEXT_PUBLIC_VIEWER_URL` point at the public API and viewer endpoints users
   will reach (not localhost or example.com placeholders).
2. Run the guard script to confirm defaults are gone and service URLs match the
   target environment:
   ```bash
   deploy/check-env.sh
   ```
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
