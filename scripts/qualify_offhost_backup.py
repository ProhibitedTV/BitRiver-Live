#!/usr/bin/env python3
"""Qualify scheduled S3-compatible PostgreSQL backup evidence for #1299.

This is intentionally a read-only verifier. It consumes the three-file backup
sets produced by scripts/backup-postgres.sh, validates the newest complete set,
and emits secret-safe release evidence. It never creates, deletes, or mutates
remote objects.
"""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import os
import re
import subprocess
import sys
import tempfile
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Iterable, Mapping, Protocol

REPORT_SCHEMA = "bitriver.offhost-backup-proof/v1"
BACKUP_SCHEMA = "bitriver.postgres-backup/v1"
BACKUP_RE = re.compile(
    r"^(?P<stem>bitriver-postgres-(?P<stamp>\d{8}T\d{6}Z)\.sql\.gz)"
    r"(?P<suffix>|\.manifest\.json|\.sha256)$"
)
RELEASE_RE = re.compile(r"^v(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?$")
COMMIT_RE = re.compile(r"^[0-9a-f]{40}$")
SHA256_RE = re.compile(r"^[0-9a-f]{64}$")


class QualificationError(ValueError):
    """Raised when remote backup evidence violates the qualification contract."""


class ObjectClient(Protocol):
    def list_objects(self, bucket: str, prefix: str) -> list[dict[str, Any]]: ...

    def download(self, bucket: str, key: str, destination: Path) -> None: ...


@dataclass(frozen=True)
class BackupSet:
    archive_key: str
    manifest_key: str
    checksum_key: str
    timestamp: dt.datetime
    archive_size: int


def utc_now() -> dt.datetime:
    return dt.datetime.now(dt.timezone.utc)


def parse_utc(value: str, *, label: str) -> dt.datetime:
    try:
        parsed = dt.datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError as exc:
        raise QualificationError(f"{label} is not valid RFC3339 UTC: {value!r}") from exc
    if parsed.tzinfo is None:
        raise QualificationError(f"{label} must include a UTC timezone")
    return parsed.astimezone(dt.timezone.utc)


def parse_stamp(value: str) -> dt.datetime:
    try:
        parsed = dt.datetime.strptime(value, "%Y%m%dT%H%M%SZ")
    except ValueError as exc:
        raise QualificationError(f"backup filename timestamp is invalid: {value!r}") from exc
    return parsed.replace(tzinfo=dt.timezone.utc)


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def redact_command(command: Iterable[str]) -> list[str]:
    # No credentials are ever passed as command-line arguments. Keep this
    # helper defensive so a future option cannot accidentally enter errors.
    sensitive = ("access-key", "secret", "token", "password", "credential")
    redacted: list[str] = []
    hide_next = False
    for raw in command:
        if hide_next:
            redacted.append("<redacted>")
            hide_next = False
            continue
        lowered = raw.lower()
        if raw.startswith("--") and any(word in lowered for word in sensitive):
            redacted.append(raw)
            hide_next = True
        else:
            redacted.append(raw)
    return redacted


class AWSCLIClient:
    """Minimal read-only S3 client backed by the operator's configured AWS CLI."""

    def __init__(self, *, region: str, endpoint_url: str | None = None) -> None:
        self.region = region
        self.endpoint_url = endpoint_url

    def _base_command(self) -> list[str]:
        command = ["aws"]
        if self.region:
            command += ["--region", self.region]
        if self.endpoint_url:
            command += ["--endpoint-url", self.endpoint_url]
        return command

    def _run(self, args: list[str]) -> subprocess.CompletedProcess[str]:
        command = self._base_command() + args
        try:
            return subprocess.run(
                command,
                check=True,
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
            )
        except FileNotFoundError as exc:
            raise QualificationError("required command not found: aws") from exc
        except subprocess.CalledProcessError as exc:
            safe = " ".join(redact_command(command))
            message = (exc.stderr or "AWS CLI request failed").strip().splitlines()[-1]
            raise QualificationError(f"AWS CLI request failed ({safe}): {message}") from exc

    def list_objects(self, bucket: str, prefix: str) -> list[dict[str, Any]]:
        result = self._run(
            [
                "s3api",
                "list-objects-v2",
                "--bucket",
                bucket,
                "--prefix",
                prefix,
                "--output",
                "json",
            ]
        )
        try:
            payload = json.loads(result.stdout or "{}")
        except json.JSONDecodeError as exc:
            raise QualificationError("AWS CLI returned invalid JSON while listing backups") from exc
        contents = payload.get("Contents", [])
        if not isinstance(contents, list):
            raise QualificationError("AWS CLI backup listing has an invalid Contents field")
        return contents

    def download(self, bucket: str, key: str, destination: Path) -> None:
        self._run(["s3", "cp", f"s3://{bucket}/{key}", str(destination), "--only-show-errors"])


