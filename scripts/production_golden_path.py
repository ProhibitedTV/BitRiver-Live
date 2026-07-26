#!/usr/bin/env python3
"""Exercise the BitRiver Live production golden path through public interfaces.

The harness intentionally uses only HTTP, RTMP, ffmpeg, and ffprobe. It never
reads the deployment database, container filesystem, or generated service
configuration. Retained evidence is sanitized and contains no credentials.
"""

from __future__ import annotations

import argparse
import contextlib
import hashlib
import http.cookiejar
import json
import os
import platform
import re
import secrets
import shutil
import signal
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path
from typing import Any, Callable, Iterator


REPORT_SCHEMA = "bitriver.production-golden-path/v1"
DEFAULT_STAGE_TIMEOUT = 120.0
DEFAULT_MEDIA_TIMEOUT = 45.0
POLL_INTERVAL = 1.0


class GoldenPathError(RuntimeError):
    """A bounded, release-gate failure."""


class HTTPError(GoldenPathError):
    """An unexpected response from a product HTTP surface."""


def sanitize_url(raw_url: str) -> str:
    """Remove user info, query parameters, and fragments from an evidence URL."""
    parsed = urllib.parse.urlsplit(str(raw_url).strip())
    if not parsed.scheme:
        return parsed.path
    hostname = parsed.hostname or ""
    if ":" in hostname and not hostname.startswith("["):
        hostname = f"[{hostname}]"
    if parsed.port is not None:
        hostname = f"{hostname}:{parsed.port}"
    return urllib.parse.urlunsplit(
        (parsed.scheme, hostname, parsed.path, "", "")
    )


def rewrite_media_host(raw_url: str, host_override: str) -> str:
    """Route loopback media URLs through a Docker-host alias when requested."""
    if not host_override.strip():
        return raw_url
    parsed = urllib.parse.urlsplit(raw_url)
    hostname = parsed.hostname or ""
    if hostname not in {"localhost", "127.0.0.1", "::1"}:
        return raw_url
    override = host_override.strip()
    if ":" in override and not override.startswith("["):
        override = f"[{override}]"
    netloc = override
    if parsed.port is not None:
        netloc = f"{override}:{parsed.port}"
    return urllib.parse.urlunsplit(
        (parsed.scheme, netloc, parsed.path, parsed.query, parsed.fragment)
    )


def sanitize_text(value: object, sentinels: list[str]) -> str:
    """Redact every runtime sentinel from human-readable failure context."""
    text = str(value)
    for sentinel in sorted(
        (item for item in sentinels if item), key=len, reverse=True
    ):
        text = text.replace(sentinel, "[REDACTED]")
        text = text.replace(urllib.parse.quote(sentinel, safe=""), "[REDACTED]")
    text = re.sub(
        r"(?i)(password|token|secret|authorization|cookie)"
        r"(\s*[=:]\s*)([^\s,;]+)",
        r"\1\2[REDACTED]",
        text,
    )
    text = re.sub(r"(?i)(https?://)([^/@\s:]+):([^/@\s]+)@", r"\1", text)
    return text[-2000:]


def json_safe(value: Any, sentinels: list[str]) -> Any:
    """Recursively constrain evidence to public, sanitized scalar values."""
    if isinstance(value, dict):
        return {
            str(key): json_safe(item, sentinels)
            for key, item in value.items()
        }
    if isinstance(value, (list, tuple)):
        return [json_safe(item, sentinels) for item in value]
    if isinstance(value, str):
        return sanitize_text(value, sentinels)
    if value is None or isinstance(value, (bool, int, float)):
        return value
    return sanitize_text(value, sentinels)


class Evidence:
    """Versioned stage evidence with deterministic failure ownership."""

    def __init__(self, output_path: Path, sentinels: list[str]) -> None:
        self.output_path = output_path
        self.sentinels = sentinels
        self.started = time.time()
        self.payload: dict[str, Any] = {
            "schema": REPORT_SCHEMA,
            "status": "running",
            "startedAt": utc_timestamp(self.started),
            "environment": {
                "platform": platform.system().lower(),
                "architecture": platform.machine().lower(),
                "python": platform.python_version(),
            },
            "fixture": {
                "video": "testsrc2",
                "width": 1920,
                "height": 1080,
                "frameRate": 15,
                "audio": "sine-1000hz",
                "sampleRate": 48000,
            },
            "stages": [],
        }
        self.current_stage = ""

    @contextlib.contextmanager
    def stage(self, name: str) -> Iterator[dict[str, Any]]:
        started = time.monotonic()
        record: dict[str, Any] = {"name": name, "status": "running"}
        self.payload["stages"].append(record)
        self.current_stage = name
        self.write()
        try:
            yield record
        except Exception as exc:
            record["status"] = "failed"
            record["durationMs"] = elapsed_ms(started)
            record["error"] = sanitize_text(exc, self.sentinels)
            self.payload["status"] = "failed"
            self.payload["failedStage"] = name
            self.finish()
            raise
        else:
            record["status"] = "passed"
            record["durationMs"] = elapsed_ms(started)
            self.write()
        finally:
            self.current_stage = ""

    def finish(self) -> None:
        if self.payload["status"] == "running":
            self.payload["status"] = "passed"
        self.payload["finishedAt"] = utc_timestamp()
        self.payload["durationMs"] = int((time.time() - self.started) * 1000)
        self.write()

    def write(self) -> None:
        self.output_path.parent.mkdir(parents=True, exist_ok=True)
        clean = json_safe(self.payload, self.sentinels)
        temporary = self.output_path.with_suffix(
            self.output_path.suffix + ".tmp"
        )
        temporary.write_text(
            json.dumps(clean, indent=2, sort_keys=True) + "\n",
            encoding="utf-8",
        )
        temporary.replace(self.output_path)


