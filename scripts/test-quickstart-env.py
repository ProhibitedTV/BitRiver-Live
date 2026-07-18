#!/usr/bin/env python3
"""Validate the CI quickstart smoke env fixture satisfies deployment requirements."""
from __future__ import annotations

import pathlib
import re
import sys
from typing import Dict, Iterable, List, Set

REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent
CI_SCRIPT_PATH = REPO_ROOT / "scripts" / "test-quickstart.sh"
COMPOSE_PATH = REPO_ROOT / "deploy" / "docker-compose.yml"


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


def main() -> int:
    ci_script_lines = CI_SCRIPT_PATH.read_text().splitlines()
    compose_lines = COMPOSE_PATH.read_text().splitlines()

    seed_env = parse_seed_env(ci_script_lines)
    required_keys = parse_required_credentials(compose_lines)

    missing = sorted(key for key in required_keys if not seed_env.get(key, "").strip())
    if missing:
        print("quickstart smoke env fixture is missing required deployment credentials:", file=sys.stderr)
        for key in missing:
            print(f"  {key}", file=sys.stderr)
        return 1

    required_runtime_keys = {
        "BITRIVER_SRS_PUBLIC_RTMP_BASE_URL",
        "BITRIVER_OME_PUBLIC_LLHLS_BASE_URL",
        "BITRIVER_TRANSCODER_PUBLIC_BASE_URL",
    }
    missing_runtime = sorted(
        key for key in required_runtime_keys if not seed_env.get(key, "").strip()
    )
    if missing_runtime:
        print(
            "quickstart smoke env fixture is missing required public media URLs:",
            file=sys.stderr,
        )
        for key in missing_runtime:
            print(f"  {key}", file=sys.stderr)
        return 1

    api_token = seed_env.get("BITRIVER_OME_API_TOKEN", "").strip()
    access_token = seed_env.get("BITRIVER_OME_ACCESS_TOKEN", "").strip()
    if not api_token:
        print("quickstart smoke env fixture must set BITRIVER_OME_API_TOKEN", file=sys.stderr)
        return 1
    if access_token and access_token != api_token:
        print(
            "quickstart smoke env fixture sets BITRIVER_OME_ACCESS_TOKEN, but it does not match BITRIVER_OME_API_TOKEN",
            file=sys.stderr,
        )
        return 1

    if "BITRIVER_OME_ACCESS_TOKEN" not in seed_env:
        print(
            "quickstart smoke env fixture should set BITRIVER_OME_ACCESS_TOKEN explicitly to avoid platform-dependent fallback behavior",
            file=sys.stderr,
        )
        return 1

    mode = seed_env.get("BITRIVER_LIVE_MODE", "").strip().lower()
    if mode != "development":
        print(
            "quickstart smoke env fixture must set BITRIVER_LIVE_MODE=development for loopback media URLs",
            file=sys.stderr,
        )
        return 1

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
