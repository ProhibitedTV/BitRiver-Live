#!/usr/bin/env python3
"""Fail when tracked public Markdown points at a missing local path."""

from __future__ import annotations

import os
import re
import subprocess
import sys
from pathlib import Path
from urllib.parse import unquote, urlsplit


INLINE_LINK = re.compile(r"!?\[[^\]]*\]\((?P<target><[^>]+>|[^)\s]+)(?:\s+['\"][^'\"]*['\"])?\)")
EXTERNAL_SCHEMES = {"data", "http", "https", "mailto", "tel"}
EXCLUDED_PREFIXES = ("docs/history/",)


def tracked_markdown(repo_root: Path) -> list[Path]:
    completed = subprocess.run(
        ["git", "ls-files", "*.md"],
        cwd=repo_root,
        check=True,
        capture_output=True,
        text=True,
    )
    return [
        repo_root / line
        for line in completed.stdout.splitlines()
        if line and not line.startswith(EXCLUDED_PREFIXES)
    ]


def exact_case_exists(path: Path, repo_root: Path) -> bool:
    repo_root = Path(os.path.abspath(repo_root))
    path = Path(os.path.abspath(path))
    try:
        relative = path.relative_to(repo_root)
    except ValueError:
        return False

    current = repo_root
    for part in relative.parts:
        if not current.is_dir():
            return False
        names = {child.name for child in current.iterdir()}
        if part not in names:
            return False
        current /= part
    return current.exists()


def local_target(raw_target: str) -> str | None:
    target = raw_target.strip("<>")
    parsed = urlsplit(target)
    if parsed.scheme.lower() in EXTERNAL_SCHEMES or target.startswith("#"):
        return None
    if parsed.scheme or parsed.netloc:
        return None
    return unquote(parsed.path)


def validate_paths(repo_root: Path, documents: list[Path]) -> list[str]:
    repo_root = Path(os.path.abspath(repo_root))
    failures: list[str] = []
    for document in documents:
        content = document.read_text(encoding="utf-8")
        for line_number, line in enumerate(content.splitlines(), start=1):
            for match in INLINE_LINK.finditer(line):
                raw_target = match.group("target")
                target = local_target(raw_target)
                if not target:
                    continue
                unresolved = (
                    repo_root / target.lstrip("/")
                    if target.startswith("/")
                    else document.parent / target
                )
                candidate = Path(os.path.abspath(unresolved))
                if not exact_case_exists(candidate, repo_root):
                    relative_document = document.relative_to(repo_root).as_posix()
                    failures.append(
                        f"{relative_document}:{line_number}: missing local target {raw_target}"
                    )
    return failures


def main() -> int:
    repo_root = Path(__file__).resolve().parent.parent
    documents = tracked_markdown(repo_root)
    failures = validate_paths(repo_root, documents)
    if failures:
        print("Markdown local-link check failed:", file=sys.stderr)
        for failure in failures:
            print(f"- {failure}", file=sys.stderr)
        return 1
    print(f"Markdown local-link check passed for {len(documents)} tracked public files.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
