#!/usr/bin/env python3
import argparse
from pathlib import Path
import re
import sys
from textwrap import dedent
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


def _parse_ome_capabilities(
    image_tag: str | None, *, force_legacy_outputs: bool = False
) -> tuple[bool, bool]:
    """Return (supports_application_outputs, supports_output_streams)."""

    if force_legacy_outputs:
        return False, False

    if image_tag is None:
        return True, True

    match = re.match(r"^v?(\d+)\.(\d+)\.(\d+)", image_tag)
    if not match:
        raise SystemExit(
            "BITRIVER_OME_IMAGE_TAG must be MAJOR.MINOR.PATCH to render OME config"
        )

    major, minor = int(match.group(1)), int(match.group(2))
    if major == 0 and minor < 16:
        return False, False

    return True, True


def _indent_block(block: str, indent: str) -> str:
    stripped = dedent(block.strip("\n"))
    return "\n".join(
        f"{indent}{line}" if line else "" for line in stripped.split("\n")
    )


def _rewrite_output_profiles(output_profiles: str, supports_output_streams: bool) -> str:
    if supports_output_streams:
        return output_profiles

    def _rewrite_profile(match: re.Match[str]) -> str:
        profile_body = match.group(1)
        streams_match = re.search(r"<OutputStreams>(.*?)</OutputStreams>", profile_body, re.DOTALL)
        if not streams_match:
            return match.group(0)

        stream_body = streams_match.group(1)
        first_stream = re.search(r"<OutputStream>(.*?)</OutputStream>", stream_body, re.DOTALL)
        if not first_stream:
            return match.group(0)

        stream_contents = first_stream.group(1)
        video = re.search(r"<Video>(.*?)</Video>", stream_contents, re.DOTALL)
        audio = re.search(r"<Audio>(.*?)</Audio>", stream_contents, re.DOTALL)

        indent_match = re.search(r"\n(\s*)<OutputStreams>", match.group(0))
        profile_indent_match = re.search(r"\n(\s*)<OutputProfile>", match.group(0))
        base_indent = indent_match.group(1) if indent_match else ""
        content_indent = (
            f"{profile_indent_match.group(1)}    " if profile_indent_match else base_indent
        )

        def _format_block(tag: str, content: str) -> str:
            stripped = dedent(content).strip()
            if not stripped:
                return f"{content_indent}<{tag}></{tag}>"

            inner = "\n".join(
                f"{content_indent}    {line.strip()}" for line in stripped.split("\n")
            )
            return f"{content_indent}<{tag}>\n{inner}\n{content_indent}</{tag}>"

        parts: list[str] = []
        if video:
            parts.append(_format_block("Video", video.group(1)))
        if audio:
            parts.append(_format_block("Audio", audio.group(1)))

        if not parts:
            return match.group(0)

        replacement = "\n" + "\n".join(parts)
        profile_body = profile_body.replace(streams_match.group(0), replacement, 1)
        return f"<OutputProfile>{profile_body}</OutputProfile>"

    return re.sub(
        r"<OutputProfile>(.*?)</OutputProfile>",
        _rewrite_profile,
        output_profiles,
        flags=re.DOTALL,
    )


