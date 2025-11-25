#!/usr/bin/env python3
import argparse
from pathlib import Path
import re
import sys
from xml.sax.saxutils import escape


def replace_tag_content(data: str, tag: str, value: str) -> str:
    open_tag = f"<{tag}>"
    close_tag = f"</{tag}>"

    start = data.find(open_tag)
    if start == -1:
        raise SystemExit(f"missing {open_tag} in template")

    end = data.find(close_tag, start)
    if end == -1:
        raise SystemExit(f"missing {close_tag} in template")

    return data[: start + len(open_tag)] + value + data[end:]


def replace_all_tag_content(data: str, tag: str, value: str) -> str:
    pattern = re.compile(rf"(<{tag}>)([^<]*)(</{tag}>)")
    replaced, count = pattern.subn(lambda match: f"{match.group(1)}{value}{match.group(3)}", data)

    if count == 0:
        raise SystemExit(f"missing <{tag}> in template")

    return replaced


def xml_escape(value: str) -> str:
    return escape(value, {"'": "&apos;", '"': "&quot;"})


def _scoped_replace_control_bindings(text: str, bind: str) -> str:
    """Replace <Bind> and <IP> tags within the control listener scope only."""

    control_match = re.search(r"<Control>(.*?)</Control>", text, re.DOTALL)
    if not control_match:
        raise SystemExit("missing <Control> section in template")

    control_start, control_end = control_match.span()
    control_body = text[control_start:control_end]

    server_match = re.search(r"<Server>(.*?)</Server>", control_body, re.DOTALL)
    if not server_match:
        raise SystemExit("missing <Server> section under <Control> in template")

    server_start, server_end = server_match.span()
    server_abs_start = control_start + server_start
    server_abs_end = control_start + server_end

    for tag in ("Bind", "IP"):
        for match in re.finditer(rf"<\s*{tag}\s*>", text):
            if match.start() < server_abs_start or match.end() > server_abs_end:
                raise SystemExit(
                    "Bind/IP entries must live under <Modules><Control><Server><Listeners><TCP>. "
                    "Move or delete the out-of-scope tag before rendering."
                )

    server_body = control_body[server_start:server_end]
    server_body = replace_all_tag_content(server_body, "Bind", bind)
    server_body = replace_all_tag_content(server_body, "IP", bind)
    control_body = control_body[:server_start] + server_body + control_body[server_end:]
    return text[:control_start] + control_body + text[control_end:]


def render(template: Path, output: Path, bind: str, username: str, password: str) -> None:
    escaped_bind = xml_escape(bind)
    text = template.read_text()

    # Normalize legacy <Server.bind> tags to <Bind> to avoid schema errors.
    text = re.sub(r"<\s*Server\.bind\s*>", "<Bind>", text)
    text = re.sub(r"</\s*Server\.bind\s*>", "</Bind>", text)

    # OvenMediaEngine 0.15.x rejects top-level <Bind>/<IP> elements ("Server.bind"),
    # so fail fast if they are present before rendering credentials.
    header_match = re.search(r"<Server[^>]*>(.*?)<Modules>", text, re.DOTALL)
    if header_match and re.search(r"<\s*Bind\s*>|<\s*IP\s*>", header_match.group(1)):
        raise SystemExit(
            "Top-level <Bind>/<IP> entries were detected in the OME template. "
            "Move bind configuration under <Modules><Control><Server><Listeners><TCP> "
            "to avoid Server.bind schema errors."
        )

    text = _scoped_replace_control_bindings(text, escaped_bind)
    text = replace_tag_content(text, "ID", xml_escape(username))
    text = replace_tag_content(text, "Password", xml_escape(password))
    output.write_text(text)


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description="Render OvenMediaEngine Server.xml from template")
    parser.add_argument("--template", required=True, type=Path, help="Path to the Server.xml template")
    parser.add_argument("--output", required=True, type=Path, help="Destination for the rendered Server.xml")
    parser.add_argument("--bind", required=True, help="Bind address for the OME server")
    parser.add_argument("--username", required=True, help="OME control username")
    parser.add_argument("--password", required=True, help="OME control password")

    args = parser.parse_args(argv)
    render(args.template, args.output, args.bind, args.username, args.password)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
