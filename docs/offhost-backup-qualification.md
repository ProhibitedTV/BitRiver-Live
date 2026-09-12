# Off-host backup qualification

This runbook covers the remaining scheduled/off-host freshness evidence for
production-release issue #1299. It verifies the **existing** PostgreSQL backup
contract produced by `scripts/backup-postgres.sh`; it does not introduce a
second backup format or scheduler.

The supported single-host recovery defaults remain:

- RPO target: 24 hours.
- RTO target: 2 hours.
- Restore rehearsal: at least every 30 days and after schema-heavy releases.

## What this proof verifies

`scripts/qualify_offhost_backup.py` is read-only. Against one configured
S3-compatible bucket/prefix it:

1. Lists all `bitriver-postgres-*.sql.gz` backup assets.
2. Refuses any incomplete remote set. Every timestamp must have an archive,
   `manifest.json`, and `.sha256` companion.
3. Requires the configured minimum retained complete sets (default: 3).
4. Downloads only the newest complete trio and verifies the checksum file
   against the downloaded archive and manifest bytes.
5. Verifies the `bitriver.postgres-backup/v1` manifest, archive name/hash/size,
   exact release/commit identity, migration fingerprint, and non-empty row-count
   evidence.
6. Measures backup age from manifest `createdAt` and fails when it exceeds the
   configured RPO threshold (default: 86,400 seconds).
7. Writes one `bitriver.offhost-backup-proof/v1` JSON report without AWS
   credentials, database credentials, or backup payload contents.

The verifier never deletes, prunes, uploads, or rewrites remote objects.

## Production-like qualification

Configure the existing scheduled producer first. The Compose example is
`deploy/docker-compose.backups.yml`; Helm deployments can use the existing
backup CronJob. In either case, enable the canonical S3-compatible upload path
in `scripts/backup-postgres.sh` and use an off-host bucket independent of the
BitRiver host's local backup volume.

Run the scheduler through at least three successful backup cycles so remote
retention is observable. Then run the verifier from a trusted operator host
with AWS CLI credentials provided through the normal AWS credential chain:

```bash
python3 scripts/qualify_offhost_backup.py \
  --bucket "$BITRIVER_BACKUP_UPLOAD_BUCKET" \
  --prefix "${BITRIVER_BACKUP_UPLOAD_PREFIX:-bitriver-live/postgres}" \
  --region "${BITRIVER_BACKUP_UPLOAD_REGION:-us-east-1}" \
  --expected-release v1.2.3-rc.XX \
  --expected-commit 0123456789abcdef0123456789abcdef01234567 \
  --max-age-seconds 86400 \
  --min-complete-sets 3 \
  --report ./offhost-backup-proof.json
```

For MinIO or another S3-compatible service, add the endpoint used by the
scheduler:

```bash
  --endpoint-url "https://object-storage.example.invalid"
```

Do not pass credentials on this command line. `aws` obtains them from its
standard environment/profile/role configuration.

## Failure evidence

A release qualification must include failure behavior, not only a happy-path
upload. Use a disposable/test prefix or bucket and prove all of the following
without touching production backups:

- Break remote access (for example, invalid test-bucket permissions or an
  unreachable disposable endpoint) and confirm the scheduled producer exits
  non-zero instead of reporting success.
- Confirm the verifier rejects any partial remote trio left by a failed upload.
- Restore remote access, run the scheduled producer again, and confirm a fresh
  complete trio qualifies.
- Remove the local disposable backup copy after a successful upload, rerun this
  verifier from another host/context, and confirm the remote trio still
  qualifies. This demonstrates that qualification does not depend on the local
  backup volume.
- Retain at least the configured minimum complete remote sets. The current
  local `prune-backups.sh` policy does not delete remote objects; remote
  retention/lifecycle remains an operator storage policy and must not expire
  backups more aggressively than the supported recovery objective.

Record the failed scheduler exit/log excerpt separately. Do not put provider
credentials, database passwords, signed URLs, or backup payloads into release
evidence.

## Evidence to retain

For the exact release candidate attach or link:

- the secret-safe `bitriver.offhost-backup-proof/v1` report;
- scheduler timestamps showing the backup was produced by the scheduled path;
- the failure-path result proving remote upload errors are visible/non-zero;
- confirmation that the local disposable copy was removed before the remote
  re-verification;
- the independent restore/recovery report already required by the production
  release process.

A passing off-host report proves remote-set completeness, integrity, freshness,
and the configured retained-set minimum. It **does not** by itself prove an
S3 provider's durability SLA, a restore, encrypted host-state recovery, alert
delivery, or stable-release promotion. Those claims require their own existing
release evidence.

## Focused tests

The verifier has deterministic unit coverage and is also invoked from the
`scripts` Go test package, so the normal repository Go gate exercises it on
Linux, macOS, and Windows:

```bash
python3 -m unittest scripts.qualify_offhost_backup_test
go test ./scripts -run TestOffhostBackupQualificationPythonSuite -count=1
```

Before merging repository changes, also run the canonical gate:

```bash
./scripts/verify.sh
```
