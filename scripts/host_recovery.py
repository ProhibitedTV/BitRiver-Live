#!/usr/bin/env python3
"""Build and verify secret-safe packaged-host recovery metadata."""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import os
import re
import stat
import sys
import tarfile
import tempfile
from pathlib import Path, PurePosixPath
from typing import Any, Iterable, Sequence


HOST_BACKUP_SCHEMA = "bitriver.host-backup/v1"
HOST_RESTORE_SCHEMA = "bitriver.host-restore-report/v1"
DISASTER_REPORT_SCHEMA = "bitriver.disaster-recovery/v1"
OBJECT_INVENTORY_SCHEMA = "bitriver.object-inventory/v1"
POSTGRES_BACKUP_SCHEMA = "bitriver.postgres-backup/v1"
RELEASE_PATTERN = re.compile(
    r"^v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)"
    r"(?:-[0-9A-Za-z.-]+)?$"
)
COMMIT_PATTERN = re.compile(r"^[0-9a-f]{40}$")
SHA256_PATTERN = re.compile(r"^[0-9a-f]{64}$")
CONTROL_PATTERN = re.compile(r"[\x00-\x1f\x7f]")
CONFIG_RELATIVE = PurePosixPath("etc/bitriver-live")
DATA_RELATIVE = PurePosixPath("var/lib/bitriver-live")
POSTGRES_RELATIVE = PurePosixPath("var/backups/bitriver-live/recovery/postgres")
OBJECT_INVENTORY_RELATIVE = PurePosixPath(
    "var/backups/bitriver-live/recovery/object-inventory.json"
)


class RecoveryError(ValueError):
    """Raised when recovery evidence or payloads violate the contract."""


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def read_json(path: Path, label: str) -> dict[str, Any]:
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise RecoveryError(f"cannot read {label}: {exc}") from exc
    if not isinstance(payload, dict):
        raise RecoveryError(f"{label} must be a JSON object")
    return payload


