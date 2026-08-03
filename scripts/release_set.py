#!/usr/bin/env python3
"""Create and verify immutable BitRiver Live release-set metadata."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
from pathlib import Path
from typing import Iterable, Mapping, Sequence
from urllib.parse import urlparse


RELEASE_SET_SCHEMA = "bitriver.release-set/v1"
PROMOTION_SCHEMA = "bitriver.stable-promotion/v1"
STABLE_SET_SCHEMA = "bitriver.stable-release-set/v1"
ROLLBACK_SCHEMA = "bitriver.rollback-release-set/v1"
REVOCATION_SCHEMA = "bitriver.candidate-revocation/v1"

SHA256_PATTERN = re.compile(r"^[a-f0-9]{64}$")
DIGEST_PATTERN = re.compile(r"^sha256:[a-f0-9]{64}$")
COMMIT_PATTERN = re.compile(r"^[a-f0-9]{40}$")
TAG_PATTERN = re.compile(
    r"^v(?P<major>0|[1-9][0-9]*)"
    r"\.(?P<minor>0|[1-9][0-9]*)"
    r"\.(?P<patch>0|[1-9][0-9]*)"
    r"(?:-(?P<prerelease>[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$"
)
SAFE_NAME_PATTERN = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._+-]*$")
CHECKSUM_LINE_PATTERN = re.compile(r"^(?P<digest>[a-f0-9]{64})  (?P<name>[^/\\]+)$")

FIRST_PARTY_IMAGES = (
    ("bitriver-live", "BITRIVER_LIVE_IMAGE_DIGEST"),
    ("bitriver-viewer", "BITRIVER_VIEWER_IMAGE_DIGEST"),
    ("bitriver-srs-controller", "BITRIVER_SRS_CONTROLLER_IMAGE_DIGEST"),
    ("bitriver-transcoder", "BITRIVER_TRANSCODER_IMAGE_DIGEST"),
    ("bitriver-ome-config", "BITRIVER_OME_CONFIG_IMAGE_DIGEST"),
)

REQUIRED_PROMOTION_GATES = (
    ("clean-host-install", 1297),
    ("backup-restore", 1299),
    ("upgrade-rollback", 1298),
    ("capacity", 1303),
    ("resilience", 1304),
    ("slo-alerts", 1305),
    ("security-review", 1306),
    ("viewer-compatibility", 1307),
)

REQUIRED_EVIDENCE_FILES = (
    "release-contract-evidence.json",
    "release-dependencies.json",
    "release-images.json",
    "production-golden-path.json",
    "release-scan-status.json",
)

RELEASE_SET_METADATA = {
    "CHECKSUMS.txt",
    "release-set.json",
    "release-set.md",
    "release-set.sigstore.json",
}

ALLOWED_STABLE_METADATA = {
    "PROMOTION-CHECKSUMS.txt",
    "rollback-release-set.json",
    "stable-release-set.json",
    "stable-release-set.md",
    "stable-release-set.sigstore.json",
}


class ReleaseSetError(RuntimeError):
    """Raised when release-set input is incomplete or inconsistent."""


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def read_json(path: Path, label: str) -> dict[str, object]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise ReleaseSetError(f"cannot read {label}: {exc}") from exc
    if not isinstance(value, dict):
        raise ReleaseSetError(f"{label} must be a JSON object")
    return value


def write_json(path: Path, value: Mapping[str, object]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        json.dumps(value, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
        newline="\n",
    )


def write_text(path: Path, value: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(value, encoding="utf-8", newline="\n")


def validate_https_url(value: object, label: str) -> str:
    if not isinstance(value, str):
        raise ReleaseSetError(f"{label} must be an HTTPS URL")
    parsed = urlparse(value)
    if parsed.scheme != "https" or not parsed.netloc or parsed.username or parsed.password:
        raise ReleaseSetError(f"{label} must be a credential-free HTTPS URL")
    return value


def validate_repository(value: str) -> str:
    if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", value):
        raise ReleaseSetError("repository must use owner/name form")
    return value


def parse_tag(tag: str, *, prerelease: bool | None = None) -> re.Match[str]:
    match = TAG_PATTERN.fullmatch(tag)
    if match is None:
        raise ReleaseSetError(
            "release tag must match vMAJOR.MINOR.PATCH or "
            "vMAJOR.MINOR.PATCH-PRERELEASE"
        )
    is_prerelease = bool(match.group("prerelease"))
    if prerelease is True and not is_prerelease:
        raise ReleaseSetError("candidate tag must contain a prerelease identifier")
    if prerelease is False and is_prerelease:
        raise ReleaseSetError("stable tag must not contain a prerelease identifier")
    return match


def stable_tag_for(candidate_tag: str) -> str:
    match = parse_tag(candidate_tag, prerelease=True)
    return (
        f"v{match.group('major')}.{match.group('minor')}.{match.group('patch')}"
    )


def ensure_flat_file(path: Path, root: Path, label: str) -> Path:
    resolved = path.resolve()
    try:
        relative = resolved.relative_to(root.resolve())
    except ValueError as exc:
        raise ReleaseSetError(f"{label} must remain inside {root}") from exc
    if len(relative.parts) != 1 or not SAFE_NAME_PATTERN.fullmatch(relative.name):
        raise ReleaseSetError(f"{label} must be a safe flat release asset name")
    if path.is_symlink() or not path.is_file():
        raise ReleaseSetError(f"{label} must be a regular non-symlink file")
    return resolved


def list_flat_files(root: Path, excluded: Iterable[Path] = ()) -> list[Path]:
    root = root.resolve()
    if not root.is_dir():
        raise ReleaseSetError(f"release asset directory does not exist: {root}")
    excluded_paths = {path.resolve() for path in excluded}
    files: list[Path] = []
    for path in sorted(root.iterdir(), key=lambda entry: entry.name):
        if path.resolve() in excluded_paths:
            continue
        if path.is_symlink() or not path.is_file():
            raise ReleaseSetError(
                f"release asset directory contains a non-regular entry: {path.name}"
            )
        if not SAFE_NAME_PATTERN.fullmatch(path.name):
            raise ReleaseSetError(f"unsafe release asset name: {path.name}")
        files.append(path)
    return files


def artifact_kind(name: str) -> str:
    if name.endswith(".spdx.json"):
        return "sbom"
    if name.endswith(".sigstore.json"):
        return "sigstore-bundle"
    if name in REQUIRED_EVIDENCE_FILES:
        return "release-evidence"
    if name.endswith((".deb", ".rpm", ".msi")):
        return "installer"
    if name.endswith((".tar.gz", ".zip")):
        return "archive"
    if name.endswith(".rb"):
        return "package-metadata"
    return "artifact"


def artifact_entry(path: Path) -> dict[str, object]:
    return {
        "name": path.name,
        "kind": artifact_kind(path.name),
        "bytes": path.stat().st_size,
        "sha256": sha256_file(path),
    }


def required_candidate_names(tag: str) -> set[str]:
    names = {
        "bitriver-live-linux-amd64.tar.gz",
        "bitriver-live-linux-arm64.tar.gz",
        "bitriver-live-darwin-amd64.tar.gz",
        "bitriver-live-darwin-arm64.tar.gz",
        "bitriver-live-windows-amd64.zip",
        "bitriver-linux-amd64.tar.gz",
        "bitriver-linux-arm64.tar.gz",
        "bitriver-darwin-amd64.tar.gz",
        "bitriver-darwin-arm64.tar.gz",
        "bitriver-windows-amd64.zip",
        "bitriver-launcher-linux-amd64.tar.gz",
        "bitriver-launcher-linux-arm64.tar.gz",
        "bitriver-launcher-darwin-amd64.tar.gz",
        "bitriver-launcher-darwin-arm64.tar.gz",
        "bitriver-launcher-windows-amd64.zip",
        f"bitriver-live_{tag}_amd64.deb",
        f"bitriver-live_{tag}_arm64.deb",
        f"bitriver-live_{tag}_amd64.rpm",
        f"bitriver-live_{tag}_arm64.rpm",
        f"bitriver-live-{tag}.msi",
        f"bitriver-viewer-{tag}.tar.gz",
        "bitriver-live.rb",
        *REQUIRED_EVIDENCE_FILES,
    }
    for image, _ in FIRST_PARTY_IMAGES:
        names.add(f"{image}-{tag}.spdx.json")
        names.add(f"{image}-{tag}.image.sigstore.json")
    return names


def validate_contract_evidence(
    evidence: Mapping[str, object], tag: str, commit: str
) -> None:
    required = {
        "releaseTag": tag,
        "commit": commit,
        "environmentValidation": "passed",
        "imageDigestValidation": "passed",
        "credentialFlow": "job-local-ephemeral",
        "retainedValues": "none",
    }
    for key, expected in required.items():
        if evidence.get(key) != expected:
            raise ReleaseSetError(
                f"release contract evidence {key} must equal {expected!r}"
            )


def validate_scan_evidence(
    evidence: Mapping[str, object], tag: str, commit: str
) -> None:
    required = {
        "releaseTag": tag,
        "commit": commit,
        "downloadedArtifactScan": "passed",
        "publicationPayloadScan": "passed",
    }
    for key, expected in required.items():
        if evidence.get(key) != expected:
            raise ReleaseSetError(
                f"release scan evidence {key} must equal {expected!r}"
            )


def validate_product_evidence(evidence: Mapping[str, object]) -> None:
    if evidence.get("schema") != "bitriver.production-golden-path/v1":
        raise ReleaseSetError("product evidence has an unsupported schema")
    if evidence.get("status") != "passed":
        raise ReleaseSetError("product evidence must report passed")
    stages = evidence.get("stages")
    if not isinstance(stages, list) or not stages:
        raise ReleaseSetError("product evidence must contain stages")
    for stage in stages:
        if not isinstance(stage, dict) or stage.get("status") != "passed":
            raise ReleaseSetError("every product evidence stage must pass")


def validate_dependency_evidence(evidence: Mapping[str, object]) -> list[dict[str, str]]:
    if evidence.get("schemaVersion") != "bitriver.release-dependencies/v1":
        raise ReleaseSetError("dependency evidence has an unsupported schema")
    if evidence.get("registryManifestAccess") is not True:
        raise ReleaseSetError("dependency evidence must prove registry manifest access")
    images = evidence.get("images")
    if not isinstance(images, list) or not images:
        raise ReleaseSetError("dependency evidence must contain images")
    normalized: list[dict[str, str]] = []
    seen: set[str] = set()
    for image in images:
        if not isinstance(image, dict):
            raise ReleaseSetError("dependency evidence contains an invalid image")
        key = image.get("envKey")
        reference = image.get("reference")
        digest = image.get("digest")
        if not all(isinstance(value, str) for value in (key, reference, digest)):
            raise ReleaseSetError("dependency evidence contains an incomplete image")
        if key in seen:
            raise ReleaseSetError(f"dependency evidence repeats {key}")
        if not DIGEST_PATTERN.fullmatch(digest):
            raise ReleaseSetError(f"dependency evidence has invalid digest for {key}")
        seen.add(key)
        normalized.append(
            {"envKey": key, "reference": reference, "digest": digest}
        )
    return sorted(normalized, key=lambda entry: entry["envKey"])


def validate_image_evidence(
    evidence: Mapping[str, object], tag: str, assets: Mapping[str, Mapping[str, object]]
) -> list[dict[str, object]]:
    if evidence.get("schemaVersion") != "bitriver.release-images/v1":
        raise ReleaseSetError("first-party image evidence has an unsupported schema")
    if evidence.get("anonymousManifestAccess") is not True:
        raise ReleaseSetError("first-party image evidence must prove anonymous access")
    if evidence.get("tag") != tag:
        raise ReleaseSetError("first-party image evidence tag mismatch")
    namespace = evidence.get("namespace")
    if not isinstance(namespace, str) or not re.fullmatch(
        r"[a-z0-9.-]+(?::[0-9]+)?/[a-z0-9._/-]+", namespace
    ):
        raise ReleaseSetError("first-party image namespace is invalid")
    images = evidence.get("images")
    if not isinstance(images, list):
        raise ReleaseSetError("first-party image evidence is missing images")
    by_name: dict[str, Mapping[str, object]] = {}
    for image in images:
        if not isinstance(image, dict) or not isinstance(image.get("name"), str):
            raise ReleaseSetError("first-party image evidence contains an invalid entry")
        name = image["name"]
        if name in by_name:
            raise ReleaseSetError(f"first-party image evidence repeats {name}")
        by_name[name] = image
    expected_names = {name for name, _ in FIRST_PARTY_IMAGES}
    if set(by_name) != expected_names:
        raise ReleaseSetError(
            "first-party image evidence names must be exactly: "
            + ", ".join(sorted(expected_names))
        )
    normalized: list[dict[str, object]] = []
    for name, env_key in FIRST_PARTY_IMAGES:
        image = by_name[name]
        digest = image.get("digest")
        reference = image.get("reference")
        if image.get("envKey") != env_key:
            raise ReleaseSetError(f"first-party image env key mismatch for {name}")
        if not isinstance(digest, str) or not DIGEST_PATTERN.fullmatch(digest):
            raise ReleaseSetError(f"first-party image digest is invalid for {name}")
        expected_reference = f"{namespace}/{name}:{tag}"
        if reference != expected_reference:
            raise ReleaseSetError(
                f"first-party image reference mismatch for {name}: "
                f"expected {expected_reference}"
            )
        sbom_name = f"{name}-{tag}.spdx.json"
        signature_name = f"{name}-{tag}.image.sigstore.json"
        if sbom_name not in assets or signature_name not in assets:
            raise ReleaseSetError(f"image {name} is missing SBOM or signature evidence")
        normalized.append(
            {
                "name": name,
                "envKey": env_key,
                "candidateReference": expected_reference,
                "immutableReference": f"{namespace}/{name}@{digest}",
                "digest": digest,
                "sbom": {
                    "asset": sbom_name,
                    "sha256": assets[sbom_name]["sha256"],
                },
                "signature": {
                    "asset": signature_name,
                    "sha256": assets[signature_name]["sha256"],
                    "type": "sigstore-image-bundle",
                },
            }
        )
    return normalized


def create_candidate_manifest(args: argparse.Namespace) -> None:
    tag = args.tag
    parse_tag(tag, prerelease=True)
    commit = args.commit.lower()
    if not COMMIT_PATTERN.fullmatch(commit):
        raise ReleaseSetError("candidate commit must be a full lowercase SHA-1")
    repository = validate_repository(args.repository)
    if args.run_id < 1 or args.run_attempt < 1:
        raise ReleaseSetError("workflow run id and attempt must be positive")
    workflow_url = validate_https_url(args.workflow_url, "workflow URL")
    root = args.assets_dir.resolve()
    excluded = (args.output, args.markdown_output, root / "CHECKSUMS.txt")
    files = list_flat_files(root, excluded)
    entries = [artifact_entry(path) for path in files]
    assets = {entry["name"]: entry for entry in entries}
    required = required_candidate_names(tag)
    missing = sorted(required.difference(assets))
    if missing:
        raise ReleaseSetError(
            "candidate release payload is missing required assets: "
            + ", ".join(missing)
        )

    evidence_paths = {
        name: ensure_flat_file(root / name, root, name)
        for name in REQUIRED_EVIDENCE_FILES
    }
    contract = read_json(evidence_paths["release-contract-evidence.json"], "contract evidence")
    dependencies = read_json(evidence_paths["release-dependencies.json"], "dependency evidence")
    image_evidence = read_json(evidence_paths["release-images.json"], "image evidence")
    product = read_json(evidence_paths["production-golden-path.json"], "product evidence")
    scan = read_json(evidence_paths["release-scan-status.json"], "scan evidence")
    validate_contract_evidence(contract, tag, commit)
    validate_scan_evidence(scan, tag, commit)
    validate_product_evidence(product)
    dependency_images = validate_dependency_evidence(dependencies)
    first_party_images = validate_image_evidence(image_evidence, tag, assets)

    release_identity = (
        f"https://github.com/{repository}/.github/workflows/release.yml@refs/tags/{tag}"
    )
    manifest: dict[str, object] = {
        "schemaVersion": RELEASE_SET_SCHEMA,
        "candidate": {
            "tag": tag,
            "stableVersion": stable_tag_for(tag),
            "sourceCommit": commit,
            "repository": repository,
            "releaseUrl": f"https://github.com/{repository}/releases/tag/{tag}",
        },
        "workflow": {
            "name": "Release",
            "runId": args.run_id,
            "runAttempt": args.run_attempt,
            "url": workflow_url,
            "identity": release_identity,
            "oidcIssuer": "https://token.actions.githubusercontent.com",
            "createdAt": args.created_at,
        },
        "toolchains": {
            "go": args.go_version,
            "node": args.node_version,
            "cosign": args.cosign_version,
        },
        "artifacts": entries,
        "images": first_party_images,
        "dependencies": dependency_images,
        "evidence": [
            {
                "asset": name,
                "sha256": assets[name]["sha256"],
                "bytes": assets[name]["bytes"],
            }
            for name in REQUIRED_EVIDENCE_FILES
        ],
        "gates": [
            {"id": "contract", "status": "passed", "evidenceAsset": "release-contract-evidence.json"},
            {"id": "go-verify", "status": "passed", "evidence": "workflow:go-tests"},
            {"id": "postgres", "status": "passed", "evidence": "workflow:postgres-tests"},
            {"id": "package-acceptance", "status": "passed", "evidence": "workflow:package-acceptance"},
            {"id": "viewer", "status": "passed", "evidence": "workflow:build-viewer"},
            {"id": "image-signatures", "status": "passed", "evidence": "manifest:images[*].signature"},
            {"id": "pull-only-product", "status": "passed", "evidenceAsset": "production-golden-path.json"},
            {"id": "artifact-scan", "status": "passed", "evidenceAsset": "release-scan-status.json"},
        ],
        "remainingExternalGates": [
            {"id": gate_id, "issue": issue, "status": "pending"}
            for gate_id, issue in REQUIRED_PROMOTION_GATES
        ],
        "integrity": {
            "checksumsAsset": "CHECKSUMS.txt",
            "manifestSignature": {
                "asset": "release-set.sigstore.json",
                "type": "sigstore-bundle",
                "identity": release_identity,
                "oidcIssuer": "https://token.actions.githubusercontent.com",
            },
        },
    }
    write_json(args.output, manifest)
    write_text(args.markdown_output, candidate_markdown(manifest))


def candidate_markdown(manifest: Mapping[str, object]) -> str:
    candidate = manifest["candidate"]
    workflow = manifest["workflow"]
    artifacts = manifest["artifacts"]
    images = manifest["images"]
    assert isinstance(candidate, dict)
    assert isinstance(workflow, dict)
    assert isinstance(artifacts, list)
    assert isinstance(images, list)
    lines = [
        "# BitRiver Live candidate release set",
        "",
        f"- Candidate: `{candidate['tag']}`",
        f"- Intended stable alias: `{candidate['stableVersion']}`",
        f"- Source commit: `{candidate['sourceCommit']}`",
        f"- Workflow: [{workflow['runId']}]({workflow['url']})",
        f"- Public artifacts: {len(artifacts)}",
        "- Production identity: exact candidate tag plus digest; `latest` is not used.",
        "",
        "## First-party images",
        "",
        "| Image | Digest |",
        "| --- | --- |",
    ]
    for image in images:
        assert isinstance(image, dict)
        lines.append(f"| `{image['name']}` | `{image['digest']}` |")
    lines.extend(
        [
            "",
            "## Verification",
            "",
            "Verify `release-set.json` with `release-set.sigstore.json`, then verify every file through `CHECKSUMS.txt`. Stable promotion copies these bytes and retags these exact digests; it does not rebuild them.",
            "",
            "External production qualification remains pending until a tracked promotion record binds every required gate to this manifest hash.",
            "",
        ]
    )
    return "\n".join(lines)


def write_checksums(root: Path, output: Path) -> None:
    root = root.resolve()
    output = output.resolve()
    ensure_parent = output.parent.resolve()
    if ensure_parent != root:
        raise ReleaseSetError("checksum output must be a flat release asset")
    files = list_flat_files(root, (output,))
    if not files:
        raise ReleaseSetError("cannot write checksums for an empty release payload")
    content = "".join(f"{sha256_file(path)}  {path.name}\n" for path in files)
    write_text(output, content)


def read_checksums(path: Path) -> dict[str, str]:
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except OSError as exc:
        raise ReleaseSetError(f"cannot read checksums: {exc}") from exc
    if not lines:
        raise ReleaseSetError("checksum file is empty")
    values: dict[str, str] = {}
    previous = ""
    for line in lines:
        match = CHECKSUM_LINE_PATTERN.fullmatch(line)
        if match is None or not SAFE_NAME_PATTERN.fullmatch(match.group("name")):
            raise ReleaseSetError("checksum file contains an invalid line")
        name = match.group("name")
        if name in values:
            raise ReleaseSetError(f"checksum file repeats {name}")
        if previous and name <= previous:
            raise ReleaseSetError("checksum entries must be sorted by asset name")
        previous = name
        values[name] = match.group("digest")
    return values


def validate_manifest_shape(manifest: Mapping[str, object]) -> None:
    if manifest.get("schemaVersion") != RELEASE_SET_SCHEMA:
        raise ReleaseSetError("release set has an unsupported schema")
    candidate = manifest.get("candidate")
    if not isinstance(candidate, dict):
        raise ReleaseSetError("release set is missing candidate metadata")
    tag = candidate.get("tag")
    commit = candidate.get("sourceCommit")
    repository = candidate.get("repository")
    if not isinstance(tag, str):
        raise ReleaseSetError("release set candidate tag is invalid")
    parse_tag(tag, prerelease=True)
    if candidate.get("stableVersion") != stable_tag_for(tag):
        raise ReleaseSetError("release set stable version does not match candidate")
    if not isinstance(commit, str) or not COMMIT_PATTERN.fullmatch(commit):
        raise ReleaseSetError("release set source commit is invalid")
    if not isinstance(repository, str):
        raise ReleaseSetError("release set repository is invalid")
    validate_repository(repository)
    artifacts = manifest.get("artifacts")
    if not isinstance(artifacts, list) or not artifacts:
        raise ReleaseSetError("release set is missing artifacts")
    images = manifest.get("images")
    if not isinstance(images, list) or len(images) != len(FIRST_PARTY_IMAGES):
        raise ReleaseSetError("release set must contain five first-party images")
    integrity = manifest.get("integrity")
    if not isinstance(integrity, dict):
        raise ReleaseSetError("release set is missing integrity metadata")
    signature = integrity.get("manifestSignature")
    if not isinstance(signature, dict) or signature.get("asset") != "release-set.sigstore.json":
        raise ReleaseSetError("release set is missing its signature reference")


def verify_candidate_root(
    root: Path,
    manifest_path: Path,
    *,
    expected_tag: str | None = None,
    expected_commit: str | None = None,
) -> dict[str, object]:
    root = root.resolve()
    manifest_path = ensure_flat_file(manifest_path, root, "release-set manifest")
    manifest = read_json(manifest_path, "release-set manifest")
    validate_manifest_shape(manifest)
    candidate = manifest["candidate"]
    assert isinstance(candidate, dict)
    if expected_tag is not None and candidate.get("tag") != expected_tag:
        raise ReleaseSetError("release set candidate tag does not match expected tag")
    if expected_commit is not None and candidate.get("sourceCommit") != expected_commit:
        raise ReleaseSetError("release set source commit does not match expected commit")

    checksums_path = ensure_flat_file(root / "CHECKSUMS.txt", root, "checksums")
    checksums = read_checksums(checksums_path)
    actual_files = list_flat_files(root, (checksums_path,))
    actual_names = {path.name for path in actual_files}
    if set(checksums) != actual_names:
        missing = sorted(actual_names.difference(checksums))
        extra = sorted(set(checksums).difference(actual_names))
        raise ReleaseSetError(
            "checksum coverage mismatch"
            + (f"; missing: {', '.join(missing)}" if missing else "")
            + (f"; extra: {', '.join(extra)}" if extra else "")
        )
    for path in actual_files:
        if sha256_file(path) != checksums[path.name]:
            raise ReleaseSetError(f"checksum mismatch for {path.name}")

    artifact_values = manifest["artifacts"]
    assert isinstance(artifact_values, list)
    manifest_assets: dict[str, Mapping[str, object]] = {}
    for entry in artifact_values:
        if not isinstance(entry, dict):
            raise ReleaseSetError("release set contains an invalid artifact entry")
        name = entry.get("name")
        if not isinstance(name, str) or not SAFE_NAME_PATTERN.fullmatch(name):
            raise ReleaseSetError("release set contains an unsafe artifact name")
        if name in manifest_assets:
            raise ReleaseSetError(f"release set repeats artifact {name}")
        manifest_assets[name] = entry
    expected_manifest_assets = actual_names.difference(RELEASE_SET_METADATA)
    if set(manifest_assets) != expected_manifest_assets:
        raise ReleaseSetError("release-set artifact inventory does not match payload")
    for name, entry in manifest_assets.items():
        path = root / name
        if entry.get("sha256") != sha256_file(path) or entry.get("bytes") != path.stat().st_size:
            raise ReleaseSetError(f"release-set artifact metadata mismatch for {name}")

    image_evidence = read_json(root / "release-images.json", "image evidence")
    validate_image_evidence(image_evidence, str(candidate["tag"]), manifest_assets)
    validate_dependency_evidence(
        read_json(root / "release-dependencies.json", "dependency evidence")
    )
    validate_contract_evidence(
        read_json(root / "release-contract-evidence.json", "contract evidence"),
        str(candidate["tag"]),
        str(candidate["sourceCommit"]),
    )
    validate_product_evidence(
        read_json(root / "production-golden-path.json", "product evidence")
    )
    validate_scan_evidence(
        read_json(root / "release-scan-status.json", "scan evidence"),
        str(candidate["tag"]),
        str(candidate["sourceCommit"]),
    )
    return manifest


def validate_promotion_record(
    record: Mapping[str, object], manifest_path: Path, manifest: Mapping[str, object]
) -> None:
    if record.get("schemaVersion") != PROMOTION_SCHEMA:
        raise ReleaseSetError("promotion record has an unsupported schema")
    candidate = manifest["candidate"]
    assert isinstance(candidate, dict)
    manifest_digest = sha256_file(manifest_path)
    required = {
        "candidateTag": candidate["tag"],
        "candidateReleaseSetSha256": manifest_digest,
        "stableTag": candidate["stableVersion"],
        "decision": "approved",
        "epicIssue": 1293,
    }
    for key, expected in required.items():
        if record.get(key) != expected:
            raise ReleaseSetError(f"promotion record {key} must equal {expected!r}")
    validate_https_url(record.get("candidateReleaseUrl"), "candidate release URL")
    if not isinstance(record.get("approvedAt"), str) or not record["approvedAt"].endswith("Z"):
        raise ReleaseSetError("promotion record approvedAt must be a UTC timestamp")
    if not isinstance(record.get("approvedBy"), str) or not record["approvedBy"].strip():
        raise ReleaseSetError("promotion record approvedBy is required")
    gates = record.get("gates")
    if not isinstance(gates, list):
        raise ReleaseSetError("promotion record is missing gates")
    by_id: dict[str, Mapping[str, object]] = {}
    for gate in gates:
        if not isinstance(gate, dict) or not isinstance(gate.get("id"), str):
            raise ReleaseSetError("promotion record contains an invalid gate")
        gate_id = gate["id"]
        if gate_id in by_id:
            raise ReleaseSetError(f"promotion record repeats gate {gate_id}")
        by_id[gate_id] = gate
    expected_ids = {gate_id for gate_id, _ in REQUIRED_PROMOTION_GATES}
    if set(by_id) != expected_ids:
        raise ReleaseSetError(
            "promotion record gates must be exactly: " + ", ".join(sorted(expected_ids))
        )
    for gate_id, issue in REQUIRED_PROMOTION_GATES:
        gate = by_id[gate_id]
        if gate.get("issue") != issue or gate.get("status") != "passed":
            raise ReleaseSetError(f"promotion gate {gate_id} must pass issue #{issue}")
        if gate.get("candidateReleaseSetSha256") != manifest_digest:
            raise ReleaseSetError(f"promotion gate {gate_id} targets another candidate")
        validate_https_url(gate.get("evidenceUrl"), f"promotion gate {gate_id} evidence URL")
        evidence_digest = gate.get("evidenceSha256")
        if not isinstance(evidence_digest, str) or not SHA256_PATTERN.fullmatch(evidence_digest):
            raise ReleaseSetError(f"promotion gate {gate_id} evidence digest is invalid")


def classify_existing_state(
    manifest: Mapping[str, object], root: Path, state: Mapping[str, object]
) -> str:
    candidate = manifest["candidate"]
    assert isinstance(candidate, dict)
    commit = candidate["sourceCommit"]
    stable_ref = state.get("stableRefCommit")
    if stable_ref is not None and stable_ref != commit:
        raise ReleaseSetError("existing stable tag points at another commit")
    release = state.get("release")
    if release is None:
        return "resume" if stable_ref is not None else "new"
    if not isinstance(release, dict):
        raise ReleaseSetError("existing release state is invalid")
    if release.get("targetCommit") != commit:
        raise ReleaseSetError("existing stable release targets another commit")
    assets = release.get("assets")
    if not isinstance(assets, list):
        raise ReleaseSetError("existing stable release assets are invalid")
    local = {
        path.name: sha256_file(path)
        for path in list_flat_files(root)
        if path.name not in ALLOWED_STABLE_METADATA
    }
    present: set[str] = set()
    for asset in assets:
        if not isinstance(asset, dict):
            raise ReleaseSetError("existing stable release contains invalid asset state")
        name = asset.get("name")
        digest = asset.get("digest")
        if not isinstance(name, str) or not isinstance(digest, str):
            raise ReleaseSetError("existing stable release asset state is incomplete")
        if name in present:
            raise ReleaseSetError(f"existing stable release repeats asset {name}")
        present.add(name)
        if name in local and digest != f"sha256:{local[name]}":
            raise ReleaseSetError(f"existing stable asset {name} has different bytes")
        if name not in local and name not in ALLOWED_STABLE_METADATA and not name.endswith("-promotion.json"):
            raise ReleaseSetError(f"existing stable release has unexpected asset {name}")
    missing = set(local).difference(present)
    draft = release.get("draft")
    if not isinstance(draft, bool):
        raise ReleaseSetError("existing stable release draft state is invalid")
    if missing and not draft:
        raise ReleaseSetError("published stable release is missing candidate assets")
    return "resume" if draft or missing else "complete"


def create_stable_metadata(args: argparse.Namespace) -> None:
    manifest_path = args.candidate_manifest.resolve()
    manifest = read_json(manifest_path, "candidate release set")
    validate_manifest_shape(manifest)
    record = read_json(args.promotion_record, "promotion record")
    validate_promotion_record(record, manifest_path, manifest)
    candidate = manifest["candidate"]
    images = manifest["images"]
    assert isinstance(candidate, dict)
    assert isinstance(images, list)
    previous: dict[str, object] | None = None
    if args.previous_stable_manifest is not None:
        previous_path = args.previous_stable_manifest.resolve()
        previous_value = read_json(previous_path, "previous stable release set")
        if previous_value.get("schemaVersion") != STABLE_SET_SCHEMA:
            raise ReleaseSetError("previous stable release set has an unsupported schema")
        previous = {
            "stableTag": previous_value.get("stableTag"),
            "manifestSha256": sha256_file(previous_path),
            "releaseUrl": previous_value.get("releaseUrl"),
            "images": previous_value.get("images"),
        }
    promoted_images = []
    for image in images:
        if not isinstance(image, dict):
            raise ReleaseSetError("candidate release set contains an invalid image")
        immutable = image.get("immutableReference")
        digest = image.get("digest")
        name = image.get("name")
        if not all(isinstance(value, str) for value in (immutable, digest, name)):
            raise ReleaseSetError("candidate release set image is incomplete")
        repository = immutable.split("@", 1)[0]
        promoted_images.append(
            {
                "name": name,
                "digest": digest,
                "candidateReference": image.get("candidateReference"),
                "immutableReference": immutable,
                "stableReference": f"{repository}:{record['stableTag']}",
            }
        )
    stable: dict[str, object] = {
        "schemaVersion": STABLE_SET_SCHEMA,
        "stableTag": record["stableTag"],
        "releaseUrl": f"https://github.com/{candidate['repository']}/releases/tag/{record['stableTag']}",
        "sourceCommit": candidate["sourceCommit"],
        "candidateTag": candidate["tag"],
        "candidateReleaseUrl": record["candidateReleaseUrl"],
        "candidateReleaseSetSha256": sha256_file(manifest_path),
        "promotionRecordSha256": sha256_file(args.promotion_record),
        "promotion": {
            "approvedAt": record["approvedAt"],
            "approvedBy": record["approvedBy"],
            "recordAsset": f"{record['stableTag']}-promotion.json",
        },
        "images": promoted_images,
        "artifactPolicy": "candidate-assets-copied-byte-for-byte",
        "latest": {
            "requested": args.publish_latest,
            "authoritative": False,
            "productionSource": "stableReference plus digest",
        },
        "previousStable": previous,
    }
    rollback = {
        "schemaVersion": ROLLBACK_SCHEMA,
        "currentStableTag": record["stableTag"],
        "currentReleaseSetSha256": None,
        "previousStable": previous,
        "rollbackAvailable": previous is not None,
        "reason": None if previous is not None else "first stable release has no previous stable set",
    }
    write_json(args.output, stable)
    rollback["currentReleaseSetSha256"] = sha256_file(args.output)
    write_json(args.rollback_output, rollback)
    lines = [
        "# BitRiver Live stable release set",
        "",
        f"- Stable tag: `{record['stableTag']}`",
        f"- Promoted candidate: `{candidate['tag']}`",
        f"- Source commit: `{candidate['sourceCommit']}`",
        f"- Candidate release-set SHA-256: `{sha256_file(manifest_path)}`",
        "- Artifact policy: candidate assets copied byte-for-byte; no rebuild.",
        "- Production identity: stable tag plus the digest recorded below.",
        "",
        "| Image | Stable reference | Digest |",
        "| --- | --- | --- |",
    ]
    for image in promoted_images:
        lines.append(
            f"| `{image['name']}` | `{image['stableReference']}` | `{image['digest']}` |"
        )
    lines.extend(
        [
            "",
            (
                f"Previous stable: `{previous['stableTag']}`."
                if previous is not None
                else "Previous stable: none (first stable release)."
            ),
            "",
        ]
    )
    write_text(args.markdown_output, "\n".join(lines))


def create_revocation(args: argparse.Namespace) -> None:
    manifest_path = args.candidate_manifest.resolve()
    manifest = read_json(manifest_path, "candidate release set")
    validate_manifest_shape(manifest)
    candidate = manifest["candidate"]
    assert isinstance(candidate, dict)
    if args.candidate_tag != candidate.get("tag"):
        raise ReleaseSetError("revocation candidate tag does not match release set")
    reason = args.reason.strip()
    if len(reason) < 12:
        raise ReleaseSetError("revocation reason must be actionable")
    value = {
        "schemaVersion": REVOCATION_SCHEMA,
        "candidateTag": candidate["tag"],
        "candidateReleaseSetSha256": sha256_file(manifest_path),
        "sourceCommit": candidate["sourceCommit"],
        "revokedAt": args.revoked_at,
        "revokedBy": args.actor,
        "reason": reason,
        "workflowUrl": validate_https_url(args.workflow_url, "revocation workflow URL"),
        "stableStateChanged": False,
    }
    write_json(args.output, value)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)

    candidate = subparsers.add_parser("candidate", help="create a candidate release set")
    candidate.add_argument("--tag", required=True)
    candidate.add_argument("--commit", required=True)
    candidate.add_argument("--repository", required=True)
    candidate.add_argument("--run-id", type=int, required=True)
    candidate.add_argument("--run-attempt", type=int, required=True)
    candidate.add_argument("--workflow-url", required=True)
    candidate.add_argument("--created-at", required=True)
    candidate.add_argument("--go-version", required=True)
    candidate.add_argument("--node-version", required=True)
    candidate.add_argument("--cosign-version", required=True)
    candidate.add_argument("--assets-dir", type=Path, required=True)
    candidate.add_argument("--output", type=Path, required=True)
    candidate.add_argument("--markdown-output", type=Path, required=True)

    checksums = subparsers.add_parser("checksums", help="write sorted checksum coverage")
    checksums.add_argument("--assets-dir", type=Path, required=True)
    checksums.add_argument("--output", type=Path, required=True)

    verify = subparsers.add_parser("verify-candidate", help="verify a candidate payload")
    verify.add_argument("--assets-dir", type=Path, required=True)
    verify.add_argument("--manifest", type=Path, required=True)
    verify.add_argument("--expected-tag")
    verify.add_argument("--expected-commit")
    verify.add_argument("--promotion-record", type=Path)
    verify.add_argument("--revocation-marker", type=Path)

    state = subparsers.add_parser("classify-state", help="classify stable publication state")
    state.add_argument("--assets-dir", type=Path, required=True)
    state.add_argument("--manifest", type=Path, required=True)
    state.add_argument("--state", type=Path, required=True)

    stable = subparsers.add_parser("stable", help="create stable and rollback metadata")
    stable.add_argument("--candidate-manifest", type=Path, required=True)
    stable.add_argument("--promotion-record", type=Path, required=True)
    stable.add_argument("--previous-stable-manifest", type=Path)
    stable.add_argument("--publish-latest", action="store_true")
    stable.add_argument("--output", type=Path, required=True)
    stable.add_argument("--markdown-output", type=Path, required=True)
    stable.add_argument("--rollback-output", type=Path, required=True)

    revoke = subparsers.add_parser("revoke", help="create an append-only revocation marker")
    revoke.add_argument("--candidate-manifest", type=Path, required=True)
    revoke.add_argument("--candidate-tag", required=True)
    revoke.add_argument("--reason", required=True)
    revoke.add_argument("--actor", required=True)
    revoke.add_argument("--revoked-at", required=True)
    revoke.add_argument("--workflow-url", required=True)
    revoke.add_argument("--output", type=Path, required=True)
    return parser


def main(argv: Sequence[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        if args.command == "candidate":
            create_candidate_manifest(args)
            print(f"candidate release set created: {args.output}")
        elif args.command == "checksums":
            write_checksums(args.assets_dir, args.output)
            print(f"checksum coverage created: {args.output}")
        elif args.command == "verify-candidate":
            manifest = verify_candidate_root(
                args.assets_dir,
                args.manifest,
                expected_tag=args.expected_tag,
                expected_commit=args.expected_commit,
            )
            if args.revocation_marker is not None and args.revocation_marker.exists():
                marker = read_json(args.revocation_marker, "candidate revocation marker")
                if marker.get("schemaVersion") != REVOCATION_SCHEMA:
                    raise ReleaseSetError("candidate revocation marker has an unsupported schema")
                raise ReleaseSetError("candidate is revoked and cannot be promoted")
            if args.promotion_record is not None:
                record = read_json(args.promotion_record, "promotion record")
                validate_promotion_record(record, args.manifest.resolve(), manifest)
            print("candidate release set verification passed")
        elif args.command == "classify-state":
            manifest = verify_candidate_root(args.assets_dir, args.manifest)
            state = read_json(args.state, "existing stable state")
            print(classify_existing_state(manifest, args.assets_dir.resolve(), state))
        elif args.command == "stable":
            create_stable_metadata(args)
            print(f"stable release set created: {args.output}")
        elif args.command == "revoke":
            create_revocation(args)
            print(f"candidate revocation marker created: {args.output}")
        return 0
    except (OSError, ReleaseSetError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
