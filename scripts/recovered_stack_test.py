from __future__ import annotations

import base64
import copy
import hashlib
import json
import tempfile
import unittest
from pathlib import Path

try:
    from scripts import prepare_release_candidate as candidate
    from scripts import recovered_stack as recovered
except ModuleNotFoundError:
    import prepare_release_candidate as candidate
    import recovered_stack as recovered


RELEASE = "v1.2.3-rc.21"
COMMIT = "a" * 40
NAMESPACE = "ghcr.io/prohibitedtv"


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


class RecoveredStackTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        self.template = Path(__file__).resolve().parents[1] / "deploy/.env.example"
        _, template_values = candidate.parse_env_template(
            self.template.read_text(encoding="utf-8")
        )
        digest_index = 1
        images = []
        for env_key, name in candidate.FIRST_PARTY_IMAGES:
            digest = f"sha256:{digest_index:064x}"
            digest_index += 1
            images.append(
                {
                    "envKey": env_key,
                    "name": name,
                    "digest": digest,
                    "candidateReference": f"{NAMESPACE}/{name}:{RELEASE}",
                    "immutableReference": f"{NAMESPACE}/{name}@{digest}",
                }
            )
        dependencies = []
        for env_key, template in candidate.THIRD_PARTY_IMAGES:
            digest = f"sha256:{digest_index:064x}"
            digest_index += 1
            dependencies.append(
                {
                    "envKey": env_key,
                    "reference": template.format_map(template_values),
                    "digest": digest,
                }
            )
        self.release_set = self.root / "release-set.json"
        self.release_payload = {
            "schemaVersion": "bitriver.release-set/v1",
            "candidate": {
                "tag": RELEASE,
                "sourceCommit": COMMIT,
                "repository": "ProhibitedTV/BitRiver-Live",
            },
            "images": images,
            "dependencies": dependencies,
        }
        self.write_release_set()
        self.environment = self.root / "recovered.env"
        self.runtime_environment = self.root / "runtime.env"
        self.sentinels = self.root / "sentinels"
        self.metadata = self.root / "metadata.json"
        self.prepare()
        recovered.activate_restored_database(
            self.environment, self.metadata, self.runtime_environment
        )
        self.expected_postgres_image = json.loads(
            self.metadata.read_text(encoding="utf-8")
        )["expectedServiceImages"]["postgres"]

    def tearDown(self) -> None:
        self.temporary.cleanup()

    def write_release_set(self) -> None:
        self.release_set.write_text(
            json.dumps(self.release_payload), encoding="utf-8"
        )

    def prepare(self) -> None:
        recovered.prepare_environment(
            release_set_path=self.release_set,
            template_path=self.template,
            expected_release=RELEASE,
            expected_commit=COMMIT,
            expected_release_set_sha256=sha256(self.release_set),
            namespace=NAMESPACE,
            config_root="/tmp/recovered/etc/bitriver-live",
            host_uid="1000",
            host_gid="1000",
            bootstrap_database="bitr_bootstrap",
            restored_database="bitr_recovered",
            output_path=self.environment,
            sentinel_path=self.sentinels,
            metadata_path=self.metadata,
            secret_factory=lambda key: f"unit-private-{key.lower()}-value",
        )

    def evidence_fixtures(self) -> dict[str, Path]:
        metadata = json.loads(self.metadata.read_text(encoding="utf-8"))
        disaster = self.root / "disaster.json"
        disaster.write_text(
            json.dumps(
                {
                    "schemaVersion": recovered.DISASTER_SCHEMA,
                    "status": "passed",
                    "completedAt": "2026-08-22T00:00:00Z",
                    "source": {
                        "release": RELEASE,
                        "commit": COMMIT,
                        "checkoutPresent": False,
                    },
                    "observed": {"rpoSeconds": 40, "rtoSeconds": 55},
                    "publishedPackage": {
                        "verified": True,
                        "releaseSetSha256": metadata["source"]["releaseSetSha256"],
                        "releaseSetSignatureVerified": False,
                        "asset": {"name": "bitriver-launcher-linux-amd64.tar.gz"},
                        "exercisedBundle": {"fileCount": 53},
                    },
                    "postgresRestoreHelperImage": self.expected_postgres_image,
                    "recoveredPostgres": {
                        "sourceRelease": RELEASE,
                        "sourceCommit": COMMIT,
                        "archive": "bitriver-postgres-fixture.sql.gz",
                        "archiveSha256": "c" * 64,
                        "manifest": "bitriver-postgres-fixture.sql.gz.manifest.json",
                        "manifestSha256": "d" * 64,
                    },
                    "bundle": {"sourceFree": True},
                    "stages": [{"id": "source-runtime-destroyed", "status": "passed"}],
                    "remainingAcceptance": [
                        recovered.GOLDEN_ACCEPTANCE,
                        recovered.SCHEDULED_ACCEPTANCE,
                    ],
                    "retainedSecrets": "none",
                }
            ),
            encoding="utf-8",
        )
        golden = self.root / "golden.json"
        golden.write_text(
            json.dumps(
                {
                    "schema": recovered.GOLDEN_SCHEMA,
                    "status": "passed",
                    "durationMs": 91_250,
                    "stages": [
                        {"name": name, "status": "passed"}
                        for name in sorted(recovered.GOLDEN_STAGES)
                    ],
                }
            ),
            encoding="utf-8",
        )
        images = self.root / "images.json"
        images.write_text(
            json.dumps(metadata["expectedServiceImages"]), encoding="utf-8"
        )
        postgres = self.root / "postgres.json"
        postgres.write_text(
            json.dumps(
                {
                    "schemaVersion": "bitriver.postgres-restore-report/v1",
                    "result": "passed",
                    "backup": {
                        "sourceRelease": RELEASE,
                        "sourceCommit": COMMIT,
                        "archive": "bitriver-postgres-fixture.sql.gz",
                        "archiveSha256": "c" * 64,
                        "manifest": "bitriver-postgres-fixture.sql.gz.manifest.json",
                        "manifestSha256": "d" * 64,
                        "observedRpoSeconds": 40,
                    },
                }
            ),
            encoding="utf-8",
        )
        runtime_postgres = self.root / "runtime-postgres.json"
        runtime_postgres.write_bytes(postgres.read_bytes())
        return {
            "disaster": disaster,
            "golden": golden,
            "images": images,
            "postgres": postgres,
            "runtime_postgres": runtime_postgres,
        }

    def complete(
        self,
        fixtures: dict[str, Path],
        output: Path,
        *,
        helper_image: str | None = None,
    ) -> None:
        expected_images = json.loads(self.metadata.read_text(encoding="utf-8"))[
            "expectedServiceImages"
        ]
        recovered.complete_disaster_report(
            metadata_path=self.metadata,
            disaster_report_path=fixtures["disaster"],
            original_postgres_report_path=fixtures["postgres"],
            runtime_postgres_report_path=fixtures["runtime_postgres"],
            runtime_postgres_helper_image=helper_image or expected_images["postgres"],
            golden_report_path=fixtures["golden"],
            observed_images_path=fixtures["images"],
            recovered_environment_path=self.environment,
            runtime_environment_path=self.runtime_environment,
            sentinel_path=self.sentinels,
            expected_release=RELEASE,
            expected_commit=COMMIT,
            pre_users=4,
            post_users=6,
            recovered_fixture_count=1,
            total_rto_seconds=180,
            output_path=output,
        )

    def test_prepare_and_activate_bind_exact_release_environment(self) -> None:
        metadata = json.loads(self.metadata.read_text(encoding="utf-8"))
        self.assertEqual(metadata["schemaVersion"], recovered.INPUT_SCHEMA)
        self.assertEqual(len(metadata["expectedServiceImages"]), 13)
        self.assertEqual(sha256(self.environment), metadata["environment"]["recoveredSha256"])
        self.assertEqual(
            sha256(self.runtime_environment), metadata["environment"]["runtimeSha256"]
        )
        self.assertIn("BITRIVER_POSTGRES_DB=bitr_bootstrap", self.environment.read_text())
        self.assertIn(
            "BITRIVER_POSTGRES_DB=bitr_recovered", self.runtime_environment.read_text()
        )

        tampered = self.root / "tampered.env"
        tampered.write_bytes(self.environment.read_bytes() + b"# tampered\n")
        with self.assertRaisesRegex(recovered.RecoveredStackError, "do not match"):
            recovered.activate_restored_database(tampered, self.metadata, self.root / "bad")

    def test_record_observed_images_requires_the_exact_release_set(self) -> None:
        expected = json.loads(self.metadata.read_text(encoding="utf-8"))[
            "expectedServiceImages"
        ]
        observations = self.root / "observations.tsv"
        observations.write_text(
            "".join(f"{service}\t{reference}\n" for service, reference in expected.items()),
            encoding="utf-8",
        )
        output = self.root / "observed.json"
        recovered.record_observed_images(self.metadata, observations, output)
        self.assertEqual(json.loads(output.read_text(encoding="utf-8")), expected)

        observations.write_text("viewer\tunrelated\n", encoding="utf-8")
        with self.assertRaisesRegex(recovered.RecoveredStackError, "do not match"):
            recovered.record_observed_images(self.metadata, observations, output)

    def test_wrapper_keeps_exercised_postgres_containers_unmodified(self) -> None:
        wrapper = (Path(__file__).parent / "test-recovered-stack-golden-path.sh").read_text(
            encoding="utf-8"
        )
        lost_host = (Path(__file__).parent / "test-disaster-recovery.sh").read_text(
            encoding="utf-8"
        )
        self.assertNotIn("apk add", wrapper)
        self.assertNotIn("docker cp", wrapper)
        self.assertNotIn("apk add", lost_host)
        self.assertNotIn("docker cp", lost_host)
        self.assertGreaterEqual(wrapper.count("--read-only"), 2)
        self.assertGreaterEqual(wrapper.count("--entrypoint /bin/sh"), 2)
        self.assertIn('--network "container:$postgres_container"', lost_host)
        self.assertIn('"$postgres_image" /restore-postgres.sh', lost_host)
        self.assertIn('--postgres-restore-helper-image "$postgres_image"', lost_host)
        self.assertIn(
            'BITRIVER_DISASTER_POSTGRES_IMAGE="$seed_postgres_image"', wrapper
        )
        self.assertIn('--runtime-postgres-helper-image "$runtime_postgres_image"', wrapper)
        self.assertIn(
            '--config-root "$(native_path "$recovered_root")/etc/bitriver-live"',
            wrapper,
        )
        self.assertNotIn('native_path "$recovered_root/etc/bitriver-live"', wrapper)

    def test_prepare_refuses_release_set_hash_and_repository_mismatch(self) -> None:
        with self.assertRaisesRegex(Exception, "SHA-256 mismatch"):
            recovered.prepare_environment(
                release_set_path=self.release_set,
                template_path=self.template,
                expected_release=RELEASE,
                expected_commit=COMMIT,
                expected_release_set_sha256="f" * 64,
                namespace=NAMESPACE,
                config_root="/tmp/recovered/etc/bitriver-live",
                host_uid="1000",
                host_gid="1000",
                bootstrap_database="bitr_bootstrap",
                restored_database="bitr_recovered",
                output_path=self.root / "bad.env",
                sentinel_path=self.root / "bad.sentinels",
                metadata_path=self.root / "bad.json",
            )
        self.release_payload["candidate"]["repository"] = "someone/else"
        self.write_release_set()
        with self.assertRaisesRegex(recovered.RecoveredStackError, "repository"):
            self.prepare()

    def test_completion_binds_golden_runtime_and_preserved_state(self) -> None:
        fixtures = self.evidence_fixtures()
        output = self.root / "complete.json"
        self.complete(fixtures, output)
        report = json.loads(output.read_text(encoding="utf-8"))
        self.assertEqual(report["remainingAcceptance"], [recovered.SCHEDULED_ACCEPTANCE])
        self.assertEqual(report["observed"]["restoreOnlyRtoSeconds"], 55)
        self.assertEqual(report["observed"]["rtoSeconds"], 180)
        self.assertEqual(report["observed"]["goldenPathSeconds"], 92)
        self.assertTrue(report["recoveredRuntime"]["verified"])
        self.assertEqual(
            report["recoveredRuntime"]["originalPostgresRestoreHelperImage"],
            self.expected_postgres_image,
        )
        self.assertEqual(
            report["recoveredRuntime"]["runtimePostgresRestoreHelperImage"],
            json.loads(self.metadata.read_text(encoding="utf-8"))[
                "expectedServiceImages"
            ]["postgres"],
        )
        self.assertEqual(report["recoveredRuntime"]["state"]["preGoldenUsers"], 4)
        self.assertEqual(
            report["stages"][-1]["id"],
            "recovered-immutable-stack-production-golden-path",
        )

    def test_completion_refuses_unrelated_failed_or_secret_evidence(self) -> None:
        fixtures = self.evidence_fixtures()
        images = json.loads(fixtures["images"].read_text(encoding="utf-8"))
        images["viewer"] = "ghcr.io/someone/unrelated@sha256:" + "f" * 64
        fixtures["images"].write_text(json.dumps(images), encoding="utf-8")
        with self.assertRaisesRegex(recovered.RecoveredStackError, "images do not match"):
            self.complete(fixtures, self.root / "bad-images.json")

        fixtures = self.evidence_fixtures()
        disaster = json.loads(fixtures["disaster"].read_text(encoding="utf-8"))
        disaster["postgresRestoreHelperImage"] = (
            "postgres:15-alpine@sha256:" + "f" * 64
        )
        fixtures["disaster"].write_text(json.dumps(disaster), encoding="utf-8")
        with self.assertRaisesRegex(recovered.RecoveredStackError, "helper image"):
            self.complete(fixtures, self.root / "bad-original-helper-image.json")

        fixtures = self.evidence_fixtures()
        with self.assertRaisesRegex(recovered.RecoveredStackError, "helper image"):
            self.complete(
                fixtures,
                self.root / "bad-helper-image.json",
                helper_image="postgres:15-alpine@sha256:" + "f" * 64,
            )

        fixtures = self.evidence_fixtures()
        golden = json.loads(fixtures["golden"].read_text(encoding="utf-8"))
        golden["stages"][0]["status"] = "failed"
        fixtures["golden"].write_text(json.dumps(golden), encoding="utf-8")
        with self.assertRaisesRegex(recovered.RecoveredStackError, "incomplete or failed"):
            self.complete(fixtures, self.root / "bad-golden.json")

        fixtures = self.evidence_fixtures()
        sentinel = self.sentinels.read_text(encoding="utf-8").splitlines()[0]
        golden = json.loads(fixtures["golden"].read_text(encoding="utf-8"))
        golden["leak"] = base64.b64encode(sentinel.encode()).decode()
        fixtures["golden"].write_text(json.dumps(golden), encoding="utf-8")
        with self.assertRaisesRegex(recovered.RecoveredStackError, "private sentinel"):
            self.complete(fixtures, self.root / "bad-secret.json")

        fixtures = self.evidence_fixtures()
        disaster = json.loads(fixtures["disaster"].read_text(encoding="utf-8"))
        disaster["source"]["commit"] = "b" * 40
        fixtures["disaster"].write_text(json.dumps(disaster), encoding="utf-8")
        with self.assertRaisesRegex(recovered.RecoveredStackError, "identity"):
            self.complete(fixtures, self.root / "bad-identity.json")

        fixtures = self.evidence_fixtures()
        runtime_postgres = json.loads(
            fixtures["runtime_postgres"].read_text(encoding="utf-8")
        )
        runtime_postgres["backup"]["archiveSha256"] = "e" * 64
        fixtures["runtime_postgres"].write_text(
            json.dumps(runtime_postgres), encoding="utf-8"
        )
        with self.assertRaisesRegex(recovered.RecoveredStackError, "not bound"):
            self.complete(fixtures, self.root / "bad-runtime-postgres.json")

        fixtures = self.evidence_fixtures()
        disaster = json.loads(fixtures["disaster"].read_text(encoding="utf-8"))
        disaster["recoveredPostgres"]["archiveSha256"] = "e" * 64
        fixtures["disaster"].write_text(json.dumps(disaster), encoding="utf-8")
        with self.assertRaisesRegex(recovered.RecoveredStackError, "aggregate disaster"):
            self.complete(fixtures, self.root / "bad-disaster-postgres.json")


if __name__ == "__main__":
    unittest.main()