def atomic_write(path: Path, contents: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary_name = tempfile.mkstemp(
        dir=path.parent, prefix=f".{path.name}.", text=True
    )
    temporary = Path(temporary_name)
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8", newline="\n") as handle:
            handle.write(contents)
            handle.flush()
            os.fsync(handle.fileno())
        os.chmod(temporary, 0o600)
        os.replace(temporary, path)
    finally:
        temporary.unlink(missing_ok=True)


def validate_identity(release: str, commit: str) -> None:
    if not RELEASE_PATTERN.fullmatch(release):
        raise RecoveryError("source release must be an exact v-prefixed release")
    if not COMMIT_PATTERN.fullmatch(commit):
        raise RecoveryError("source commit must be 40 lowercase hexadecimal characters")


def parse_timestamp(value: str) -> dt.datetime:
    try:
        parsed = dt.datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError as exc:
        raise RecoveryError("createdAt must be an RFC3339 UTC timestamp") from exc
    if parsed.tzinfo is None or parsed.utcoffset() != dt.timedelta(0):
        raise RecoveryError("createdAt must include the UTC timezone")
    return parsed.astimezone(dt.timezone.utc)


def safe_relative(path: Path, root: Path) -> str:
    relative = path.relative_to(root).as_posix()
    if not relative or CONTROL_PATTERN.search(relative):
        raise RecoveryError("recovery tree contains an empty or control-character path")
    return relative


def ensure_within(path: Path, allowed_root: Path) -> Path:
    try:
        resolved = path.resolve(strict=True)
    except OSError as exc:
        raise RecoveryError(f"cannot resolve recovery tree entry: {exc}") from exc
    try:
        resolved.relative_to(allowed_root.resolve(strict=True))
    except ValueError as exc:
        raise RecoveryError("recovery tree contains a symlink outside the host root") from exc
    return resolved


def tree_inventory(root: Path, allowed_root: Path) -> dict[str, Any]:
    if not root.is_dir():
        raise RecoveryError(f"required recovery directory is missing: {root}")
    digest = hashlib.sha256()
    file_count = 0
    directory_count = 0
    total_bytes = 0

    def record(parts: Iterable[str]) -> None:
        digest.update("\0".join(parts).encode("utf-8"))
        digest.update(b"\n")

    def visit(directory: Path) -> None:
        nonlocal file_count, directory_count, total_bytes
        try:
            entries = sorted(os.scandir(directory), key=lambda item: item.name)
        except OSError as exc:
            raise RecoveryError(f"cannot inventory recovery tree: {exc}") from exc
        for entry in entries:
            path = Path(entry.path)
            relative = safe_relative(path, root)
            if entry.is_symlink():
                resolved = ensure_within(path, allowed_root)
                if resolved.is_dir():
                    raise RecoveryError("directory symlinks are not supported in recovery state")
                if not resolved.is_file():
                    raise RecoveryError("recovery tree symlink does not target a regular file")
                metadata = resolved.stat()
                content_sha = sha256_file(resolved)
                file_count += 1
                total_bytes += metadata.st_size
                record(
                    (
                        "file",
                        relative,
                        format(stat.S_IMODE(metadata.st_mode), "04o"),
                        str(metadata.st_size),
                        content_sha,
                    )
                )
            elif entry.is_dir(follow_symlinks=False):
                metadata = path.stat(follow_symlinks=False)
                directory_count += 1
                record(
                    (
                        "directory",
                        relative,
                        format(stat.S_IMODE(metadata.st_mode), "04o"),
                    )
                )
                visit(path)
            elif entry.is_file(follow_symlinks=False):
                metadata = path.stat(follow_symlinks=False)
                content_sha = sha256_file(path)
                file_count += 1
                total_bytes += metadata.st_size
                record(
                    (
                        "file",
                        relative,
                        format(stat.S_IMODE(metadata.st_mode), "04o"),
                        str(metadata.st_size),
                        content_sha,
                    )
                )
            else:
                raise RecoveryError("recovery tree contains a non-file, non-directory entry")

    visit(root)
    return {
        "directoryCount": directory_count,
        "fileCount": file_count,
        "totalBytes": total_bytes,
        "fingerprintSha256": digest.hexdigest(),
    }


def parse_checksum_file(
    checksum_path: Path, expected_files: dict[str, Path]
) -> dict[str, str]:
    try:
        lines = checksum_path.read_text(encoding="utf-8").splitlines()
    except OSError as exc:
        raise RecoveryError(f"cannot read checksum file: {exc}") from exc
    if len(lines) != len(expected_files):
        raise RecoveryError("checksum set must cover every expected asset exactly once")
    found: dict[str, str] = {}
    for line in lines:
        match = re.fullmatch(r"([0-9a-f]{64})  ([^/\\]+)", line)
        if match is None:
            raise RecoveryError("checksum set contains an invalid entry")
        digest, name = match.groups()
        if name not in expected_files or name in found:
            raise RecoveryError("checksum set contains an unexpected or duplicate asset")
        found[name] = digest
    for name, path in expected_files.items():
        if not path.is_file() or sha256_file(path) != found[name]:
            raise RecoveryError(f"checksum mismatch for {name}")
    return found


def postgres_assets(backup: Path) -> tuple[Path, Path]:
    return Path(f"{backup}.manifest.json"), Path(f"{backup}.sha256")


def validate_postgres_set(
    backup: Path, *, expected_release: str, expected_commit: str
) -> dict[str, Any]:
    manifest_path, checksum_path = postgres_assets(backup)
    expected = {
        backup.name: backup,
        manifest_path.name: manifest_path,
    }
    parse_checksum_file(checksum_path, expected)
    manifest = read_json(manifest_path, "Postgres backup manifest")
    if manifest.get("schemaVersion") != POSTGRES_BACKUP_SCHEMA:
        raise RecoveryError("unsupported Postgres backup manifest schema")
    source = manifest.get("source")
    if not isinstance(source, dict):
        raise RecoveryError("Postgres backup manifest has no source identity")
    if source.get("release") != expected_release or source.get("commit") != expected_commit:
        raise RecoveryError("Postgres backup identity does not match the host recovery set")
    archive = manifest.get("archive")
    if not isinstance(archive, dict):
        raise RecoveryError("Postgres backup manifest has no archive identity")
    if archive.get("name") != backup.name or archive.get("sha256") != sha256_file(backup):
        raise RecoveryError("Postgres backup manifest archive identity is inconsistent")
    return {
        "archiveName": backup.name,
        "manifestName": manifest_path.name,
        "checksumName": checksum_path.name,
        "archiveSha256": sha256_file(backup),
        "manifestSha256": sha256_file(manifest_path),
        "checksumSha256": sha256_file(checksum_path),
        "migrationFingerprintSha256": manifest.get("database", {}).get(
            "migrationFingerprintSha256", ""
        ),
    }


def validate_object_inventory(path: Path) -> dict[str, Any]:
    inventory = read_json(path, "object inventory")
    if inventory.get("schemaVersion") != OBJECT_INVENTORY_SCHEMA:
        raise RecoveryError("unsupported object inventory schema")
    result = {
        "objectCount": inventory.get("objectCount"),
        "totalBytes": inventory.get("totalBytes"),
        "fingerprintSha256": inventory.get("fingerprintSha256"),
        "assetSha256": sha256_file(path),
    }
    if (
        not isinstance(result["objectCount"], int)
        or result["objectCount"] < 0
        or not isinstance(result["totalBytes"], int)
        or result["totalBytes"] < 0
        or not isinstance(result["fingerprintSha256"], str)
        or not SHA256_PATTERN.fullmatch(result["fingerprintSha256"])
    ):
        raise RecoveryError("object inventory contains invalid aggregate values")
    return result


def build_object_inventory(root: Path) -> dict[str, Any]:
    aggregate = tree_inventory(root, root)
    return {
        "schemaVersion": OBJECT_INVENTORY_SCHEMA,
        "objectCount": aggregate["fileCount"],
        "totalBytes": aggregate["totalBytes"],
        "fingerprintSha256": aggregate["fingerprintSha256"],
    }


def preflight_host_state(root_prefix: Path) -> None:
    prefix = root_prefix.resolve(strict=True)
    tree_inventory(prefix / Path(CONFIG_RELATIVE.as_posix()), prefix)
    tree_inventory(prefix / Path(DATA_RELATIVE.as_posix()), prefix)


def build_backup_manifest(args: argparse.Namespace) -> None:
    validate_identity(args.source_release, args.source_commit)
    created = parse_timestamp(args.created_at)
    if created > dt.datetime.now(dt.timezone.utc) + dt.timedelta(minutes=5):
        raise RecoveryError("createdAt cannot be materially in the future")
    prefix = args.root_prefix.resolve(strict=True)
    config = prefix / Path(CONFIG_RELATIVE.as_posix())
    data = prefix / Path(DATA_RELATIVE.as_posix())
    postgres = validate_postgres_set(
        args.postgres_backup,
        expected_release=args.source_release,
        expected_commit=args.source_commit,
    )
    external_objects = (
        validate_object_inventory(args.object_inventory)
        if args.object_inventory is not None
        else None
    )
    manifest = {
        "schemaVersion": HOST_BACKUP_SCHEMA,
        "createdAt": args.created_at,
        "source": {
            "release": args.source_release,
            "commit": args.source_commit,
        },
        "archive": {
            "name": args.archive.name,
            "format": "tar+gzip+openssl-aes-256-cbc",
            "sha256": sha256_file(args.archive),
            "sizeBytes": args.archive.stat().st_size,
            "checksumAsset": f"{args.archive.name}.sha256",
        },
        "encryption": {
            "tool": "openssl",
            "cipher": "aes-256-cbc",
            "kdf": "PBKDF2-HMAC-SHA256",
            "iterations": args.iterations,
            "passphraseSource": "restricted-file",
        },
        "payload": {
            "configuration": {
                "archiveRoot": CONFIG_RELATIVE.as_posix(),
                **tree_inventory(config, prefix),
            },
            "data": {
                "archiveRoot": DATA_RELATIVE.as_posix(),
                **tree_inventory(data, prefix),
            },
            "postgres": postgres,
            "externalObjects": external_objects,
        },
    }
    atomic_write(args.output, json.dumps(manifest, indent=2, sort_keys=True) + "\n")


def outer_assets(archive: Path) -> tuple[Path, Path]:
    return Path(f"{archive}.manifest.json"), Path(f"{archive}.sha256")


def verify_backup_set(
    archive: Path, *, expected_release: str, expected_commit: str
) -> dict[str, Any]:
    validate_identity(expected_release, expected_commit)
    manifest_path, checksum_path = outer_assets(archive)
    parse_checksum_file(
        checksum_path,
        {archive.name: archive, manifest_path.name: manifest_path},
    )
    manifest = read_json(manifest_path, "host backup manifest")
    if manifest.get("schemaVersion") != HOST_BACKUP_SCHEMA:
        raise RecoveryError("unsupported host backup manifest schema")
    source = manifest.get("source")
    if not isinstance(source, dict):
        raise RecoveryError("host backup manifest has no source identity")
    if source.get("release") != expected_release or source.get("commit") != expected_commit:
        raise RecoveryError("host backup identity does not match the requested release")
    archive_metadata = manifest.get("archive")
    if not isinstance(archive_metadata, dict):
        raise RecoveryError("host backup manifest has no archive identity")
    if (
        archive_metadata.get("name") != archive.name
        or archive_metadata.get("sha256") != sha256_file(archive)
        or archive_metadata.get("sizeBytes") != archive.stat().st_size
    ):
        raise RecoveryError("host backup manifest archive identity is inconsistent")
    encryption = manifest.get("encryption")
    if not isinstance(encryption, dict) or encryption != {
        "tool": "openssl",
        "cipher": "aes-256-cbc",
        "kdf": "PBKDF2-HMAC-SHA256",
        "iterations": encryption.get("iterations") if isinstance(encryption, dict) else None,
        "passphraseSource": "restricted-file",
    }:
        raise RecoveryError("host backup manifest encryption metadata is invalid")
    iterations = encryption.get("iterations")
    if not isinstance(iterations, int) or iterations < 100_000:
        raise RecoveryError("host backup PBKDF2 iteration count is below policy")
    parse_timestamp(str(manifest.get("createdAt", "")))
    payload = manifest.get("payload")
    if not isinstance(payload, dict):
        raise RecoveryError("host backup manifest has no payload inventory")
    for key, expected_root in (
        ("configuration", CONFIG_RELATIVE.as_posix()),
        ("data", DATA_RELATIVE.as_posix()),
    ):
        inventory = payload.get(key)
        if not isinstance(inventory, dict) or inventory.get("archiveRoot") != expected_root:
            raise RecoveryError(f"host backup manifest has invalid {key} inventory")
        for field in ("directoryCount", "fileCount", "totalBytes"):
            if not isinstance(inventory.get(field), int) or inventory[field] < 0:
                raise RecoveryError(f"host backup manifest has invalid {key} {field}")
        fingerprint = inventory.get("fingerprintSha256")
        if not isinstance(fingerprint, str) or not SHA256_PATTERN.fullmatch(fingerprint):
            raise RecoveryError(f"host backup manifest has invalid {key} fingerprint")
    postgres = payload.get("postgres")
    if not isinstance(postgres, dict):
        raise RecoveryError("host backup manifest has no Postgres inventory")
    for field in ("archiveName", "manifestName", "checksumName"):
        value = postgres.get(field)
        if not isinstance(value, str) or Path(value).name != value:
            raise RecoveryError("host backup manifest has an unsafe Postgres asset name")
    external = payload.get("externalObjects")
    if external is not None:
        if not isinstance(external, dict):
            raise RecoveryError("host backup manifest object inventory is invalid")
        for field in ("fingerprintSha256", "assetSha256"):
            value = external.get(field)
            if not isinstance(value, str) or not SHA256_PATTERN.fullmatch(value):
                raise RecoveryError("host backup manifest object inventory hash is invalid")
    return manifest


def validate_member_name(name: str) -> PurePosixPath:
    if not name or CONTROL_PATTERN.search(name) or name.startswith("/"):
        raise RecoveryError("encrypted archive contains an unsafe path")
    normalized = PurePosixPath(name.rstrip("/"))
    if not normalized.parts or any(part in ("", ".", "..") for part in normalized.parts):
        raise RecoveryError("encrypted archive contains an unsafe path")
    return normalized


def is_at_or_below(path: PurePosixPath, root: PurePosixPath) -> bool:
    return path == root or path.parts[: len(root.parts)] == root.parts


def validate_archive_stream(manifest: dict[str, Any], stream: Any) -> None:
    postgres = manifest["payload"]["postgres"]
    expected_postgres = {
        POSTGRES_RELATIVE / postgres["archiveName"],
        POSTGRES_RELATIVE / postgres["manifestName"],
        POSTGRES_RELATIVE / postgres["checksumName"],
    }
    expected_object = (
        OBJECT_INVENTORY_RELATIVE
        if manifest["payload"].get("externalObjects") is not None
        else None
    )
    seen: set[PurePosixPath] = set()
    roots_seen: set[PurePosixPath] = set()
    postgres_seen: set[PurePosixPath] = set()
    object_seen = False
    try:
        archive = tarfile.open(fileobj=stream, mode="r|gz")
        for member in archive:
            path = validate_member_name(member.name)
            if path in seen:
                raise RecoveryError("encrypted archive repeats a path")
            seen.add(path)
            if not (member.isdir() or member.isfile()):
                raise RecoveryError("encrypted archive contains a link or special entry")
            if is_at_or_below(path, CONFIG_RELATIVE):
                roots_seen.add(CONFIG_RELATIVE)
            elif is_at_or_below(path, DATA_RELATIVE):
                roots_seen.add(DATA_RELATIVE)
            elif path in expected_postgres and member.isfile():
                postgres_seen.add(path)
            elif expected_object is not None and path == expected_object and member.isfile():
                object_seen = True
            else:
                raise RecoveryError("encrypted archive contains an unexpected path")
    except (tarfile.TarError, OSError) as exc:
        raise RecoveryError(f"cannot inspect encrypted archive: {exc}") from exc
    if roots_seen != {CONFIG_RELATIVE, DATA_RELATIVE}:
        raise RecoveryError("encrypted archive is missing a required host-state root")
    if postgres_seen != expected_postgres:
        raise RecoveryError("encrypted archive has an incomplete Postgres backup set")
    if expected_object is not None and not object_seen:
        raise RecoveryError("encrypted archive is missing the object inventory")


def build_restore_report(args: argparse.Namespace) -> None:
    manifest = verify_backup_set(
        args.archive,
        expected_release=args.expected_release,
        expected_commit=args.expected_commit,
    )
    prefix = args.root_prefix.resolve(strict=True)
    payload = manifest["payload"]
    observed_config = tree_inventory(
        prefix / Path(CONFIG_RELATIVE.as_posix()), prefix
    )
    observed_data = tree_inventory(prefix / Path(DATA_RELATIVE.as_posix()), prefix)
    expected_config = {
        key: payload["configuration"][key]
        for key in ("directoryCount", "fileCount", "totalBytes", "fingerprintSha256")
    }
    expected_data = {
        key: payload["data"][key]
        for key in ("directoryCount", "fileCount", "totalBytes", "fingerprintSha256")
    }
    postgres_info = payload["postgres"]
    recovered_postgres = prefix / Path(POSTGRES_RELATIVE.as_posix()) / postgres_info["archiveName"]
    validate_postgres_set(
        recovered_postgres,
        expected_release=args.expected_release,
        expected_commit=args.expected_commit,
    )
    external_match = True
    external = payload.get("externalObjects")
    if external is not None:
        recovered_inventory = prefix / Path(OBJECT_INVENTORY_RELATIVE.as_posix())
        external_match = (
            recovered_inventory.is_file()
            and sha256_file(recovered_inventory) == external["assetSha256"]
            and validate_object_inventory(recovered_inventory)
            == external
        )
    config_match = observed_config == expected_config
    data_match = observed_data == expected_data
    now = dt.datetime.now(dt.timezone.utc)
    created = parse_timestamp(manifest["createdAt"])
    rpo = max(0, int((now - created).total_seconds()))
    rto = max(0, int(now.timestamp()) - args.started_at_epoch)
    passed = config_match and data_match and external_match
    report = {
        "schemaVersion": HOST_RESTORE_SCHEMA,
        "status": "passed" if passed else "failed",
        "completedAt": now.isoformat(timespec="seconds").replace("+00:00", "Z"),
        "source": manifest["source"],
        "backup": {
            "createdAt": manifest["createdAt"],
            "archiveSha256": manifest["archive"]["sha256"],
            "postgres": postgres_info,
        },
        "observed": {
            "rpoSeconds": rpo,
            "rtoSeconds": rto,
        },
        "invariants": {
            "configuration": {
                "matched": config_match,
                "fileCount": observed_config["fileCount"],
                "totalBytes": observed_config["totalBytes"],
            },
            "data": {
                "matched": data_match,
                "fileCount": observed_data["fileCount"],
                "totalBytes": observed_data["totalBytes"],
            },
            "postgresSet": {"verified": True},
            "externalObjects": {
                "required": external is not None,
                "matched": external_match,
            },
        },
        "retainedSecrets": "none",
    }
    atomic_write(args.output, json.dumps(report, indent=2, sort_keys=True) + "\n")
    if not passed:
        raise RecoveryError("restored host-state invariants do not match the backup")


def build_disaster_report(args: argparse.Namespace) -> None:
    validate_identity(args.source_release, args.source_commit)
    host = read_json(args.host_report, "host restore report")
    postgres = read_json(args.postgres_report, "Postgres restore report")
    expected_objects = read_json(args.expected_object_inventory, "expected object inventory")
    observed_objects = read_json(args.observed_object_inventory, "observed object inventory")
    if host.get("schemaVersion") != HOST_RESTORE_SCHEMA or host.get("status") != "passed":
        raise RecoveryError("host restore report is not a passing supported report")
    if (
        postgres.get("schemaVersion") != "bitriver.postgres-restore-report/v1"
        or postgres.get("result") != "passed"
    ):
        raise RecoveryError("Postgres restore report is not a passing supported report")
    if host.get("source") != {
        "release": args.source_release,
        "commit": args.source_commit,
    }:
        raise RecoveryError("host restore report identity does not match the disaster drill")
    host_backup = host.get("backup")
    host_postgres = host_backup.get("postgres") if isinstance(host_backup, dict) else None
    postgres_backup = postgres.get("backup")
    if not isinstance(host_postgres, dict) or not isinstance(postgres_backup, dict):
        raise RecoveryError("recovery reports do not contain Postgres backup identity")
    postgres_binding = {
        "sourceRelease": args.source_release,
        "sourceCommit": args.source_commit,
        "archive": host_postgres.get("archiveName"),
        "archiveSha256": host_postgres.get("archiveSha256"),
        "manifest": host_postgres.get("manifestName"),
        "manifestSha256": host_postgres.get("manifestSha256"),
    }
    if any(
        not isinstance(postgres_binding[key], str)
        for key in postgres_binding
    ) or any(
        not SHA256_PATTERN.fullmatch(str(postgres_binding[key]))
        for key in ("archiveSha256", "manifestSha256")
    ):
        raise RecoveryError("host restore report has invalid Postgres backup identity")
    if any(postgres_backup.get(key) != value for key, value in postgres_binding.items()):
        raise RecoveryError("Postgres restore report does not match the host recovery set")
    host_observed = host.get("observed")
    host_rpo = host_observed.get("rpoSeconds") if isinstance(host_observed, dict) else None
    postgres_rpo = postgres_backup.get("observedRpoSeconds")
    if any(
        isinstance(value, bool) or not isinstance(value, int) or value < 0
        for value in (host_rpo, postgres_rpo)
    ):
        raise RecoveryError("recovery reports contain an invalid observed RPO")
    if expected_objects.get("schemaVersion") != OBJECT_INVENTORY_SCHEMA:
        raise RecoveryError("expected object inventory schema is unsupported")
    if observed_objects != expected_objects:
        raise RecoveryError("restored external object inventory does not match the backup")
    if args.destroyed_source_root.exists():
        raise RecoveryError("source host root still exists after destructive recovery cut")
    bundle = args.bundle_root.resolve(strict=True)
    required = (
        "deploy/docker-compose.yml",
        "scripts/backup-postgres.sh",
        "scripts/restore-postgres.sh",
        "scripts/backup-host-state.sh",
        "scripts/restore-host-state.sh",
        "scripts/host_recovery.py",
    )
    if any(not (bundle / relative).is_file() for relative in required):
        raise RecoveryError("source-free recovery bundle is incomplete")
    if any((bundle / forbidden).exists() for forbidden in (".git", "cmd", "internal")):
        raise RecoveryError("recovery bundle unexpectedly contains source checkout state")
    installed = args.installed_root.resolve(strict=True)
    if any(not (installed / relative).exists() for relative in required):
        raise RecoveryError("recovered packaged-host installation is incomplete")
    now = dt.datetime.now(dt.timezone.utc)
    total_rto = max(0, int(now.timestamp()) - args.started_at_epoch)
    bundle_inventory = tree_inventory(bundle, bundle)
    report = {
        "schemaVersion": DISASTER_REPORT_SCHEMA,
        "status": "passed",
        "completedAt": now.isoformat(timespec="seconds").replace("+00:00", "Z"),
        "source": {
            "release": args.source_release,
            "commit": args.source_commit,
            "checkoutPresent": False,
        },
        "observed": {
            "rpoSeconds": max(host_rpo, postgres_rpo),
            "hostRpoSeconds": host_rpo,
            "postgresRpoSeconds": postgres_rpo,
            "rtoSeconds": total_rto,
        },
        "bundle": {
            "sourceFree": True,
            "fileCount": bundle_inventory["fileCount"],
            "fingerprintSha256": bundle_inventory["fingerprintSha256"],
        },
        "stages": [
            {"id": "encrypted-host-restore", "status": "passed"},
            {"id": "fresh-package-install", "status": "passed"},
            {"id": "postgres-fresh-database-restore", "status": "passed"},
            {"id": "external-object-invariants", "status": "passed"},
            {"id": "source-runtime-destroyed", "status": "passed"},
        ],
        "retainedSecrets": "none",
        "remainingAcceptance": [
            "exact published release-set package qualification",
            "production golden path on the recovered immutable stack",
            "production-like scheduled off-host RPO evidence",
        ],
    }
    atomic_write(args.output, json.dumps(report, indent=2, sort_keys=True) + "\n")


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)

    objects = subparsers.add_parser("object-inventory")
    objects.add_argument("--root", type=Path, required=True)
    objects.add_argument("--output", type=Path, required=True)

    preflight = subparsers.add_parser("preflight-host")
    preflight.add_argument("--root-prefix", type=Path, required=True)

    manifest = subparsers.add_parser("backup-manifest")
    manifest.add_argument("--root-prefix", type=Path, required=True)
    manifest.add_argument("--archive", type=Path, required=True)
    manifest.add_argument("--postgres-backup", type=Path, required=True)
    manifest.add_argument("--source-release", required=True)
    manifest.add_argument("--source-commit", required=True)
    manifest.add_argument("--created-at", required=True)
    manifest.add_argument("--iterations", type=int, default=200_000)
    manifest.add_argument("--object-inventory", type=Path)
    manifest.add_argument("--output", type=Path, required=True)

    verify = subparsers.add_parser("verify-backup")
    verify.add_argument("--archive", type=Path, required=True)
    verify.add_argument("--expected-release", required=True)
    verify.add_argument("--expected-commit", required=True)

    inspect = subparsers.add_parser("validate-archive")
    inspect.add_argument("--manifest", type=Path, required=True)

    report = subparsers.add_parser("restore-report")
    report.add_argument("--archive", type=Path, required=True)
    report.add_argument("--root-prefix", type=Path, required=True)
    report.add_argument("--expected-release", required=True)
    report.add_argument("--expected-commit", required=True)
    report.add_argument("--started-at-epoch", type=int, required=True)
    report.add_argument("--output", type=Path, required=True)

    disaster = subparsers.add_parser("disaster-report")
    disaster.add_argument("--host-report", type=Path, required=True)
    disaster.add_argument("--postgres-report", type=Path, required=True)
    disaster.add_argument("--expected-object-inventory", type=Path, required=True)
    disaster.add_argument("--observed-object-inventory", type=Path, required=True)
    disaster.add_argument("--bundle-root", type=Path, required=True)
    disaster.add_argument("--installed-root", type=Path, required=True)
    disaster.add_argument("--destroyed-source-root", type=Path, required=True)
    disaster.add_argument("--source-release", required=True)
    disaster.add_argument("--source-commit", required=True)
    disaster.add_argument("--started-at-epoch", type=int, required=True)
    disaster.add_argument("--output", type=Path, required=True)
    return parser


def main(argv: Sequence[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        if args.command == "object-inventory":
            atomic_write(
                args.output,
                json.dumps(build_object_inventory(args.root), indent=2, sort_keys=True)
                + "\n",
            )
        elif args.command == "preflight-host":
            preflight_host_state(args.root_prefix)
        elif args.command == "backup-manifest":
            if args.iterations < 100_000:
                raise RecoveryError("PBKDF2 iteration count must be at least 100000")
            build_backup_manifest(args)
        elif args.command == "verify-backup":
            manifest = verify_backup_set(
                args.archive,
                expected_release=args.expected_release,
                expected_commit=args.expected_commit,
            )
            print(manifest["encryption"]["iterations"])
        elif args.command == "validate-archive":
            manifest = read_json(args.manifest, "host backup manifest")
            if manifest.get("schemaVersion") != HOST_BACKUP_SCHEMA:
                raise RecoveryError("unsupported host backup manifest schema")
            validate_archive_stream(manifest, sys.stdin.buffer)
        elif args.command == "restore-report":
            build_restore_report(args)
        elif args.command == "disaster-report":
            build_disaster_report(args)
        else:
            raise RecoveryError("unsupported recovery command")
    except (OSError, RecoveryError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
