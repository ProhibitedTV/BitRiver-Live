#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
  cat <<'USAGE'
Usage: ./scripts/test-published-disaster-recovery.sh [options]

Download and qualify one exact public Linux amd64 launcher package through the
disposable lost-host recovery rehearsal.

Required:
  --release TAG            Exact prerelease tag.
  --source-commit SHA      Full lowercase source commit.

Options:
  --artifact-dir DIR       Fresh retained evidence directory.
  -h, --help               Show this help.
USAGE
}

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
repository="ProhibitedTV/BitRiver-Live"
release=""
source_commit=""
artifact_dir=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --release) [[ $# -ge 2 ]] || { echo "--release requires a value" >&2; exit 2; }; release=$2; shift 2 ;;
    --source-commit) [[ $# -ge 2 ]] || { echo "--source-commit requires a value" >&2; exit 2; }; source_commit=$2; shift 2 ;;
    --artifact-dir) [[ $# -ge 2 ]] || { echo "--artifact-dir requires a value" >&2; exit 2; }; artifact_dir=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

[[ $release =~ ^v[0-9]+\.[0-9]+\.[0-9]+-[0-9A-Za-z.-]+$ ]] || {
  echo "--release must be an exact prerelease tag" >&2
  exit 2
}
[[ $source_commit =~ ^[0-9a-f]{40}$ ]] || {
  echo "--source-commit must be 40 lowercase hexadecimal characters" >&2
  exit 2
}
if [[ -z $artifact_dir ]]; then
  artifact_dir="$repo_root/.artifacts/published-disaster-recovery/$release"
fi
mkdir -p "$artifact_dir"
artifact_dir="$(cd "$artifact_dir" && pwd -P)"
if find "$artifact_dir" -mindepth 1 -print -quit | grep -q .; then
  echo "artifact directory must be empty: $artifact_dir" >&2
  exit 2
fi

workdir="$(mktemp -d)"
cleanup() {
  rm -rf -- "$workdir"
}
trap cleanup EXIT

download_root="$workdir/download"
extract_root="$workdir/extracted"
package_name="bitriver-launcher-linux-amd64.tar.gz"
release_set="$download_root/release-set.json"
package_archive="$download_root/$package_name"
base_url="https://github.com/$repository/releases/download/$release"
mkdir -p "$download_root" "$extract_root"

curl --fail --location --silent --show-error --retry 3 \
  "$base_url/release-set.json" --output "$release_set"
curl --fail --location --silent --show-error --retry 3 \
  "$base_url/$package_name" --output "$package_archive"

# Validate public identity and bytes before archive listing or extraction.
bash "$script_dir/python.sh" "$script_dir/host_recovery.py" \
  verify-release-package \
  --release-set "$release_set" \
  --package "$package_archive" \
  --expected-release "$release" \
  --expected-commit "$source_commit" \
  --output "$workdir/pre-extraction-package-binding.json"

members="$workdir/archive-members.txt"
tar -tzf "$package_archive" >"$members"
while IFS= read -r member || [[ -n $member ]]; do
  member=${member%$'\r'}
  trimmed=${member%/}
  case "$trimmed" in
    ""|/*|../*|*/../*|*/..|*\\*)
      echo "unsafe launcher archive member: $member" >&2
      exit 2
      ;;
  esac
done <"$members"
tar -xzf "$package_archive" -C "$extract_root"

bundle_root="$extract_root/bitriver-launcher-linux-amd64/share/bitriver-live"
[[ -d $bundle_root ]] || {
  echo "launcher archive does not contain the expected source-free bundle" >&2
  exit 2
}

BITRIVER_DISASTER_RECOVERY_ARTIFACT_DIR="$artifact_dir" \
  bash "$script_dir/test-disaster-recovery.sh" \
    --bundle-root "$bundle_root" \
    --release-set "$release_set" \
    --package-archive "$package_archive" \
    --release "$release" \
    --source-commit "$source_commit"

bash "$script_dir/scan-release-evidence.sh" --root "$artifact_dir"
echo "Published recovery evidence retained in $artifact_dir"
