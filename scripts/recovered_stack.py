#!/usr/bin/env python3
"""Prepare and bind exact-candidate recovered-stack golden-path evidence."""

from __future__ import annotations

import argparse
import base64
import copy
import datetime as dt
import hashlib
import json
import re
import sys
import urllib.parse
from pathlib import Path
from typing import Any, Callable, Mapping, Sequence

SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

import host_recovery
import prepare_release_candidate as candidate
import stateful_compose_upgrade as upgrade


INPUT_SCHEMA = "bitriver.recovered-stack-input/v1"
GOLDEN_SCHEMA = "bitriver.production-golden-path/v1"
DISASTER_SCHEMA = "bitriver.disaster-recovery/v1"
GOLDEN_ACCEPTANCE = "production golden path on the recovered immutable stack"
SCHEDULED_ACCEPTANCE = "production-like scheduled off-host RPO evidence"
SHA256_PATTERN = re.compile(r"[0-9a-f]{64}")
GOLDEN_STAGES = {
    "surface-preflight",
    "accounts-and-channel",
    "rtmp-publish-and-live-state",
    "live-media-content",
    "offline-transition",
    "chat-and-moderation",
    "vod-upload-publish-playback",
    "final-status",
}
POSTGRES_BINDING_KEYS = (
    "sourceRelease",
    "sourceCommit",
    "archive",
    "archiveSha256",
    "manifest",
    "manifestSha256",
)


class RecoveredStackError(ValueError):
    """Raised when recovered-stack evidence is incomplete or unrelated."""


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
        raise RecoveredStackError(f"cannot read {label}: {exc}") from exc
    if not isinstance(payload, dict):
        raise RecoveredStackError(f"{label} must be a JSON object")
    return payload


def expected_service_images(
    release: Mapping[str, object],
    *,
    release_tag: str,
    namespace: str,
    template_values: Mapping[str, str],
) -> dict[str, str]:
    image_values = release.get("imageValues")
    dependency_values = release.get("dependencyValues")
    if not isinstance(image_values, dict) or not isinstance(dependency_values, dict):
        raise RecoveredStackError("release image evidence is incomplete")

    first_party_services = {
        "bitriver-live": ("BITRIVER_LIVE_IMAGE_DIGEST", "bitriver-live"),
        "viewer": ("BITRIVER_VIEWER_IMAGE_DIGEST", "bitriver-viewer"),
        "srs-controller": (
            "BITRIVER_SRS_CONTROLLER_IMAGE_DIGEST",
            "bitriver-srs-controller",
        ),
        "transcoder": ("BITRIVER_TRANSCODER_IMAGE_DIGEST", "bitriver-transcoder"),
        "ome-config": ("BITRIVER_OME_CONFIG_IMAGE_DIGEST", "bitriver-ome-config"),
        "ome-health-token-check": (
            "BITRIVER_OME_CONFIG_IMAGE_DIGEST",
            "bitriver-ome-config",
        ),
    }
    expected = {
        service: f"{namespace}/{name}:{release_tag}@{image_values[key]}"
        for service, (key, name) in first_party_services.items()
    }

    dependency_references = {
        key: template.format_map(template_values)
        for key, template in candidate.THIRD_PARTY_IMAGES
    }
    dependency_services = {
        "postgres": "BITRIVER_POSTGRES_IMAGE_DIGEST",
        "postgres-migrations": "BITRIVER_POSTGRES_IMAGE_DIGEST",
        "redis": "BITRIVER_REDIS_IMAGE_DIGEST",
        "srs": "BITRIVER_SRS_IMAGE_DIGEST",
        "srs-config": "BITRIVER_DEBIAN_IMAGE_DIGEST",
        "ome": "BITRIVER_OME_IMAGE_DIGEST",
        "transcoder-public": "BITRIVER_NGINX_IMAGE_DIGEST",
    }
    for service, key in dependency_services.items():
        reference = dependency_references.get(key)
        digest = dependency_values.get(key)
        if not isinstance(reference, str) or not isinstance(digest, str):
            raise RecoveredStackError(f"release dependency evidence is missing {key}")
        expected[service] = f"{reference}@{digest}"
    return dict(sorted(expected.items()))


