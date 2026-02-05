#!/usr/bin/env python3
"""Check Go function comment coverage."""

from __future__ import annotations

import argparse
import pathlib
import re
import subprocess
import sys
from dataclasses import dataclass

FUNC_RE = re.compile(r"^func\s*(?:\([^\n]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*\(")
GENERATED_RE = re.compile(r"^// Code generated .* DO NOT EDIT\.")


@dataclass
class FunctionRecord:
    path: pathlib.Path
    line_number: int
    name: str
    exported: bool
    has_comment: bool


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Validate Go function comment coverage for exported/unexported functions.",
    )
    parser.add_argument("--root", default=".", help="Repository root to scan (default: current directory).")
    parser.add_argument(
        "--unexported-dir",
        action="append",
        default=["cmd", "internal"],
        help="Directory prefix to include in unexported coverage checks. May be passed multiple times.",
    )
    parser.add_argument(
        "--unexported-threshold",
        type=float,
        default=80.0,
        help="Minimum percentage for unexported functions in selected dirs (default: 80).",
    )
    parser.add_argument(
        "--strict-unexported",
        action="store_true",
        help="Require 100%% comment coverage for unexported functions in selected dirs.",
    )
    parser.add_argument(
        "--git-base",
        default="",
        help=(
            "Limit enforcement to Go files changed compared to this git ref (for regression checks in CI). "
            "Example: origin/main"
        ),
    )
    return parser.parse_args()


def is_generated(lines: list[str]) -> bool:
    return any(GENERATED_RE.match(line.strip()) for line in lines[:20])


def has_immediate_comment(lines: list[str], idx: int) -> bool:
    prev = idx - 1
    if prev < 0:
        return False

    previous = lines[prev].strip()
    if not previous:
        return False

    if previous.startswith("//"):
        return True

    if "*/" in previous:
        walker = prev
        while walker >= 0:
            token = lines[walker].strip()
            if not token:
                return False
            if "/*" in token:
                return True
            walker -= 1
    return False


def changed_go_paths(root: pathlib.Path, git_base: str) -> set[pathlib.Path]:
    result = subprocess.run(
        ["git", "diff", "--name-only", f"{git_base}...HEAD"],
        cwd=root,
        check=True,
        text=True,
        capture_output=True,
    )
    selected: set[pathlib.Path] = set()
    for line in result.stdout.splitlines():
        candidate = pathlib.Path(line.strip())
        if not candidate or candidate.suffix != ".go":
            continue
        selected.add(candidate)
    return selected


def iter_go_files(root: pathlib.Path, only_paths: set[pathlib.Path] | None) -> list[pathlib.Path]:
    excluded_dirs = {"vendor", "third_party", ".git"}
    go_files: list[pathlib.Path] = []
    for path in root.rglob("*.go"):
        rel = path.relative_to(root)
        if any(part in excluded_dirs for part in rel.parts):
            continue
        if rel.name.endswith("_test.go"):
            continue
        if only_paths is not None and rel not in only_paths:
            continue
        go_files.append(path)
    return sorted(go_files)


def collect_functions(root: pathlib.Path, only_paths: set[pathlib.Path] | None) -> list[FunctionRecord]:
    funcs: list[FunctionRecord] = []
    for path in iter_go_files(root, only_paths):
        try:
            text = path.read_text(encoding="utf-8")
        except UnicodeDecodeError:
            continue

        lines = text.splitlines()
        if is_generated(lines):
            continue

        for idx, line in enumerate(lines):
            stripped = line.lstrip()
            if not stripped.startswith("func"):
                continue
            match = FUNC_RE.match(stripped)
            if not match:
                continue
            name = match.group(1)
            funcs.append(
                FunctionRecord(
                    path=path.relative_to(root),
                    line_number=idx + 1,
                    name=name,
                    exported=name[0].isupper(),
                    has_comment=has_immediate_comment(lines, idx),
                )
            )
    return funcs


def format_missing(records: list[FunctionRecord]) -> list[str]:
    return [
        f"  - {record.path}:{record.line_number} {record.name} ({'exported' if record.exported else 'unexported'})"
        for record in records
    ]


def in_scope(path: pathlib.Path, prefixes: list[str]) -> bool:
    path_str = path.as_posix()
    for prefix in prefixes:
        candidate = prefix.rstrip("/")
        if candidate and (path_str == candidate or path_str.startswith(candidate + "/")):
            return True
    return False


def main() -> int:
    args = parse_args()
    root = pathlib.Path(args.root).resolve()

    only_paths: set[pathlib.Path] | None = None
    if args.git_base:
        only_paths = changed_go_paths(root, args.git_base)

    funcs = collect_functions(root, only_paths)
    exported = [item for item in funcs if item.exported]
    unexported_scoped = [item for item in funcs if (not item.exported and in_scope(item.path, args.unexported_dir))]

    missing_exported = [item for item in exported if not item.has_comment]
    missing_unexported_scoped = [item for item in unexported_scoped if not item.has_comment]

    target_threshold = 100.0 if args.strict_unexported else args.unexported_threshold
    unexported_coverage = 100.0 if not unexported_scoped else (len(unexported_scoped) - len(missing_unexported_scoped)) / len(unexported_scoped) * 100.0

    passed = True

    if missing_exported:
        passed = False
        print("Exported functions missing immediate comments:")
        print("\n".join(format_missing(missing_exported)))
        print()

    if unexported_coverage + 1e-9 < target_threshold:
        passed = False
        print(f"Unexported function coverage is below threshold ({unexported_coverage:.1f}% < {target_threshold:.1f}%).")
        if missing_unexported_scoped:
            print("Missing comments for scoped unexported functions:")
            print("\n".join(format_missing(missing_unexported_scoped)))
            print()

    scope_text = "changed files" if args.git_base else "repo"
    print(
        "Function comment coverage summary "
        f"({scope_text}): exported {len(exported) - len(missing_exported)}/{len(exported)}; "
        f"unexported(scope={','.join(args.unexported_dir)}) "
        f"{len(unexported_scoped) - len(missing_unexported_scoped)}/{len(unexported_scoped)} "
        f"({unexported_coverage:.1f}%)."
    )

    return 0 if passed else 1


if __name__ == "__main__":
    sys.exit(main())
