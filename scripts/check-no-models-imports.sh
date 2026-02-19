#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT_DIR"

mapfile -t go_files < <(rg --files -g '*.go' | grep -v '^internal/models/')

if ((${#go_files[@]} == 0)); then
  echo "No Go files to check."
  exit 0
fi

violations=""
for file in "${go_files[@]}"; do
  # Exception hook for explicit shim validation tests outside internal/models.
  case "$file" in
    # Add allowlisted test files here when needed.
    # internal/somepkg/models_shim_test.go)
    #   continue
    #   ;;
  esac

  matches="$(grep -nE '"bitriver-live/internal/models"' "$file" || true)"
  if [[ -n "$matches" ]]; then
    while IFS= read -r line; do
      violations+="${file}:${line}"$'\n'
    done <<<"$matches"
  fi
done

if [[ -n "$violations" ]]; then
  {
    echo "Forbidden imports detected: bitriver-live/internal/models"
    echo "Only files under internal/models may import this package."
    printf '%s' "$violations"
  } >&2
  exit 1
fi

echo "No forbidden bitriver-live/internal/models imports found."
