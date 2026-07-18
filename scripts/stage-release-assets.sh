#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: ./scripts/stage-release-assets.sh --output DIR [--manifest FILE]

Copy the canonical source-free deployment assets into DIR while preserving
their repository-relative paths. The default manifest is
deploy/install/release-assets.txt.
USAGE
}

repo_root=$(CDPATH=; cd -- "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
manifest="$repo_root/deploy/install/release-assets.txt"
output=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --output)
      [[ $# -ge 2 ]] || { echo "--output requires a value" >&2; exit 2; }
      output=$2
      shift 2
      ;;
    --manifest)
      [[ $# -ge 2 ]] || { echo "--manifest requires a value" >&2; exit 2; }
      manifest=$2
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

[[ -n $output ]] || { echo "--output is required" >&2; exit 2; }
[[ -f $manifest ]] || { echo "release asset manifest not found: $manifest" >&2; exit 1; }

mkdir -p "$output"
output=$(CDPATH=; cd -- "$output" && pwd -P)

copy_asset() {
  local relative=$1
  local source="$repo_root/$relative"
  local destination="$output/$relative"

  case "$relative" in
    /*|../*|*/../*|*/..)
      echo "unsafe release asset path: $relative" >&2
      exit 1
      ;;
  esac

  if [[ -d $source ]]; then
    mkdir -p "$destination"
    cp -a "$source/." "$destination/"
  elif [[ -f $source ]]; then
    mkdir -p "$(dirname "$destination")"
    cp -p "$source" "$destination"
  else
    echo "release asset does not exist: $relative" >&2
    exit 1
  fi
}

while IFS= read -r line || [[ -n $line ]]; do
  line=${line%$'\r'}
  [[ -n $line && $line != \#* ]] || continue
  copy_asset "$line"
done <"$manifest"

# Bind-mount parents are deliberately empty in the publication payload.
# Installers make operator-owned copies before runtime writes data/config here.
mkdir -p "$output/deploy/data" "$output/deploy/transcoder-data"

printf 'Staged source-free BitRiver Live assets in %s\n' "$output"