def prepare_environment(
    *,
    release_set_path: Path,
    template_path: Path,
    expected_release: str,
    expected_commit: str,
    expected_release_set_sha256: str,
    namespace: str,
    config_root: str,
    host_uid: str,
    host_gid: str,
    bootstrap_database: str,
    restored_database: str,
    output_path: Path,
    sentinel_path: Path,
    metadata_path: Path,
    secret_factory: Callable[[str], str] = candidate.default_secret_factory,
) -> None:
    host_recovery.validate_identity(expected_release, expected_commit)
    if not SHA256_PATTERN.fullmatch(expected_release_set_sha256):
        raise RecoveredStackError("release-set SHA-256 must be 64 lowercase hex characters")
    if not re.fullmatch(r"[a-z][a-z0-9_]{2,62}", bootstrap_database):
        raise RecoveredStackError("bootstrap database name is unsafe")
    if not re.fullmatch(r"[a-z][a-z0-9_]{2,62}", restored_database):
        raise RecoveredStackError("restored database name is unsafe")
    if bootstrap_database == restored_database:
        raise RecoveredStackError("bootstrap and restored database names must differ")
    if not config_root or any(character in config_root for character in "\r\n\0"):
        raise RecoveredStackError("configuration root is invalid")
    if not host_uid.isdigit() or not host_gid.isdigit():
        raise RecoveredStackError("host UID and GID must be numeric")

    template_content = template_path.read_text(encoding="utf-8")
    _, template_values = candidate.parse_env_template(template_content)
    release = upgrade.read_release_set(
        release_set_path,
        expected_sha256=expected_release_set_sha256,
        expected_tag=expected_release,
        expected_commit=expected_commit,
        namespace=namespace,
        template_values=template_values,
    )
    release_set = read_json(release_set_path, "release-set manifest")
    identity = release_set.get("candidate")
    if not isinstance(identity, dict) or identity.get("repository") != "ProhibitedTV/BitRiver-Live":
        raise RecoveredStackError("release-set repository identity is unsupported")

    rendered, sentinels = candidate.prepare_environment(
        template_content,
        candidate.parse_tag(expected_release),
        namespace,
        resolve_digests=False,
        first_party_digests=release["imageValues"],  # type: ignore[arg-type]
        third_party_digests=release["dependencyValues"],  # type: ignore[arg-type]
        product_loopback=True,
        secret_factory=secret_factory,
    )
    recovered_environment = upgrade.replace_env_values(
        rendered,
        {
            "BITRIVER_CONFIG_ROOT": config_root,
            "BITRIVER_HOST_UID": host_uid,
            "BITRIVER_HOST_GID": host_gid,
            "BITRIVER_RELEASE_COMMIT": expected_commit,
            "BITRIVER_LIVE_PORT": "18080",
            "BITRIVER_POSTGRES_DB": bootstrap_database,
            "BITRIVER_TRANSCODE_LADDER": "1080p:2500",
        },
    )
    runtime_environment = upgrade.replace_env_values(
        recovered_environment,
        {"BITRIVER_POSTGRES_DB": restored_database},
    )
    candidate.atomic_private_write(output_path, recovered_environment)
    candidate.atomic_private_write(
        sentinel_path,
        "".join(f"{value}\n" for value in sorted(set(sentinels))),
    )
    metadata = {
        "schemaVersion": INPUT_SCHEMA,
        "source": {
            "release": expected_release,
            "commit": expected_commit,
            "releaseSetSha256": expected_release_set_sha256,
        },
        "environment": {
            "configRoot": config_root,
            "hostUid": host_uid,
            "hostGid": host_gid,
            "bootstrapDatabase": bootstrap_database,
            "restoredDatabase": restored_database,
            "recoveredSha256": hashlib.sha256(recovered_environment.encode()).hexdigest(),
            "runtimeSha256": hashlib.sha256(runtime_environment.encode()).hexdigest(),
            "templateSha256": sha256_file(template_path),
        },
        "expectedServiceImages": expected_service_images(
            release,
            release_tag=expected_release,
            namespace=namespace,
            template_values=template_values,
        ),
    }
    candidate.atomic_private_write(
        metadata_path,
        json.dumps(metadata, indent=2, sort_keys=True) + "\n",
    )


