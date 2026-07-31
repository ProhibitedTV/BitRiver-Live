#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: ./scripts/stage-release-assets.sh --output DIR [--manifest FILE] [--release-tag TAG]

Copy the canonical source-free deployment assets into DIR while preserving
their repository-relative paths. The default manifest is
deploy/install/release-assets.txt. When TAG is supplied, stamp the exact
SemVer release tag into the staged first-party image defaults without changing
the source deploy/.env.example.
USAGE
}

repo_root=$(CDPATH=; cd -- "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
manifest="$repo_root/deploy/install/release-assets.txt"
output=""
release_tag=""

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
    --release-tag)
      [[ $# -ge 2 ]] || { echo "--release-tag requires a value" >&2; exit 2; }
      release_tag=$2
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
if [[ -n $release_tag && ! $release_tag =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$ ]]; then
  echo "release tag must match vMAJOR.MINOR.PATCH or vMAJOR.MINOR.PATCH-PRERELEASE" >&2
  exit 2
fi
if [[ $release_tag == *-* ]]; then
  prerelease=${release_tag#*-}
  IFS=. read -r -a identifiers <<<"$prerelease"
  for identifier in "${identifiers[@]}"; do
    if [[ $identifier =~ ^[0-9]+$ && ${#identifier} -gt 1 && $identifier == 0* ]]; then
      echo "numeric prerelease identifiers must not contain leading zeroes" >&2
      exit 2
    fi
  done
fi

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

if [[ -n $release_tag ]]; then
  staged_env="$output/deploy/.env.example"
  [[ -f $staged_env ]] || {
    echo "staged release env not found: $staged_env" >&2
    exit 1
  }
  tmp_env="$staged_env.release-tag.$$"
  cleanup_tmp_env() {
    [[ -z ${tmp_env:-} ]] || rm -f -- "$tmp_env"
  }
  trap cleanup_tmp_env EXIT
  awk -v release_tag="$release_tag" '
    BEGIN {
      split("BITRIVER_LIVE_IMAGE_TAG BITRIVER_VIEWER_IMAGE_TAG BITRIVER_SRS_CONTROLLER_IMAGE_TAG BITRIVER_TRANSCODER_IMAGE_TAG BITRIVER_OME_CONFIG_IMAGE_TAG", keys, " ")
      for (i in keys) wanted[keys[i]] = 0
    }
    {
      key = $0
      sub(/=.*/, "", key)
      if (key in wanted) {
        ending = ($0 ~ /\r$/) ? "\r" : ""
        print key "=" release_tag ending
        wanted[key]++
        next
      }
      print
    }
    END {
      invalid = 0
      for (key in wanted) {
        if (wanted[key] != 1) {
          printf "expected exactly one staged %s assignment, found %d\n", key, wanted[key] > "/dev/stderr"
          invalid = 1
        }
      }
      exit invalid
    }
  ' "$staged_env" >"$tmp_env"
  mv -- "$tmp_env" "$staged_env"
  tmp_env=""
  trap - EXIT
fi

printf 'Staged source-free BitRiver Live assets in %s\n' "$output"
