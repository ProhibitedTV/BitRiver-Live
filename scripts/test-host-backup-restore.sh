#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
workdir="$(mktemp -d)"
source_root="$workdir/source-host"
restore_root="$workdir/restored-host"
backup_dir="$workdir/backups"
postgres_dir="$workdir/postgres"
object_root="$workdir/external-objects"
passphrase_file="$workdir/recovery-passphrase"
wrong_passphrase_file="$workdir/wrong-passphrase"
object_inventory="$workdir/object-inventory.json"
fake_bin="$workdir/fake-bin"
release="v1.2.3-rc.21"
commit="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
timestamp="20260815T010203Z"
secret_sentinel="host-recovery-secret-sentinel-5dfe3ac1"

cleanup() {
  rm -rf -- "$workdir"
}
trap cleanup EXIT

fail() {
  echo "Host backup/restore test failed: $*" >&2
  exit 1
}

mkdir -p \
  "$source_root/etc/bitriver-live/deploy/ome" \
  "$source_root/var/lib/bitriver-live/api" \
  "$source_root/var/lib/bitriver-live/transcoder/public/live" \
  "$postgres_dir" \
  "$object_root/recordings"
printf 'BITRIVER_POSTGRES_PASSWORD=%s\n' "$secret_sentinel" \
  >"$source_root/etc/bitriver-live/bitriver.env"
printf '<Server>%s</Server>\n' "$secret_sentinel" \
  >"$source_root/etc/bitriver-live/deploy/ome/Server.generated.xml"
ln -s "$source_root/etc/bitriver-live/deploy/ome/Server.generated.xml" \
  "$source_root/etc/bitriver-live/Server.generated.xml"
printf '{"durable":true}\n' >"$source_root/var/lib/bitriver-live/api/state.json"
printf '#EXTM3U\n#EXT-X-ENDLIST\n' \
  >"$source_root/var/lib/bitriver-live/transcoder/public/live/index.m3u8"
printf 'object-fixture\n' >"$object_root/recordings/fixture.bin"
chmod 0600 "$source_root/etc/bitriver-live/bitriver.env"
chmod 0640 "$source_root/etc/bitriver-live/deploy/ome/Server.generated.xml"
printf '%s\n' 'correct horse battery staple recovery passphrase' >"$passphrase_file"
printf '%s\n' 'wrong horse battery staple recovery passphrase' >"$wrong_passphrase_file"
chmod 0600 "$passphrase_file" "$wrong_passphrase_file"

real_tar="$(command -v tar)"
mkdir -p "$fake_bin"
cat >"$fake_bin/tar" <<'TAR_WRAPPER'
#!/usr/bin/env bash
set -Eeuo pipefail
"$BITRIVER_TEST_REAL_TAR" "$@"
printf '{"durable":"changed-after-snapshot"}\n' \
  >"$BITRIVER_TEST_MUTATE_AFTER_TAR"
TAR_WRAPPER
chmod 0755 "$fake_bin/tar"

postgres_backup="$postgres_dir/bitriver-postgres-${timestamp}.sql.gz"
printf 'postgres-backup-fixture\n' | gzip -c >"$postgres_backup"
postgres_sha="$(sha256sum "$postgres_backup")"
postgres_sha="${postgres_sha%% *}"
cat >"${postgres_backup}.manifest.json" <<JSON
{
  "schemaVersion": "bitriver.postgres-backup/v1",
  "source": {"release": "$release", "commit": "$commit"},
  "archive": {"name": "$(basename "$postgres_backup")", "sha256": "$postgres_sha"},
  "database": {"migrationFingerprintSha256": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}
}
JSON
postgres_manifest_sha="$(sha256sum "${postgres_backup}.manifest.json")"
postgres_manifest_sha="${postgres_manifest_sha%% *}"
{
  printf '%s  %s\n' "$postgres_sha" "$(basename "$postgres_backup")"
  printf '%s  %s\n' "$postgres_manifest_sha" "$(basename "${postgres_backup}.manifest.json")"
} >"${postgres_backup}.sha256"

bash "$repo_root/scripts/python.sh" "$repo_root/scripts/host_recovery.py" \
  object-inventory \
  --root "$object_root" \
  --output "$object_inventory"

PATH="$fake_bin:$PATH" \
BITRIVER_TEST_REAL_TAR="$real_tar" \
BITRIVER_TEST_MUTATE_AFTER_TAR="$source_root/var/lib/bitriver-live/api/state.json" \
BITRIVER_HOST_BACKUP_CREATED_AT="$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
  bash "$repo_root/scripts/backup-host-state.sh" \
  --root-prefix "$source_root" \
  --output-dir "$backup_dir" \
  --postgres-backup "$postgres_backup" \
  --source-release "$release" \
  --source-commit "$commit" \
  --passphrase-file "$passphrase_file" \
  --object-inventory "$object_inventory" \
  --timestamp "$timestamp"