def activate_restored_database(
    environment_path: Path,
    metadata_path: Path,
    output_path: Path,
) -> None:
    metadata = read_json(metadata_path, "recovered-stack input metadata")
    if metadata.get("schemaVersion") != INPUT_SCHEMA:
        raise RecoveredStackError("recovered-stack input schema is unsupported")
    environment = metadata.get("environment")
    if not isinstance(environment, dict):
        raise RecoveredStackError("recovered-stack environment metadata is missing")
    recovered_sha = environment.get("recoveredSha256")
    runtime_sha = environment.get("runtimeSha256")
    database = environment.get("restoredDatabase")
    if sha256_file(environment_path) != recovered_sha:
        raise RecoveredStackError("recovered environment bytes do not match the prepared input")
    if not isinstance(database, str):
        raise RecoveredStackError("restored database identity is missing")
    content = environment_path.read_text(encoding="utf-8")
    activated = upgrade.replace_env_values(content, {"BITRIVER_POSTGRES_DB": database})
    if hashlib.sha256(activated.encode()).hexdigest() != runtime_sha:
        raise RecoveredStackError("activated runtime environment hash is inconsistent")
    candidate.atomic_private_write(output_path, activated)


def record_observed_images(
    metadata_path: Path,
    observations_path: Path,
    output_path: Path,
) -> None:
    metadata = read_json(metadata_path, "recovered-stack input metadata")
    expected = metadata.get("expectedServiceImages")
    if not isinstance(expected, dict):
        raise RecoveredStackError("expected service-image metadata is missing")
    observed: dict[str, str] = {}
    for line_number, raw_line in enumerate(
        observations_path.read_text(encoding="utf-8").splitlines(), start=1
    ):
        if not raw_line:
            continue
        fields = raw_line.split("\t")
        if len(fields) != 2 or not all(fields):
            raise RecoveredStackError(
                f"observed image line {line_number} must be service-tab-reference"
            )
        service, reference = fields
        if service in observed:
            raise RecoveredStackError(f"duplicate observed image service: {service}")
        observed[service] = reference
    if observed != expected:
        raise RecoveredStackError("observed recovered-stack images do not match the release set")
    host_recovery.atomic_write(
        output_path, json.dumps(observed, indent=2, sort_keys=True) + "\n"
    )


def secret_encodings(value: str) -> set[str]:
    encoded = value.encode()
    return {
        value,
        urllib.parse.quote(value, safe=""),
        base64.b64encode(encoded).decode(),
        base64.urlsafe_b64encode(encoded).decode(),
        encoded.hex(),
    }


def refuse_retained_secrets(paths: Sequence[Path], sentinel_path: Path) -> None:
    sentinels = [
        line.strip()
        for line in sentinel_path.read_text(encoding="utf-8").splitlines()
        if line.strip()
    ]
    for path in paths:
        content = path.read_text(encoding="utf-8", errors="replace")
        for sentinel in sentinels:
            if any(candidate_value in content for candidate_value in secret_encodings(sentinel)):
                raise RecoveredStackError(f"retained evidence contains a private sentinel: {path.name}")


