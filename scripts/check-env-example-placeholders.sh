#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
COMPOSE_PATH="$ROOT_DIR/deploy/docker-compose.yml"
ENV_EXAMPLE_PATH="$ROOT_DIR/deploy/.env.example"
PYTHON_RUNNER=()

if [[ ! -f "$COMPOSE_PATH" ]]; then
  echo "Missing compose file: $COMPOSE_PATH" >&2
  exit 1
fi

if [[ ! -f "$ENV_EXAMPLE_PATH" ]]; then
  echo "Missing env example file: $ENV_EXAMPLE_PATH" >&2
  exit 1
fi

if python3 -c 'import sys' >/dev/null 2>&1; then
  PYTHON_RUNNER=(python3)
elif py -3 -c 'import sys' >/dev/null 2>&1; then
  PYTHON_RUNNER=(py -3)
elif python -c 'import sys' >/dev/null 2>&1; then
  PYTHON_RUNNER=(python)
else
  echo "Missing required Python interpreter: install python3, python, or the Windows py launcher." >&2
  exit 1
fi

"${PYTHON_RUNNER[@]}" - "$COMPOSE_PATH" "$ENV_EXAMPLE_PATH" <<'PY'
from __future__ import annotations

import math
import pathlib
import re
import sys
from typing import Dict, Iterable, List, Set

compose_path = pathlib.Path(sys.argv[1])
env_example_path = pathlib.Path(sys.argv[2])


def parse_required_credentials(lines: Iterable[str]) -> Set[str]:
    required: Set[str] = set()
    pattern = re.compile(r"^\s{2}([A-Z0-9_]+):\s+\$\{[^}]*\?set via \.env}\s*$")

    in_anchor = False
    for raw in lines:
        line = raw.rstrip("\n")
        if line.startswith("x-required-credentials:"):
            in_anchor = True
            continue
        if in_anchor and line and not line.startswith(" "):
            break
        if not in_anchor:
            continue
        match = pattern.match(line)
        if match:
            required.add(match.group(1))

    if not required:
        raise ValueError("Unable to parse required credentials from deploy/docker-compose.yml")
    return required


def parse_env_file(lines: Iterable[str]) -> Dict[str, str]:
    env: Dict[str, str] = {}
    for raw in lines:
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if "=" not in line:
            continue
        key, value = line.split("=", 1)
        env[key.strip()] = value.strip()
    return env


def shannon_entropy(value: str) -> float:
    if not value:
        return 0.0
    counts = {ch: value.count(ch) for ch in set(value)}
    entropy = 0.0
    length = len(value)
    for count in counts.values():
        p = count / length
        entropy -= p * math.log2(p)
    return entropy


def has_example_marker(value: str) -> bool:
    lower = value.lower()
    markers = (
        "example",
        "example.com",
        "sample",
        "placeholder",
        "changeme",
        "-example",
        "_example",
    )
    return any(marker in lower for marker in markers)


def looks_random_secret(value: str) -> bool:
    compact = value.strip()
    if len(compact) < 24:
        return False
    jwt_like = re.match(r"^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$", compact)
    if jwt_like:
        return True
    tokenish = re.match(r"^[A-Za-z0-9+/=_-]{24,}$", compact)
    if not tokenish:
        return False
    return shannon_entropy(compact) >= 3.6


required_keys = parse_required_credentials(compose_path.read_text().splitlines())
env_values = parse_env_file(env_example_path.read_text().splitlines())

failures: List[str] = []

for key in sorted(required_keys):
    value = env_values.get(key, "").strip()
    if not value:
        failures.append(f"{key}: required credential placeholder must be set and non-empty in deploy/.env.example")
        continue

    is_secret_bearing = bool(re.search(r"(PASSWORD|TOKEN|SECRET|KEY)", key))
    if key.endswith("_EMAIL"):
        if "example.com" not in value.lower():
            failures.append(f"{key}: email placeholders must use the example.com domain")

    if is_secret_bearing and not has_example_marker(value):
        failures.append(
            f"{key}: secret placeholder must include an explicit sample marker (for example '*-example', 'Example', or 'sample')"
        )

    if looks_random_secret(value) and not has_example_marker(value):
        failures.append(
            f"{key}: value looks like a production token; replace with clearly marked example placeholder"
        )

if failures:
    print("deploy/.env.example placeholder hygiene check failed:", file=sys.stderr)
    for failure in failures:
        print(f"  - {failure}", file=sys.stderr)
    sys.exit(1)

print("deploy/.env.example placeholder hygiene check passed.")
PY
