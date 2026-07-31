#!/usr/bin/env bash
set -euo pipefail

repo_root=$(CDPATH=; cd -- "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
tmp_root=$(mktemp -d "${TMPDIR:-/tmp}/bitriver-linux-packages.XXXXXX")
cleanup() {
  rm -rf -- "$tmp_root"
}
trap cleanup EXIT

if ! command -v nfpm >/dev/null 2>&1; then
  if [[ ${BITRIVER_INSTALL_NFPM:-0} != 1 ]]; then
    echo "SKIP: nfpm is not installed (set BITRIVER_INSTALL_NFPM=1 for the pinned integration test)"
    exit 0
  fi
  go_command=$(command -v go || true)
  if [[ -z $go_command && -x /usr/local/go/bin/go ]]; then
    go_command=/usr/local/go/bin/go
  fi
  [[ -n $go_command ]] || { echo "go is required to install pinned nfpm" >&2; exit 1; }
  GOBIN="$tmp_root/bin" "$go_command" install github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.47.0
  export PATH="$tmp_root/bin:$PATH"
fi

launcher_root="$tmp_root/launcher-root"
prerelease_launcher_root="$tmp_root/prerelease-launcher-root"
mkdir -p \
  "$launcher_root/bin" "$launcher_root/share/bitriver-live" \
  "$prerelease_launcher_root/bin" "$prerelease_launcher_root/share/bitriver-live" \
  "$tmp_root/packages"
bash "$repo_root/scripts/stage-release-assets.sh" --output "$launcher_root/share/bitriver-live"
bash "$repo_root/scripts/stage-release-assets.sh" \
  --output "$prerelease_launcher_root/share/bitriver-live" \
  --release-tag v1.2.3-rc.1
for root in "$launcher_root" "$prerelease_launcher_root"; do
  printf '#!/bin/sh\nexit 0\n' >"$root/bin/bitriver"
  printf '#!/bin/sh\nexit 0\n' >"$root/bin/bitriver-live"
  chmod +x "$root/bin/bitriver" "$root/bin/bitriver-live"
done

for arch in amd64 arm64; do
  for format in deb rpm; do
    target="$tmp_root/packages/bitriver-live_v1.2.3_${arch}.${format}"
    (
      cd "$launcher_root"
      NFPM_ARCH="$arch" \
      NFPM_VERSION=1.2.3 \
      NFPM_PRERELEASE='' \
      LAUNCHER_ROOT="$launcher_root" \
      GOMAXPROCS=2 \
        nfpm pkg --packager "$format" --config "$repo_root/deploy/installers/nfpm.yaml" --target "$target"
    )
    [[ -s $target ]] || { echo "package was not created: $target" >&2; exit 1; }
  done
done

for format in deb rpm; do
  target="$tmp_root/packages/bitriver-live_v1.2.3-rc.1_amd64.${format}"
  (
    cd "$prerelease_launcher_root"
    NFPM_ARCH=amd64 \
    NFPM_VERSION=1.2.3 \
    NFPM_PRERELEASE=rc.1 \
    LAUNCHER_ROOT="$prerelease_launcher_root" \
    GOMAXPROCS=2 \
      nfpm pkg --packager "$format" --config "$repo_root/deploy/installers/nfpm.yaml" --target "$target"
  )
  [[ -s $target ]] || { echo "prerelease package was not created: $target" >&2; exit 1; }
done

if command -v dpkg-deb >/dev/null 2>&1; then
  for package in "$tmp_root"/packages/*.deb; do
    listing=$(dpkg-deb --contents "$package")
    grep -q './usr/local/bin/bitriver$' <<<"$listing"
    grep -q './usr/local/sbin/bitriver-host$' <<<"$listing"
    grep -q './usr/local/share/bitriver-live/deploy/docker-compose.yml$' <<<"$listing"
    grep -q './usr/local/share/bitriver-live/deploy/ome/Server.xml$' <<<"$listing"
    if grep -q 'Server.generated.xml$' <<<"$listing"; then
      echo "generated OME configuration leaked into $package" >&2
      exit 1
    fi
  done
  prerelease_version=$(dpkg-deb --field "$tmp_root/packages/bitriver-live_v1.2.3-rc.1_amd64.deb" Version)
  [[ $prerelease_version == *rc.1* ]] || {
    echo "Debian prerelease metadata lost rc.1: $prerelease_version" >&2
    exit 1
  }
  prerelease_deb_root="$tmp_root/prerelease-deb"
  dpkg-deb --extract "$tmp_root/packages/bitriver-live_v1.2.3-rc.1_amd64.deb" "$prerelease_deb_root"
  prerelease_env="$prerelease_deb_root/usr/local/share/bitriver-live/deploy/.env.example"
  for key in \
    BITRIVER_LIVE_IMAGE_TAG \
    BITRIVER_VIEWER_IMAGE_TAG \
    BITRIVER_SRS_CONTROLLER_IMAGE_TAG \
    BITRIVER_TRANSCODER_IMAGE_TAG \
    BITRIVER_OME_CONFIG_IMAGE_TAG; do
    [[ $(grep -c "^${key}=v1.2.3-rc.1" "$prerelease_env") -eq 1 ]] || {
      echo "prerelease Debian payload does not contain exactly one ${key}=v1.2.3-rc.1" >&2
      exit 1
    }
  done
fi

echo "PASS: nfpm built stable and tag-correct prerelease amd64/arm64 deb/rpm packages from the canonical release bundle"