def utc_timestamp(value: float | None = None) -> str:
    return time.strftime(
        "%Y-%m-%dT%H:%M:%SZ", time.gmtime(value if value is not None else time.time())
    )


def elapsed_ms(started: float) -> int:
    return int((time.monotonic() - started) * 1000)


def bounded_poll(
    label: str,
    timeout: float,
    check: Callable[[], Any],
    interval: float = POLL_INTERVAL,
) -> Any:
    """Return the first truthy result or fail with the last public state."""
    deadline = time.monotonic() + timeout
    last_state: object = "not observed"
    while True:
        try:
            result = check()
            if result:
                return result
            last_state = result
        except (HTTPError, urllib.error.URLError, TimeoutError) as exc:
            last_state = str(exc)
        if time.monotonic() >= deadline:
            raise GoldenPathError(
                f"{label} did not become ready within {timeout:.0f}s; "
                f"last state: {last_state}"
            )
        time.sleep(min(interval, max(0.0, deadline - time.monotonic())))


class ProductClient:
    """Cookie-aware JSON client for one real viewer account."""

    def __init__(
        self,
        base_url: str,
        timeout: float,
        on_session_token: Callable[[str], None] | None = None,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout
        self.cookies = http.cookiejar.CookieJar()
        self.on_session_token = on_session_token
        self.observed_session_tokens: set[str] = set()
        self.opener = urllib.request.build_opener(
            urllib.request.HTTPCookieProcessor(self.cookies)
        )

    @staticmethod
    def _origin(url: str) -> tuple[str, str, int | None]:
        parsed = urllib.parse.urlsplit(url)
        port = parsed.port
        if port is None:
            port = {"http": 80, "https": 443}.get(parsed.scheme.lower())
        return parsed.scheme.lower(), (parsed.hostname or "").lower(), port

    def _secure_session_token_for_internal_http(
        self, url: str
    ) -> str | None:
        if self._origin(url) != self._origin(self.base_url):
            return None
        if urllib.parse.urlsplit(url).scheme.lower() != "http":
            return None
        for cookie in self.cookies:
            if (
                cookie.name == "bitriver_session"
                and cookie.secure
                and cookie.value
            ):
                return cookie.value
        return None

    def _record_session_tokens(self) -> None:
        if self.on_session_token is None:
            return
        for cookie in self.cookies:
            if (
                cookie.name != "bitriver_session"
                or not cookie.value
                or cookie.value in self.observed_session_tokens
            ):
                continue
            self.observed_session_tokens.add(cookie.value)
            self.on_session_token(cookie.value)

    def request(
        self,
        path: str,
        *,
        method: str = "GET",
        payload: Any | None = None,
        body: bytes | None = None,
        headers: dict[str, str] | None = None,
        expected: tuple[int, ...] = (200,),
        parse_json: bool = True,
    ) -> Any:
        url = path if "://" in path else f"{self.base_url}{path}"
        request_headers = {"Accept": "application/json"}
        if headers:
            request_headers.update(headers)
        has_explicit_authorization = any(
            name.lower() == "authorization" for name in request_headers
        )
        if not has_explicit_authorization:
            session_token = self._secure_session_token_for_internal_http(url)
            if session_token is not None:
                request_headers["Authorization"] = f"Bearer {session_token}"
        if payload is not None:
            body = json.dumps(payload).encode("utf-8")
            request_headers["Content-Type"] = "application/json"
        request = urllib.request.Request(
            url, data=body, method=method, headers=request_headers
        )
        try:
            with self.opener.open(request, timeout=self.timeout) as response:
                status = response.status
                data = response.read()
                content_type = response.headers.get("Content-Type", "")
        except urllib.error.HTTPError as exc:
            status = exc.code
            data = exc.read()
            content_type = exc.headers.get("Content-Type", "")
        except urllib.error.URLError as exc:
            raise HTTPError(
                f"{method} {sanitize_url(url)} failed: {exc.reason}"
            ) from exc
        self._record_session_tokens()
        if status not in expected:
            detail = data.decode("utf-8", errors="replace")[:500]
            raise HTTPError(
                f"{method} {sanitize_url(url)} returned HTTP {status}: {detail}"
            )
        if not parse_json:
            return {
                "status": status,
                "contentType": content_type.split(";", 1)[0],
                "body": data,
            }
        if not data:
            return None
        try:
            return json.loads(data)
        except json.JSONDecodeError as exc:
            raise HTTPError(
                f"{method} {sanitize_url(url)} returned invalid JSON"
            ) from exc

    def multipart_upload(
        self,
        path: str,
        fields: dict[str, str],
        file_field: str,
        filename: str,
        content_type: str,
        file_data: bytes,
        internal_host: str,
    ) -> Any:
        boundary = f"bitriver-{secrets.token_hex(16)}"
        chunks: list[bytes] = []
        for name, value in fields.items():
            chunks.extend(
                [
                    f"--{boundary}\r\n".encode(),
                    (
                        f'Content-Disposition: form-data; name="{name}"\r\n\r\n'
                    ).encode(),
                    value.encode(),
                    b"\r\n",
                ]
            )
        chunks.extend(
            [
                f"--{boundary}\r\n".encode(),
                (
                    f'Content-Disposition: form-data; name="{file_field}"; '
                    f'filename="{filename}"\r\n'
                ).encode(),
                f"Content-Type: {content_type}\r\n\r\n".encode(),
                file_data,
                b"\r\n",
                f"--{boundary}--\r\n".encode(),
            ]
        )
        return self.request(
            path,
            method="POST",
            body=b"".join(chunks),
            headers={
                "Content-Type": f"multipart/form-data; boundary={boundary}",
                "Host": internal_host,
                "Idempotency-Key": f"golden-{secrets.token_hex(12)}",
            },
            expected=(200, 201, 202),
        )


def require_mapping(value: Any, label: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise GoldenPathError(f"{label} was not a JSON object")
    return value


def require_string(mapping: dict[str, Any], key: str, label: str) -> str:
    value = str(mapping.get(key, "")).strip()
    if not value:
        raise GoldenPathError(f"{label} did not include {key}")
    return value


def command_path(candidate: str, label: str) -> str:
    resolved = shutil.which(candidate)
    if resolved is None:
        raise GoldenPathError(f"{label} executable was not found: {candidate}")
    return resolved


def start_live_publisher(
    ffmpeg: str,
    rtmp_url: str,
    duration: int,
) -> subprocess.Popen[str]:
    command = [
        ffmpeg,
        "-hide_banner",
        "-loglevel",
        "error",
        "-nostdin",
        "-re",
        "-f",
        "lavfi",
        "-i",
        "testsrc2=size=1920x1080:rate=15",
        "-f",
        "lavfi",
        "-i",
        "sine=frequency=1000:sample_rate=48000",
        "-t",
        str(duration),
        "-c:v",
        "libx264",
        "-preset",
        "ultrafast",
        "-tune",
        "zerolatency",
        "-pix_fmt",
        "yuv420p",
        "-g",
        "30",
        "-keyint_min",
        "30",
        "-b:v",
        "2500k",
        "-c:a",
        "aac",
        "-b:a",
        "128k",
        "-f",
        "flv",
        rtmp_url,
    ]
    return subprocess.Popen(
        command,
        stdin=subprocess.DEVNULL,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.PIPE,
        text=True,
    )


def stop_process(process: subprocess.Popen[str], sentinels: list[str]) -> str:
    if process.poll() is None:
        if os.name == "nt":
            process.terminate()
        else:
            process.send_signal(signal.SIGINT)
    try:
        _, stderr = process.communicate(timeout=12)
    except subprocess.TimeoutExpired:
        process.kill()
        _, stderr = process.communicate(timeout=5)
    return sanitize_text(stderr or "", sentinels)


def generate_vod_fixture(ffmpeg: str, output_path: Path) -> None:
    command = [
        ffmpeg,
        "-hide_banner",
        "-loglevel",
        "error",
        "-nostdin",
        "-f",
        "lavfi",
        "-i",
        "testsrc2=size=1920x1080:rate=15",
        "-f",
        "lavfi",
        "-i",
        "sine=frequency=1000:sample_rate=48000",
        "-t",
        "4",
        "-c:v",
        "libx264",
        "-preset",
        "ultrafast",
        "-pix_fmt",
        "yuv420p",
        "-c:a",
        "aac",
        "-movflags",
        "+faststart",
        "-y",
        str(output_path),
    ]
    run_checked(command, timeout=45, label="generate VOD fixture", sentinels=[])


def run_checked(
    command: list[str],
    *,
    timeout: float,
    label: str,
    sentinels: list[str],
) -> subprocess.CompletedProcess[str]:
    try:
        result = subprocess.run(
            command,
            stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            timeout=timeout,
            check=False,
        )
    except subprocess.TimeoutExpired as exc:
        raise GoldenPathError(f"{label} exceeded {timeout:.0f}s") from exc
    if result.returncode != 0:
        detail = sanitize_text(result.stderr or result.stdout, sentinels)
        raise GoldenPathError(
            f"{label} exited {result.returncode}: {detail}"
        )
    return result


def fetch_text(url: str, timeout: float) -> str:
    request = urllib.request.Request(
        url,
        headers={
            "Accept": "application/vnd.apple.mpegurl, application/x-mpegURL, */*",
            "User-Agent": "BitRiver-Golden-Path/1",
        },
    )
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            data = response.read()
            status = response.status
    except urllib.error.HTTPError as exc:
        raise HTTPError(
            f"GET {sanitize_url(url)} returned HTTP {exc.code}"
        ) from exc
    except urllib.error.URLError as exc:
        raise HTTPError(
            f"GET {sanitize_url(url)} failed: {exc.reason}"
        ) from exc
    if status != 200:
        raise HTTPError(f"GET {sanitize_url(url)} returned HTTP {status}")
    text = data.decode("utf-8", errors="replace")
    if "#EXTM3U" not in text:
        raise HTTPError(f"GET {sanitize_url(url)} was not an HLS manifest")
    return text


def playlist_uris(manifest: str) -> list[str]:
    return [
        line.strip()
        for line in manifest.splitlines()
        if line.strip() and not line.lstrip().startswith("#")
    ]


def playlist_part_uris(manifest: str) -> list[str]:
    return re.findall(r'URI="([^"]+)"', manifest)


def resolve_media_playlist(
    manifest_url: str, timeout: float
) -> tuple[str, str]:
    manifest = fetch_text(manifest_url, timeout)
    if "#EXT-X-STREAM-INF" not in manifest:
        return manifest_url, manifest
    uris = playlist_uris(manifest)
    if not uris:
        raise GoldenPathError(
            f"HLS master {sanitize_url(manifest_url)} had no variants"
        )
    media_url = urllib.parse.urljoin(manifest_url, uris[0])
    return media_url, fetch_text(media_url, timeout)


def media_sequence(manifest: str) -> int | None:
    match = re.search(r"(?m)^#EXT-X-MEDIA-SEQUENCE:(\d+)\s*$", manifest)
    return int(match.group(1)) if match else None


def prove_playlist_advances(
    manifest_url: str,
    *,
    timeout: float,
    settle_seconds: float = 4.0,
) -> dict[str, Any]:
    """Require media entries and an advancing live playlist."""

    def first_snapshot() -> tuple[str, str] | None:
        media_url, manifest = resolve_media_playlist(
            manifest_url, min(timeout, 15)
        )
        if not (playlist_uris(manifest) or playlist_part_uris(manifest)):
            return None
        return media_url, manifest

    media_url, first = bounded_poll(
        f"HLS media at {sanitize_url(manifest_url)}",
        timeout,
        first_snapshot,
    )
    first_hash = hashlib.sha256(first.encode()).hexdigest()
    first_sequence = media_sequence(first)
    deadline = time.monotonic() + timeout
    second = first
    second_sequence = first_sequence
    while time.monotonic() < deadline:
        time.sleep(settle_seconds)
        _, second = resolve_media_playlist(media_url, min(timeout, 15))
        second_sequence = media_sequence(second)
        sequence_advanced = (
            first_sequence is not None
            and second_sequence is not None
            and second_sequence > first_sequence
        )
        if sequence_advanced or second != first:
            return {
                "manifestUrl": sanitize_url(manifest_url),
                "mediaPlaylistUrl": sanitize_url(media_url),
                "firstDigest": first_hash[:16],
                "secondDigest": hashlib.sha256(second.encode()).hexdigest()[:16],
                "firstSequence": first_sequence,
                "secondSequence": second_sequence,
                "advanced": True,
            }
    raise GoldenPathError(
        f"HLS playlist {sanitize_url(media_url)} did not advance "
        f"within {timeout:.0f}s"
    )


def probe_media(
    ffmpeg: str,
    ffprobe: str,
    media_url: str,
    *,
    timeout: float,
    sentinels: list[str],
) -> dict[str, Any]:
    run_checked(
        [
            ffmpeg,
            "-hide_banner",
            "-loglevel",
            "error",
            "-nostdin",
            "-rw_timeout",
            str(int(timeout * 1_000_000)),
            "-i",
            media_url,
            "-t",
            "3",
            "-map",
            "0:v:0",
            "-map",
            "0:a:0?",
            "-f",
            "null",
            "-",
        ],
        timeout=timeout + 10,
        label=f"decode media from {sanitize_url(media_url)}",
        sentinels=sentinels,
    )
    result = run_checked(
        [
            ffprobe,
            "-v",
            "error",
            "-rw_timeout",
            str(int(timeout * 1_000_000)),
            "-select_streams",
            "v:0",
            "-show_entries",
            "stream=codec_name,width,height,avg_frame_rate",
            "-of",
            "json",
            media_url,
        ],
        timeout=timeout + 10,
        label=f"probe media from {sanitize_url(media_url)}",
        sentinels=sentinels,
    )
    try:
        payload = json.loads(result.stdout)
        stream = payload["streams"][0]
        width = int(stream["width"])
        height = int(stream["height"])
    except (KeyError, IndexError, TypeError, ValueError, json.JSONDecodeError) as exc:
        raise GoldenPathError(
            f"ffprobe returned no usable video stream for {sanitize_url(media_url)}"
        ) from exc
    if width != 1920 or height != 1080:
        raise GoldenPathError(
            f"expected 1920x1080 media at {sanitize_url(media_url)}, "
            f"observed {width}x{height}"
        )
    return {
        "url": sanitize_url(media_url),
        "decodedSeconds": 3,
        "codec": str(stream.get("codec_name", "")),
        "width": width,
        "height": height,
        "frameRate": str(stream.get("avg_frame_rate", "")),
    }


def select_transcoder_manifest(playback: dict[str, Any]) -> str:
    renditions = playback.get("renditions", [])
    if not isinstance(renditions, list):
        raise GoldenPathError("playback renditions were not a list")
    candidates = [
        item
        for item in renditions
        if isinstance(item, dict) and str(item.get("manifestUrl", "")).strip()
    ]
    if not candidates:
        raise GoldenPathError("live playback included no transcoder manifest")
    preferred = next(
        (
            item
            for item in candidates
            if str(item.get("name", "")).lower() == "1080p"
        ),
        candidates[0],
    )
    return str(preferred["manifestUrl"]).strip()


def signup(
    client: ProductClient,
    display_name: str,
    email: str,
    password: str,
) -> dict[str, Any]:
    response = require_mapping(
        client.request(
            "/api/auth/signup",
            method="POST",
            payload={
                "displayName": display_name,
                "email": email,
                "password": password,
            },
            expected=(201,),
        ),
        "signup response",
    )
    return require_mapping(response.get("user"), "signup user")


def run_golden_path(args: argparse.Namespace, evidence: Evidence) -> None:
    ffmpeg = command_path(args.ffmpeg, "ffmpeg")
    ffprobe = command_path(args.ffprobe, "ffprobe")

    def record_session_token(token: str) -> None:
        if token not in evidence.sentinels:
            evidence.sentinels.append(token)
            write_sentinels(args.sentinel_file, evidence.sentinels)

    creator = ProductClient(
        args.base_url, args.http_timeout, record_session_token
    )
    viewer = ProductClient(
        args.base_url, args.http_timeout, record_session_token
    )
    anonymous = ProductClient(args.base_url, args.http_timeout)
    suffix = secrets.token_hex(8)
    creator_email = f"golden-creator-{suffix}@example.invalid"
    viewer_email = f"golden-viewer-{suffix}@example.invalid"
    creator_password = f"Gp!{secrets.token_urlsafe(24)}"
    viewer_password = f"Gp!{secrets.token_urlsafe(24)}"
    chat_content = f"golden-path-message-{suffix}"
    evidence.sentinels.extend(
        [creator_email, viewer_email, creator_password, viewer_password]
    )
    metrics_headers: dict[str, str] = {}
    if args.metrics_bearer_file is not None:
        metrics_bearer = args.metrics_bearer_file.read_text(
            encoding="utf-8"
        ).strip()
        if not metrics_bearer:
            raise GoldenPathError("metrics bearer file was empty")
        evidence.sentinels.append(metrics_bearer)
        metrics_headers["Authorization"] = f"Bearer {metrics_bearer}"
    write_sentinels(args.sentinel_file, evidence.sentinels)

    with evidence.stage("surface-preflight") as stage:
        health = require_mapping(
            anonymous.request("/healthz"), "health response"
        )
        ready = require_mapping(
            anonymous.request("/readyz"), "readiness response"
        )
        viewer_page = anonymous.request(
            args.viewer_path,
            parse_json=False,
            expected=(200,),
        )
        metrics = anonymous.request(
            "/metrics",
            parse_json=False,
            expected=(200,),
            headers=metrics_headers,
        )
        if b"bitriver" not in metrics["body"].lower():
            raise GoldenPathError("metrics response did not expose BitRiver metrics")
        stage["health"] = str(health.get("status", "ok"))
        stage["readiness"] = str(ready.get("status", "ready"))
        stage["viewerContentType"] = viewer_page["contentType"]
        stage["metricsContentType"] = metrics["contentType"]

    with evidence.stage("accounts-and-channel") as stage:
        creator_user = signup(
            creator, f"Golden Creator {suffix}", creator_email, creator_password
        )
        viewer_user = signup(
            viewer, f"Golden Viewer {suffix}", viewer_email, viewer_password
        )
        creator_id = require_string(creator_user, "id", "creator")
        viewer_id = require_string(viewer_user, "id", "viewer")
        status = require_mapping(
            creator.request("/api/status"), "authenticated status response"
        )
        channel = require_mapping(
            creator.request(
                "/api/channels",
                method="POST",
                payload={
                    "title": f"Golden Path {suffix}",
                    "category": "testing",
                    "tags": ["release-gate", "1080p"],
                },
                expected=(201,),
            ),
            "channel response",
        )
        channel_id = require_string(channel, "id", "channel")
        stream_key = require_string(channel, "streamKey", "channel")
        evidence.sentinels.append(stream_key)
        write_sentinels(args.sentinel_file, evidence.sentinels)
        stage["creatorId"] = creator_id
        stage["viewerId"] = viewer_id
        stage["channelId"] = channel_id
        stage["creatorBootstrap"] = "passed"
        stage["aggregateStatus"] = str(status.get("status", "unknown"))
        stage["statusChecks"] = len(status.get("checks", []))

    publisher: subprocess.Popen[str] | None = None
    rtmp_url = (
        f"{args.rtmp_base_url.rstrip('/')}/"
        f"{urllib.parse.quote(stream_key, safe='')}"
    )
    try:
        with evidence.stage("rtmp-publish-and-live-state") as stage:
            publisher = start_live_publisher(
                ffmpeg, rtmp_url, args.publisher_duration
            )

            def live_playback() -> dict[str, Any] | None:
                if publisher is not None and publisher.poll() is not None:
                    stderr = stop_process(publisher, evidence.sentinels)
                    raise GoldenPathError(
                        f"RTMP publisher exited before live state: {stderr}"
                    )
                response = require_mapping(
                    anonymous.request(
                        f"/api/channels/{urllib.parse.quote(channel_id)}/playback"
                    ),
                    "playback response",
                )
                playback = response.get("playback")
                if response.get("live") is True and isinstance(playback, dict):
                    return playback
                return None

            playback = bounded_poll(
                "channel live playback",
                args.stage_timeout,
                live_playback,
            )
            stage["channelId"] = channel_id
            stage["live"] = True
            stage["sessionId"] = str(playback.get("sessionId", ""))

        with evidence.stage("live-media-content") as stage:
            ome_url = rewrite_media_host(
                require_string(playback, "playbackUrl", "OME playback"),
                args.media_host_override,
            )
            transcoder_url = rewrite_media_host(
                select_transcoder_manifest(playback),
                args.media_host_override,
            )
            ome_advance = prove_playlist_advances(
                ome_url, timeout=args.stage_timeout
            )
            transcoder_advance = prove_playlist_advances(
                transcoder_url, timeout=args.stage_timeout
            )
            ome_probe = probe_media(
                ffmpeg,
                ffprobe,
                ome_url,
                timeout=args.media_timeout,
                sentinels=evidence.sentinels,
            )
            transcoder_probe = probe_media(
                ffmpeg,
                ffprobe,
                transcoder_url,
                timeout=args.media_timeout,
                sentinels=evidence.sentinels,
            )
            stage["ovenMediaEngine"] = {
                "playlist": ome_advance,
                "probe": ome_probe,
            }
            stage["transcoder"] = {
                "playlist": transcoder_advance,
                "probe": transcoder_probe,
            }
            stage["transcoderRenditionCount"] = len(
                playback.get("renditions", [])
            )
    finally:
        if publisher is not None:
            publisher_error = stop_process(publisher, evidence.sentinels)
            if publisher_error and evidence.payload["status"] == "running":
                evidence.payload["publisherDiagnostic"] = publisher_error

    with evidence.stage("offline-transition") as stage:
        def offline_state() -> bool:
            response = require_mapping(
                anonymous.request(
                    f"/api/channels/{urllib.parse.quote(channel_id)}/playback"
                ),
                "offline playback response",
            )
            return response.get("live") is False

        bounded_poll(
            "channel offline transition",
            args.stage_timeout,
            offline_state,
        )
        stage["channelId"] = channel_id
        stage["live"] = False

    with evidence.stage("chat-and-moderation") as stage:
        message = require_mapping(
            viewer.request(
                f"/api/channels/{urllib.parse.quote(channel_id)}/chat",
                method="POST",
                payload={"userId": viewer_id, "content": chat_content},
                expected=(201,),
            ),
            "chat message response",
        )
        message_id = require_string(message, "id", "chat message")

        def chat_history_contains_message() -> bool:
            history = anonymous.request(
                f"/api/channels/{urllib.parse.quote(channel_id)}/chat?limit=50"
            )
            if not isinstance(history, list):
                raise GoldenPathError("chat history was not a JSON array")
            return any(
                isinstance(item, dict)
                and item.get("id") == message_id
                and item.get("content") == chat_content
                for item in history
            )

        bounded_poll(
            "persisted chat history",
            args.stage_timeout,
            chat_history_contains_message,
        )
        moderation = require_mapping(
            creator.request(
                f"/api/channels/{urllib.parse.quote(channel_id)}/chat/moderation",
                method="POST",
                payload={
                    "action": "timeout",
                    "targetId": viewer_id,
                    "durationMs": 60_000,
                    "reason": "production golden-path gate",
                },
                expected=(202,),
            ),
            "moderation response",
        )

        def moderation_restriction_is_persisted() -> bool:
            restrictions = creator.request(
                f"/api/channels/{urllib.parse.quote(channel_id)}"
                "/chat/moderation/restrictions"
            )
            if not isinstance(restrictions, list):
                raise GoldenPathError(
                    "chat restriction list was not a JSON array"
                )
            return any(
                isinstance(item, dict)
                and item.get("targetId") == viewer_id
                and item.get("type") == "timeout"
                for item in restrictions
            )

        bounded_poll(
            "persisted chat moderation restriction",
            args.stage_timeout,
            moderation_restriction_is_persisted,
        )
        stage["messageId"] = message_id
        stage["historyObserved"] = True
        stage["moderationAction"] = moderation.get("action")
        stage["restrictionObserved"] = True

    with evidence.stage("vod-upload-publish-playback") as stage:
        with tempfile.TemporaryDirectory(prefix="bitriver-golden-vod-") as temp:
            fixture_path = Path(temp) / "golden-1080p.mp4"
            generate_vod_fixture(ffmpeg, fixture_path)
            upload = require_mapping(
                creator.multipart_upload(
                    "/api/uploads",
                    {
                        "channelId": channel_id,
                        "title": f"Golden VOD {suffix}",
                        "filename": fixture_path.name,
                    },
                    "file",
                    fixture_path.name,
                    "video/mp4",
                    fixture_path.read_bytes(),
                    args.internal_api_host,
                ),
                "upload response",
            )
        upload_id = require_string(upload, "id", "upload")

        last_upload_observation: dict[str, Any] = {
            "status": "not-observed",
            "progress": 0,
        }

        def ready_upload() -> dict[str, Any] | None:
            uploads = creator.request(
                "/api/uploads?channelId="
                + urllib.parse.quote(channel_id, safe="")
            )
            if not isinstance(uploads, list):
                raise GoldenPathError("upload list was not a JSON array")
            current = next(
                (
                    item
                    for item in uploads
                    if isinstance(item, dict) and item.get("id") == upload_id
                ),
                None,
            )
            if current is None:
                return None
            last_upload_observation["status"] = str(
                current.get("status", "unknown")
            )
            try:
                last_upload_observation["progress"] = int(
                    current.get("progress", 0)
                )
            except (TypeError, ValueError):
                last_upload_observation["progress"] = 0
            if current.get("status") == "failed":
                raise GoldenPathError(
                    f"VOD processing failed: {current.get('error', 'unknown error')}"
                )
            if (
                current.get("status") == "ready"
                and current.get("recordingId")
                and current.get("playbackUrl")
            ):
                return current
            return None

        try:
            completed = bounded_poll(
                "VOD transcoding",
                args.vod_timeout,
                ready_upload,
                interval=2.0,
            )
        except GoldenPathError as exc:
            raise GoldenPathError(
                f"{exc}; observed upload status="
                f"{last_upload_observation['status']} "
                f"progress={last_upload_observation['progress']}"
            ) from exc
        recording_id = require_string(
            completed, "recordingId", "ready upload"
        )
        published = require_mapping(
            creator.request(
                f"/api/recordings/{urllib.parse.quote(recording_id)}/publish",
                method="POST",
                expected=(200,),
            ),
            "published recording",
        )
        if not published.get("publishedAt"):
            raise GoldenPathError("published recording had no publishedAt value")
        vods = require_mapping(
            anonymous.request(
                f"/api/channels/{urllib.parse.quote(channel_id)}/vods"
            ),
            "public VOD response",
        )
        items = vods.get("items", [])
        vod = next(
            (
                item
                for item in items
                if isinstance(item, dict) and item.get("id") == recording_id
            ),
            None,
        )
        if vod is None:
            raise GoldenPathError(
                "public channel VOD list did not contain the published recording"
            )
        vod_url = rewrite_media_host(
            require_string(vod, "playbackUrl", "public VOD"),
            args.media_host_override,
        )

        def ready_vod_playlist() -> tuple[str, str] | None:
            media_url, manifest = resolve_media_playlist(
                vod_url, min(args.media_timeout, 15)
            )
            if not (playlist_uris(manifest) or playlist_part_uris(manifest)):
                return None
            return media_url, manifest

        vod_media_url, _ = bounded_poll(
            "public VOD manifest",
            args.vod_timeout,
            ready_vod_playlist,
            interval=2.0,
        )
        vod_probe = probe_media(
            ffmpeg,
            ffprobe,
            vod_media_url,
            timeout=args.media_timeout,
            sentinels=evidence.sentinels,
        )
        stage["uploadId"] = upload_id
        stage["recordingId"] = recording_id
        stage["sourceDurationSeconds"] = 4
        stage["status"] = completed.get("status")
        stage["published"] = True
        stage["publicListObserved"] = True
        stage["probe"] = vod_probe

    with evidence.stage("final-status") as stage:
        last_down: list[str] = []

        def aggregate_status_ready() -> dict[str, Any] | None:
            nonlocal last_down
            current = require_mapping(
                creator.request("/api/status"), "final status response"
            )
            last_down = []
            for item in current.get("checks", []):
                if not isinstance(item, dict) or item.get("status") != "down":
                    continue
                name = str(item.get("name", "unknown"))
                detail = sanitize_text(
                    str(item.get("detail", "")), evidence.sentinels
                )
                last_down.append(
                    f"{name}: {detail}" if detail else name
                )
            if last_down:
                return None
            return current

        try:
            status = bounded_poll(
                "aggregate service status",
                args.stage_timeout,
                aggregate_status_ready,
            )
        except GoldenPathError as exc:
            detail = "; ".join(last_down) if last_down else "not observed"
            raise GoldenPathError(
                f"{exc}; last down components: {detail}"
            ) from exc
        stage["aggregateStatus"] = str(status.get("status", "unknown"))
        stage["checks"] = len(status.get("checks", []))
        stage["downComponents"] = 0


def write_sentinels(path: Path | None, values: list[str]) -> None:
    if path is None:
        return
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        "".join(f"{value}\n" for value in values if value),
        encoding="utf-8",
    )


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Exercise accounts, 1080p RTMP/live playback, chat moderation, "
            "VOD processing, and health against an already-running BitRiver stack."
        )
    )
    parser.add_argument("--base-url", default="http://localhost:18080")
    parser.add_argument(
        "--rtmp-base-url", default="rtmp://localhost:1935/live"
    )
    parser.add_argument("--viewer-path", default="/viewer")
    parser.add_argument(
        "--internal-api-host",
        default="bitriver-live:8080",
        help="Host header used so upload source URLs are reachable by the transcoder",
    )
    parser.add_argument(
        "--media-host-override",
        default="",
        help=(
            "Replace loopback hosts in returned media URLs, for example "
            "host.docker.internal when the harness runs in a client container"
        ),
    )
    parser.add_argument(
        "--metrics-bearer-file",
        type=Path,
        help="Read a protected metrics bearer value from a file; never pass it on the command line",
    )
    parser.add_argument(
        "--artifact-dir", type=Path, required=True
    )
    parser.add_argument("--sentinel-file", type=Path)
    parser.add_argument("--ffmpeg", default="ffmpeg")
    parser.add_argument("--ffprobe", default="ffprobe")
    parser.add_argument(
        "--http-timeout", type=float, default=15.0
    )
    parser.add_argument(
        "--stage-timeout", type=float, default=DEFAULT_STAGE_TIMEOUT
    )
    parser.add_argument(
        "--media-timeout", type=float, default=DEFAULT_MEDIA_TIMEOUT
    )
    parser.add_argument("--vod-timeout", type=float, default=240.0)
    parser.add_argument("--publisher-duration", type=int, default=180)
    args = parser.parse_args(argv)
    for name in (
        "http_timeout",
        "stage_timeout",
        "media_timeout",
        "vod_timeout",
    ):
        if getattr(args, name) <= 0:
            parser.error(f"--{name.replace('_', '-')} must be positive")
    if args.publisher_duration < 30:
        parser.error("--publisher-duration must be at least 30 seconds")
    return args


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv if argv is not None else sys.argv[1:])
    report_path = args.artifact_dir / "production-golden-path.json"
    evidence = Evidence(report_path, [])
    try:
        run_golden_path(args, evidence)
        evidence.finish()
    except Exception as exc:
        if evidence.payload["status"] == "running":
            evidence.payload["status"] = "failed"
            evidence.payload["failedStage"] = evidence.current_stage or "bootstrap"
            evidence.payload["error"] = sanitize_text(exc, evidence.sentinels)
            evidence.finish()
        print(
            "production golden path failed at "
            f"{evidence.payload.get('failedStage', 'unknown')}: "
            f"{sanitize_text(exc, evidence.sentinels)}",
            file=sys.stderr,
        )
        print(f"sanitized report: {report_path}", file=sys.stderr)
        return 1
    print(f"production golden path passed: {report_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
