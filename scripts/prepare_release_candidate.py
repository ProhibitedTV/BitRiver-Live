#!/usr/bin/env python3
"""Prepare secret-safe metadata and environment input for a release candidate."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import secrets
import stat
import subprocess
import sys
import tempfile
import time
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Callable, Iterable, Mapping, Sequence


TAG_PATTERN = re.compile(
    r"^v(?P<major>0|[1-9][0-9]*)"
    r"\.(?P<minor>0|[1-9][0-9]*)"
    r"\.(?P<patch>0|[1-9][0-9]*)"
    r"(?:-(?P<prerelease>[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$"
)
DIGEST_PATTERN = re.compile(r"^sha256:[a-f0-9]{64}$")
ENV_LINE_PATTERN = re.compile(r"^(?P<key>[A-Z][A-Z0-9_]*)=(?P<value>.*)$")

FIRST_PARTY_TAG_KEYS = (
    "BITRIVER_LIVE_IMAGE_TAG",
    "BITRIVER_VIEWER_IMAGE_TAG",
    "BITRIVER_SRS_CONTROLLER_IMAGE_TAG",
    "BITRIVER_TRANSCODER_IMAGE_TAG",
    "BITRIVER_OME_CONFIG_IMAGE_TAG",
)

FIRST_PARTY_IMAGES = (
    ("BITRIVER_LIVE_IMAGE_DIGEST", "bitriver-live"),
    ("BITRIVER_VIEWER_IMAGE_DIGEST", "bitriver-viewer"),
    ("BITRIVER_SRS_CONTROLLER_IMAGE_DIGEST", "bitriver-srs-controller"),
    ("BITRIVER_TRANSCODER_IMAGE_DIGEST", "bitriver-transcoder"),
    ("BITRIVER_OME_CONFIG_IMAGE_DIGEST", "bitriver-ome-config"),
)

REQUIRED_TEMPLATE_KEYS = (
    "BITRIVER_DEPLOY_IMAGE_SOURCE",
    "BITRIVER_LIVE_MODE",
    "BITRIVER_LIVE_ALLOW_SELF_SIGNUP",
    "BITRIVER_LIVE_METRICS_TOKEN",
    "BITRIVER_POSTGRES_USER",
    "BITRIVER_POSTGRES_PASSWORD",
    "BITRIVER_REDIS_PASSWORD",
    "BITRIVER_LIVE_ADMIN_EMAIL",
    "BITRIVER_LIVE_ADMIN_PASSWORD",
    "BITRIVER_SRS_TOKEN",
    "BITRIVER_OME_USERNAME",
    "BITRIVER_OME_PASSWORD",
    "BITRIVER_OME_API_TOKEN",
    "BITRIVER_TRANSCODER_TOKEN",
    "BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD",
    "BITRIVER_SRS_PUBLIC_RTMP_BASE_URL",
    "BITRIVER_OME_PUBLIC_LLHLS_BASE_URL",
    "BITRIVER_TRANSCODER_PUBLIC_BASE_URL",
    "NEXT_PUBLIC_API_BASE_URL",
    "NEXT_PUBLIC_VIEWER_URL",
    "BITRIVER_SRS_IMAGE_TAG",
    "BITRIVER_OME_IMAGE_TAG",
    *FIRST_PARTY_TAG_KEYS,
)

SAMPLE_SECRET_KEYS = (
    "BITRIVER_LIVE_METRICS_TOKEN",
    "BITRIVER_POSTGRES_PASSWORD",
    "BITRIVER_REDIS_PASSWORD",
    "BITRIVER_LIVE_ADMIN_PASSWORD",
    "BITRIVER_SRS_TOKEN",
    "BITRIVER_OME_PASSWORD",
    "BITRIVER_OME_API_TOKEN",
    "BITRIVER_TRANSCODER_TOKEN",
    "BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD",
)

THIRD_PARTY_IMAGES = (
    ("BITRIVER_REDIS_IMAGE_DIGEST", "redis:7-alpine"),
    ("BITRIVER_POSTGRES_IMAGE_DIGEST", "postgres:15-alpine"),
    ("BITRIVER_SRS_IMAGE_DIGEST", "ossrs/srs:{BITRIVER_SRS_IMAGE_TAG}"),
    (
        "BITRIVER_OME_IMAGE_DIGEST",
        "airensoft/ovenmediaengine:{BITRIVER_OME_IMAGE_TAG}",
    ),
    ("BITRIVER_NGINX_IMAGE_DIGEST", "nginx:alpine"),
    ("BITRIVER_ALPINE_3_IMAGE_DIGEST", "alpine:3"),
    ("BITRIVER_ALPINE_3_19_IMAGE_DIGEST", "alpine:3.19"),
    ("BITRIVER_DEBIAN_IMAGE_DIGEST", "debian:12-slim"),
)


class CandidateError(RuntimeError):
    """Raised when candidate input cannot be prepared safely."""


@dataclass(frozen=True)
class ReleaseMetadata:
    tag: str
    version: str
    major: int
    minor: int
    patch: int
    prerelease: str
    is_prerelease: bool
    publish_latest: bool
    msi_version: str
    nfpm_version: str
    nfpm_prerelease: str

    def github_outputs(self) -> dict[str, str]:
        values = asdict(self)
        return {
            key: str(value).lower() if isinstance(value, bool) else str(value)
            for key, value in values.items()
        }


def parse_tag(tag: str) -> ReleaseMetadata:
    match = TAG_PATTERN.fullmatch(tag)
    if match is None:
        raise CandidateError(
            "release tag must match vMAJOR.MINOR.PATCH or "
            "vMAJOR.MINOR.PATCH-PRERELEASE"
        )

    prerelease = match.group("prerelease") or ""
    for identifier in prerelease.split(".") if prerelease else ():
        if identifier.isdigit() and len(identifier) > 1 and identifier.startswith("0"):
            raise CandidateError(
                "numeric prerelease identifiers must not contain leading zeroes"
            )

    major = int(match.group("major"))
    minor = int(match.group("minor"))
    patch = int(match.group("patch"))
    version = f"{major}.{minor}.{patch}"
    is_prerelease = bool(prerelease)
    return ReleaseMetadata(
        tag=tag,
        version=version,
        major=major,
        minor=minor,
        patch=patch,
        prerelease=prerelease,
        is_prerelease=is_prerelease,
        publish_latest=not is_prerelease,
        msi_version=version,
        nfpm_version=version,
        nfpm_prerelease=prerelease,
    )


def parse_env_template(content: str) -> tuple[list[str], dict[str, str]]:
    lines = content.splitlines()
    values: dict[str, str] = {}
    for line in lines:
        match = ENV_LINE_PATTERN.fullmatch(line)
        if match is not None:
            values[match.group("key")] = match.group("value")
    return lines, values


def render_env(lines: Sequence[str], updates: Mapping[str, str]) -> str:
    remaining = dict(updates)
    rendered: list[str] = []
    for line in lines:
        match = ENV_LINE_PATTERN.fullmatch(line)
        if match is None:
            rendered.append(line)
            continue
        key = match.group("key")
        if key in remaining:
            rendered.append(f"{key}={remaining.pop(key)}")
        else:
            rendered.append(line)

    if remaining:
        rendered.append("")
        rendered.append("# Release-candidate job-local overrides.")
        rendered.extend(f"{key}={value}" for key, value in sorted(remaining.items()))
    return "\n".join(rendered) + "\n"


def default_secret_factory(label: str) -> str:
    del label
    return "BrL!9-" + secrets.token_urlsafe(32)


def default_digest_resolver(image: str) -> str:
    completed = subprocess.run(
        [
            "docker",
            "buildx",
            "imagetools",
            "inspect",
            image,
            "--format",
            "{{.Manifest.Digest}}",
        ],
        check=False,
        capture_output=True,
        text=True,
        timeout=90,
    )
    if completed.returncode != 0:
        detail = completed.stderr.strip().splitlines()
        suffix = f": {detail[-1]}" if detail else ""
        raise CandidateError(f"cannot resolve registry digest for {image}{suffix}")
    digest = completed.stdout.strip()
    if not DIGEST_PATTERN.fullmatch(digest):
        raise CandidateError(f"registry returned an invalid digest for {image}")
    return digest


def resolve_digest_with_retry(
    image: str,
    resolver: Callable[[str], str],
    attempts: int,
    delay_seconds: float,
) -> str:
    last_error: Exception | None = None
    for attempt in range(1, attempts + 1):
        try:
            digest = resolver(image)
            if not DIGEST_PATTERN.fullmatch(digest):
                raise CandidateError(f"registry returned an invalid digest for {image}")
            return digest
        except (CandidateError, OSError, subprocess.SubprocessError) as exc:
            last_error = exc
            if attempt < attempts:
                time.sleep(delay_seconds)
    raise CandidateError(
        f"cannot resolve registry digest for {image} after {attempts} attempts: "
        f"{last_error}"
    )


def prepare_environment(
    template_content: str,
    metadata: ReleaseMetadata,
    namespace: str,
    *,
    resolve_digests: bool,
    digest_resolver: Callable[[str], str] = default_digest_resolver,
    digest_attempts: int = 4,
    digest_delay_seconds: float = 3.0,
    secret_factory: Callable[[str], str] = default_secret_factory,
    first_party_digests: Mapping[str, str] | None = None,
    product_loopback: bool = False,
    unpublished_first_party_digests: bool = False,
) -> tuple[str, list[str]]:
    if not re.fullmatch(r"[a-z0-9.-]+(?::[0-9]+)?/[a-z0-9._/-]+", namespace):
        raise CandidateError(
            "image namespace must be a lowercase registry path such as "
            "ghcr.io/prohibitedtv"
        )
    if digest_attempts < 1:
        raise CandidateError("digest attempts must be at least one")

    lines, template_values = parse_env_template(template_content)
    missing = sorted(key for key in REQUIRED_TEMPLATE_KEYS if key not in template_values)
    if missing:
        raise CandidateError(
            "environment template is missing required keys: " + ", ".join(missing)
        )

    original_secrets = {key: template_values[key] for key in SAMPLE_SECRET_KEYS}
    generated: dict[str, str] = {}
    for key in SAMPLE_SECRET_KEYS:
        generated[key] = secret_factory(key)

    generated["BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD"] = generated[
        "BITRIVER_REDIS_PASSWORD"
    ]
    if any(not value for value in generated.values()):
        raise CandidateError("secret generator returned an empty value")
    if len(set(generated.values())) < len(set(SAMPLE_SECRET_KEYS)) - 1:
        raise CandidateError("secret generator returned duplicate credential values")
    for key, sample in original_secrets.items():
        if generated[key] == sample:
            raise CandidateError(f"secret generator reused sample value for {key}")

    public_values = {
        "BITRIVER_SRS_PUBLIC_RTMP_BASE_URL": "rtmp://ingest.release-validator.invalid:1935/live",
        "BITRIVER_OME_PUBLIC_LLHLS_BASE_URL": "http://stream.release-validator.invalid/live",
        "BITRIVER_TRANSCODER_PUBLIC_BASE_URL": "http://stream.release-validator.invalid/hls",
        "NEXT_PUBLIC_API_BASE_URL": "http://stream.release-validator.invalid",
        "NEXT_PUBLIC_VIEWER_URL": "http://stream.release-validator.invalid/viewer",
    }
    if product_loopback:
        public_values = {
            "BITRIVER_SRS_PUBLIC_RTMP_BASE_URL": "rtmp://localhost:1935/live",
            "BITRIVER_OME_PUBLIC_LLHLS_BASE_URL": "http://localhost:18080/live",
            "BITRIVER_TRANSCODER_PUBLIC_BASE_URL": "http://localhost:9080/hls",
            "NEXT_PUBLIC_API_BASE_URL": "",
            "NEXT_PUBLIC_VIEWER_URL": "http://localhost:18080/viewer",
        }

    updates: dict[str, str] = {
        "BITRIVER_DEPLOY_IMAGE_SOURCE": "pull",
        "BITRIVER_LIVE_MODE": "production",
        "BITRIVER_LIVE_ALLOW_SELF_SIGNUP": "true",
        "BITRIVER_LIVE_ADMIN_EMAIL": "release-validator@example.invalid",
        "BITRIVER_OME_USERNAME": "release-validator",
        "BITRIVER_IMAGE_NAMESPACE": namespace,
        **public_values,
        **generated,
    }
    updates.update({key: metadata.tag for key in FIRST_PARTY_TAG_KEYS})
    if first_party_digests is not None and unpublished_first_party_digests:
        raise CandidateError(
            "real first-party evidence and unpublished digest sentinels are mutually exclusive"
        )
    if unpublished_first_party_digests:
        first_party_digests = {
            digest_key: "sha256:"
            + hashlib.sha256(f"{metadata.tag}:{image_name}".encode()).hexdigest()
            for digest_key, image_name in FIRST_PARTY_IMAGES
        }
    if first_party_digests is not None:
        missing_first_party = sorted(
            digest_key
            for digest_key, _ in FIRST_PARTY_IMAGES
            if digest_key not in first_party_digests
        )
        if missing_first_party:
            raise CandidateError(
                "first-party digest evidence is missing keys: "
                + ", ".join(missing_first_party)
            )
        for digest_key, _ in FIRST_PARTY_IMAGES:
            digest = first_party_digests[digest_key]
            if not DIGEST_PATTERN.fullmatch(digest):
                raise CandidateError(
                    f"first-party evidence has an invalid digest for {digest_key}"
                )
            updates[digest_key] = f"@{digest}"

    if resolve_digests:
        digest_values = dict(template_values)
        digest_values.update(updates)
        for digest_key, image_template in THIRD_PARTY_IMAGES:
            image = image_template.format_map(digest_values)
            digest = resolve_digest_with_retry(
                image,
                digest_resolver,
                digest_attempts,
                digest_delay_seconds,
            )
            updates[digest_key] = f"@{digest}"

    sentinel_values = sorted(set(generated.values()))
    return render_env(lines, updates), sentinel_values


def resolve_first_party_images(
    metadata: ReleaseMetadata,
    namespace: str,
    *,
    digest_resolver: Callable[[str], str] = default_digest_resolver,
    digest_attempts: int = 8,
    digest_delay_seconds: float = 10.0,
) -> dict[str, object]:
    if not re.fullmatch(r"[a-z0-9.-]+(?::[0-9]+)?/[a-z0-9._/-]+", namespace):
        raise CandidateError(
            "image namespace must be a lowercase registry path such as "
            "ghcr.io/prohibitedtv"
        )
    images: list[dict[str, str]] = []
    for digest_key, image_name in FIRST_PARTY_IMAGES:
        reference = f"{namespace}/{image_name}:{metadata.tag}"
        digest = resolve_digest_with_retry(
            reference,
            digest_resolver,
            digest_attempts,
            digest_delay_seconds,
        )
        images.append(
            {
                "name": image_name,
                "reference": reference,
                "digest": digest,
                "envKey": digest_key,
            }
        )
    return {
        "schemaVersion": "bitriver.release-images/v1",
        "tag": metadata.tag,
        "namespace": namespace,
        "anonymousManifestAccess": True,
        "images": images,
    }


def read_first_party_evidence(
    path: Path, metadata: ReleaseMetadata, namespace: str
) -> dict[str, str]:
    try:
        evidence = json.loads(path.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, OSError) as exc:
        raise CandidateError(f"cannot read first-party image evidence: {exc}") from exc
    if evidence.get("schemaVersion") != "bitriver.release-images/v1":
        raise CandidateError("first-party image evidence has an unsupported schema")
    if evidence.get("tag") != metadata.tag or evidence.get("namespace") != namespace:
        raise CandidateError(
            "first-party image evidence does not match the candidate tag/namespace"
        )
    if evidence.get("anonymousManifestAccess") is not True:
        raise CandidateError(
            "first-party image evidence does not prove anonymous manifest access"
        )
    values: dict[str, str] = {}
    images = evidence.get("images")
    if not isinstance(images, list):
        raise CandidateError("first-party image evidence is missing its image list")
    for image in images:
        if not isinstance(image, dict):
            raise CandidateError("first-party image evidence contains an invalid entry")
        key = image.get("envKey")
        digest = image.get("digest")
        if isinstance(key, str) and isinstance(digest, str):
            values[key] = digest
    return values


def atomic_private_write(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary_name = tempfile.mkstemp(
        prefix=f".{path.name}.", dir=str(path.parent), text=True
    )
    temporary_path = Path(temporary_name)
    try:
        os.fchmod(descriptor, stat.S_IRUSR | stat.S_IWUSR)
        with os.fdopen(descriptor, "w", encoding="utf-8", newline="\n") as handle:
            handle.write(content)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary_path, path)
        os.chmod(path, stat.S_IRUSR | stat.S_IWUSR)
    except BaseException:
        temporary_path.unlink(missing_ok=True)
        raise


def write_metadata(path: Path, metadata: ReleaseMetadata, output_format: str) -> None:
    if output_format == "github":
        content = "".join(
            f"{key}={value}\n" for key, value in metadata.github_outputs().items()
        )
    else:
        content = json.dumps(asdict(metadata), indent=2, sort_keys=True) + "\n"
    atomic_private_write(path, content)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)

    metadata = subparsers.add_parser("metadata", help="validate and derive tag metadata")
    metadata.add_argument("--tag", required=True)
    metadata.add_argument("--output", type=Path, required=True)
    metadata.add_argument(
        "--format", choices=("github", "json"), default="github", dest="output_format"
    )

    env = subparsers.add_parser(
        "env", help="prepare a rotated production validation environment"
    )
    env.add_argument("--tag", required=True)
    env.add_argument("--namespace", required=True)
    env.add_argument("--template", type=Path, required=True)
    env.add_argument("--output", type=Path, required=True)
    env.add_argument("--sentinel-output", type=Path, required=True)
    env.add_argument("--first-party-evidence", type=Path)
    env.add_argument("--resolve-digests", action="store_true")
    env.add_argument(
        "--product-loopback",
        action="store_true",
        help="use disposable loopback media URLs for the running product gate",
    )
    env.add_argument(
        "--unpublished-first-party-digests",
        action="store_true",
        help="use format-only digest sentinels before tagged images exist",
    )
    env.add_argument("--digest-attempts", type=int, default=4)
    env.add_argument("--digest-delay-seconds", type=float, default=3.0)

    images = subparsers.add_parser(
        "images", help="prove anonymous access to tagged first-party images"
    )
    images.add_argument("--tag", required=True)
    images.add_argument("--namespace", required=True)
    images.add_argument("--output", type=Path, required=True)
    images.add_argument("--digest-attempts", type=int, default=8)
    images.add_argument("--digest-delay-seconds", type=float, default=10.0)
    return parser


def main(argv: Sequence[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        metadata = parse_tag(args.tag)
        if args.command == "metadata":
            write_metadata(args.output, metadata, args.output_format)
            print(f"release metadata prepared for {metadata.tag}")
            return 0
        if args.command == "images":
            evidence = resolve_first_party_images(
                metadata,
                args.namespace,
                digest_attempts=args.digest_attempts,
                digest_delay_seconds=args.digest_delay_seconds,
            )
            atomic_private_write(
                args.output.resolve(),
                json.dumps(evidence, indent=2, sort_keys=True) + "\n",
            )
            print(
                "anonymous tagged image manifests verified "
                f"({len(evidence['images'])} images)"
            )
            return 0

        output = args.output.resolve()
        sentinel_output = args.sentinel_output.resolve()
        template = args.template.resolve()
        if output == sentinel_output or output == template or sentinel_output == template:
            raise CandidateError("template, environment, and sentinel paths must differ")

        first_party_digests = None
        if args.first_party_evidence is not None:
            first_party_digests = read_first_party_evidence(
                args.first_party_evidence.resolve(), metadata, args.namespace
            )
        rendered, sentinel_values = prepare_environment(
            args.template.read_text(encoding="utf-8"),
            metadata,
            args.namespace,
            resolve_digests=args.resolve_digests,
            digest_attempts=args.digest_attempts,
            digest_delay_seconds=args.digest_delay_seconds,
            first_party_digests=first_party_digests,
            product_loopback=args.product_loopback,
            unpublished_first_party_digests=args.unpublished_first_party_digests,
        )
        atomic_private_write(output, rendered)
        atomic_private_write(sentinel_output, "\n".join(sentinel_values) + "\n")
        print(
            "release validation environment prepared "
            f"({len(sentinel_values)} credential sentinels retained separately)"
        )
        return 0
    except (CandidateError, OSError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
