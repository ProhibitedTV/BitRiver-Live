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

    text = replace_all_tag_content(text, "Bind", escaped_bind)
    text = replace_all_tag_content(text, "IP", escaped_bind)
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