def _relative_key(key: str, prefix: str) -> str:
    normalized = prefix.strip("/")
    if not normalized:
        return key
    marker = normalized + "/"
    if key.startswith(marker):
        return key[len(marker) :]
    if key == normalized:
        return ""
    return key


def discover_sets(objects: Iterable[Mapping[str, Any]], prefix: str) -> list[BackupSet]:
    grouped: dict[str, dict[str, Any]] = {}
    matched_keys: set[str] = set()

    for item in objects:
        key = item.get("Key")
        if not isinstance(key, str):
            continue
        relative = _relative_key(key, prefix)
        match = BACKUP_RE.match(relative)
        if not match:
            continue
        matched_keys.add(key)
        stem = match.group("stem")
        suffix = match.group("suffix")
        group = grouped.setdefault(stem, {"stamp": match.group("stamp")})
        label = {
            "": "archive",
            ".manifest.json": "manifest",
            ".sha256": "checksum",
        }[suffix]
        if label in group:
            raise QualificationError(f"duplicate remote {label} object for backup set {stem}")
        group[label] = dict(item)

    if not grouped:
        raise QualificationError("no BitRiver PostgreSQL backup objects found under the requested prefix")

    incomplete: list[str] = []
    complete: list[BackupSet] = []
    normalized_prefix = prefix.strip("/")
    prefix_part = f"{normalized_prefix}/" if normalized_prefix else ""

    for stem, group in sorted(grouped.items()):
        missing = [name for name in ("archive", "manifest", "checksum") if name not in group]
        if missing:
            incomplete.append(f"{stem}: missing {', '.join(missing)}")
            continue
        archive = group["archive"]
        archive_size = archive.get("Size")
        if not isinstance(archive_size, int) or archive_size <= 0:
            raise QualificationError(f"remote archive has an invalid size: {stem}")
        complete.append(
            BackupSet(
                archive_key=prefix_part + stem,
                manifest_key=prefix_part + stem + ".manifest.json",
                checksum_key=prefix_part + stem + ".sha256",
                timestamp=parse_stamp(group["stamp"]),
                archive_size=archive_size,
            )
        )

    if incomplete:
        raise QualificationError("incomplete remote backup set(s): " + "; ".join(incomplete))
    if not complete:
        raise QualificationError("no complete BitRiver PostgreSQL backup sets found")
    return sorted(complete, key=lambda item: item.timestamp)


def parse_checksum_file(path: Path, archive_name: str, manifest_name: str) -> dict[str, str]:
    expected_names = {archive_name, manifest_name}
    parsed: dict[str, str] = {}
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line:
            continue
        parts = line.split(None, 1)
        if len(parts) != 2:
            raise QualificationError("remote checksum file contains a malformed line")
        digest, name = parts
        name = name.lstrip("*")
        if not SHA256_RE.fullmatch(digest):
            raise QualificationError("remote checksum file contains an invalid SHA-256 digest")
        if name not in expected_names:
            raise QualificationError(f"remote checksum file references unexpected asset: {name}")
        if name in parsed:
            raise QualificationError(f"remote checksum file contains duplicate asset: {name}")
        parsed[name] = digest
    if set(parsed) != expected_names:
        missing = sorted(expected_names - set(parsed))
        raise QualificationError("remote checksum file does not cover the complete backup set: " + ", ".join(missing))
    return parsed


