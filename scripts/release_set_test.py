#!/usr/bin/env python3

from __future__ import annotations

import argparse
import importlib.util
import io
import json
import sys
import tempfile
import unittest
from contextlib import redirect_stderr
from pathlib import Path


SCRIPT_PATH = Path(__file__).with_name("release_set.py")
SPEC = importlib.util.spec_from_file_location("release_set", SCRIPT_PATH)
assert SPEC is not None and SPEC.loader is not None
release_set = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = release_set
SPEC.loader.exec_module(release_set)


TAG = "v1.2.3-rc.13"
COMMIT = "a" * 40
REPOSITORY = "ProhibitedTV/BitRiver-Live"


class ReleaseSetTests(unittest.TestCase):
    def make_payload(self, root: Path) -> tuple[Path, Path]:
        root.mkdir(parents=True, exist_ok=True)
        for name in release_set.required_candidate_names(TAG):
            (root / name).write_bytes(f"fixture:{name}\n".encode())

        namespace = "ghcr.io/prohibitedtv"
        images = []
        for index, (name, env_key) in enumerate(release_set.FIRST_PARTY_IMAGES, 1):
            images.append(
                {
                    "name": name,
                    "envKey": env_key,
                    "reference": f"{namespace}/{name}:{TAG}",
                    "digest": "sha256:" + f"{index:x}" * 64,
                }
            )
        self.write_json(
            root / "release-images.json",
            {
                "schemaVersion": "bitriver.release-images/v1",
                "anonymousManifestAccess": True,
                "namespace": namespace,
                "tag": TAG,
                "images": images,
            },
        )
        self.write_json(
            root / "release-dependencies.json",
            {
                "schemaVersion": "bitriver.release-dependencies/v1",
                "registryManifestAccess": True,
                "images": [
                    {
                        "envKey": "BITRIVER_REDIS_IMAGE_DIGEST",
                        "reference": "redis:7-alpine",
                        "digest": "sha256:" + "b" * 64,
                    }
                ],
            },
        )
        self.write_json(
            root / "release-contract-evidence.json",
            {
                "schemaVersion": 1,
                "releaseTag": TAG,
                "commit": COMMIT,
                "environmentValidation": "passed",
                "imageDigestValidation": "passed",
                "firstPartyDigestValidation": "prepublication-format-only",
                "credentialFlow": "job-local-ephemeral",
                "imageNamespace": namespace,
                "retainedValues": "none",
            },
        )
        self.write_json(
            root / "production-golden-path.json",
            {
                "schema": "bitriver.production-golden-path/v1",
                "status": "passed",
                "stages": [
                    {"name": "surface-preflight", "status": "passed"},
                    {"name": "final-status", "status": "passed"},
                ],
            },
        )
        self.write_json(
            root / "release-scan-status.json",
            {
                "schemaVersion": 1,
                "releaseTag": TAG,
                "commit": COMMIT,
                "downloadedArtifactScan": "passed",
                "publicationPayloadScan": "passed",
            },
        )
        manifest = root / "release-set.json"
        markdown = root / "release-set.md"
        release_set.create_candidate_manifest(
            argparse.Namespace(
                tag=TAG,
                commit=COMMIT,
                repository=REPOSITORY,
                run_id=12345,
                run_attempt=1,
                workflow_url=(
                    "https://github.com/ProhibitedTV/BitRiver-Live/actions/runs/12345"
                ),
                created_at="2026-08-03T00:00:00Z",
                go_version="go1.26.5",
                node_version="v24.0.0",
                cosign_version="v3.0.4",
                assets_dir=root,
                output=manifest,
                markdown_output=markdown,
            )
        )
        (root / "release-set.sigstore.json").write_text(
            '{"fixture":"root-signature"}\n', encoding="utf-8"
        )
        release_set.write_checksums(root, root / "CHECKSUMS.txt")
        return manifest, markdown

    @staticmethod
    def write_json(path: Path, value: object) -> None:
        path.write_text(
            json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8"
        )

    def promotion_record(self, manifest: Path) -> dict[str, object]:
        manifest_sha = release_set.sha256_file(manifest)
        return {
            "schemaVersion": release_set.PROMOTION_SCHEMA,
            "candidateTag": TAG,
            "candidateReleaseUrl": (
                "https://github.com/ProhibitedTV/BitRiver-Live/releases/tag/" + TAG
            ),
            "candidateReleaseSetSha256": manifest_sha,
            "stableTag": "v1.2.3",
            "decision": "approved",
            "epicIssue": 1293,
            "approvedAt": "2026-08-03T01:00:00Z",
            "approvedBy": "ProhibitedTV",
            "gates": [
                {
                    "id": gate_id,
                    "issue": issue,
                    "status": "passed",
                    "candidateReleaseSetSha256": manifest_sha,
                    "evidenceUrl": (
                        "https://github.com/ProhibitedTV/BitRiver-Live/releases/"
                        f"download/{TAG}/{gate_id}.json"
                    ),
                    "evidenceSha256": f"{index:x}" * 64,
                }
                for index, (gate_id, issue) in enumerate(
                    release_set.REQUIRED_PROMOTION_GATES, 1
                )
            ],
        }

    def test_candidate_output_is_deterministic_and_verifiable(self) -> None:
        with tempfile.TemporaryDirectory() as first_dir, tempfile.TemporaryDirectory() as second_dir:
            first = Path(first_dir)
            second = Path(second_dir)
            first_manifest, first_markdown = self.make_payload(first)
            second_manifest, second_markdown = self.make_payload(second)

            self.assertEqual(first_manifest.read_bytes(), second_manifest.read_bytes())
            self.assertEqual(first_markdown.read_bytes(), second_markdown.read_bytes())
            manifest = release_set.verify_candidate_root(
                first, first_manifest, expected_tag=TAG, expected_commit=COMMIT
            )
            self.assertEqual(manifest["schemaVersion"], release_set.RELEASE_SET_SCHEMA)
            self.assertEqual(len(manifest["images"]), 5)
            self.assertEqual(len(manifest["remainingExternalGates"]), 8)

    def test_tampered_or_uncovered_assets_are_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            manifest, _ = self.make_payload(root)
            target = root / "bitriver-live-linux-amd64.tar.gz"
            target.write_text("tampered\n", encoding="utf-8")
            with self.assertRaisesRegex(release_set.ReleaseSetError, "checksum mismatch"):
                release_set.verify_candidate_root(root, manifest)

            release_set.write_checksums(root, root / "CHECKSUMS.txt")
            (root / "unexpected.bin").write_bytes(b"not covered")
            with self.assertRaisesRegex(release_set.ReleaseSetError, "coverage mismatch"):
                release_set.verify_candidate_root(root, manifest)

    def test_missing_signature_and_unsafe_manifest_names_are_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            missing = root / f"bitriver-live-{TAG}.image.sigstore.json"
            for name in release_set.required_candidate_names(TAG):
                (root / name).write_text("fixture\n", encoding="utf-8")
            missing.unlink()
            # Fill only enough evidence to prove the payload completeness check fails first.
            with self.assertRaisesRegex(release_set.ReleaseSetError, "missing required assets"):
                release_set.create_candidate_manifest(
                    argparse.Namespace(
                        tag=TAG,
                        commit=COMMIT,
                        repository=REPOSITORY,
                        run_id=1,
                        run_attempt=1,
                        workflow_url="https://github.com/a/b/actions/runs/1",
                        created_at="2026-08-03T00:00:00Z",
                        go_version="go1.26.5",
                        node_version="v24",
                        cosign_version="v3",
                        assets_dir=root,
                        output=root / "release-set.json",
                        markdown_output=root / "release-set.md",
                    )
                )

        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            manifest_path, _ = self.make_payload(root)
            manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
            manifest["artifacts"][0]["name"] = "../escape"
            self.write_json(manifest_path, manifest)
            release_set.write_checksums(root, root / "CHECKSUMS.txt")
            with self.assertRaisesRegex(release_set.ReleaseSetError, "unsafe artifact name"):
                release_set.verify_candidate_root(root, manifest_path)

    def test_promotion_record_binds_all_gates_to_one_manifest(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            manifest_path, _ = self.make_payload(root)
            manifest = release_set.verify_candidate_root(root, manifest_path)
            record = self.promotion_record(manifest_path)
            release_set.validate_promotion_record(record, manifest_path, manifest)

            record["gates"] = record["gates"][:-1]
            with self.assertRaisesRegex(release_set.ReleaseSetError, "gates must be exactly"):
                release_set.validate_promotion_record(record, manifest_path, manifest)

            record = self.promotion_record(manifest_path)
            record["gates"][0]["candidateReleaseSetSha256"] = "f" * 64
            with self.assertRaisesRegex(release_set.ReleaseSetError, "targets another candidate"):
                release_set.validate_promotion_record(record, manifest_path, manifest)

    def test_revocation_is_append_only_and_blocks_candidate_verification(self) -> None:
        with tempfile.TemporaryDirectory() as directory, tempfile.TemporaryDirectory() as marker_directory:
            root = Path(directory)
            manifest_path, _ = self.make_payload(root)
            marker = Path(marker_directory) / "candidate-revocation.json"
            release_set.create_revocation(
                argparse.Namespace(
                    candidate_manifest=manifest_path,
                    candidate_tag=TAG,
                    reason="Production canary found a release blocker.",
                    actor="ProhibitedTV",
                    revoked_at="2026-08-03T02:00:00Z",
                    workflow_url=(
                        "https://github.com/ProhibitedTV/BitRiver-Live/actions/runs/456"
                    ),
                    output=marker,
                )
            )
            value = json.loads(marker.read_text(encoding="utf-8"))
            self.assertFalse(value["stableStateChanged"])
            with redirect_stderr(io.StringIO()):
                status = release_set.main(
                    [
                        "verify-candidate",
                        "--assets-dir",
                        str(root),
                        "--manifest",
                        str(manifest_path),
                        "--revocation-marker",
                        str(marker),
                    ]
                )
            self.assertEqual(status, 2)

    def test_stable_and_first_release_rollback_metadata(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            manifest_path, _ = self.make_payload(root)
            record_path = root / "v1.2.3-promotion.json"
            self.write_json(record_path, self.promotion_record(manifest_path))
            stable_path = root / "stable-release-set.json"
            rollback_path = root / "rollback-release-set.json"
            markdown_path = root / "stable-release-set.md"
            release_set.create_stable_metadata(
                argparse.Namespace(
                    candidate_manifest=manifest_path,
                    promotion_record=record_path,
                    previous_stable_manifest=None,
                    publish_latest=True,
                    output=stable_path,
                    markdown_output=markdown_path,
                    rollback_output=rollback_path,
                )
            )
            stable = json.loads(stable_path.read_text(encoding="utf-8"))
            rollback = json.loads(rollback_path.read_text(encoding="utf-8"))
            self.assertEqual(stable["artifactPolicy"], "candidate-assets-copied-byte-for-byte")
            self.assertEqual(stable["promotion"]["approvedBy"], "ProhibitedTV")
            self.assertIsNone(stable["previousStable"])
            self.assertFalse(rollback["rollbackAvailable"])
            self.assertIn("first stable", rollback["reason"])

            second_stable = root / "second-stable-release-set.json"
            second_rollback = root / "second-rollback-release-set.json"
            second_markdown = root / "second-stable-release-set.md"
            release_set.create_stable_metadata(
                argparse.Namespace(
                    candidate_manifest=manifest_path,
                    promotion_record=record_path,
                    previous_stable_manifest=None,
                    publish_latest=True,
                    output=second_stable,
                    markdown_output=second_markdown,
                    rollback_output=second_rollback,
                )
            )
            self.assertEqual(stable_path.read_bytes(), second_stable.read_bytes())
            self.assertEqual(rollback_path.read_bytes(), second_rollback.read_bytes())
            self.assertEqual(markdown_path.read_bytes(), second_markdown.read_bytes())

    def test_existing_state_is_idempotent_but_never_overwrites(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            manifest_path, _ = self.make_payload(root)
            manifest = release_set.verify_candidate_root(root, manifest_path)
            self.assertEqual(
                release_set.classify_existing_state(
                    manifest, root, {"stableRefCommit": None, "release": None}
                ),
                "new",
            )
            self.assertEqual(
                release_set.classify_existing_state(
                    manifest, root, {"stableRefCommit": COMMIT, "release": None}
                ),
                "resume",
            )
            assets = [
                {
                    "name": path.name,
                    "digest": "sha256:" + release_set.sha256_file(path),
                }
                for path in release_set.list_flat_files(root)
            ]
            complete = {
                "stableRefCommit": COMMIT,
                "release": {
                    "targetCommit": COMMIT,
                    "draft": False,
                    "assets": assets,
                },
            }
            self.assertEqual(
                release_set.classify_existing_state(manifest, root, complete),
                "complete",
            )
            complete["stableRefCommit"] = "b" * 40
            with self.assertRaisesRegex(release_set.ReleaseSetError, "another commit"):
                release_set.classify_existing_state(manifest, root, complete)

    def test_candidate_and_stable_tag_contract(self) -> None:
        self.assertEqual(release_set.stable_tag_for(TAG), "v1.2.3")
        with self.assertRaisesRegex(release_set.ReleaseSetError, "prerelease"):
            release_set.stable_tag_for("v1.2.3")
        with self.assertRaisesRegex(release_set.ReleaseSetError, "stable tag"):
            release_set.parse_tag(TAG, prerelease=False)


if __name__ == "__main__":
    unittest.main()
