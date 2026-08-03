#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
CHECKER="$ROOT_DIR/scripts/check-repository-hygiene.sh"
TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT

fixture="$TMP_ROOT/repository hygiene fixture"
git init -q "$fixture"
printf 'tracked source\n' >"$fixture/source.txt"
git -C "$fixture" add source.txt

BITRIVER_REPOSITORY_ROOT="$fixture" "$CHECKER" >/dev/null

mkdir -p "$fixture/.gocache-forced/00"
printf 'generated cache payload\n' >"$fixture/.gocache-forced/00/cache-entry-a"
git -C "$fixture" add -f .gocache-forced/00/cache-entry-a

output_file="$TMP_ROOT/checker-output.txt"
set +e
BITRIVER_REPOSITORY_ROOT="$fixture" "$CHECKER" >"$output_file" 2>&1
checker_rc=$?
set -e

if [[ "$checker_rc" -eq 0 ]]; then
  echo "FAIL: hygiene guard accepted a force-added root Go cache" >&2
  exit 1
fi

if ! grep -Fq '.gocache-forced/00/cache-entry-a' "$output_file"; then
  echo "FAIL: hygiene guard did not identify the tracked cache path" >&2
  cat "$output_file" >&2
  exit 1
fi

echo "PASS: repository hygiene guard accepts source and rejects forced Go caches"