def validate_manifest(
    manifest: Mapping[str, Any],
    *,
    backup: BackupSet,
    archive_sha: str,
    expected_release: str | None,
    expected_commit: str | None,
) -> tuple[dt.datetime, str, str]:
    if manifest.get("schemaVersion") != BACKUP_SCHEMA:
        raise QualificationError("remote backup manifest has an unsupported schemaVersion")

    created_raw = manifest.get("createdAt")
    if not isinstance(created_raw, str):
        raise QualificationError("remote backup manifest is missing createdAt")
    created_at = parse_utc(created_raw, label="backup manifest createdAt")
    if abs((created_at - backup.timestamp).total_seconds()) > 300:
        raise QualificationError("backup manifest createdAt differs from the filename timestamp by more than 5 minutes")

    source = manifest.get("source")
    if not isinstance(source, Mapping):
        raise QualificationError("remote backup manifest is missing source identity")
    release = source.get("release")
    commit = source.get("commit")
    if not isinstance(release, str) or not RELEASE_RE.fullmatch(release):
        raise QualificationError("remote backup manifest source release is not an exact v-prefixed release")
    if not isinstance(commit, str) or not COMMIT_RE.fullmatch(commit):
        raise QualificationError("remote backup manifest source commit is not a full lowercase SHA")
    if expected_release and release != expected_release:
        raise QualificationError(f"remote backup source release mismatch: expected {expected_release}, got {release}")
    if expected_commit and commit != expected_commit:
        raise QualificationError(f"remote backup source commit mismatch: expected {expected_commit}, got {commit}")

    archive = manifest.get("archive")
    if not isinstance(archive, Mapping):
        raise QualificationError("remote backup manifest is missing archive metadata")
    archive_name = Path(backup.archive_key).name
    if archive.get("name") != archive_name:
        raise QualificationError("remote backup manifest archive name does not match the remote object")
    if archive.get("sha256") != archive_sha:
        raise QualificationError("remote backup manifest archive SHA-256 does not match downloaded bytes")
    if archive.get("sizeBytes") != backup.archive_size:
        raise QualificationError("remote backup manifest archive size does not match remote object metadata")
    if archive.get("checksumAsset") != Path(backup.checksum_key).name:
        raise QualificationError("remote backup manifest checksum asset does not match the remote object")

    database = manifest.get("database")
    if not isinstance(database, Mapping):
        raise QualificationError("remote backup manifest is missing database evidence")
    fingerprint = database.get("migrationFingerprintSha256")
    if not isinstance(fingerprint, str) or not SHA256_RE.fullmatch(fingerprint):
        raise QualificationError("remote backup manifest has an invalid migration fingerprint")
    row_counts = database.get("rowCounts")
    if not isinstance(row_counts, Mapping) or not row_counts:
        raise QualificationError("remote backup manifest does not contain non-empty row-count evidence")

    return created_at, release, commit


