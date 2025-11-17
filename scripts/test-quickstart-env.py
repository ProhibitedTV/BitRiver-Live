#!/usr/bin/env python3
"""Check quickstart env defaults align with CI .env seeding."""
from __future__ import annotations

import pathlib
import re
import sys
from typing import Dict, Iterable, List, Tuple

REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent
QUICKSTART_PATH = REPO_ROOT / "scripts" / "quickstart.sh"
CI_SCRIPT_PATH = REPO_ROOT / "scripts" / "test-quickstart.sh"


def _extract_block(lines: List[str], start_marker: str, end_marker: str) -> List[str]:
    collecting = False
    block: List[str] = []
    for line in lines:
        if collecting:
            if line.strip() == end_marker:
                break
            block.append(line.rstrip("\n"))
        elif line.strip() == start_marker:
            collecting = True
    if not block:
        raise ValueError(f"Unable to find block starting with '{start_marker}'")
    return block


def parse_env_defaults(lines: Iterable[str]) -> Dict[str, str]:
    defaults: Dict[str, str] = {}
    start = "declare -A env_defaults=("
    end = ")"
    block = _extract_block(list(lines), start, end)
    pattern = re.compile(r"\[([^\]]+)\]='(.*)'")
    for entry in block:
        entry = entry.strip()
        if not entry or entry.startswith("#"):
            continue
        match = pattern.fullmatch(entry)
        if not match:
            raise ValueError(f"Unexpected env_defaults line: {entry}")
        key, value = match.groups()
        defaults[key] = value
    return defaults


def parse_required_keys(lines: Iterable[str]) -> List[str]:
    start = "required_env_keys=("
    end = ")"
    block = _extract_block(list(lines), start, end)
    keys: List[str] = []
    for entry in block:
        entry = entry.strip()
        if not entry or entry.startswith("#"):
            continue
        keys.append(entry)
    return keys


def parse_seed_env(lines: Iterable[str]) -> Dict[str, str]:
    start = "cat >\"$ENV_FILE\" <<'ENV'"
    end = "ENV"
    block = _extract_block(list(lines), start, end)
    env: Dict[str, str] = {}
    for line in block:
        if not line or line.startswith("#"):
            continue
        if "=" not in line:
            raise ValueError(f"Unexpected env line: {line}")
        key, value = line.split("=", 1)
        env[key] = value
    return env


def diff_values(defaults: Dict[str, str], seed_env: Dict[str, str], keys: Iterable[str]) -> List[Tuple[str, str, str]]:
    mismatches: List[Tuple[str, str, str]] = []
    for key in keys:
        default_val = defaults.get(key)
        seed_val = seed_env.get(key)
        if default_val is None:
            mismatches.append((key, "<missing in env_defaults>", seed_val or ""))
            continue
        if seed_val is None:
            mismatches.append((key, default_val, "<missing in test-quickstart .env>"))
            continue
        if default_val != seed_val:
            mismatches.append((key, default_val, seed_val))
    return mismatches


def main() -> int:
    quickstart_lines = QUICKSTART_PATH.read_text().splitlines()
    ci_script_lines = CI_SCRIPT_PATH.read_text().splitlines()

    env_defaults = parse_env_defaults(quickstart_lines)
    required_keys = parse_required_keys(quickstart_lines)
    seed_env = parse_seed_env(ci_script_lines)

    mismatches = diff_values(env_defaults, seed_env, required_keys)

    if mismatches:
        print("quickstart env defaults diverged from test-quickstart.sh:", file=sys.stderr)
        for key, default_val, seed_val in mismatches:
            print(f"  {key}: quickstart='{default_val}' test-quickstart='{seed_val}'", file=sys.stderr)
        return 1

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
