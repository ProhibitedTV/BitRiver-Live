#!/usr/bin/env python3

from __future__ import annotations

import importlib.util
import io
import os
import stat
import sys
import tempfile
import unittest
from contextlib import redirect_stderr, redirect_stdout
from pathlib import Path


SCRIPT_PATH = Path(__file__).with_name("prepare_release_candidate.py")
SPEC = importlib.util.spec_from_file_location("prepare_release_candidate", SCRIPT_PATH)
assert SPEC is not None and SPEC.loader is not None
candidate = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = candidate
SPEC.loader.exec_module(candidate)


class ReleaseMetadataTests(unittest.TestCase):
    def test_stable_tag_metadata(self) -> None:
        metadata = candidate.parse_tag("v1.2.3")
        self.assertEqual(metadata.version, "1.2.3")
        self.assertEqual(metadata.msi_version, "1.2.3")
        self.assertFalse(metadata.is_prerelease)
        self.assertTrue(metadata.publish_latest)
        self.assertEqual(metadata.nfpm_prerelease, "")

    def test_release_candidate_metadata(self) -> None:
        metadata = candidate.parse_tag("v1.2.3-rc.1")
        self.assertEqual(metadata.version, "1.2.3")
        self.assertEqual(metadata.prerelease, "rc.1")
        self.assertTrue(metadata.is_prerelease)
        self.assertFalse(metadata.publish_latest)
        self.assertEqual(metadata.github_outputs()["is_prerelease"], "true")

    def test_invalid_tags_are_rejected(self) -> None:
        for tag in (
            "1.2.3",
            "v1.2",
            "v01.2.3",
            "v1.2.3-rc.01",
            "v1.2.3+build",
            "release-v1.2.3",
        ):
            with self.subTest(tag=tag), self.assertRaises(candidate.CandidateError):
                candidate.parse_tag(tag)


class ReleaseEnvironmentTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.template_path = SCRIPT_PATH.parents[1] / "deploy" / ".env.example"
        cls.template = cls.template_path.read_text(encoding="utf-8")

    def setUp(self) -> None:
        self.secret_counter = 0

    def secret_factory(self, label: str) -> str:
        self.secret_counter += 1
        return f"Generated!9-{self.secret_counter:02d}-{label}"

    @staticmethod
    def digest_resolver(image: str) -> str:
        digit = "1" if image.startswith("redis:") else "2"
        return "sha256:" + digit * 64

    def test_environment_rotates_samples_and_resolves_digests(self) -> None:
        metadata = candidate.parse_tag("v1.2.3-rc.1")
        rendered, sentinels = candidate.prepare_environment(
            self.template,
            metadata,
            "ghcr.io/prohibitedtv",
            resolve_digests=True,
            digest_resolver=self.digest_resolver,
            digest_attempts=1,
            digest_delay_seconds=0,
            secret_factory=self.secret_factory,
            product_loopback=True,
        )

        self.assertIn("BITRIVER_IMAGE_NAMESPACE=ghcr.io/prohibitedtv", rendered)
        self.assertIn("BITRIVER_DEPLOY_IMAGE_SOURCE=pull", rendered)
        self.assertIn("BITRIVER_LIVE_MODE=production", rendered)
        self.assertIn("BITRIVER_LIVE_ALLOW_SELF_SIGNUP=true", rendered)
        self.assertIn("BITRIVER_OME_USERNAME=release-validator", rendered)
        for key in candidate.FIRST_PARTY_TAG_KEYS:
            self.assertIn(f"{key}=v1.2.3-rc.1", rendered)
        for key, _ in candidate.THIRD_PARTY_IMAGES:
            self.assertRegex(rendered, rf"{key}=@sha256:[12]{{64}}")
        for sample in (
            "P0stgres-Example!",
            "R3dis-Example!",
            "Sup3rSecureAdmin-Example!",
            "OME-Example-Access-Token",
        ):
            self.assertNotIn(sample, rendered)
        self.assertGreaterEqual(len(sentinels), 8)
        self.assertTrue(all(value in rendered for value in sentinels))
        self.assertEqual(
            self._env_value(rendered, "BITRIVER_REDIS_PASSWORD"),
            self._env_value(
                rendered, "BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD"
            ),
        )

    def test_contract_profile_uses_non_loopback_public_urls(self) -> None:
        rendered, _ = candidate.prepare_environment(
            self.template,
            candidate.parse_tag("v1.2.3"),
            "ghcr.io/prohibitedtv",
            resolve_digests=False,
            secret_factory=self.secret_factory,
        )
        self.assertIn(
            "BITRIVER_SRS_PUBLIC_RTMP_BASE_URL=rtmp://ingest.release-validator.invalid:1935/live",
            rendered,
        )
        self.assertIn(
            "BITRIVER_OME_PUBLIC_LLHLS_BASE_URL=http://stream.release-validator.invalid/live",
            rendered,
        )
        self.assertNotIn("NEXT_PUBLIC_API_BASE_URL=\n", rendered)

    def test_unpublished_contract_profile_uses_format_only_first_party_digests(self) -> None:
        rendered, _ = candidate.prepare_environment(
            self.template,
            candidate.parse_tag("v1.2.3-rc.1"),
            "ghcr.io/prohibitedtv",
            resolve_digests=False,
            unpublished_first_party_digests=True,
            secret_factory=self.secret_factory,
        )
        for digest_key, _ in candidate.FIRST_PARTY_IMAGES:
            self.assertRegex(rendered, rf"{digest_key}=@sha256:[a-f0-9]{{64}}")

        with self.assertRaisesRegex(candidate.CandidateError, "mutually exclusive"):
            candidate.prepare_environment(
                self.template,
                candidate.parse_tag("v1.2.3-rc.1"),
                "ghcr.io/prohibitedtv",
                resolve_digests=False,
                unpublished_first_party_digests=True,
                first_party_digests={
                    key: "sha256:" + "a" * 64
                    for key, _ in candidate.FIRST_PARTY_IMAGES
                },
                secret_factory=self.secret_factory,
            )

    def test_missing_template_key_is_rejected(self) -> None:
        incomplete = self.template.replace("BITRIVER_SRS_IMAGE_TAG=v5.0.185\n", "")
        with self.assertRaisesRegex(candidate.CandidateError, "BITRIVER_SRS_IMAGE_TAG"):
            candidate.prepare_environment(
                incomplete,
                candidate.parse_tag("v1.2.3"),
                "ghcr.io/prohibitedtv",
                resolve_digests=False,
                secret_factory=self.secret_factory,
            )

    def test_invalid_namespace_and_digest_are_rejected(self) -> None:
        metadata = candidate.parse_tag("v1.2.3")
        with self.assertRaisesRegex(candidate.CandidateError, "lowercase"):
            candidate.prepare_environment(
                self.template,
                metadata,
                "ghcr.io/ProhibitedTV",
                resolve_digests=False,
                secret_factory=self.secret_factory,
            )
        with self.assertRaisesRegex(candidate.CandidateError, "invalid digest"):
            candidate.prepare_environment(
                self.template,
                metadata,
                "ghcr.io/prohibitedtv",
                resolve_digests=True,
                digest_resolver=lambda _: "not-a-digest",
                digest_attempts=1,
                digest_delay_seconds=0,
                secret_factory=self.secret_factory,
            )

    def test_first_party_images_are_proven_and_applied(self) -> None:
        metadata = candidate.parse_tag("v1.2.3-rc.1")
        evidence = candidate.resolve_first_party_images(
            metadata,
            "ghcr.io/prohibitedtv",
            digest_resolver=lambda image: "sha256:" + (
                "a" if image.endswith("bitriver-live:v1.2.3-rc.1") else "b"
            )
            * 64,
            digest_attempts=1,
            digest_delay_seconds=0,
        )
        self.assertEqual(evidence["schemaVersion"], "bitriver.release-images/v1")
        self.assertTrue(evidence["anonymousManifestAccess"])
        self.assertEqual(len(evidence["images"]), 5)
        digest_values = {
            image["envKey"]: image["digest"] for image in evidence["images"]
        }
        rendered, _ = candidate.prepare_environment(
            self.template,
            metadata,
            "ghcr.io/prohibitedtv",
            resolve_digests=False,
            first_party_digests=digest_values,
            secret_factory=self.secret_factory,
        )
        for digest_key, _ in candidate.FIRST_PARTY_IMAGES:
            self.assertRegex(rendered, rf"{digest_key}=@sha256:[ab]{{64}}")

    def test_first_party_evidence_must_match_candidate(self) -> None:
        metadata = candidate.parse_tag("v1.2.3-rc.1")
        with tempfile.TemporaryDirectory() as directory:
            evidence_path = Path(directory) / "images.json"
            evidence_path.write_text(
                '{"schemaVersion":"bitriver.release-images/v1",'
                '"tag":"v1.2.3-rc.2","namespace":"ghcr.io/prohibitedtv",'
                '"anonymousManifestAccess":true,"images":[]}',
                encoding="utf-8",
            )
            with self.assertRaisesRegex(candidate.CandidateError, "does not match"):
                candidate.read_first_party_evidence(
                    evidence_path, metadata, "ghcr.io/prohibitedtv"
                )

    def test_cli_writes_private_separate_files_without_printing_values(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            env_output = root / "candidate.env"
            sentinel_output = root / "candidate.sentinels"
            stdout = io.StringIO()
            stderr = io.StringIO()
            with redirect_stdout(stdout), redirect_stderr(stderr):
                status = candidate.main(
                    [
                        "env",
                        "--tag",
                        "v1.2.3-rc.1",
                        "--namespace",
                        "ghcr.io/prohibitedtv",
                        "--template",
                        str(self.template_path),
                        "--output",
                        str(env_output),
                        "--sentinel-output",
                        str(sentinel_output),
                    ]
                )
            self.assertEqual(status, 0, stderr.getvalue())
            env_content = env_output.read_text(encoding="utf-8")
            sentinel_content = sentinel_output.read_text(encoding="utf-8")
            for secret in sentinel_content.splitlines():
                self.assertIn(secret, env_content)
                self.assertNotIn(secret, stdout.getvalue())
                self.assertNotIn(secret, stderr.getvalue())
            if os.name != "nt":
                self.assertEqual(stat.S_IMODE(env_output.stat().st_mode), 0o600)
                self.assertEqual(stat.S_IMODE(sentinel_output.stat().st_mode), 0o600)

    @staticmethod
    def _env_value(content: str, key: str) -> str:
        prefix = key + "="
        return next(line[len(prefix) :] for line in content.splitlines() if line.startswith(prefix))


if __name__ == "__main__":
    unittest.main(verbosity=2)
