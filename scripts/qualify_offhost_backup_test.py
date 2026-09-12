from __future__ import annotations

import datetime as dt
import hashlib
import json
import tempfile
import unittest
from pathlib import Path
from typing import Any, Callable

from scripts import qualify_offhost_backup as proof


RELEASE = "v1.2.3-rc.22"
COMMIT = "a" * 40
PREFIX = "bitriver-live/postgres"
BUCKET = "backup-evidence"
NOW = dt.datetime(2026, 9, 12, 18, 0, 0, tzinfo=dt.timezone.utc)


class FakeObjectClient:
    def __init__(self, objects: dict[str, bytes]) -> None:
        self.objects = dict(objects)
        self.downloaded: list[str] = []

    def list_objects(self, bucket: str, prefix: str) -> list[dict[str, object]]:
        self._check_bucket(bucket)
        return [
            {"Key": key, "Size": len(value)}
            for key, value in sorted(self.objects.items())
            if key.startswith(prefix)
        ]

    def download(self, bucket: str, key: str, destination: Path) -> None:
        self._check_bucket(bucket)
        try:
            payload = self.objects[key]
        except KeyError as exc:
            raise AssertionError(f"unexpected download: {key}") from exc
        self.downloaded.append(key)
        destination.write_bytes(payload)

    @staticmethod
    def _check_bucket(bucket: str) -> None:
        if bucket != BUCKET:
            raise AssertionError(f"unexpected bucket: {bucket}")


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def make_set(
    stamp: str,
    *,
    prefix: str = PREFIX,
    release: str = RELEASE,
    commit: str = COMMIT,
    archive: bytes | None = None,
) -> dict[str, bytes]:
    archive = archive if archive is not None else (f"backup-{stamp}".encode("utf-8") * 7)
    stem = f"bitriver-postgres-{stamp}.sql.gz"
    archive_key = f"{prefix}/{stem}"
    created = dt.datetime.strptime(stamp, "%Y%m%dT%H%M%SZ").replace(tzinfo=dt.timezone.utc)
    manifest = {
        "schemaVersion": proof.BACKUP_SCHEMA,
        "createdAt": created.isoformat().replace("+00:00", "Z"),
        "source": {"release": release, "commit": commit},
        "archive": {
            "name": stem,
            "format": "postgresql-plain-sql+gzip",
            "sha256": sha256(archive),
            "sizeBytes": len(archive),
            "checksumAsset": stem + ".sha256",
        },
        "database": {
            "name": "bitriver",
            "serverVersion": "15.14",
            "serverVersionNum": 150014,
            "migrationFingerprintSha256": "b" * 64,
            "migrations": [
                {
                    "filename": "0001_initial.sql",
                    "version": "0001",
                    "checksumSha256": "c" * 64,
                    "status": "applied",
                    "release": release,
                    "commit": commit,
                }
            ],
            "rowCounts": {"schema_migrations": 1, "users": 4},
        },
        "tools": {"pgDump": "pg_dump (PostgreSQL) 15.14", "psql": "psql (PostgreSQL) 15.14"},
        "consistency": {"snapshot": "postgres-exported-snapshot", "tableRowCounts": "exact"},
    }
    manifest_bytes = (json.dumps(manifest, indent=2, sort_keys=True) + "\n").encode("utf-8")
    checksum = (
        f"{sha256(archive)}  {stem}\n"
        f"{sha256(manifest_bytes)}  {stem}.manifest.json\n"
    ).encode("utf-8")
    return {
        archive_key: archive,
        archive_key + ".manifest.json": manifest_bytes,
        archive_key + ".sha256": checksum,
    }


def fixture_remote(*, prefix: str = PREFIX) -> dict[str, bytes]:
    objects: dict[str, bytes] = {}
    for stamp in ("20260910T020000Z", "20260911T020000Z", "20260912T020000Z"):
        objects.update(make_set(stamp, prefix=prefix))
    return objects


def mutate_manifest(
    objects: dict[str, bytes],
    stamp: str,
    mutate: Callable[[dict[str, Any]], None],
    *,
    prefix: str = PREFIX,
) -> None:
    archive_key = f"{prefix}/bitriver-postgres-{stamp}.sql.gz"
    manifest_key = archive_key + ".manifest.json"
    checksum_key = archive_key + ".sha256"
    payload = json.loads(objects[manifest_key])
    mutate(payload)
    manifest_bytes = (json.dumps(payload, indent=2, sort_keys=True) + "\n").encode("utf-8")
    objects[manifest_key] = manifest_bytes
    objects[checksum_key] = (
        f"{sha256(objects[archive_key])}  {Path(archive_key).name}\n"
        f"{sha256(manifest_bytes)}  {Path(manifest_key).name}\n"
    ).encode("utf-8")