archive="$backup_dir/bitriver-host-${timestamp}.tar.gz.enc"
manifest="${archive}.manifest.json"
checksum="${archive}.sha256"
[[ -f $archive && -f $manifest && -f $checksum ]] || fail "complete host recovery set was not published"
grep -Fq '"changed-after-snapshot"' \
  "$source_root/var/lib/bitriver-live/api/state.json" ||
  fail "test tar wrapper did not mutate the live source after snapshotting"
if grep -aFq "$secret_sentinel" "$archive" "$manifest" "$checksum"; then
  fail "host recovery set exposed a configuration secret"
fi
archive_before="$(sha256sum "$archive")"
manifest_before="$(sha256sum "$manifest")"
checksum_before="$(sha256sum "$checksum")"
if BITRIVER_HOST_BACKUP_CREATED_AT="$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
  bash "$repo_root/scripts/backup-host-state.sh" \
    --root-prefix "$source_root" \
    --output-dir "$backup_dir" \
    --postgres-backup "$postgres_backup" \
    --source-release "$release" \
    --source-commit "$commit" \
    --passphrase-file "$passphrase_file" \
    --timestamp "$timestamp" >/dev/null 2>&1; then
  fail "same-timestamp host backup collision unexpectedly succeeded"
fi
[[ "$(sha256sum "$archive")" == "$archive_before" ]] || fail "collision altered the encrypted archive"
[[ "$(sha256sum "$manifest")" == "$manifest_before" ]] || fail "collision altered the host manifest"
[[ "$(sha256sum "$checksum")" == "$checksum_before" ]] || fail "collision altered the checksum set"

report="$workdir/host-restore-report.json"
bash "$repo_root/scripts/restore-host-state.sh" \
  --archive "$archive" \
  --root-prefix "$restore_root" \
  --expected-release "$release" \
  --expected-commit "$commit" \
  --passphrase-file "$passphrase_file" \
  --report "$report"

cmp "$source_root/etc/bitriver-live/bitriver.env" \
  "$restore_root/etc/bitriver-live/bitriver.env"
grep -Fxq '{"durable":true}' \
  "$restore_root/var/lib/bitriver-live/api/state.json" ||
  fail "restored data did not match the immutable backup snapshot"
cmp "$source_root/var/lib/bitriver-live/transcoder/public/live/index.m3u8" \
  "$restore_root/var/lib/bitriver-live/transcoder/public/live/index.m3u8"
[[ -f $restore_root/var/backups/bitriver-live/recovery/postgres/$(basename "$postgres_backup") ]] ||
  fail "restored host omitted the Postgres archive"
[[ -f $restore_root/var/backups/bitriver-live/recovery/object-inventory.json ]] ||
  fail "restored host omitted the external object inventory"
grep -Fq '"schemaVersion": "bitriver.host-restore-report/v1"' "$report"
grep -Fq '"status": "passed"' "$report"
if grep -Fq "$secret_sentinel" "$report"; then
  fail "restore report exposed a configuration secret"
fi

wrong_release_root="$workdir/wrong-release"
if bash "$repo_root/scripts/restore-host-state.sh" \
  --archive "$archive" \
  --root-prefix "$wrong_release_root" \
  --expected-release "v1.2.3-rc.22" \
  --expected-commit "$commit" \
  --passphrase-file "$passphrase_file" >/dev/null 2>&1; then
  fail "wrong release restore unexpectedly succeeded"
fi
[[ ! -e $wrong_release_root/etc/bitriver-live ]] || fail "wrong release mutated the target"

wrong_passphrase_root="$workdir/wrong-passphrase-root"
if bash "$repo_root/scripts/restore-host-state.sh" \
  --archive "$archive" \
  --root-prefix "$wrong_passphrase_root" \
  --expected-release "$release" \
  --expected-commit "$commit" \
  --passphrase-file "$wrong_passphrase_file" >/dev/null 2>&1; then
  fail "wrong passphrase restore unexpectedly succeeded"
fi
[[ ! -e $wrong_passphrase_root/etc/bitriver-live ]] || fail "wrong passphrase mutated the target"

nonfresh_root="$workdir/nonfresh"
mkdir -p "$nonfresh_root/etc/bitriver-live"
if bash "$repo_root/scripts/restore-host-state.sh" \
  --archive "$archive" \
  --root-prefix "$nonfresh_root" \
  --expected-release "$release" \
  --expected-commit "$commit" \
  --passphrase-file "$passphrase_file" >/dev/null 2>&1; then
  fail "non-fresh target restore unexpectedly succeeded"
fi

printf 'corruption' >>"$archive"
corrupt_root="$workdir/corrupt"
if bash "$repo_root/scripts/restore-host-state.sh" \
  --archive "$archive" \
  --root-prefix "$corrupt_root" \
  --expected-release "$release" \
  --expected-commit "$commit" \
  --passphrase-file "$passphrase_file" >/dev/null 2>&1; then
  fail "corrupt host archive unexpectedly restored"
fi
[[ ! -e $corrupt_root/etc/bitriver-live ]] || fail "corrupt archive mutated the target"

echo "Encrypted host backup/restore rehearsal passed."
