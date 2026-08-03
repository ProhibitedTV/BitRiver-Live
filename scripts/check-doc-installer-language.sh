#!/usr/bin/env bash
set -Eeuo pipefail

files=(
  "README.md"
  "CHANGELOG.md"
  "docs/quickstart.md"
  "docs/installing-on-ubuntu.md"
  "docs/production-release.md"
  "docs/production-status.md"
  "docs/releases/README.md"
  "docs/releases/v1.2.3-draft.md"
  "docs/viewer-deployment.md"
  "docs/labs/cross-platform-plan.md"
  "docs/advanced-deployments.md"
)

patterns=(
  "placeholders for launchd/Windows Service"
  "Release artifacts:\\s*Ship"
  "not yet shipped"
  "installer milestones.*future"
  "no (tagged )?GitHub Release"
  "no published GitHub Release"
  "does not currently publish a GitHub Release"
  "planned first publication"
  "v1\.2\.3-rc\.(1|4)([^0-9]|$)"
  "actions/(upload|deploy)-pages"
)

failed=0
for file in "${files[@]}"; do
  for pattern in "${patterns[@]}"; do
    if grep -Ein "$pattern" "$file" >/dev/null; then
      echo "stale installer wording detected in $file (pattern: $pattern)" >&2
      grep -Ein "$pattern" "$file" >&2
      failed=1
    fi
  done
done

required_current_candidate_files=(
  "README.md"
  "CHANGELOG.md"
  "docs/quickstart.md"
  "docs/installing-on-ubuntu.md"
  "docs/production-status.md"
  "docs/releases/README.md"
)
for file in "${required_current_candidate_files[@]}"; do
  if ! grep -Fq 'v1.2.3-rc.12' "$file"; then
    echo "current release candidate missing from $file" >&2
    failed=1
  fi
done

for screenshot in \
  'docs/assets/screenshots/viewer-home.png' \
  'docs/assets/screenshots/live-directory.png'; do
  if [[ ! -s $screenshot ]] || ! grep -Fq "$screenshot" README.md; then
    echo "README product capture is missing or empty: $screenshot" >&2
    failed=1
  fi
done

if [[ ! -s docs/releases/v1.2.3-rc.12.md ]]; then
  echo "current release note is missing: docs/releases/v1.2.3-rc.12.md" >&2
  failed=1
fi

if [[ $failed -ne 0 ]]; then
  cat >&2 <<'MSG'
Documentation consistency check failed.
Update installer/release milestone wording to reflect shipped status.
MSG
  exit 1
fi

echo "docs installer wording check passed"