class OffhostBackupQualificationTests(unittest.TestCase):
    def qualify(self, objects: dict[str, bytes], **overrides: object) -> dict[str, object]:
        options: dict[str, object] = {
            "bucket": BUCKET,
            "prefix": PREFIX,
            "max_age_seconds": 86400,
            "min_complete_sets": 3,
            "expected_release": RELEASE,
            "expected_commit": COMMIT,
            "now": NOW,
        }
        options.update(overrides)
        return proof.qualify(FakeObjectClient(objects), **options)  # type: ignore[arg-type]

    def test_qualifies_fresh_complete_remote_history(self) -> None:
        client = FakeObjectClient(fixture_remote())
        report = proof.qualify(
            client,
            bucket=BUCKET,
            prefix=PREFIX,
            max_age_seconds=86400,
            min_complete_sets=3,
            expected_release=RELEASE,
            expected_commit=COMMIT,
            now=NOW,
        )

        self.assertEqual(report["schemaVersion"], proof.REPORT_SCHEMA)
        self.assertEqual(report["status"], "pass")
        self.assertEqual(report["remote"]["completeSetCount"], 3)  # type: ignore[index]
        self.assertEqual(report["latest"]["observedRpoSeconds"], 16 * 60 * 60)  # type: ignore[index]
        self.assertTrue(report["latest"]["checksumVerified"])  # type: ignore[index]
        self.assertTrue(report["latest"]["candidateIdentityVerified"])  # type: ignore[index]
        self.assertTrue(report["claims"]["remoteFreshness"])  # type: ignore[index]
        self.assertFalse(report["claims"]["scheduledProvenance"])  # type: ignore[index]
        self.assertFalse(report["claims"]["restoreProvenByThisReport"])  # type: ignore[index]
        self.assertEqual(
            client.downloaded,
            [
                f"{PREFIX}/bitriver-postgres-20260912T020000Z.sql.gz",
                f"{PREFIX}/bitriver-postgres-20260912T020000Z.sql.gz.manifest.json",
                f"{PREFIX}/bitriver-postgres-20260912T020000Z.sql.gz.sha256",
            ],
        )

    def test_preserves_trailing_separator_from_producer_prefix(self) -> None:
        prefix = "bitriver-live/postgres/"
        report = proof.qualify(
            FakeObjectClient(fixture_remote(prefix=prefix)),
            bucket=BUCKET,
            prefix=prefix,
            max_age_seconds=86400,
            min_complete_sets=3,
            expected_release=RELEASE,
            expected_commit=COMMIT,
            now=NOW,
        )
        self.assertEqual(report["remote"]["prefix"], prefix)  # type: ignore[index]
        self.assertEqual(  # type: ignore[index]
            report["latest"]["archiveKey"],
            "bitriver-live/postgres//bitriver-postgres-20260912T020000Z.sql.gz",
        )

    def test_refuses_missing_or_invalid_candidate_identity(self) -> None:
        with self.assertRaisesRegex(proof.QualificationError, "expected release"):
            self.qualify(fixture_remote(), expected_release="")
        with self.assertRaisesRegex(proof.QualificationError, "expected commit"):
            self.qualify(fixture_remote(), expected_commit="")

        parser = proof.build_parser()
        with self.assertRaises(SystemExit):
            parser.parse_args(["--bucket", BUCKET, "--report", "proof.json"])

    def test_refuses_stale_newest_set(self) -> None:
        with self.assertRaisesRegex(proof.QualificationError, "stale"):
            self.qualify(fixture_remote(), now=dt.datetime(2026, 9, 13, 3, 0, tzinfo=dt.timezone.utc))

    def test_refuses_incomplete_remote_set(self) -> None:
        objects = fixture_remote()
        del objects[f"{PREFIX}/bitriver-postgres-20260911T020000Z.sql.gz.manifest.json"]
        with self.assertRaisesRegex(proof.QualificationError, "incomplete remote backup set"):
            self.qualify(objects)

    def test_refuses_insufficient_remote_retention(self) -> None:
        objects: dict[str, bytes] = {}
        objects.update(make_set("20260911T020000Z"))
        objects.update(make_set("20260912T020000Z"))
        with self.assertRaisesRegex(proof.QualificationError, "at least 3 required"):
            self.qualify(objects)

    def test_refuses_archive_checksum_mismatch(self) -> None:
        objects = fixture_remote()
        newest = f"{PREFIX}/bitriver-postgres-20260912T020000Z.sql.gz"
        objects[newest] += b"tamper"
        with self.assertRaisesRegex(proof.QualificationError, "checksum does not match"):
            self.qualify(objects)

    def test_refuses_manifest_checksum_mismatch(self) -> None:
        objects = fixture_remote()
        newest_manifest = f"{PREFIX}/bitriver-postgres-20260912T020000Z.sql.gz.manifest.json"
        payload = json.loads(objects[newest_manifest])
        payload["database"]["rowCounts"]["users"] = 5
        objects[newest_manifest] = (json.dumps(payload, indent=2, sort_keys=True) + "\n").encode("utf-8")
        with self.assertRaisesRegex(proof.QualificationError, "checksum does not match downloaded backup manifest"):
            self.qualify(objects)

    def test_refuses_manifest_archive_metadata_mismatch_even_with_rewritten_checksum(self) -> None:
        objects = fixture_remote()
        mutate_manifest(
            objects,
            "20260912T020000Z",
            lambda payload: payload["archive"].__setitem__("sizeBytes", payload["archive"]["sizeBytes"] + 1),
        )
        with self.assertRaisesRegex(proof.QualificationError, "archive size"):
            self.qualify(objects)

    def test_refuses_empty_or_invalid_migration_evidence(self) -> None:
        objects = fixture_remote()
        mutate_manifest(objects, "20260912T020000Z", lambda payload: payload["database"].__setitem__("migrations", []))
        with self.assertRaisesRegex(proof.QualificationError, "migrations must be a non-empty array"):
            self.qualify(objects)

        objects = fixture_remote()
        mutate_manifest(
            objects,
            "20260912T020000Z",
            lambda payload: payload["database"]["migrations"][0].__setitem__("status", "failed"),
        )
        with self.assertRaisesRegex(proof.QualificationError, "is not applied"):
            self.qualify(objects)

        objects = fixture_remote()
        mutate_manifest(
            objects,
            "20260912T020000Z",
            lambda payload: payload["database"]["migrations"][0].__setitem__("checksumSha256", "bad"),
        )
        with self.assertRaisesRegex(proof.QualificationError, "migration\[0\] checksum is invalid"):
            self.qualify(objects)

    def test_refuses_invalid_row_count_evidence(self) -> None:
        objects = fixture_remote()
        mutate_manifest(
            objects,
            "20260912T020000Z",
            lambda payload: payload["database"]["rowCounts"].__setitem__("users", "4"),
        )
        with self.assertRaisesRegex(proof.QualificationError, "non-negative integer"):
            self.qualify(objects)

        objects = fixture_remote()
        mutate_manifest(objects, "20260912T020000Z", lambda payload: payload["database"].__setitem__("rowCounts", {}))
        with self.assertRaisesRegex(proof.QualificationError, "non-empty row-count evidence"):
            self.qualify(objects)

    def test_refuses_missing_tool_and_consistency_evidence(self) -> None:
        objects = fixture_remote()
        mutate_manifest(objects, "20260912T020000Z", lambda payload: payload.__setitem__("tools", {}))
        with self.assertRaisesRegex(proof.QualificationError, "tools.pgDump"):
            self.qualify(objects)

        objects = fixture_remote()
        mutate_manifest(
            objects,
            "20260912T020000Z",
            lambda payload: payload["consistency"].__setitem__("snapshot", "best-effort"),
        )
        with self.assertRaisesRegex(proof.QualificationError, "snapshot consistency"):
            self.qualify(objects)

    def test_refuses_wrong_release_and_commit(self) -> None:
        with self.assertRaisesRegex(proof.QualificationError, "source release mismatch"):
            self.qualify(fixture_remote(), expected_release="v1.2.3-rc.23")
        with self.assertRaisesRegex(proof.QualificationError, "source commit mismatch"):
            self.qualify(fixture_remote(), expected_commit="d" * 40)

    def test_refuses_unknown_source_identity(self) -> None:
        objects = fixture_remote()
        objects.update(make_set("20260912T030000Z", release="unknown", commit="unknown"))
        with self.assertRaisesRegex(proof.QualificationError, "source release"):
            self.qualify(objects, now=dt.datetime(2026, 9, 12, 4, 0, tzinfo=dt.timezone.utc))

    def test_report_is_atomic_and_does_not_contain_credentials(self) -> None:
        report = self.qualify(fixture_remote())
        report["testMarker"] = "safe"
        with tempfile.TemporaryDirectory() as temp_dir:
            output = Path(temp_dir) / "proof.json"
            proof.write_report(report, output)
            text = output.read_text(encoding="utf-8")
            self.assertIn('"testMarker": "safe"', text)
            self.assertNotIn("AWS_SECRET_ACCESS_KEY", text)
            self.assertNotIn("super-secret-fixture", text)
            self.assertFalse(any(output.parent.glob("*.partial.*")))

    def test_cli_error_redaction_never_echoes_credential_values(self) -> None:
        command = ["aws", "--secret-access-key", "super-secret-fixture", "s3api", "list-objects-v2"]
        redacted = " ".join(proof.redact_command(command))
        self.assertNotIn("super-secret-fixture", redacted)
        self.assertIn("<redacted>", redacted)


if __name__ == "__main__":
    unittest.main()
