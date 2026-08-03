#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="${BITRIVER_REPOSITORY_ROOT:-$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)}"
cd "$ROOT_DIR"

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "error: repository hygiene guard requires a Git work tree" >&2
  exit 1
fi

violations=()
while IFS= read -r -d '' path; do
  case "$path" in
    .gocache/*|.gocache-*)
      violations+=("$path")
      ;;
  esac
done < <(git ls-files -z)

if ((${#violations[@]} > 0)); then
  echo "Repository hygiene guard failed. Remove tracked Go build-cache artifacts:" >&2
  printf ' - %s\n' "${violations[@]}" >&2
  exit 1
fi

echo "Repository hygiene guard passed."
