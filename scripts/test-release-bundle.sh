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

source_env_snapshot="$tmp_root/source-env.example"
cp "$repo_root/deploy/.env.example" "$source_env_snapshot"
tagged_bundle_root="$tmp_root/tagged release bundle"
release_tag="v1.2.3-rc.12"
bash "$repo_root/scripts/stage-release-assets.sh" \
  --output "$tagged_bundle_root" \
  --release-tag "$release_tag"

tagged_env="$tagged_bundle_root/deploy/.env.example"
first_party_tag_keys=(
  BITRIVER_LIVE_IMAGE_TAG
  BITRIVER_VIEWER_IMAGE_TAG
  BITRIVER_SRS_CONTROLLER_IMAGE_TAG
  BITRIVER_TRANSCODER_IMAGE_TAG
  BITRIVER_OME_CONFIG_IMAGE_TAG
)
for key in "${first_party_tag_keys[@]}"; do
  [[ $(grep -c "^${key}=${release_tag}" "$tagged_env") -eq 1 ]] || {
    echo "tagged release bundle does not contain exactly one ${key}=${release_tag}" >&2
    exit 1
  }
done
if grep -Eq '^BITRIVER_(LIVE|VIEWER|SRS_CONTROLLER|TRANSCODER|OME_CONFIG)_IMAGE_TAG=v1\.2\.3$' "$tagged_env"; then
  echo "tagged release bundle retained a stable first-party image tag" >&2
  exit 1
fi
cmp "$source_env_snapshot" "$repo_root/deploy/.env.example"
if bash "$repo_root/scripts/stage-release-assets.sh" \
  --output "$tmp_root/invalid-tag-bundle" \
  --release-tag '1.2.3-rc.12' >/dev/null 2>&1; then
  echo "release asset staging accepted a tag without the required v prefix" >&2
  exit 1
fi
[[ ! -e $tmp_root/invalid-tag-bundle ]] || {
  echo "invalid release tag created a partial bundle" >&2
  exit 1
}
if bash "$repo_root/scripts/stage-release-assets.sh" \
  --output "$tmp_root/invalid-prerelease-bundle" \
  --release-tag 'v1.2.3-rc.012' >/dev/null 2>&1; then
  echo "release asset staging accepted a numeric prerelease identifier with leading zeroes" >&2
  exit 1
fi
[[ ! -e $tmp_root/invalid-prerelease-bundle ]] || {
  echo "invalid prerelease tag created a partial bundle" >&2
  exit 1
}

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

grep -Fq "scripts/stage-release-assets.sh --output \"\$out_dir\"" "$repo_root/.github/workflows/release.yml"
grep -Fq "scripts/stage-release-assets.sh --output \"\$launcher_root/share/bitriver-live\"" "$repo_root/.github/workflows/release.yml"
[[ $(grep -Fc -- '--release-tag "$RELEASE_TAG"' "$repo_root/.github/workflows/release.yml") -eq 3 ]] || {
  echo "every release asset staging path must pass the exact immutable release tag" >&2
  exit 1
}
grep -q 'image_name: bitriver-ome-config' "$repo_root/.github/workflows/release.yml"
grep -q 'BITRIVER_IMAGE_NAMESPACE:-ghcr.io/prohibitedtv}/bitriver-ome-config' "$repo_root/deploy/docker-compose.yml"
grep -q 'dst: /usr/local/sbin/bitriver-host' "$repo_root/deploy/installers/nfpm.yaml"

echo "PASS: release bundle is complete, source-free, tag-correct, and excludes generated credentials"
