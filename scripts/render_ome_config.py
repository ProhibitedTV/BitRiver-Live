#!/usr/bin/env python3
import argparse
from pathlib import Path
import re
import sys
from xml.sax.saxutils import escape


def replace_tag_content(data: str, tag: str, value: str) -> str:
    """Replace the *first* occurrence of <tag>...</tag> in data."""
    open_tag = f"<{tag}>"
    close_tag = f"</{tag}>"

    start = data.find(open_tag)
    if start == -1:
        raise SystemExit(f"missing {open_tag} in template")

    end = data.find(close_tag, start)
    if end == -1:
        raise SystemExit(f"missing {close_tag} in template")

    return data[: start + len(open_tag)] + value + data[end:]


def replace_all_tag_content(
    data: str, tag: str, value: str, *, required: bool = True
) -> str:
    """
    Replace *all* <tag>...</tag> occurrences in data.

    If required=False and no tags are found, data is returned unchanged.
    """
    pattern = re.compile(rf"(<{tag}>)([^<]*)(</{tag}>)")
    replaced, count = pattern.subn(
        lambda match: f"{match.group(1)}{value}{match.group(3)}", data
    )

    if count == 0 and required:
        raise SystemExit(f"missing <{tag}> in template")

    return replaced


def xml_escape(value: str) -> str:
    return escape(value, {"'": "&apos;", '"': "&quot;"})


def _replace_root_bindings(text: str, address: str, port: str, tls_port: str) -> str:
    """
    Replace <Bind> tags in the <Server> header.

    Newer templates use <Address> inside <Bind>; older ones may still have <IP>.
    We support both so quickstart can be rerun across versions.
    """
    header_match = re.search(r"<Server[^>]*>(.*?)<Modules>", text, re.DOTALL)
    if not header_match:
        raise SystemExit("missing <Server> header before <Modules> in template")

    header_start, header_end = header_match.span(1)
    header_body = header_match.group(1)

    bind_match = re.search(r"<Bind>(.*?)</Bind>", header_body, re.DOTALL)
    if not bind_match:
        raise SystemExit("missing <Bind> section under <Server> header in template")

    bind_start, bind_end = bind_match.span(1)
    bind_body = bind_match.group(1)

    # Prefer <Address> if present (new schema); otherwise fall back to <IP> (legacy).
    if "<Address>" in bind_body:
        bind_body = replace_tag_content(bind_body, "Address", address)
    elif "<IP>" in bind_body:
        bind_body = replace_tag_content(bind_body, "IP", address)

    # Port / TLSPort are still required.
    for tag, value in ("Port", port), ("TLSPort", tls_port):
        bind_body = replace_tag_content(bind_body, tag, value)

    header_body = header_body[:bind_start] + bind_body + header_body[bind_end:]
    return text[:header_start] + header_body + text[header_end:]


def _replace_root_ip(text: str, ip: str) -> str:
    """
    Replace the <IP> tag in the <Server> header (outside <Modules>).

    Some templates may omit a root-level <IP>; in that case we simply leave
    the header unchanged instead of failing.
    """
    header_match = re.search(r"<Server[^>]*>(.*?)<Modules>", text, re.DOTALL)
    if not header_match:
        raise SystemExit("missing <Server> header before <Modules> in template")

    header_start, header_end = header_match.span(1)
    header_body = header_match.group(1)

    if "<IP>" not in header_body:
        # Nothing to replace; keep the template as-is.
        return text

    header_body = replace_tag_content(header_body, "IP", ip)
    return text[:header_start] + header_body + text[header_end:]


def _scoped_replace_control_bindings(text: str, bind: str) -> str:
    """
    Legacy helper: rewrite <Bind>/<IP>/<Address> inside <Modules><Control><Server>
    if that section exists. If it does *not* exist (newer templates), this is a no-op.

    This function is now tolerant:
    - It does NOT enforce where Bind/IP/Address appear globally.
    - It does NOT fail if <Control> is missing.
    """
    control_match = re.search(r"<Control>(.*?)</Control>", text, re.DOTALL)
    if not control_match:
        # No legacy Control module; nothing to rewrite.
        return text

    control_start, control_end = control_match.span()
    control_body = text[control_start:control_end]

    server_match = re.search(r"<Server>(.*?)</Server>", control_body, re.DOTALL)
    if not server_match:
        # Unexpected legacy layout; leave as-is rather than fail.
        return text

    server_start, server_end = server_match.span()
    server_body = control_body[server_start:server_end]

    # Always rewrite <Bind> if present under Control.
    if "<Bind>" in server_body:
        server_body = replace_all_tag_content(server_body, "Bind", bind, required=False)

    # If this legacy section has IP or Address tags, also rewrite them, but
    # do not consider them required.
    if "<IP>" in server_body:
        server_body = replace_all_tag_content(server_body, "IP", bind, required=False)
    if "<Address>" in server_body:
        server_body = replace_all_tag_content(server_body, "Address", bind, required=False)

    # Splice Control section back together.
    control_body = control_body[:server_start] + server_body + control_body[server_end:]
    return text[:control_start] + control_body + text[control_end:]


def render(
    template: Path,
    output: Path,
    bind: str,
    server_ip: str,
    server_port: str,
    tls_port: str,
    username: str,
    password: str,
) -> None:
    escaped_bind = xml_escape(bind)
    escaped_port = xml_escape(server_port)
    escaped_tls_port = xml_escape(tls_port)
    text = template.read_text()

    # Normalize old <Server.bind> wrappers to <Bind> so very old templates don't break.
    text = re.sub(r"<\s*Server\.bind\s*>", "<Bind>", text)
    text = re.sub(r"</\s*Server\.bind\s*>", "</Bind>", text)

    text = _replace_root_bindings(text, escaped_bind, escaped_port, escaped_tls_port)
    text = _replace_root_ip(text, xml_escape(server_ip))
    text = _scoped_replace_control_bindings(text, escaped_bind)

    # These are still expected to exist somewhere (typically under <Authentication>).
    text = replace_tag_content(text, "ID", xml_escape(username))
    text = replace_tag_content(text, "Password", xml_escape(password))

    output.write_text(text)


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(
        description="Render OvenMediaEngine Server.xml from template"
    )
    parser.add_argument(
        "--template", required=True, type=Path, help="Path to the Server.xml template"
    )
    parser.add_argument(
        "--output",
        required=True,
        type=Path,
        help="Destination for the rendered Server.xml",
    )
    parser.add_argument(
        "--bind", required=True, help="Bind address for the OME server"
    )
    parser.add_argument(
        "--server-ip",
        help="Public IP address advertised by OME; defaults to --bind",
    )
    parser.add_argument(
        "--username", required=True, help="OME control username"
    )
    parser.add_argument(
        "--password", required=True, help="OME control password"
    )
    parser.add_argument(
        "--port", required=True, help="OME server port"
    )
    parser.add_argument(
        "--tls-port", required=True, help="OME server TLS port"
    )

    args = parser.parse_args(argv)
    server_ip = args.server_ip if args.server_ip is not None else args.bind
    render(
        args.template,
        args.output,
        args.bind,
        server_ip,
        args.port,
        args.tls_port,
        args.username,
        args.password,
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