def validate_golden_report(payload: dict[str, Any]) -> int:
    if payload.get("schema") != GOLDEN_SCHEMA or payload.get("status") != "passed":
        raise RecoveredStackError("production golden-path report is not a passing supported report")
    stages = payload.get("stages")
    if not isinstance(stages, list):
        raise RecoveredStackError("production golden-path stages are missing")
    observed: set[str] = set()
    for stage in stages:
        if not isinstance(stage, dict):
            raise RecoveredStackError("production golden-path stage is invalid")
        name = stage.get("name")
        if name in observed or name not in GOLDEN_STAGES or stage.get("status") != "passed":
            raise RecoveredStackError("production golden-path stages are incomplete or failed")
        observed.add(str(name))
    if observed != GOLDEN_STAGES:
        raise RecoveredStackError("production golden-path stages are incomplete or failed")
    duration = payload.get("durationMs")
    if isinstance(duration, bool) or not isinstance(duration, int) or duration < 1:
        raise RecoveredStackError("production golden-path duration is invalid")
    return duration


def complete_disaster_report(
    *,
    metadata_path: Path,
    disaster_report_path: Path,
    original_postgres_report_path: Path,
    runtime_postgres_report_path: Path,
    golden_report_path: Path,
    observed_images_path: Path,
    recovered_environment_path: Path,
    runtime_environment_path: Path,
    sentinel_path: Path,
    expected_release: str,
    expected_commit: str,
    pre_users: int,
    post_users: int,
    recovered_fixture_count: int,
    total_rto_seconds: int,
    output_path: Path,
) -> None:
    host_recovery.validate_identity(expected_release, expected_commit)
    for value, label in (
        (pre_users, "pre-golden user count"),
        (post_users, "post-golden user count"),
        (recovered_fixture_count, "recovered fixture count"),
        (total_rto_seconds, "total RTO"),
    ):
        if isinstance(value, bool) or not isinstance(value, int) or value < 1:
            raise RecoveredStackError(f"{label} must be a positive integer")
    if post_users <= pre_users:
        raise RecoveredStackError("golden path did not add persistent recovered-stack state")

    refuse_retained_secrets(
        (
            metadata_path,
            disaster_report_path,
            original_postgres_report_path,
            runtime_postgres_report_path,
            golden_report_path,
            observed_images_path,
        ),
        sentinel_path,
    )
    metadata = read_json(metadata_path, "recovered-stack input metadata")
    disaster = read_json(disaster_report_path, "disaster recovery report")
    original_postgres = read_json(
        original_postgres_report_path, "original Postgres restore report"
    )
    runtime_postgres = read_json(
        runtime_postgres_report_path, "runtime Postgres restore report"
    )
    golden = read_json(golden_report_path, "production golden-path report")
    observed_images = read_json(observed_images_path, "observed runtime image report")

    if metadata.get("schemaVersion") != INPUT_SCHEMA:
        raise RecoveredStackError("recovered-stack input schema is unsupported")
    source = metadata.get("source")
    expected_source = {"release": expected_release, "commit": expected_commit}
    if not isinstance(source, dict) or any(source.get(key) != value for key, value in expected_source.items()):
        raise RecoveredStackError("recovered-stack input identity does not match the completion")
    environment = metadata.get("environment")
    if not isinstance(environment, dict):
        raise RecoveredStackError("recovered-stack environment metadata is missing")
    if sha256_file(recovered_environment_path) != environment.get("recoveredSha256"):
        raise RecoveredStackError("recovered environment did not survive lost-host restore byte-for-byte")
    if sha256_file(runtime_environment_path) != environment.get("runtimeSha256"):
        raise RecoveredStackError("active recovered environment is not the prepared database switch")

    if disaster.get("schemaVersion") != DISASTER_SCHEMA or disaster.get("status") != "passed":
        raise RecoveredStackError("disaster recovery report is not a passing supported report")
    if disaster.get("source") != {**expected_source, "checkoutPresent": False}:
        raise RecoveredStackError("disaster recovery report identity does not match")
    published = disaster.get("publishedPackage")
    if not isinstance(published, dict) or published.get("verified") is not True:
        raise RecoveredStackError("disaster recovery report is not bound to a published package")
    if published.get("releaseSetSha256") != source.get("releaseSetSha256"):
        raise RecoveredStackError("disaster and runtime release-set identities do not match")
    for postgres_report in (original_postgres, runtime_postgres):
        if (
            postgres_report.get("schemaVersion")
            != "bitriver.postgres-restore-report/v1"
            or postgres_report.get("result") != "passed"
        ):
            raise RecoveredStackError("Postgres runtime restore evidence is not passing")
    original_backup = original_postgres.get("backup")
    runtime_backup = runtime_postgres.get("backup")
    if not isinstance(original_backup, dict) or not isinstance(runtime_backup, dict):
        raise RecoveredStackError("Postgres runtime restore evidence has no backup identity")
    postgres_binding = {key: original_backup.get(key) for key in POSTGRES_BINDING_KEYS}
    if any(runtime_backup.get(key) != value for key, value in postgres_binding.items()):
        raise RecoveredStackError("runtime Postgres restore is not bound to the recovered backup")
    if any(
        original_backup.get(key) != value
        for key, value in (
            ("sourceRelease", expected_release),
            ("sourceCommit", expected_commit),
        )
    ):
        raise RecoveredStackError("runtime Postgres restore identity does not match")
    if disaster.get("recoveredPostgres") != postgres_binding:
        raise RecoveredStackError(
            "aggregate disaster report is not bound to the recovered Postgres backup"
        )
    remaining = disaster.get("remainingAcceptance")
    if not isinstance(remaining, list) or set(remaining) != {GOLDEN_ACCEPTANCE, SCHEDULED_ACCEPTANCE}:
        raise RecoveredStackError("disaster recovery remaining acceptance is unexpected")

    golden_duration_ms = validate_golden_report(golden)
    expected_images = metadata.get("expectedServiceImages")
    if not isinstance(expected_images, dict) or observed_images != expected_images:
        raise RecoveredStackError("observed recovered-stack images do not match the release set")

    original_rto = disaster.get("observed", {}).get("rtoSeconds")
    if isinstance(original_rto, bool) or not isinstance(original_rto, int) or original_rto < 0:
        raise RecoveredStackError("disaster recovery restore RTO is invalid")
    if total_rto_seconds < original_rto:
        raise RecoveredStackError("total recovered-stack RTO cannot be shorter than restore RTO")

    report = copy.deepcopy(disaster)
    report["completedAt"] = (
        dt.datetime.now(dt.timezone.utc).isoformat(timespec="seconds").replace("+00:00", "Z")
    )
    report["observed"]["restoreOnlyRtoSeconds"] = original_rto
    report["observed"]["rtoSeconds"] = total_rto_seconds
    report["observed"]["goldenPathSeconds"] = (golden_duration_ms + 999) // 1000
    report["recoveredRuntime"] = {
        "verified": True,
        "inputSha256": sha256_file(metadata_path),
        "originalDisasterReportSha256": sha256_file(disaster_report_path),
        "runtimePostgresRestoreReportSha256": sha256_file(runtime_postgres_report_path),
        "postgresBackup": postgres_binding,
        "goldenPathReportSha256": sha256_file(golden_report_path),
        "environment": {
            "recoveredSha256": environment["recoveredSha256"],
            "runtimeSha256": environment["runtimeSha256"],
            "database": environment["restoredDatabase"],
        },
        "images": observed_images,
        "state": {
            "preGoldenUsers": pre_users,
            "postGoldenUsers": post_users,
            "recoveredFixtureCount": recovered_fixture_count,
            "preExistingStatePreserved": True,
            "goldenPathStatePersisted": True,
        },
    }
    report["stages"].append(
        {"id": "recovered-immutable-stack-production-golden-path", "status": "passed"}
    )
    report["remainingAcceptance"] = [SCHEDULED_ACCEPTANCE]
    host_recovery.atomic_write(output_path, json.dumps(report, indent=2, sort_keys=True) + "\n")
    refuse_retained_secrets((output_path,), sentinel_path)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)

    prepare = subparsers.add_parser("prepare-environment")
    prepare.add_argument("--release-set", type=Path, required=True)
    prepare.add_argument("--template", type=Path, required=True)
    prepare.add_argument("--expected-release", required=True)
    prepare.add_argument("--expected-commit", required=True)
    prepare.add_argument("--expected-release-set-sha256", required=True)
    prepare.add_argument("--namespace", default="ghcr.io/prohibitedtv")
    prepare.add_argument("--config-root", required=True)
    prepare.add_argument("--host-uid", required=True)
    prepare.add_argument("--host-gid", required=True)
    prepare.add_argument("--bootstrap-database", default="bitr_bootstrap")
    prepare.add_argument("--restored-database", default="bitr_recovered")
    prepare.add_argument("--output", type=Path, required=True)
    prepare.add_argument("--sentinel-output", type=Path, required=True)
    prepare.add_argument("--metadata-output", type=Path, required=True)

    activate = subparsers.add_parser("activate-restored-database")
    activate.add_argument("--environment", type=Path, required=True)
    activate.add_argument("--metadata", type=Path, required=True)
    activate.add_argument("--output", type=Path, required=True)

    observe = subparsers.add_parser("record-observed-images")
    observe.add_argument("--metadata", type=Path, required=True)
    observe.add_argument("--observations", type=Path, required=True)
    observe.add_argument("--output", type=Path, required=True)

    complete = subparsers.add_parser("complete-disaster-report")
    complete.add_argument("--metadata", type=Path, required=True)
    complete.add_argument("--disaster-report", type=Path, required=True)
    complete.add_argument("--original-postgres-report", type=Path, required=True)
    complete.add_argument("--runtime-postgres-report", type=Path, required=True)
    complete.add_argument("--golden-report", type=Path, required=True)
    complete.add_argument("--observed-images", type=Path, required=True)
    complete.add_argument("--recovered-environment", type=Path, required=True)
    complete.add_argument("--runtime-environment", type=Path, required=True)
    complete.add_argument("--sentinel-file", type=Path, required=True)
    complete.add_argument("--expected-release", required=True)
    complete.add_argument("--expected-commit", required=True)
    complete.add_argument("--pre-users", type=int, required=True)
    complete.add_argument("--post-users", type=int, required=True)
    complete.add_argument("--recovered-fixture-count", type=int, required=True)
    complete.add_argument("--total-rto-seconds", type=int, required=True)
    complete.add_argument("--output", type=Path, required=True)
    return parser


