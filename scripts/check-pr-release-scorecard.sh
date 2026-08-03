#!/usr/bin/env bash
set -Eeuo pipefail

body_file=""
changed_files=""
strict_mode="advisory"
warnings=()

usage() {
  cat <<'USAGE'
Usage: ./scripts/check-pr-release-scorecard.sh --body PR_BODY.md [--changed-files FILE] [--advisory|--strict|--strict-if-risky]

Validates the PR release scorecard fields used by .github/pull_request_template.md.
Default mode is advisory: warnings are printed but do not fail the command.

Options:
  --body FILE            PR body markdown to validate.
  --changed-files FILE   Optional newline-delimited changed-file list.
  --advisory             Print warnings and exit 0. This is the default.
  --strict               Exit 1 when warnings are found.
  --strict-if-risky      Exit 1 on warnings only when medium/high risk is
                         selected or changed paths affect code, CI, deployment,
                         dependencies, packaging, or operator workflows.
  -h, --help             Show this help.
USAGE
}

add_warning() {
  warnings+=("$1")
}

has_heading() {
  local heading="$1"
  grep -Eiq "^#+[[:space:]]+${heading}[[:space:]]*$" "$body_file"
}

has_checked_label() {
  local label="$1"
  awk -v target="$label" '
    /^[[:space:]]*-[[:space:]]*\[[xX]\]/ {
      line = tolower($0)
      target_lower = tolower(target)
      sub(/^[[:space:]]*-[[:space:]]*\[[xX]\][[:space:]]*/, "", line)
      if (index(line, target_lower) == 1) {
        found = 1
      }
    }
    END { exit found ? 0 : 1 }
  ' "$body_file"
}

has_checked_any() {
  local label
  for label in "$@"; do
    if has_checked_label "$label"; then
      return 0
    fi
  done
  return 1
}

changed_matches() {
  local pattern="$1"
  [[ -n "$changed_files" && -s "$changed_files" ]] || return 1
  grep -Eiq "$pattern" "$changed_files"
}

changed_files_require_strict() {
  changed_matches '(^|/)(\.github/|cmd/|internal/|scripts/|deploy/|third_party/|web/viewer/)|(^|/)(go\.mod|go\.sum|\.go-version|package\.json|package-lock\.json|Dockerfile[^/]*)$'
}

validate_required_shape() {
  local heading
  for heading in \
    "Release scorecard" \
    "Change classification" \
    "Risk level" \
    "Evidence map" \
    "Operator/release impact" \
    "Medium/high-risk review prompts"; do
    if ! has_heading "$heading"; then
      add_warning "Missing '${heading}' heading from the release scorecard."
    fi
  done
}

validate_selected_fields() {
  if ! has_checked_any \
    "docs-only" \
    "test-only" \
    "build/CI" \
    "viewer/UI" \
    "API/control plane" \
    "deployment/Compose/env" \
    "auth/security" \
    "data/migrations" \
    "release packaging" \
    "operator workflow"; then
    add_warning "Select at least one change classification."
  fi

  if ! has_checked_any "low" "medium" "high"; then
    add_warning "Select exactly one risk level: low, medium, or high."
  fi

  if ! has_checked_any \
    "No operator-facing impact" \
    "Docs updated" \
    "Release notes/changelog follow-up needed" \
    "Upgrade notes required" \
    "Rollback/canary notes included"; then
    add_warning "Select an operator/release impact option."
  fi
}

validate_medium_high_evidence() {
  if has_checked_any "medium" "high" && ! has_checked_any \
    "Unit/focused tests" \
    "Viewer lint/tests" \
    "./scripts/verify.sh" \
    "Compose/contract/release gate" \
    "Manual operator-path check" \
    "Docs/release notes" \
    "Blocked/skipped checks explained"; then
    add_warning "Medium/high-risk changes need verification evidence or an explicit blocked/skipped-check explanation."
  fi
}

validate_changed_file_mismatches() {
  [[ -n "$changed_files" ]] || return 0

  if changed_matches '(^|/)(deploy/docker-compose\.yml|deploy/\.env\.example|deploy/ome/|\.env$)|docker-compose' &&
    ! has_checked_label "deployment/Compose/env"; then
    add_warning "Changed deployment/env/Compose files should select 'deployment/Compose/env'."
  fi

  if changed_matches '(^|/)deploy/migrations/' &&
    ! has_checked_label "data/migrations"; then
    add_warning "Changed migrations should select 'data/migrations'."
  fi

  if changed_matches '(^|/)(\.github/|scripts/|go\.mod$|go\.sum$|package\.json$|package-lock\.json$|Dockerfile)' &&
    ! has_checked_any "build/CI" "release packaging"; then
    add_warning "Changed workflow/build/script files should select 'build/CI' or 'release packaging'."
  fi

  if changed_matches 'auth|security|cors|session|credential|secret|oauth|mfa|rate.limit' &&
    ! has_checked_label "auth/security"; then
    add_warning "Changed auth/security-sensitive files should select 'auth/security'."
  fi

  if changed_matches 'release|production-release|install|launcher|wrapper|quickstart|smoke' &&
    ! has_checked_any "release packaging" "operator workflow"; then
    add_warning "Changed release/operator-path files should select 'release packaging' or 'operator workflow'."
  fi

  if has_checked_label "low" && changed_files_require_strict; then
    add_warning "Low risk looks inconsistent with code, workflow, dependency, deployment, security, or release-path changes."
  fi
}

while (($# > 0)); do
  case "$1" in
    --body)
      [[ $# -ge 2 ]] || {
        echo "--body requires a file path." >&2
        exit 2
      }
      body_file="$2"
      shift 2
      ;;
    --changed-files)
      [[ $# -ge 2 ]] || {
        echo "--changed-files requires a file path." >&2
        exit 2
      }
      changed_files="$2"
      shift 2
      ;;
    --strict)
      strict_mode="strict"
      shift
      ;;
    --strict-if-risky)
      strict_mode="risk"
      shift
      ;;
    --advisory)
      strict_mode="advisory"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "$body_file" ]]; then
  echo "--body is required." >&2
  usage >&2
  exit 2
fi

if [[ ! -f "$body_file" ]]; then
  echo "PR body file not found: $body_file" >&2
  exit 2
fi

if [[ -n "$changed_files" && ! -f "$changed_files" ]]; then
  echo "Changed-files list not found: $changed_files" >&2
  exit 2
fi

validate_required_shape
validate_selected_fields
validate_medium_high_evidence
validate_changed_file_mismatches

if ((${#warnings[@]} > 0)); then
  echo "PR release scorecard warnings:"
  printf ' - %s\n' "${warnings[@]}"
  if [[ "$strict_mode" == "strict" ]]; then
    echo "Strict mode: warnings fail this command."
    exit 1
  fi
  if [[ "$strict_mode" == "risk" ]] &&
    { has_checked_any "medium" "high" || changed_files_require_strict; }; then
    echo "Risk-triggered strict mode: warnings fail this command."
    exit 1
  fi
  echo "Advisory mode: warnings do not fail this command."
  exit 0
fi

echo "PR release scorecard check passed."
