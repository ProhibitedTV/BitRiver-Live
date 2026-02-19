#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT_DIR"

compose_file="deploy/docker-compose.yml"
env_example="deploy/.env.example"
contract_doc="docs/contract.md"
generated_begin='<!-- BEGIN GENERATED ENV -->'
generated_end='<!-- END GENERATED ENV -->'

echo "Checking deployment contract invariants..."

if [[ ! -f "$compose_file" ]]; then
  echo "Missing required file: $compose_file" >&2
  exit 1
fi

echo "Found $compose_file"

if command -v docker >/dev/null 2>&1; then
  echo "Validating docker compose config"
  docker compose -f "$compose_file" config >/dev/null
else
  echo "Skipping docker compose config validation: docker is not installed or not on PATH."
fi

if [[ ! -f "$env_example" ]]; then
  echo "Missing required file: $env_example" >&2
  exit 1
fi

echo "Found $env_example"

if [[ ! -f "$contract_doc" ]]; then
  echo "Missing required file: $contract_doc" >&2
  exit 1
fi

if ! grep -Fq "$generated_begin" "$contract_doc"; then
  echo "Missing generated section begin marker in $contract_doc: $generated_begin" >&2
  exit 1
fi

if ! grep -Fq "$generated_end" "$contract_doc"; then
  echo "Missing generated section end marker in $contract_doc: $generated_end" >&2
  exit 1
fi

echo "Found generated section markers in $contract_doc"

./scripts/generate-contract-doc.sh --check

python3 - <<'PY'
from pathlib import Path
import re
import sys

root = Path.cwd()
contract_path = root / "docs/contract.md"
lines = contract_path.read_text(encoding="utf-8").splitlines()

artifact_pattern = re.compile(r"`(deploy/[^`]*generated[^`]*)`")
script_pattern = re.compile(r"`(scripts/[^`]+\.sh)`")

artifacts = {}
for idx, line in enumerate(lines):
    for match in artifact_pattern.finditer(line):
        artifacts.setdefault(match.group(1), []).append(idx)

errors = []

for artifact, positions in artifacts.items():
    artifact_path = root / artifact
    if artifact_path.exists():
        continue

    script_found = None
    for pos in positions:
        start = max(0, pos - 2)
        end = min(len(lines), pos + 9)
        for candidate in lines[start:end]:
            script_match = script_pattern.search(candidate)
            if not script_match:
                continue
            script_path = script_match.group(1)
            if (root / script_path).exists():
                script_found = script_path
                break
            errors.append(
                f"Artifact {artifact} is missing and references non-existent generator script {script_path}."
            )
        if script_found:
            break

    if not script_found:
        errors.append(
            f"Artifact {artifact} is missing and no existing generator script was found near its contract entry."
        )

if errors:
    print("Contract generated artifact checks failed:", file=sys.stderr)
    for err in errors:
        print(f"- {err}", file=sys.stderr)
    sys.exit(1)

print("Generated artifact references are valid")
PY

echo "Contract invariants check passed."
