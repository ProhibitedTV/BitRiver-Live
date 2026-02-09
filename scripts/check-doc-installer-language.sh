#!/usr/bin/env bash
set -Eeuo pipefail

files=(
  "README.md"
  "docs/cross-platform-plan.md"
  "docs/quickstart.md"
  "docs/advanced-deployments.md"
)

patterns=(
  "placeholders for launchd/Windows Service"
  "Release artifacts:\\s*Ship"
  "not yet shipped"
  "installer milestones.*future"
)

failed=0
for file in "${files[@]}"; do
  for pattern in "${patterns[@]}"; do
    if rg -n -i "$pattern" "$file" >/dev/null; then
      echo "stale installer wording detected in $file (pattern: $pattern)" >&2
      rg -n -i "$pattern" "$file" >&2
      failed=1
    fi
  done
done

if [[ $failed -ne 0 ]]; then
  cat >&2 <<'MSG'
Documentation consistency check failed.
Update installer/release milestone wording to reflect shipped status.
MSG
  exit 1
fi

echo "docs installer wording check passed"
