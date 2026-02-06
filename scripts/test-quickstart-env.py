#!/usr/bin/env python3
"""Check quickstart env defaults align with CI .env seeding."""
from __future__ import annotations

import json
import os
import pathlib
import subprocess
import sys
import textwrap
from typing import Dict, Iterable, List, Tuple

REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent
CI_SCRIPT_PATH = REPO_ROOT / "scripts" / "test-quickstart.sh"
QUICKSTART_SCRIPT_PATH = REPO_ROOT / "scripts" / "quickstart.sh"
BITRIVER_CMD_DIR = REPO_ROOT / "cmd" / "bitriver"

# CI seeds some values to keep docker-compose smoke tests deterministic. These
# are intentionally not sourced from env init defaults (for example, generated
# secrets and release image tags).
CI_ONLY_KEYS = {
    "BITRIVER_LIVE_IMAGE_TAG",
    "BITRIVER_VIEWER_IMAGE_TAG",
    "BITRIVER_SRS_CONTROLLER_IMAGE_TAG",
    "BITRIVER_TRANSCODER_IMAGE_TAG",
    "BITRIVER_LIVE_MODE",
    "BITRIVER_LIVE_POSTGRES_DSN",
    "BITRIVER_POSTGRES_DB",
    "BITRIVER_POSTGRES_USER",
    "BITRIVER_POSTGRES_PASSWORD",
    "BITRIVER_REDIS_PASSWORD",
    "BITRIVER_REDIS_PORT",
    "BITRIVER_SRS_API_PORT",
    "BITRIVER_TRANSCODER_PUBLIC_BASE_URL",
    "BITRIVER_SRS_RTMP_PORT",
    "BITRIVER_LIVE_ADMIN_EMAIL",
    "BITRIVER_LIVE_ADMIN_PASSWORD",
    "BITRIVER_SRS_TOKEN",
    "BITRIVER_OME_USERNAME",
    "BITRIVER_OME_PASSWORD",
    "BITRIVER_OME_API_TOKEN",
    "BITRIVER_OME_ACCESS_TOKEN",
    "BITRIVER_TRANSCODER_TOKEN",
    "BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD",
}

LEGACY_DEFAULTS_START_MARKER = "cat >\"$ENV_FILE\" <<'ENV'"
LEGACY_DEFAULTS_END_MARKER = "ENV"


class ExtractionError(RuntimeError):
    """Raised when env defaults cannot be extracted from known sources."""


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


def parse_env_defaults_from_go_source() -> Dict[str, str]:
    helper_name = "quickstart_env_defaults_extractor_tmp.go"
    helper_path = BITRIVER_CMD_DIR / helper_name
    helper_source = textwrap.dedent(
        """
        package main

        import (
            "encoding/json"
            "fmt"
            "os"
            "strings"
        )

        func init() {
            templateLines, err := readEnvTemplate(defaultExampleEnv())
            if err != nil {
                fmt.Fprintf(os.Stderr, "read env template: %v\\n", err)
                os.Exit(1)
            }

            generated, _ := generateEnvValues(map[string]string{})
            merged := mergeEnv(templateLines, map[string]string{}, generated)

            values := map[string]string{}
            for _, line := range strings.Split(merged, "\\n") {
                line = strings.TrimSpace(line)
                if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
                    continue
                }
                key, value, _ := strings.Cut(line, "=")
                values[key] = value
            }

            if err := json.NewEncoder(os.Stdout).Encode(values); err != nil {
                fmt.Fprintf(os.Stderr, "encode defaults: %v\\n", err)
                os.Exit(1)
            }
            os.Exit(0)
        }
        """
    ).strip()

    helper_path.write_text(helper_source + "\n")
    try:
        proc = subprocess.run(
            ["go", "run", "./cmd/bitriver"],
            cwd=REPO_ROOT,
            check=True,
            capture_output=True,
            text=True,
            env={
                **dict(os.environ),
                "GOTOOLCHAIN": "local",
                "GOPROXY": "off",
                "GOSUMDB": "off",
            },
        )
    finally:
        helper_path.unlink(missing_ok=True)

    return json.loads(proc.stdout)


def parse_env_defaults_from_legacy_shell(lines: Iterable[str]) -> Dict[str, str]:
    block = _extract_block(list(lines), LEGACY_DEFAULTS_START_MARKER, LEGACY_DEFAULTS_END_MARKER)
    env: Dict[str, str] = {}
    for line in block:
        if not line or line.startswith("#"):
            continue
        if "=" not in line:
            raise ValueError(f"Unexpected env line in legacy quickstart defaults: {line}")
        key, value = line.split("=", 1)
        env[key] = value
    return env


def parse_env_defaults(quickstart_lines: Iterable[str]) -> Dict[str, str]:
    quickstart_line_list = list(quickstart_lines)
    has_legacy_markers = any(
        line.strip() == LEGACY_DEFAULTS_START_MARKER for line in quickstart_line_list
    )

    legacy_error: Exception | None = None
    if has_legacy_markers:
        try:
            return parse_env_defaults_from_legacy_shell(quickstart_line_list)
        except ValueError as err:
            legacy_error = err

    try:
        return parse_env_defaults_from_go_source()
    except (subprocess.CalledProcessError, json.JSONDecodeError, OSError) as err:
        detail: List[str] = [
            "Unable to extract quickstart env defaults from known sources.",
            f"Expected either legacy shell defaults in {QUICKSTART_SCRIPT_PATH} "
            f"or Go defaults from {BITRIVER_CMD_DIR}.",
            "If quickstart defaults moved, update extraction logic in scripts/test-quickstart-env.py.",
        ]
        if legacy_error is not None:
            detail.append(f"Legacy parser error: {legacy_error}")
        detail.append(f"Go-source parser error: {err}")
        raise ExtractionError("\n".join(detail)) from err


def parse_required_keys(lines: Iterable[str]) -> List[str]:
    seeded_keys = list(parse_seed_env(lines).keys())
    return [key for key in seeded_keys if key not in CI_ONLY_KEYS]


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
    quickstart_lines = QUICKSTART_SCRIPT_PATH.read_text().splitlines()
    ci_script_lines = CI_SCRIPT_PATH.read_text().splitlines()

    try:
        env_defaults = parse_env_defaults(quickstart_lines)
        required_keys = parse_required_keys(ci_script_lines)
        seed_env = parse_seed_env(ci_script_lines)
    except (ExtractionError, ValueError, subprocess.CalledProcessError) as err:
        print(f"quickstart env extraction failed: {err}", file=sys.stderr)
        return 1

    mismatches = diff_values(env_defaults, seed_env, required_keys)

    if mismatches:
        print("quickstart env defaults diverged from test-quickstart.sh:", file=sys.stderr)
        for key, default_val, seed_val in mismatches:
            print(f"  {key}: quickstart='{default_val}' test-quickstart='{seed_val}'", file=sys.stderr)
        return 1

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
