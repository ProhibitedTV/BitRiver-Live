#!/usr/bin/env bash
set -euo pipefail

repo_root=$(CDPATH=; cd -- "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
tmp_root=$(mktemp -d "${TMPDIR:-/tmp}/bitriver-release-bundle.XXXXXX")
cleanup() {
  rm -rf -- "$tmp_root"
}
trap cleanup EXIT

bundle_root="$tmp_root/release bundle with spaces"
bash "$repo_root/scripts/stage-release-assets.sh" --output "$bundle_root"

manifest="$repo_root/deploy/install/release-assets.txt"
while IFS= read -r line || [[ -n $line ]]; do
  line=${line%$'\r'}
  [[ -n $line && $line != \#* ]] || continue
  source="$repo_root/$line"
  staged="$bundle_root/$line"
  if [[ -d $source ]]; then
    [[ -d $staged ]] || { echo "missing staged directory: $line" >&2; exit 1; }
  else
    [[ -f $staged ]] || { echo "missing staged file: $line" >&2; exit 1; }
    cmp "$source" "$staged"
  fi
done <"$manifest"

required=(
  deploy/docker-compose.yml
  deploy/.env.example
  deploy/postgres-migrate.sh
  deploy/migrations/0011_channel_schedule.sql
  deploy/ome/Server.xml
  deploy/srs/conf/srs.conf
  deploy/nginx/transcoder-public.conf
  deploy/install/compose-host.sh
  deploy/systemd/bitriver-live-compose.service
  scripts/render-srs-config.sh
  scripts/bitriver-live-wrapper.sh
  scripts/stage-release-assets.sh
  docs/installing-on-ubuntu.md
)
for relative in "${required[@]}"; do
  [[ -f $bundle_root/$relative ]] || { echo "required release asset missing: $relative" >&2; exit 1; }
done

for generated in \
  deploy/ome/Server.generated.xml \
  deploy/srs/conf/srs.generated.conf \
  .env; do
  if [[ -e $bundle_root/$generated ]]; then
    echo "credential-bearing/generated runtime output was packaged: $generated" >&2
    exit 1
  fi
done

grep -q 'scripts/stage-release-assets.sh --output "$out_dir"' "$repo_root/.github/workflows/release.yml"
grep -q 'scripts/stage-release-assets.sh --output "$launcher_root/share/bitriver-live"' "$repo_root/.github/workflows/release.yml"
grep -q 'image_name: bitriver-ome-config' "$repo_root/.github/workflows/release.yml"
grep -q 'ghcr.io/bitriver-live/bitriver-ome-config' "$repo_root/deploy/docker-compose.yml"
grep -q 'dst: /usr/local/sbin/bitriver-host' "$repo_root/deploy/installers/nfpm.yaml"

echo "PASS: release bundle is complete, source-free, and excludes generated credentials"
