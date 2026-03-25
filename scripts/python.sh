#!/usr/bin/env bash
set -Eeuo pipefail

run_candidate() {
  local candidate="$1"
  shift

  case "$candidate" in
    python3)
      exec python3 "$@"
      ;;
    python)
      exec python "$@"
      ;;
    py)
      exec py -3 "$@"
      ;;
  esac
}

if command -v python3 >/dev/null 2>&1 && python3 --version >/dev/null 2>&1; then
  run_candidate python3 "$@"
fi

if command -v python >/dev/null 2>&1 && python --version >/dev/null 2>&1; then
  run_candidate python "$@"
fi

if command -v py >/dev/null 2>&1 && py -3 --version >/dev/null 2>&1; then
  run_candidate py "$@"
fi

cat >&2 <<'MSG'
Python 3 is required for this check, but no usable interpreter was found.

Install Python 3 so one of these commands works:
  - python3
  - python
  - py -3

On Windows, the Python launcher (py.exe) alone is not enough unless it can
start an installed Python 3 interpreter.
MSG
exit 1
