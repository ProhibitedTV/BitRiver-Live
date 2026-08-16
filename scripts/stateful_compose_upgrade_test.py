import copy
import hashlib
import importlib.util
import json
import sys
import tempfile
import unittest
from pathlib import Path


SCRIPT_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPT_DIR))
SPEC = importlib.util.spec_from_file_location(
    "stateful_compose_upgrade", SCRIPT_DIR / "stateful_compose_upgrade.py"
)
assert SPEC is not None and SPEC.loader is not None
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)

import prepare_release_candidate as candidate  # noqa: E402


class StatefulComposeUpgradePreparationTests(unittest.TestCase):
    namespace = "ghcr.io/prohibitedtv"
    source_tag = "v1.2.3-rc.19"
    source_commit = "1" * 40
    candidate_tag = "v1.2.3-rc.20"
    candidate_commit = "2" * 40

    def release_set(self, tag: str, commit: str, salt: str) -> dict:
        template = (SCRIPT_DIR.parent / "deploy" / ".env.example").read_text(
            encoding="utf-8"
        )
        _, values = candidate.parse_env_template(template)
        return {
            "schemaVersion": "bitriver.release-set/v1",
            "candidate": {"tag": tag, "sourceCommit": commit},
            "images": [
                {
                    "name": name,
                    "envKey": env_key,
                    "digest": "sha256:" + hashlib.sha256(
                        f"{salt}:{name}".encode()
                    ).hexdigest(),
                    "candidateReference": f"{self.namespace}/{name}:{tag}",
                    "immutableReference": f"{self.namespace}/{name}@sha256:"
                    + hashlib.sha256(f"{salt}:{name}".encode()).hexdigest(),
                }
                for env_key, name in candidate.FIRST_PARTY_IMAGES
            ],
            "dependencies": [
                {
                    "envKey": env_key,
                    "reference": image.format_map(values),
                    "digest": "sha256:"
                    + hashlib.sha256(image.format_map(values).encode()).hexdigest(),
                }
                for env_key, image in candidate.THIRD_PARTY_IMAGES
            ],
        }

    def write_json(self, path: Path, value: dict) -> str:
        content = json.dumps(value, sort_keys=True) + "\n"
        path.write_text(content, encoding="utf-8")
        return MODULE.sha256_file(path)

    def prepare(self, root: Path, source: dict, target: dict, **overrides) -> None:
        source_path = root / "source.json"
        target_path = root / "candidate.json"
        source_sha = self.write_json(source_path, source)
        target_sha = self.write_json(target_path, target)
        arguments = {
            "source_release_set": source_path,
            "candidate_release_set": target_path,
            "source_template": SCRIPT_DIR.parent / "deploy" / ".env.example",
            "source_output": root / "source.env",
            "candidate_output": root / "candidate.env",
            "sentinel_output": root / "sentinels",
            "metadata_output": root / "metadata.json",
            "source_tag": self.source_tag,
            "source_commit": self.source_commit,
            "source_sha256": source_sha,
            "candidate_tag": self.candidate_tag,
            "candidate_commit": self.candidate_commit,
            "candidate_sha256": target_sha,
            "namespace": self.namespace,
            "secret_factory": lambda key: f"generated-{key.lower()}-fixture",
        }
        arguments.update(overrides)
        MODULE.prepare_upgrade_pair(**arguments)

    def test_prepares_credential_stable_digest_bound_pair(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source = self.release_set(self.source_tag, self.source_commit, "source")
            target = self.release_set(self.candidate_tag, self.candidate_commit, "target")
            target["dependencies"] = copy.deepcopy(source["dependencies"])
            self.prepare(root, source, target)
            source_env = (root / "source.env").read_text(encoding="utf-8")
            target_env = (root / "candidate.env").read_text(encoding="utf-8")
            self.assertIn("BITRIVER_LIVE_IMAGE_TAG=v1.2.3-rc.19", source_env)
            self.assertIn("BITRIVER_LIVE_IMAGE_TAG=v1.2.3-rc.20", target_env)
            self.assertIn("BITRIVER_LIVE_PORT=18080", source_env)
            self.assertIn("BITRIVER_TRANSCODE_LADDER=1080p:2500", target_env)
            metadata = json.loads((root / "metadata.json").read_text())
            self.assertTrue(metadata["credentialsStableAcrossPair"])
            for key in candidate.SAMPLE_SECRET_KEYS:
                source_line = next(
                    line for line in source_env.splitlines() if line.startswith(f"{key}=")
                )
                self.assertIn(source_line, target_env.splitlines())

    def test_rejects_release_set_hash_mismatch(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source = self.release_set(self.source_tag, self.source_commit, "source")
            target = self.release_set(self.candidate_tag, self.candidate_commit, "target")
            target["dependencies"] = copy.deepcopy(source["dependencies"])
            with self.assertRaisesRegex(MODULE.UpgradePreparationError, "SHA-256 mismatch"):
                self.prepare(root, source, target, source_sha256="0" * 64)

    def test_rejects_missing_first_party_image(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source = self.release_set(self.source_tag, self.source_commit, "source")
            target = self.release_set(self.candidate_tag, self.candidate_commit, "target")
            target["dependencies"] = copy.deepcopy(source["dependencies"])
            target["images"].pop()
            with self.assertRaisesRegex(MODULE.UpgradePreparationError, "every first-party image"):
                self.prepare(root, source, target)

    def test_applies_candidate_dependency_digest_change(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source = self.release_set(self.source_tag, self.source_commit, "source")
            target = self.release_set(self.candidate_tag, self.candidate_commit, "target")
            target["dependencies"][0]["digest"] = "sha256:" + "f" * 64
            self.prepare(root, source, target)
            target_env = (root / "candidate.env").read_text(encoding="utf-8")
            changed_key = target["dependencies"][0]["envKey"]
            self.assertIn(f"{changed_key}=@sha256:{'f' * 64}", target_env)
            metadata = json.loads((root / "metadata.json").read_text())
            self.assertEqual(
                metadata["changedDependencies"],
                [target["dependencies"][0]["reference"]],
            )


if __name__ == "__main__":
    unittest.main()
