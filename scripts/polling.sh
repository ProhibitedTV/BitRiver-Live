#!/usr/bin/env bash

# bounded_poll retries a check function at a fixed interval until success, fatal
# failure, or timeout.
#
# Usage:
#   bounded_poll <timeout_seconds> <interval_seconds> <check_fn> [check_fn_args...]
#
# check_fn return codes:
#   0 => success (stop polling, return 0)
#   1 => retryable state (sleep + retry)
#   2 => fatal state (stop polling, return 2)
#   other => unexpected error (stop polling, return original code)
bounded_poll() {
  if [ "$#" -lt 3 ]; then
    echo "error: bounded_poll requires timeout, interval, and check function" >&2
    return 64
  fi

  local timeout_seconds="$1"
  local interval_seconds="$2"
  local check_fn="$3"
  shift 3

  if [ "$timeout_seconds" -lt 0 ] 2>/dev/null; then
    echo "error: bounded_poll timeout must be >= 0" >&2
    return 64
  fi
  if [ "$interval_seconds" -lt 1 ] 2>/dev/null; then
    echo "error: bounded_poll interval must be >= 1" >&2
    return 64
  fi

  local deadline=$((SECONDS + timeout_seconds))
  local check_rc=0

  while true; do
    "$check_fn" "$@"
    check_rc=$?

    case "$check_rc" in
    0)
      return 0
      ;;
    1)
      ;;
    2)
      return 2
      ;;
    *)
      return "$check_rc"
      ;;
    esac

    if [ "$SECONDS" -ge "$deadline" ]; then
      return 124
    fi

    sleep "$interval_seconds"
  done
}
