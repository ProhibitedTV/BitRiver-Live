#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT_DIR"

readonly TARGET_IMPORT='"bitriver-live/internal/models"'

# Files matching these prefixes are allowed to import internal/models.
# Keep this list short and explicit; by default only the shim package itself
# can import it.
ALLOWLIST_PREFIXES=(
  "internal/models/"
)

is_allowlisted() {
  local file="$1"
  local prefix
  for prefix in "${ALLOWLIST_PREFIXES[@]}"; do
    if [[ "$file" == "$prefix"* ]]; then
      return 0
    fi
  done
  return 1
}

go_files=()
while IFS= read -r file; do
  go_files+=("${file#./}")
done < <(find . -type f -name '*.go' | LC_ALL=C sort)

if ((${#go_files[@]} == 0)); then
  echo "No Go files to check."
  exit 0
fi

violations=""
for file in "${go_files[@]}"; do
  if is_allowlisted "$file"; then
    continue
  fi

  matches="$(grep -nF "$TARGET_IMPORT" "$file" || true)"
  if [[ -n "$matches" ]]; then
    while IFS= read -r line; do
      violations+="${file}:${line}"$'\n'
    done <<<"$matches"
  fi
done

if [[ -n "$violations" ]]; then
  {
    echo "Forbidden imports detected: bitriver-live/internal/models"
    echo "Allowed importers: ${ALLOWLIST_PREFIXES[*]}"
    echo "Move callers to internal/domain instead of importing internal/models."
    printf '%s' "$violations"
  } >&2
  exit 1
fi

echo "No forbidden bitriver-live/internal/models imports found."
