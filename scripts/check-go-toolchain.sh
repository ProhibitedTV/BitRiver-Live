#!/usr/bin/env bash
set -euo pipefail

minimum_major=1
minimum_minor=26

if ! command -v go >/dev/null 2>&1; then
  echo "Go ${minimum_major}.${minimum_minor} or newer is required (go command not found)." >&2
  exit 1
fi

version_output="$(go env GOVERSION 2>/dev/null || true)"
if [[ ! "$version_output" =~ go([0-9]+)\.([0-9]+) ]]; then
  echo "Unable to determine Go version from: ${version_output:-unknown}" >&2
  exit 1
fi

major="${BASH_REMATCH[1]}"
minor="${BASH_REMATCH[2]}"
if (( major < minimum_major )) || { (( major == minimum_major )) && (( minor < minimum_minor )); }; then
  echo "Go ${minimum_major}.${minimum_minor} or newer is required (found ${version_output#*go})." >&2
  exit 1
fi
