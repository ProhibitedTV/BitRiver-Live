#!/usr/bin/env python3
"""Validate exact release sets and prepare one credential-stable upgrade pair."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
from pathlib import Path
from typing import Callable, Mapping, Sequence

import prepare_release_candidate as candidate


class UpgradePreparationError(ValueError):
    """Raised when immutable upgrade inputs do not match their declared pair."""


DIGEST_PATTERN = re.compile(r"sha256:[0-9a-f]{64}")


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def read_release_set(
    path: Path,
    *,
    expected_sha256: str,
    expected_tag: str,
    expected_commit: str,
    namespace: str,
    template_values: Mapping[str, str],
) -> dict[str, object]:
    if sha256_file(path) != expected_sha256:
        raise UpgradePreparationError(
            f"release-set SHA-256 mismatch for {expected_tag}"
        )
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, OSError) as exc:
        raise UpgradePreparationError(
            f"cannot read release set for {expected_tag}: {exc}"
        ) from exc
    if payload.get("schemaVersion") != "bitriver.release-set/v1":
        raise UpgradePreparationError(
            f"unsupported release-set schema for {expected_tag}"
        )
    identity = payload.get("candidate")
    if not isinstance(identity, dict):
        raise UpgradePreparationError(
            f"release set for {expected_tag} has no candidate identity"
        )
    if identity.get("tag") != expected_tag or identity.get("sourceCommit") != expected_commit:
        raise UpgradePreparationError(
            f"release set identity mismatch for {expected_tag}"
        )

    expected_images = dict(candidate.FIRST_PARTY_IMAGES)
    image_values: dict[str, str] = {}
    report_images: dict[str, str] = {}
    images = payload.get("images")
    if not isinstance(images, list) or len(images) != len(expected_images):
        raise UpgradePreparationError(
            f"release set for {expected_tag} must contain every first-party image exactly once"
        )
    for image in images:
        if not isinstance(image, dict):
            raise UpgradePreparationError(
                f"release set for {expected_tag} contains an invalid image entry"
            )
        env_key = image.get("envKey")
        name = image.get("name")
        digest = image.get("digest")
        if not isinstance(env_key, str) or expected_images.get(env_key) != name:
            raise UpgradePreparationError(
                f"release set for {expected_tag} contains an unexpected first-party image"
            )
        if env_key in image_values:
            raise UpgradePreparationError(
                f"release set for {expected_tag} repeats {env_key}"
            )
        if not isinstance(digest, str) or not DIGEST_PATTERN.fullmatch(digest):
            raise UpgradePreparationError(
                f"release set for {expected_tag} has an invalid digest for {name}"
            )
        tagged = f"{namespace}/{name}:{expected_tag}"
        immutable = f"{namespace}/{name}@{digest}"
        if image.get("candidateReference") != tagged or image.get("immutableReference") != immutable:
            raise UpgradePreparationError(
                f"release set for {expected_tag} has mismatched references for {name}"
            )
        image_values[env_key] = digest
        report_images[str(name)] = immutable
    if set(image_values) != set(expected_images):
        raise UpgradePreparationError(
            f"release set for {expected_tag} is missing first-party image evidence"
        )

    expected_dependencies = {
        env_key: image_template.format_map(template_values)
        for env_key, image_template in candidate.THIRD_PARTY_IMAGES
    }
    dependency_values: dict[str, str] = {}
    report_dependencies: dict[str, str] = {}
    dependencies = payload.get("dependencies")
    if not isinstance(dependencies, list) or len(dependencies) != len(expected_dependencies):
        raise UpgradePreparationError(
            f"release set for {expected_tag} must contain every dependency exactly once"
        )
    for dependency in dependencies:
        if not isinstance(dependency, dict):
            raise UpgradePreparationError(
                f"release set for {expected_tag} contains an invalid dependency entry"
            )
        env_key = dependency.get("envKey")
        reference = dependency.get("reference")
        digest = dependency.get("digest")
        if not isinstance(env_key, str) or expected_dependencies.get(env_key) != reference:
            raise UpgradePreparationError(
                f"release set for {expected_tag} contains an unexpected dependency"
            )
        if env_key in dependency_values:
            raise UpgradePreparationError(
                f"release set for {expected_tag} repeats {env_key}"
            )
        if not isinstance(digest, str) or not DIGEST_PATTERN.fullmatch(digest):
            raise UpgradePreparationError(
                f"release set for {expected_tag} has an invalid dependency digest"
            )
        dependency_values[env_key] = digest
        report_dependencies[str(reference)] = digest
    if set(dependency_values) != set(expected_dependencies):
        raise UpgradePreparationError(
            f"release set for {expected_tag} is missing dependency evidence"
        )

    return {
        "tag": expected_tag,
        "commit": expected_commit,
        "releaseSetSha256": expected_sha256,
        "imageValues": image_values,
        "images": report_images,
        "dependencyValues": dependency_values,
        "dependencies": report_dependencies,
    }


def replace_env_values(content: str, updates: Mapping[str, str]) -> str:
    lines, _ = candidate.parse_env_template(content)
    return candidate.render_env(lines, dict(updates))


def prepare_upgrade_pair(
    *,
    source_release_set: Path,
    candidate_release_set: Path,
    source_template: Path,
    source_output: Path,
    candidate_output: Path,
    sentinel_output: Path,
    metadata_output: Path,
    source_tag: str,
    source_commit: str,
    source_sha256: str,
    candidate_tag: str,
    candidate_commit: str,
    candidate_sha256: str,
    namespace: str,
    secret_factory: Callable[[str], str] = candidate.default_secret_factory,
) -> None:
    template_content = source_template.read_text(encoding="utf-8")
    _, template_values = candidate.parse_env_template(template_content)
    source = read_release_set(
        source_release_set,
        expected_sha256=source_sha256,
        expected_tag=source_tag,
        expected_commit=source_commit,
        namespace=namespace,
        template_values=template_values,
    )
    target = read_release_set(
        candidate_release_set,
        expected_sha256=candidate_sha256,
        expected_tag=candidate_tag,
        expected_commit=candidate_commit,
        namespace=namespace,
        template_values=template_values,
    )
    source_metadata = candidate.parse_tag(source_tag)
    rendered_source, sentinels = candidate.prepare_environment(
        template_content,
        source_metadata,
        namespace,
        resolve_digests=False,
        first_party_digests=source["imageValues"],
        third_party_digests=source["dependencyValues"],
        product_loopback=True,
        secret_factory=secret_factory,
    )
    rendered_source = replace_env_values(
        rendered_source,
        {
            "BITRIVER_CONFIG_ROOT": "..",
            "BITRIVER_RELEASE_COMMIT": source_commit,
            "BITRIVER_LIVE_PORT": "18080",
            "BITRIVER_TRANSCODE_LADDER": "1080p:2500",
        },
    )
    candidate_updates = {
        key: candidate_tag for key in candidate.FIRST_PARTY_TAG_KEYS
    }
    candidate_updates.update(
        {
            key: f"@{value}"
            for key, value in target["imageValues"].items()
        }
    )
    candidate_updates.update(
        {
            key: f"@{value}"
            for key, value in target["dependencyValues"].items()
        }
    )
    candidate_updates["BITRIVER_RELEASE_COMMIT"] = candidate_commit
    rendered_candidate = replace_env_values(rendered_source, candidate_updates)

    candidate.atomic_private_write(source_output, rendered_source)
    candidate.atomic_private_write(candidate_output, rendered_candidate)
    candidate.atomic_private_write(
        sentinel_output,
        "".join(f"{value}\n" for value in sorted(set(sentinels))),
    )
    metadata = {
        "schemaVersion": "bitriver.stateful-compose-upgrade-input/v1",
        "source": {
            key: source[key]
            for key in ("tag", "commit", "releaseSetSha256", "images")
        },
        "candidate": {
            key: target[key]
            for key in ("tag", "commit", "releaseSetSha256", "images")
        },
        "sourceDependencies": source["dependencies"],
        "candidateDependencies": target["dependencies"],
        "changedDependencies": sorted(
            reference
            for reference, digest in source["dependencies"].items()
            if target["dependencies"].get(reference) != digest
        ),
        "sourceEnvSha256": hashlib.sha256(rendered_source.encode()).hexdigest(),
        "candidateEnvSha256": hashlib.sha256(rendered_candidate.encode()).hexdigest(),
        "credentialsStableAcrossPair": all(
            line in rendered_candidate
            for line in rendered_source.splitlines()
            if any(line.startswith(f"{key}=") for key in candidate.SAMPLE_SECRET_KEYS)
        ),
    }
    if metadata["credentialsStableAcrossPair"] is not True:
        raise UpgradePreparationError(
            "candidate environment did not preserve source credentials"
        )
    candidate.atomic_private_write(
        metadata_output,
        json.dumps(metadata, indent=2, sort_keys=True) + "\n",
    )


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source-release-set", type=Path, required=True)
    parser.add_argument("--candidate-release-set", type=Path, required=True)
    parser.add_argument("--source-template", type=Path, required=True)
    parser.add_argument("--source-output", type=Path, required=True)
    parser.add_argument("--candidate-output", type=Path, required=True)
    parser.add_argument("--sentinel-output", type=Path, required=True)
    parser.add_argument("--metadata-output", type=Path, required=True)
    parser.add_argument("--source-tag", required=True)
    parser.add_argument("--source-commit", required=True)
    parser.add_argument("--source-sha256", required=True)
    parser.add_argument("--candidate-tag", required=True)
    parser.add_argument("--candidate-commit", required=True)
    parser.add_argument("--candidate-sha256", required=True)
    parser.add_argument("--namespace", default="ghcr.io/prohibitedtv")
    return parser


def main(argv: Sequence[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        prepare_upgrade_pair(
            source_release_set=args.source_release_set,
            candidate_release_set=args.candidate_release_set,
            source_template=args.source_template,
            source_output=args.source_output,
            candidate_output=args.candidate_output,
            sentinel_output=args.sentinel_output,
            metadata_output=args.metadata_output,
            source_tag=args.source_tag,
            source_commit=args.source_commit,
            source_sha256=args.source_sha256,
            candidate_tag=args.candidate_tag,
            candidate_commit=args.candidate_commit,
            candidate_sha256=args.candidate_sha256,
            namespace=args.namespace,
        )
    except (OSError, UpgradePreparationError, candidate.CandidateError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2
    print("stateful Compose upgrade environments prepared")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