def qualify(
    client: ObjectClient,
    *,
    bucket: str,
    prefix: str,
    max_age_seconds: int,
    min_complete_sets: int,
    expected_release: str | None = None,
    expected_commit: str | None = None,
    now: dt.datetime | None = None,
) -> dict[str, Any]:
    if not bucket.strip():
        raise QualificationError("bucket must not be empty")
    if max_age_seconds <= 0:
        raise QualificationError("max age must be greater than zero")
    if min_complete_sets <= 0:
        raise QualificationError("minimum complete sets must be greater than zero")
    if expected_release and not RELEASE_RE.fullmatch(expected_release):
        raise QualificationError("expected release must be an exact v-prefixed release")
    if expected_commit and not COMMIT_RE.fullmatch(expected_commit):
        raise QualificationError("expected commit must be a full lowercase SHA")

    current = now or utc_now()
    if current.tzinfo is None:
        raise QualificationError("qualification time must include a timezone")
    current = current.astimezone(dt.timezone.utc)

    complete_sets = discover_sets(client.list_objects(bucket, prefix), prefix)
    if len(complete_sets) < min_complete_sets:
        raise QualificationError(
            f"remote retention has only {len(complete_sets)} complete set(s); "
            f"at least {min_complete_sets} required"
        )

    newest = complete_sets[-1]
    with tempfile.TemporaryDirectory(prefix="bitriver-offhost-proof-") as temp_dir:
        root = Path(temp_dir)
        archive_path = root / Path(newest.archive_key).name
        manifest_path = root / Path(newest.manifest_key).name
        checksum_path = root / Path(newest.checksum_key).name
        client.download(bucket, newest.archive_key, archive_path)
        client.download(bucket, newest.manifest_key, manifest_path)
        client.download(bucket, newest.checksum_key, checksum_path)

        if archive_path.stat().st_size != newest.archive_size:
            raise QualificationError("downloaded archive size does not match remote object metadata")
        archive_sha = sha256_file(archive_path)
        manifest_sha = sha256_file(manifest_path)
        checksums = parse_checksum_file(
            checksum_path,
            archive_path.name,
            manifest_path.name,
        )
        if checksums[archive_path.name] != archive_sha:
            raise QualificationError("remote checksum does not match downloaded backup archive")
        if checksums[manifest_path.name] != manifest_sha:
            raise QualificationError("remote checksum does not match downloaded backup manifest")

        try:
            manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
        except json.JSONDecodeError as exc:
            raise QualificationError("remote backup manifest is not valid JSON") from exc
        if not isinstance(manifest, Mapping):
            raise QualificationError("remote backup manifest must be a JSON object")
        created_at, release, commit = validate_manifest(
            manifest,
            backup=newest,
            archive_sha=archive_sha,
            expected_release=expected_release,
            expected_commit=expected_commit,
        )

    age_seconds = int((current - created_at).total_seconds())
    if age_seconds < 0:
        raise QualificationError("newest remote backup is dated in the future")
    if age_seconds > max_age_seconds:
        raise QualificationError(
            f"newest remote backup is stale: age {age_seconds}s exceeds RPO target {max_age_seconds}s"
        )

    oldest = complete_sets[0]
    return {
        "schemaVersion": REPORT_SCHEMA,
        "status": "pass",
        "observedAt": current.isoformat().replace("+00:00", "Z"),
        "policy": {
            "maxAgeSeconds": max_age_seconds,
            "minCompleteSets": min_complete_sets,
        },
        "remote": {
            "provider": "s3-compatible",
            "bucket": bucket,
            "prefix": prefix.strip("/"),
            "completeSetCount": len(complete_sets),
            "oldestSetTimestamp": oldest.timestamp.isoformat().replace("+00:00", "Z"),
            "newestSetTimestamp": newest.timestamp.isoformat().replace("+00:00", "Z"),
        },
        "latest": {
            "archiveKey": newest.archive_key,
            "archiveSizeBytes": newest.archive_size,
            "archiveSha256": archive_sha,
            "manifestSha256": manifest_sha,
            "createdAt": created_at.isoformat().replace("+00:00", "Z"),
            "source": {"release": release, "commit": commit},
            "observedRpoSeconds": age_seconds,
            "checksumVerified": True,
            "manifestVerified": True,
        },
        "claims": {
            "scheduledRemoteFreshness": True,
            "completeRemoteSet": True,
            "remoteRetentionMinimum": True,
            "localCopyRequired": False,
            "restoreProvenByThisReport": False,
            "providerDurabilitySlaProven": False,
        },
    }


def write_report(report: Mapping[str, Any], output: Path) -> None:
    output.parent.mkdir(parents=True, exist_ok=True)
    temporary = output.with_name(output.name + f".partial.{os.getpid()}")
    try:
        temporary.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
        temporary.replace(output)
    finally:
        try:
            temporary.unlink()
        except FileNotFoundError:
            pass


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Verify fresh, complete, retained BitRiver PostgreSQL backup sets in S3-compatible storage."
    )
    parser.add_argument("--bucket", required=True)
    parser.add_argument("--prefix", default="bitriver-live/postgres")
    parser.add_argument("--region", default="us-east-1")
    parser.add_argument("--endpoint-url")
    parser.add_argument("--max-age-seconds", type=int, default=86400)
    parser.add_argument("--min-complete-sets", type=int, default=3)
    parser.add_argument("--expected-release")
    parser.add_argument("--expected-commit")
    parser.add_argument("--now", help="RFC3339 UTC override for deterministic qualification/testing")
    parser.add_argument("--report", type=Path, required=True)
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        now = parse_utc(args.now, label="--now") if args.now else None
        report = qualify(
            AWSCLIClient(region=args.region, endpoint_url=args.endpoint_url),
            bucket=args.bucket,
            prefix=args.prefix,
            max_age_seconds=args.max_age_seconds,
            min_complete_sets=args.min_complete_sets,
            expected_release=args.expected_release,
            expected_commit=args.expected_commit,
            now=now,
        )
        write_report(report, args.report)
    except (QualificationError, OSError) as exc:
        print(f"error: off-host backup qualification failed: {exc}", file=sys.stderr)
        return 1
    print(f"off-host backup qualification passed: {args.report}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
