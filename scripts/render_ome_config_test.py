#!/usr/bin/env python3
"""Integration test for render_ome_config.py."""

from __future__ import annotations

from pathlib import Path
import re
import sys
import tempfile
import unittest

# Make sure the renderer module is importable when executed from the repo root.
REPO_ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(Path(__file__).resolve().parent))

import render_ome_config


def _ensure_bind_address_placeholder(text: str) -> str:
    """Guarantee the template includes a bind address placeholder."""

    bind_match = re.search(r"<Bind>(.*?)</Bind>", text, re.DOTALL)
    if not bind_match:
        return text

    bind_body = bind_match.group(1)
    if "<Address>" in bind_body or "<IP>" in bind_body:
        return text

    bind_with_placeholder = (
        "\n        <Address>ADDRESS_PLACEHOLDER</Address>" + bind_body
    )

    start, end = bind_match.span(1)
    return text[:start] + bind_with_placeholder + text[end:]


def _ensure_credentials_placeholders(text: str) -> str:
    """Guarantee the template includes credentials placeholders."""

    if "<ID>" not in text:
        text = text.replace(
            "<Modules>",
            "    <ID>OME_USER_PLACEHOLDER</ID>\n"
            "    <Password>OME_PASSWORD_PLACEHOLDER</Password>\n\n    <Modules>",
            1,
        )

    if "<Password>" not in text:
        text = text.replace(
            "<Modules>",
            "    <Password>OME_PASSWORD_PLACEHOLDER</Password>\n\n    <Modules>",
            1,
        )

    return text


def _prepare_template(tmpdir: Path) -> tuple[Path, Path]:
    source = REPO_ROOT / "deploy" / "ome" / "Server.xml"
    text = source.read_text()

    text = _ensure_bind_address_placeholder(text)
    text = _ensure_credentials_placeholders(text)

    template_copy = tmpdir / "Server.xml"
    output_path = tmpdir / "Rendered.xml"
    template_copy.write_text(text)
    return template_copy, output_path


class RenderOmeConfigTest(unittest.TestCase):
    def test_render_substitutes_expected_fields(self) -> None:
        with tempfile.TemporaryDirectory() as td:
            tmpdir = Path(td)
            template, output = _prepare_template(tmpdir)

            render_ome_config.render(
                template,
                output,
                bind="10.0.0.1",
                server_ip="203.0.113.9",
                server_port="12345",
                tls_port="12346",
                username="ome-user",
                password="s3cret",
                access_token="access-token-123",
            )

            rendered = output.read_text()

            self.assertRegex(
                rendered,
                r"<(Address|IP)>10\.0\.0\.1</(Address|IP)>",
                "Bind address should be rewritten",
            )
            self.assertIn("<IP>203.0.113.9</IP>", rendered)
            self.assertIn("<Port>12345</Port>", rendered)
            self.assertIn("<TLSPort>12346</TLSPort>", rendered)
            self.assertIn("<ID>ome-user</ID>", rendered)
            self.assertIn("<Password>s3cret</Password>", rendered)
            self.assertIn("<AccessToken>access-token-123</AccessToken>", rendered)

            self.assertNotIn("Server.bind", rendered)
            self.assertNotIn("<Modules><Control", rendered)
            self.assertNotIn("<Bind><IP>", rendered)


if __name__ == "__main__":
    unittest.main()