def main(argv: Sequence[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        if args.command == "prepare-environment":
            prepare_environment(
                release_set_path=args.release_set,
                template_path=args.template,
                expected_release=args.expected_release,
                expected_commit=args.expected_commit,
                expected_release_set_sha256=args.expected_release_set_sha256,
                namespace=args.namespace,
                config_root=args.config_root,
                host_uid=args.host_uid,
                host_gid=args.host_gid,
                bootstrap_database=args.bootstrap_database,
                restored_database=args.restored_database,
                output_path=args.output,
                sentinel_path=args.sentinel_output,
                metadata_path=args.metadata_output,
            )
        elif args.command == "activate-restored-database":
            activate_restored_database(args.environment, args.metadata, args.output)
        elif args.command == "record-observed-images":
            record_observed_images(args.metadata, args.observations, args.output)
        else:
            complete_disaster_report(
                metadata_path=args.metadata,
                disaster_report_path=args.disaster_report,
                original_postgres_report_path=args.original_postgres_report,
                runtime_postgres_report_path=args.runtime_postgres_report,
                golden_report_path=args.golden_report,
                observed_images_path=args.observed_images,
                recovered_environment_path=args.recovered_environment,
                runtime_environment_path=args.runtime_environment,
                sentinel_path=args.sentinel_file,
                expected_release=args.expected_release,
                expected_commit=args.expected_commit,
                pre_users=args.pre_users,
                post_users=args.post_users,
                recovered_fixture_count=args.recovered_fixture_count,
                total_rto_seconds=args.total_rto_seconds,
                output_path=args.output,
            )
    except (
        OSError,
        RecoveredStackError,
        candidate.CandidateError,
        upgrade.UpgradePreparationError,
        host_recovery.RecoveryError,
    ) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
