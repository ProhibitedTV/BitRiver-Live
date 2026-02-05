#!/bin/sh
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT_DIR"

: "${GOTOOLCHAIN:=local}"
: "${GOPROXY:=off}"
: "${GOSUMDB:=off}"

export GOTOOLCHAIN GOPROXY GOSUMDB

if ! command -v govulncheck >/dev/null 2>&1; then
  echo "govulncheck not found in PATH." >&2
  echo "Install it with: GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go install golang.org/x/vuln/cmd/govulncheck@latest" >&2
  exit 1
fi

echo "Running govulncheck for root module (vendor mode)."
GOFLAGS="-mod=vendor" govulncheck ./...

replace_dirs=$(awk '
  /^replace \(/ {inblock=1; next}
  /^\)/ {inblock=0; next}
  /^replace / || inblock {
    for (i = 1; i <= NF; i++) {
      if ($i == "=>") {
        target = $(i + 1)
        if (target ~ /^\.\/third_party\//) {
          print target
        }
      }
    }
  }
' go.mod | sort -u)

if [ -n "$replace_dirs" ]; then
  echo "Running govulncheck for replaced third_party modules."
  for dir in $replace_dirs; do
    if [ -f "$dir/go.mod" ]; then
      echo "- $dir"
      (cd "$dir" && GOFLAGS="" govulncheck ./...)
    else
      echo "- Skipping $dir (no go.mod found)." >&2
    fi
  done
fi
