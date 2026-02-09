#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT_DIR"

if [ ! -f go.mod ]; then
  echo "go.mod not found" >&2
  exit 1
fi

duplicates=""

while IFS='|' read -r module target; do
  [ -n "$module" ] || continue
  [ -n "$target" ] || continue

  case "$target" in
    ./third_party/*)
      if [ -e "vendor/$module" ]; then
        duplicates="${duplicates}\n- $module (third_party: $target, vendor: vendor/$module)"
      fi
      ;;
  esac
done <<EOF_REPLACES
$(awk '
  /^replace \(/ {inblock=1; next}
  /^\)/ {inblock=0; next}
  /^replace / || inblock {
    module=""
    target=""
    for (i = 1; i <= NF; i++) {
      if (module == "" && $i != "replace") {
        module = $i
      }
      if ($i == "=>" && i + 1 <= NF) {
        target = $(i + 1)
      }
    }
    if (module != "" && target != "") {
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", module)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", target)
      print module "|" target
    }
  }
' go.mod)
EOF_REPLACES

if [ -n "$duplicates" ]; then
  echo "error: duplicate dependency trees detected." >&2
  echo "Dependencies must be sourced from third_party/ via go.mod replace directives only." >&2
  printf '%b\n' "$duplicates" >&2
  exit 1
fi

echo "dependency source check passed (no third_party/vendor duplicates)."
