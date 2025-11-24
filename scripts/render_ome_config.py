#!/usr/bin/env python3
import argparse
from pathlib import Path
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


def xml_escape(value: str) -> str:
    return escape(value, {"'": "&apos;", '"': "&quot;"})


def render(template: Path, output: Path, bind: str, username: str, password: str) -> None:
    text = template.read_text()
    text = replace_tag_content(text, "Bind", xml_escape(bind))
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
