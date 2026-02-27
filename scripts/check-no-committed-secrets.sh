#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT_DIR"

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "error: this script must run inside a git work tree" >&2
  exit 1
fi

violations=()

is_exempt_path() {
  local path="$1"
  case "$path" in
    deploy/.env.example)
      return 0
      ;;
  esac
  return 1
}

while IFS= read -r -d '' path; do
  if is_exempt_path "$path"; then
    continue
  fi

  case "$path" in
    .env)
      violations+=("$path (root .env is not allowed in git)")
      ;;
    *.pem|*.key|*.p12|*.pfx|id_rsa|id_ed25519)
      violations+=("$path (private key/certificate artifact)")
      ;;
    *.secret|*.secrets|*.env.local)
      violations+=("$path (secret dump/local env file)")
      ;;
  esac

done < <(git ls-files -z)

if ((${#violations[@]} > 0)); then
  echo "Committed secret guard failed. Remove these tracked files:" >&2
  printf ' - %s\n' "${violations[@]}" >&2
  exit 1
fi

echo "Committed secret guard passed."
