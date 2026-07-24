import json
import tempfile
import unittest
from pathlib import Path

import production_golden_path as golden


class SanitizationTests(unittest.TestCase):
    def test_sanitize_url_removes_credentials_query_and_fragment(self):
        self.assertEqual(
            golden.sanitize_url(
                "https://operator:private@example.com:8443/live/main.m3u8"
                "?access_token=secret#part"
            ),
            "https://example.com:8443/live/main.m3u8",
        )

    def test_sanitize_text_redacts_plain_and_encoded_sentinels(self):
        sentinel = "sk_live/a+b"
        value = (
            f"publish rtmp://localhost/live/{sentinel} "
            f"or {golden.urllib.parse.quote(sentinel, safe='')}"
        )
        cleaned = golden.sanitize_text(value, [sentinel])
        self.assertNotIn(sentinel, cleaned)
        self.assertNotIn(golden.urllib.parse.quote(sentinel, safe=""), cleaned)
        self.assertIn("[REDACTED]", cleaned)

    def test_rewrite_media_host_preserves_port_path_and_query(self):
        self.assertEqual(
            golden.rewrite_media_host(
                "http://localhost:9080/hls/live/master.m3u8?session=one",
                "host.docker.internal",
            ),
            "http://host.docker.internal:9080/hls/live/master.m3u8?session=one",
        )
        self.assertEqual(
            golden.rewrite_media_host(
                "https://media.example/live/master.m3u8",
                "host.docker.internal",
            ),
            "https://media.example/live/master.m3u8",
        )


class PollingTests(unittest.TestCase):
    def test_bounded_poll_returns_first_truthy_value(self):
        attempts = iter([None, None, {"ready": True}])
        self.assertEqual(
            golden.bounded_poll(
                "test readiness", 1, lambda: next(attempts), interval=0
            ),
            {"ready": True},
        )

    def test_bounded_poll_names_deadline_and_last_state(self):
        with self.assertRaisesRegex(
            golden.GoldenPathError,
            r"test readiness did not become ready within 0s; last state:",
        ):
            golden.bounded_poll(
                "test readiness", 0.001, lambda: None, interval=0
            )


class PlaylistTests(unittest.TestCase):
    def test_playlist_helpers_cover_segments_and_llhls_parts(self):
        manifest = """#EXTM3U
#EXT-X-MEDIA-SEQUENCE:42
#EXT-X-PART:DURATION=0.2,URI="part-42.m4s"
#EXTINF:2.0,
segment-42.ts
"""
        self.assertEqual(golden.media_sequence(manifest), 42)
        self.assertEqual(golden.playlist_uris(manifest), ["segment-42.ts"])
        self.assertEqual(golden.playlist_part_uris(manifest), ["part-42.m4s"])

    def test_select_transcoder_manifest_prefers_1080p(self):
        self.assertEqual(
            golden.select_transcoder_manifest(
                {
                    "renditions": [
                        {"name": "720p", "manifestUrl": "https://cdn/720.m3u8"},
                        {"name": "1080p", "manifestUrl": "https://cdn/1080.m3u8"},
                    ]
                }
            ),
            "https://cdn/1080.m3u8",
        )


class EvidenceTests(unittest.TestCase):
    def test_failure_report_is_versioned_and_contains_no_sentinel(self):
        sentinel = "private-runtime-value"
        with tempfile.TemporaryDirectory() as temp:
            report = Path(temp) / "evidence.json"
            evidence = golden.Evidence(report, [sentinel])
            with self.assertRaisesRegex(RuntimeError, sentinel):
                with evidence.stage("media"):
                    raise RuntimeError(f"failed with {sentinel}")

            payload = json.loads(report.read_text(encoding="utf-8"))
            self.assertEqual(payload["schema"], golden.REPORT_SCHEMA)
            self.assertEqual(payload["status"], "failed")
            self.assertEqual(payload["failedStage"], "media")
            self.assertNotIn(sentinel, report.read_text(encoding="utf-8"))


if __name__ == "__main__":
    unittest.main()
