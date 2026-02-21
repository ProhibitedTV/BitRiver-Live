#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
WORKFLOWS_DIR="$ROOT_DIR/.github/workflows"

if [[ ! -d "$WORKFLOWS_DIR" ]]; then
  echo "Missing workflows directory: $WORKFLOWS_DIR" >&2
  exit 1
fi

failures=()

add_failure() {
  failures+=("$1")
}

has_pull_request_trigger() {
  local file="$1"
  rg -q '^[[:space:]]pull_request:' "$file"
}

has_verify_gate_invocation() {
  local file="$1"
  rg -q '(\./scripts/verify\.sh|\./scripts/test-all\.sh)' "$file"
}

check_pr_workflows_require_verify_gate() {
  local file
  while IFS= read -r file; do
    if has_pull_request_trigger "$file" && ! has_verify_gate_invocation "$file"; then
      add_failure "$(basename "$file"): workflows triggered by pull_request must run ./scripts/verify.sh (or ./scripts/test-all.sh). Add a step such as: run: ./scripts/test-all.sh"
    fi
  done < <(find "$WORKFLOWS_DIR" -maxdepth 1 -type f -name '*.yml' | sort)
}

check_duplicate_verify_checks() {
  local -a patterns=(
    'go test ./... -count=1 -timeout=120s|Go unit tests are already run by scripts/verify.sh'
    './scripts/check-architecture-deps.sh|Architecture dependency checks already run in scripts/verify.sh'
    './scripts/check-no-models-imports.sh|internal/models import guard already runs in scripts/verify.sh'
    './scripts/check-dependency-source.sh|dependency-source checks already run in scripts/verify.sh'
    './scripts/check-contract-invariants.sh|contract invariants already run in scripts/verify.sh'
    './scripts/require-image-digests.sh|image digest checks already run in scripts/verify.sh'
    'docker compose -f deploy/docker-compose.yml config|compose config validation already runs in scripts/verify.sh'
    'npm --prefix web/viewer run lint|viewer lint already runs in scripts/verify.sh when viewer checks are enabled'
    'npm --prefix web/viewer run test|viewer tests already run in scripts/verify.sh when viewer checks are enabled'
  )

  local file line match pattern message start end context
  while IFS= read -r file; do
    local base
    base="$(basename "$file")"
    if [[ "$base" == "release.yml" ]]; then
      continue
    fi

    for entry in "${patterns[@]}"; do
      pattern="${entry%%|*}"
      message="${entry#*|}"

      while IFS= read -r match; do
        [[ -n "$match" ]] || continue
        line="${match%%:*}"

        start=$((line - 3))
        if ((start < 1)); then
          start=1
        fi
        end=$((line - 1))

        context=""
        if ((end >= start)); then
          context="$(sed -n "${start},${end}p" "$file")"
        fi

        if ! grep -q 'ci-contract: allow-duplicate' <<<"$context"; then
          add_failure "$(basename "$file"):$line duplicates verify.sh checks (${message}). Remove the duplicate command or add a nearby justification comment: '# ci-contract: allow-duplicate <reason>'."
        fi
      done < <(rg -n --fixed-strings "$pattern" "$file" || true)
    done
  done < <(find "$WORKFLOWS_DIR" -maxdepth 1 -type f -name '*.yml' | sort)
}

check_standalone_workflow_triggers() {
  local file base
  while IFS= read -r file; do
    base="$(basename "$file")"

    # CI orchestrator intentionally handles pull_request; release pipeline is out of CI-contract scope.
    if [[ "$base" == 'ci.yml' || "$base" == 'release.yml' ]]; then
      continue
    fi

    if rg -q '^[[:space:]](push|pull_request|pull_request_target|schedule):' "$file"; then
      add_failure "${base}: standalone CI workflows must use workflow_dispatch and/or workflow_call only. Remove automatic triggers (push/pull_request/schedule)."
    fi
  done < <(find "$WORKFLOWS_DIR" -maxdepth 1 -type f -name '*.yml' | sort)
}

check_pr_workflows_require_verify_gate
check_duplicate_verify_checks
check_standalone_workflow_triggers

if ((${#failures[@]} > 0)); then
  echo "CI contract violations found:" >&2
  printf ' - %s\n' "${failures[@]}" >&2
  exit 1
fi

echo "CI contract checks passed."