def _rewrite_application_outputs(
    text: str,
    *,
    supports_application_outputs: bool,
    supports_output_streams: bool,
) -> str:
    def _process_application(match: re.Match[str]) -> str:
        application_body = match.group(1)

        outputs_match = re.search(r"<Outputs>(.*?)</Outputs>", application_body, re.DOTALL)
        outputs_body = outputs_match.group(1) if outputs_match else None
        outputs_full = outputs_match.group(0) if outputs_match else None
        outputs_indent_match = re.search(r"\n(\s*)<Outputs>", application_body)
        outputs_indent = outputs_indent_match.group(1) if outputs_indent_match else ""

        output_profiles_inside = None
        if outputs_body:
            inside_match = re.search(
                r"<OutputProfiles>(.*?)</OutputProfiles>", outputs_body, re.DOTALL
            )
            if inside_match:
                rewritten_block = _rewrite_output_profiles(
                    inside_match.group(0), supports_output_streams
                )
                outputs_body = outputs_body.replace(
                    inside_match.group(0), rewritten_block, 1
                )
                output_profiles_inside = rewritten_block

        search_area = application_body
        if outputs_match:
            start, end = outputs_match.span()
            search_area = application_body[:start] + application_body[end:]

        output_profiles_top = None
        output_profiles_top_original = None
        top_match = re.search(r"<OutputProfiles>(.*?)</OutputProfiles>", search_area, re.DOTALL)
        if top_match:
            output_profiles_top_original = top_match.group(0)
            output_profiles_top = _rewrite_output_profiles(
                output_profiles_top_original, supports_output_streams
            )

        if supports_application_outputs:
            if outputs_full is None:
                return match.group(0)

            chosen_profiles = output_profiles_inside or output_profiles_top
            outputs_body = outputs_body or ""
            if chosen_profiles:
                child_indent = f"{outputs_indent}    "
                profiles_block = _indent_block(chosen_profiles, child_indent)
                if output_profiles_inside:
                    outputs_body = outputs_body.replace(
                        output_profiles_inside, profiles_block, 1
                    )
                else:
                    outputs_body = f"\n{profiles_block}\n" + outputs_body.lstrip("\n")

            new_outputs = f"<Outputs>{outputs_body}</Outputs>"
            new_application = application_body.replace(outputs_full, new_outputs, 1)
            if output_profiles_top_original:
                new_application = new_application.replace(
                    output_profiles_top_original, "", 1
                )

            return f"<Application>{new_application}</Application>"

        new_application = application_body
        if outputs_full:
            new_application = new_application.replace(outputs_full, "", 1)

        if output_profiles_top and output_profiles_top_original:
            new_application = new_application.replace(
                output_profiles_top_original, output_profiles_top, 1
            )
            return f"<Application>{new_application}</Application>"

        if output_profiles_inside:
            insert_at = new_application.find("<Publishers>")
            if insert_at == -1:
                insert_at = len(new_application)
                suffix = ""
            else:
                suffix = "\n\n"

            profiles_block = _indent_block(output_profiles_inside, outputs_indent)
            insertion = f"\n{profiles_block}{suffix}"
            new_application = (
                new_application[:insert_at] + insertion + new_application[insert_at:]
            )

        return f"<Application>{new_application}</Application>"

    return re.sub(
        r"<Application>(.*?)</Application>", _process_application, text, flags=re.DOTALL
    )


def render(
    template: Path,
    output: Path,
    bind: str,
    server_ip: str,
    server_port: str,
    tls_port: str,
    tcp_relay: str,
    ice_candidate: str,
    image_tag: str | None = None,
    force_legacy_outputs: bool = False,
) -> None:
    escaped_bind = xml_escape(bind)
    escaped_port = xml_escape(server_port)
    escaped_tls_port = xml_escape(tls_port)
    text = template.read_text()
    supports_application_outputs, supports_output_streams = _parse_ome_capabilities(
        image_tag, force_legacy_outputs=force_legacy_outputs
    )

    # Normalize old <Server.bind> wrappers to <Bind> so very old templates don't break.
    text = re.sub(r"<\s*Server\.bind\s*>", "<Bind>", text)
    text = re.sub(r"</\s*Server\.bind\s*>", "</Bind>", text)

    text = _replace_root_bindings(text, escaped_bind, escaped_port, escaped_tls_port)
    text = _replace_root_ip(text, xml_escape(server_ip))
    text = _scoped_replace_control_bindings(text, escaped_bind)
    text = replace_all_tag_content(text, "TcpRelay", xml_escape(tcp_relay))
    text = replace_all_tag_content(text, "IceCandidate", xml_escape(ice_candidate))
    text = _rewrite_application_outputs(
        text,
        supports_application_outputs=supports_application_outputs,
        supports_output_streams=supports_output_streams,
    )

    # Normalize excessive blank lines introduced during templating rewrites.
    text = re.sub(r"\n{3,}", "\n\n", text)

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
        "--tcp-relay",
        required=True,
        help="Address advertised in <TcpRelay> inside <IceCandidates> (e.g. *:3478)",
    )
    parser.add_argument(
        "--ice-candidate",
        required=True,
        help="Advertised ICE candidate (e.g. external-host:10000-10009/udp)",
    )
    parser.add_argument(
        "--port", required=True, help="OME server port"
    )
    parser.add_argument(
        "--tls-port", required=True, help="OME server TLS port"
    )
    parser.add_argument(
        "--image-tag",
        required=False,
        help="OME image tag used to decide whether to keep <Outputs> blocks",
    )
    parser.add_argument(
        "--force-legacy-outputs",
        action="store_true",
        help=(
            "Force legacy rendering for <Application> outputs (omit <Outputs> and <OutputStreams>), overriding --image-tag"
        ),
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
        args.tcp_relay,
        args.ice_candidate,
        args.image_tag,
        args.force_legacy_outputs,
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
