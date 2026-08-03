from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from scripts.check_doc_links import local_target, validate_paths


class CheckDocLinksTests(unittest.TestCase):
    def test_external_and_anchor_targets_are_ignored(self) -> None:
        self.assertIsNone(local_target("https://example.com/path"))
        self.assertIsNone(local_target("mailto:security@example.com"))
        self.assertIsNone(local_target("#local-heading"))

    def test_existing_relative_file_and_directory_pass(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            docs = root / "docs"
            docs.mkdir()
            (root / "README.md").write_text("# Root\n", encoding="utf-8")
            source = docs / "guide.md"
            source.write_text("[root](../README.md) [docs](./)\n", encoding="utf-8")
            self.assertEqual(validate_paths(root, [source]), [])

    def test_missing_and_wrong_case_targets_fail(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            docs = root / "docs"
            docs.mkdir()
            (docs / "Actual.md").write_text("# Actual\n", encoding="utf-8")
            source = docs / "guide.md"
            source.write_text(
                "[missing](missing.md) [wrong case](actual.md)\n",
                encoding="utf-8",
            )
            failures = validate_paths(root, [source])
            self.assertEqual(len(failures), 2)
            self.assertIn("missing.md", failures[0])
            self.assertIn("actual.md", failures[1])


if __name__ == "__main__":
    unittest.main()
