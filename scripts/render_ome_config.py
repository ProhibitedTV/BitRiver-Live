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


def replace_optional_tag_content(data: str, tag: str, value: str) -> str:
    if f"<{tag}>" not in data:
        return data
    return replace_tag_content(data, tag, value)


def remove_first_tag_block(data: str, tag: str) -> str:
    """Remove the first occurrence of <tag>...</tag> in data, if present."""

    pattern = re.compile(rf"\s*<{tag}>.*?</{tag}>\s*", re.DOTALL)
    return pattern.sub("\n", data, count=1)


def _managers_authentication_supported(image_tag: str | None) -> bool:
    """Return True when the provided image tag advertises managers auth support."""

    if image_tag is None:
        return True

    match = re.match(r"^v?(?P<major>\d+)\.(?P<minor>\d+)\.(?P<patch>\d+)", image_tag)
    if match is None:
        return False

    major = int(match.group("major"))
    minor = int(match.group("minor"))

    if major == 0 and minor < 16:
        return False

    return True


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
    Replace the <Bind> block under <Server>.

    We don't care whether <Bind> appears before or after <Modules>/<VirtualHosts>;
    we simply find the first <Bind>...</Bind> inside <Server> and treat it as
    the server-level bind config.
    """
    server_match = re.search(r"<Server[^>]*>(.*)</Server>", text, re.DOTALL)
    if not server_match:
        raise SystemExit("missing <Server> root element in template")

    server_start, server_end = server_match.span(1)
    server_body = server_match.group(1)

    bind_match = re.search(r"<Bind>(.*?)</Bind>", server_body, re.DOTALL)
    if not bind_match:
        raise SystemExit("missing <Bind> section under <Server> in template")

    bind_start, bind_end = bind_match.span(1)
    bind_body = bind_match.group(1)

    # Prefer <Address> if present (new schema); otherwise fall back to <IP> (legacy).
    if "<Address>" in bind_body:
        bind_body = replace_tag_content(bind_body, "Address", address)
    elif "<IP>" in bind_body:
        bind_body = replace_tag_content(bind_body, "IP", address)

    # Rewrite all <Signalling> port pairs when present; otherwise, fall back to
    # the first <Port>/<TLSPort> in the bind body.
    def _rewrite_signalling(match: re.Match[str]) -> str:
        inner = match.group(1)
        inner = replace_tag_content(inner, "Port", port)
        inner = replace_tag_content(inner, "TLSPort", tls_port)
        return f"<Signalling>{inner}</Signalling>"

    bind_body, signalling_rewrites = re.subn(
        r"<Signalling>(.*?)</Signalling>", _rewrite_signalling, bind_body, flags=re.DOTALL
    )

    if signalling_rewrites == 0:
        for tag, value in (("Port", port), ("TLSPort", tls_port)):
            bind_body = replace_tag_content(bind_body, tag, value)

    server_body = server_body[:bind_start] + bind_body + server_body[bind_end:]
    return text[:server_start] + server_body + text[server_end:]


def _replace_root_ip(text: str, ip: str) -> str:
    """
    Replace the <IP> tag directly under <Server> (outside <Bind>).

    Some templates may omit a root-level <IP>; in that case we simply leave
    the document unchanged instead of failing.
    """
    server_match = re.search(r"<Server[^>]*>(.*)</Server>", text, re.DOTALL)
    if not server_match:
        raise SystemExit("missing <Server> root element in template")

    server_start, server_end = server_match.span(1)
    server_body = server_match.group(1)

    for match in re.finditer(r"<IP>(.*?)</IP>", server_body, re.DOTALL):
        ip_start, ip_end = match.span(1)

        # Skip IPs that live inside <Bind> or <VirtualHosts> blocks.
        bind_open = server_body.rfind("<Bind>", 0, ip_start)
        bind_close = server_body.rfind("</Bind>", 0, ip_start)
        if bind_open != -1 and (bind_close == -1 or bind_close < bind_open):
            continue

        vhost_open = server_body.rfind("<VirtualHosts>", 0, ip_start)
        vhost_close = server_body.rfind("</VirtualHosts>", 0, ip_start)
        if vhost_open != -1 and (vhost_close == -1 or vhost_close < vhost_open):
            continue

        server_body = server_body[:ip_start] + ip + server_body[ip_end:]
        return text[:server_start] + server_body + text[server_end:]

    # No root-level IP found; leave document unchanged.
    return text


def _scoped_replace_control_bindings(text: str, bind: str) -> str:
    """
    Legacy helper: rewrite <Bind>/<IP>/<Address> inside <Modules><Control><Server>
    if that section exists. If it does *not* exist (newer templates), this is a no-op.
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
    api_token: str,
    *,
    include_managers_authentication: bool,
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

    text = replace_tag_content(text, "ID", xml_escape(username))
    text = replace_tag_content(text, "Password", xml_escape(password))

    if not include_managers_authentication:
        text = remove_first_tag_block(text, "AccessTokens")
        text = remove_first_tag_block(text, "Authentication")
    elif api_token:
        text = replace_tag_content(text, "AccessToken", xml_escape(api_token))
    else:
        text = remove_first_tag_block(text, "AccessTokens")

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
        "--api-token",
        required=True,
        help="OME API server access token",
    )
    parser.add_argument(
        "--port", required=True, help="OME server port"
    )
    parser.add_argument(
        "--tls-port", required=True, help="OME server TLS port"
    )

    parser.add_argument(
        "--omit-managers-auth",
        action="store_true",
        help="Drop the managers <Authentication> block when the target image does not support it",
    )

    parser.add_argument(
        "--image-tag",
        help=(
            "OME image tag used to detect manager authentication support; "
            "non-semver or <0.16.0 tags omit AccessTokens/Authentication"
        ),
    )

    args = parser.parse_args(argv)
    server_ip = args.server_ip if args.server_ip is not None else args.bind
    managers_authentication_supported = _managers_authentication_supported(args.image_tag)
    include_managers_authentication = managers_authentication_supported and not args.omit_managers_auth
    api_token = args.api_token if managers_authentication_supported else ""
    render(
        args.template,
        args.output,
        args.bind,
        server_ip,
        args.port,
        args.tls_port,
        args.username,
        args.password,
        api_token,
        include_managers_authentication=include_managers_authentication,
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
