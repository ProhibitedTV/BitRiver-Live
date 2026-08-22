from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import io
import json
import os
import shutil
import tarfile
import tempfile
import unittest
from pathlib import Path

try:
    from scripts import host_recovery as recovery
except ModuleNotFoundError:
    import host_recovery as recovery


RELEASE = "v1.2.3-rc.21"
COMMIT = "a" * 40


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def write_launcher_package(path: Path, *, unsafe_link: bool = False) -> None:
    root = "bitriver-launcher-linux-amd64"
    required = (
        "bin/bitriver",
        "bin/bitriver-live",
        "share/bitriver-live/deploy/docker-compose.yml",
        "share/bitriver-live/scripts/backup-postgres.sh",
        "share/bitriver-live/scripts/restore-postgres.sh",
        "share/bitriver-live/scripts/backup-host-state.sh",
        "share/bitriver-live/scripts/restore-host-state.sh",
        "share/bitriver-live/scripts/host_recovery.py",
    )
    with tarfile.open(path, mode="w:gz") as archive:
        for relative in required:
            contents = f"fixture:{relative}\n".encode()
            member = tarfile.TarInfo(f"{root}/{relative}")
            member.size = len(contents)
            member.mode = 0o755
            archive.addfile(member, io.BytesIO(contents))
        if unsafe_link:
            member = tarfile.TarInfo(f"{root}/share/bitriver-live/escape")
            member.type = tarfile.SYMTYPE
            member.linkname = "/etc/passwd"
            archive.addfile(member)


class HostRecoveryTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        self.source = self.root / "source"
        self.config = self.source / "etc/bitriver-live"
        self.data = self.source / "var/lib/bitriver-live"
        (self.config / "deploy/ome").mkdir(parents=True)
        (self.data / "api").mkdir(parents=True)
        (self.data / "transcoder/public/live").mkdir(parents=True)
        (self.config / "bitriver.env").write_text(
            "BITRIVER_POSTGRES_PASSWORD=unit-secret-value\n", encoding="utf-8"
        )
        (self.config / "deploy/ome/Server.generated.xml").write_text(
            "<Server>unit-secret-value</Server>\n", encoding="utf-8"
        )
        (self.data / "api/state.json").write_text('{"ready":true}\n', encoding="utf-8")
        (self.data / "transcoder/public/live/index.m3u8").write_text(
            "#EXTM3U\n", encoding="utf-8"
        )
        os.chmod(self.config / "bitriver.env", 0o600)
        os.chmod(self.config / "deploy/ome/Server.generated.xml", 0o640)

        self.postgres = self.root / "bitriver-postgres-20260815T000000Z.sql.gz"
        self.postgres.write_bytes(b"postgres-backup-fixture")
        self.postgres_manifest = Path(f"{self.postgres}.manifest.json")
        self.postgres_manifest.write_text(
            json.dumps(
                {
                    "schemaVersion": recovery.POSTGRES_BACKUP_SCHEMA,
                    "source": {"release": RELEASE, "commit": COMMIT},
                    "archive": {
                        "name": self.postgres.name,
                        "sha256": sha256(self.postgres),
                    },
                    "database": {"migrationFingerprintSha256": "b" * 64},
                }
            )
            + "\n",
            encoding="utf-8",
        )
        self.postgres_checksum = Path(f"{self.postgres}.sha256")
        self.postgres_checksum.write_text(
            f"{sha256(self.postgres)}  {self.postgres.name}\n"
            f"{sha256(self.postgres_manifest)}  {self.postgres_manifest.name}\n",
            encoding="utf-8",
        )

        self.archive = self.root / "bitriver-host-20260815T000000Z.tar.gz.enc"
        self.archive.write_bytes(b"encrypted-fixture-does-not-contain-unit-secret")
        self.manifest = Path(f"{self.archive}.manifest.json")
        recovery.build_backup_manifest(
            argparse.Namespace(
                source_release=RELEASE,
                source_commit=COMMIT,
                created_at="2026-08-15T00:00:00Z",
                root_prefix=self.source,
                archive=self.archive,
                postgres_backup=self.postgres,
                object_inventory=None,
                iterations=200_000,
                output=self.manifest,
            )
        )
        self.checksum = Path(f"{self.archive}.sha256")
        self.checksum.write_text(
            f"{sha256(self.archive)}  {self.archive.name}\n"
            f"{sha256(self.manifest)}  {self.manifest.name}\n",
            encoding="utf-8",
        )

    def tearDown(self) -> None:
        self.temporary.cleanup()

    def test_manifest_is_secret_safe_and_complete(self) -> None:
        manifest = recovery.verify_backup_set(
            self.archive, expected_release=RELEASE, expected_commit=COMMIT
        )
        rendered = self.manifest.read_text(encoding="utf-8")
        self.assertNotIn("unit-secret-value", rendered)
        self.assertEqual(manifest["archive"]["sha256"], sha256(self.archive))
        self.assertEqual(manifest["payload"]["configuration"]["fileCount"], 2)
        self.assertEqual(manifest["payload"]["data"]["fileCount"], 2)
        self.assertEqual(
            manifest["payload"]["postgres"]["migrationFingerprintSha256"],
            "b" * 64,
        )

    def test_wrong_release_and_corruption_are_refused(self) -> None:
        with self.assertRaisesRegex(recovery.RecoveryError, "requested release"):
            recovery.verify_backup_set(
                self.archive,
                expected_release="v1.2.3-rc.22",
                expected_commit=COMMIT,
            )
        self.archive.write_bytes(b"corrupted")
        with self.assertRaisesRegex(recovery.RecoveryError, "checksum mismatch"):
            recovery.verify_backup_set(
                self.archive, expected_release=RELEASE, expected_commit=COMMIT
            )

    def test_external_symlink_is_refused(self) -> None:
        outside = self.root / "outside-secret"
        outside.write_text("secret", encoding="utf-8")
        link = self.config / "escaped"
        try:
            link.symlink_to(outside)
        except OSError as exc:
            self.skipTest(f"symlink creation unavailable: {exc}")
        with self.assertRaisesRegex(recovery.RecoveryError, "outside the host root"):
            recovery.tree_inventory(self.config, self.source)

    def tar_stream(self, unsafe: str | None = None, link: bool = False) -> io.BytesIO:
        manifest = json.loads(self.manifest.read_text(encoding="utf-8"))
        postgres = manifest["payload"]["postgres"]
        stream = io.BytesIO()
        with tarfile.open(fileobj=stream, mode="w:gz") as archive:
            for name in (
                "etc/bitriver-live",
                "var/lib/bitriver-live",
            ):
                member = tarfile.TarInfo(name)
                member.type = tarfile.DIRTYPE
                archive.addfile(member)
            for name in (
                postgres["archiveName"],
                postgres["manifestName"],
                postgres["checksumName"],
            ):
                contents = b"fixture"
                member = tarfile.TarInfo(f"{recovery.POSTGRES_RELATIVE}/{name}")
                member.size = len(contents)
                archive.addfile(member, io.BytesIO(contents))
            if unsafe is not None:
                member = tarfile.TarInfo(unsafe)
                member.size = 1
                archive.addfile(member, io.BytesIO(b"x"))
            if link:
                member = tarfile.TarInfo("etc/bitriver-live/linked")
                member.type = tarfile.SYMTYPE
                member.linkname = "/etc/passwd"
                archive.addfile(member)
        stream.seek(0)
        return stream

    def test_archive_contract_accepts_only_canonical_regular_payload(self) -> None:
        manifest = json.loads(self.manifest.read_text(encoding="utf-8"))
        recovery.validate_archive_stream(manifest, self.tar_stream())
        with self.assertRaisesRegex(recovery.RecoveryError, "unsafe path"):
            recovery.validate_archive_stream(manifest, self.tar_stream("../escape"))
        with self.assertRaisesRegex(recovery.RecoveryError, "link or special"):
            recovery.validate_archive_stream(manifest, self.tar_stream(link=True))

    def test_restore_report_matches_fresh_recovered_state(self) -> None:
        target = self.root / "target"
        shutil.copytree(self.config, target / "etc/bitriver-live")
        shutil.copytree(self.data, target / "var/lib/bitriver-live")
        recovered_postgres = target / Path(recovery.POSTGRES_RELATIVE.as_posix())
        recovered_postgres.mkdir(parents=True)
        for path in (self.postgres, self.postgres_manifest, self.postgres_checksum):
            shutil.copy2(path, recovered_postgres / path.name)
        report_path = self.root / "restore-report.json"
        recovery.build_restore_report(
            argparse.Namespace(
                archive=self.archive,
                root_prefix=target,
                expected_release=RELEASE,
                expected_commit=COMMIT,
                started_at_epoch=int(dt.datetime.now(dt.timezone.utc).timestamp()),
                output=report_path,
            )
        )
        report = json.loads(report_path.read_text(encoding="utf-8"))
        self.assertEqual(report["schemaVersion"], recovery.HOST_RESTORE_SCHEMA)
        self.assertEqual(report["status"], "passed")
        self.assertTrue(report["invariants"]["configuration"]["matched"])
        self.assertTrue(report["invariants"]["data"]["matched"])
        self.assertNotIn("unit-secret-value", report_path.read_text(encoding="utf-8"))

    def test_release_package_refuses_identity_and_inventory_mismatch(self) -> None:
        package = self.root / recovery.RECOVERY_PACKAGE_NAME
        write_launcher_package(package)
        release_set = self.root / "release-set.json"
        base = {
            "schemaVersion": recovery.RELEASE_SET_SCHEMA,
            "candidate": {
                "tag": RELEASE,
                "sourceCommit": COMMIT,
                "repository": "ProhibitedTV/BitRiver-Live",
            },
            "artifacts": [
                {
                    "name": package.name,
                    "kind": "archive",
                    "bytes": package.stat().st_size,
                    "sha256": sha256(package),
                }
            ],
        }
        cases = (
            ("schemaVersion", "unsupported/v1", "schema"),
            ("candidate.tag", "v1.2.3-rc.22", "tag"),
            ("candidate.sourceCommit", "b" * 40, "commit"),
            ("candidate.repository", "someone/else", "repository"),
            ("artifacts.0.name", "different.tar.gz", "exactly one"),
            ("artifacts.0.kind", "installer", "kind"),
            ("artifacts.0.bytes", package.stat().st_size + 1, "size"),
            ("artifacts.0.sha256", "c" * 64, "SHA-256"),
        )
        for dotted, value, expected in cases:
            with self.subTest(field=dotted):
                payload = json.loads(json.dumps(base))
                target: object = payload
                parts = dotted.split(".")
                for part in parts[:-1]:
                    target = target[int(part)] if part.isdigit() else target[part]  # type: ignore[index]
                target[parts[-1]] = value  # type: ignore[index]
                release_set.write_text(json.dumps(payload), encoding="utf-8")
                with self.assertRaisesRegex(recovery.RecoveryError, expected):
                    recovery.validate_release_package(
                        release_set,
                        package,
                        expected_release=RELEASE,
                        expected_commit=COMMIT,
                    )
        write_launcher_package(package, unsafe_link=True)
        base["artifacts"][0]["bytes"] = package.stat().st_size
        base["artifacts"][0]["sha256"] = sha256(package)
        release_set.write_text(json.dumps(base), encoding="utf-8")
        with self.assertRaisesRegex(recovery.RecoveryError, "link or special"):
            recovery.validate_release_package(
                release_set,
                package,
                expected_release=RELEASE,
                expected_commit=COMMIT,
            )

    def test_disaster_report_requires_source_free_complete_evidence(self) -> None:
        host_report = self.root / "host-report.json"
        host_report.write_text(
            json.dumps(
                {
                    "schemaVersion": recovery.HOST_RESTORE_SCHEMA,
                    "status": "passed",
                    "source": {"release": RELEASE, "commit": COMMIT},
                    "observed": {"rpoSeconds": 12, "rtoSeconds": 4},
                    "backup": {
                        "postgres": {
                            "archiveName": self.postgres.name,
                            "archiveSha256": sha256(self.postgres),
                            "manifestName": self.postgres_manifest.name,
                            "manifestSha256": sha256(self.postgres_manifest),
                        }
                    },
                }
            ),
            encoding="utf-8",
        )
        postgres_report = self.root / "postgres-report.json"
        postgres_report.write_text(
            json.dumps(
                {
                    "schemaVersion": "bitriver.postgres-restore-report/v1",
                    "result": "passed",
                    "backup": {
                        "archive": self.postgres.name,
                        "archiveSha256": sha256(self.postgres),
                        "manifest": self.postgres_manifest.name,
                        "manifestSha256": sha256(self.postgres_manifest),
                        "sourceRelease": RELEASE,
                        "sourceCommit": COMMIT,
                        "observedRpoSeconds": 37,
                    },
                }
            ),
            encoding="utf-8",
        )
        object_inventory = self.root / "objects.json"
        recovery.atomic_write(
            object_inventory,
            json.dumps(
                {
                    "schemaVersion": recovery.OBJECT_INVENTORY_SCHEMA,
                    "objectCount": 1,
                    "totalBytes": 7,
                    "fingerprintSha256": "c" * 64,
                },
                indent=2,
                sort_keys=True,
            )
            + "\n",
        )
        required = (
            "deploy/docker-compose.yml",
            "scripts/backup-postgres.sh",
            "scripts/restore-postgres.sh",
            "scripts/backup-host-state.sh",
            "scripts/restore-host-state.sh",
            "scripts/host_recovery.py",
        )
        bundle = self.root / "bundle"
        installed = self.root / "installed"
        for root in (bundle, installed):
            for relative in required:
                path = root / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("fixture\n", encoding="utf-8")
        output = self.root / "disaster-report.json"
        recovery.build_disaster_report(
            argparse.Namespace(
                host_report=host_report,
                postgres_report=postgres_report,
                expected_object_inventory=object_inventory,
                observed_object_inventory=object_inventory,
                bundle_root=bundle,
                installed_root=installed,
                destroyed_source_root=self.root / "destroyed-source",
                source_release=RELEASE,
                source_commit=COMMIT,
                started_at_epoch=int(dt.datetime.now(dt.timezone.utc).timestamp()),
                output=output,
            )
        )
        report = json.loads(output.read_text(encoding="utf-8"))
        self.assertEqual(report["schemaVersion"], recovery.DISASTER_REPORT_SCHEMA)
        self.assertEqual(report["status"], "passed")
        self.assertTrue(report["bundle"]["sourceFree"])
        self.assertEqual(report["observed"]["hostRpoSeconds"], 12)
        self.assertEqual(report["observed"]["postgresRpoSeconds"], 37)
        self.assertEqual(report["observed"]["rpoSeconds"], 37)
        self.assertEqual(
            report["recoveredPostgres"],
            {
                "sourceRelease": RELEASE,
                "sourceCommit": COMMIT,
                "archive": self.postgres.name,
                "archiveSha256": sha256(self.postgres),
                "manifest": self.postgres_manifest.name,
                "manifestSha256": sha256(self.postgres_manifest),
            },
        )
        self.assertFalse(report["publishedPackage"]["verified"])
        self.assertIn(
            "exact published release-set package qualification",
            report["remainingAcceptance"],
        )

        package = self.root / recovery.RECOVERY_PACKAGE_NAME
        write_launcher_package(package)
        release_set = self.root / "release-set.json"
        release_set.write_text(
            json.dumps(
                {
                    "schemaVersion": recovery.RELEASE_SET_SCHEMA,
                    "candidate": {
                        "tag": RELEASE,
                        "sourceCommit": COMMIT,
                        "repository": "ProhibitedTV/BitRiver-Live",
                    },
                    "artifacts": [
                        {
                            "name": package.name,
                            "kind": "archive",
                            "bytes": package.stat().st_size,
                            "sha256": sha256(package),
                        }
                    ],
                }
            ),
            encoding="utf-8",
        )
        published = recovery.validate_release_package(
            release_set,
            package,
            expected_release=RELEASE,
            expected_commit=COMMIT,
        )
        self.assertTrue(published["verified"])
        self.assertFalse(published["releaseSetSignatureVerified"])
        self.assertEqual(published["asset"]["sha256"], sha256(package))
        extract_root = self.root / "published-launcher"
        with tarfile.open(package, mode="r:gz") as archive:
            archive.extractall(extract_root, filter="data")
        published_bundle = (
            extract_root
            / "bitriver-launcher-linux-amd64"
            / "share"
            / "bitriver-live"
        )
        recovery.build_disaster_report(
            argparse.Namespace(
                host_report=host_report,
                postgres_report=postgres_report,
                expected_object_inventory=object_inventory,
                observed_object_inventory=object_inventory,
                bundle_root=published_bundle,
                installed_root=installed,
                destroyed_source_root=self.root / "destroyed-source",
                source_release=RELEASE,
                source_commit=COMMIT,
                release_set=release_set,
                package_archive=package,
                started_at_epoch=int(dt.datetime.now(dt.timezone.utc).timestamp()),
                output=output,
            )
        )
        report = json.loads(output.read_text(encoding="utf-8"))
        self.assertTrue(report["publishedPackage"]["verified"])
        self.assertNotIn(
            "exact published release-set package qualification",
            report["remainingAcceptance"],
        )
        self.assertEqual(
            report["stages"][0]["id"], "published-package-release-set-binding"
        )
        (published_bundle / "scripts/host_recovery.py").write_text(
            "tampered exercised bundle\n", encoding="utf-8"
        )
        with self.assertRaisesRegex(recovery.RecoveryError, "bundle does not match"):
            recovery.validate_release_package(
                release_set,
                package,
                expected_release=RELEASE,
                expected_commit=COMMIT,
                bundle_root=published_bundle,
            )
        package.write_bytes(package.read_bytes() + b"tampered-package-fixture")
        with self.assertRaisesRegex(recovery.RecoveryError, "does not match"):
            recovery.validate_release_package(
                release_set,
                package,
                expected_release=RELEASE,
                expected_commit=COMMIT,
            )
        write_launcher_package(package)
        unrelated = json.loads(postgres_report.read_text(encoding="utf-8"))
        unrelated["backup"]["archiveSha256"] = "d" * 64
        postgres_report.write_text(json.dumps(unrelated), encoding="utf-8")
        with self.assertRaisesRegex(recovery.RecoveryError, "does not match"):
            recovery.build_disaster_report(
                argparse.Namespace(
                    host_report=host_report,
                    postgres_report=postgres_report,
                    expected_object_inventory=object_inventory,
                    observed_object_inventory=object_inventory,
                    bundle_root=bundle,
                    installed_root=installed,
                    destroyed_source_root=self.root / "destroyed-source",
                    source_release=RELEASE,
                    source_commit=COMMIT,
                    started_at_epoch=int(dt.datetime.now(dt.timezone.utc).timestamp()),
                    output=output,
                )
            )
        postgres_report.write_text(
            json.dumps(
                {
                    "schemaVersion": "bitriver.postgres-restore-report/v1",
                    "result": "passed",
                    "backup": {
                        "archive": self.postgres.name,
                        "archiveSha256": sha256(self.postgres),
                        "manifest": self.postgres_manifest.name,
                        "manifestSha256": sha256(self.postgres_manifest),
                        "sourceRelease": RELEASE,
                        "sourceCommit": COMMIT,
                        "observedRpoSeconds": 37,
                    },
                }
            ),
            encoding="utf-8",
        )
        (bundle / "cmd").mkdir()
        with self.assertRaisesRegex(recovery.RecoveryError, "source checkout"):
            recovery.build_disaster_report(
                argparse.Namespace(
                    host_report=host_report,
                    postgres_report=postgres_report,
                    expected_object_inventory=object_inventory,
                    observed_object_inventory=object_inventory,
                    bundle_root=bundle,
                    installed_root=installed,
                    destroyed_source_root=self.root / "destroyed-source",
                    source_release=RELEASE,
                    source_commit=COMMIT,
                    started_at_epoch=int(dt.datetime.now(dt.timezone.utc).timestamp()),
                    output=output,
                )
            )


if __name__ == "__main__":
    unittest.main()
