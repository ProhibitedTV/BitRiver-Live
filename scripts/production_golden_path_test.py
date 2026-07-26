import json
import http.server
import tempfile
import threading
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


class ProductClientTests(unittest.TestCase):
    def test_secure_session_cookie_uses_same_origin_bearer_fallback(self):
        token = "test-session-token"
        primary_headers = []
        cross_origin_headers = []

        class PrimaryHandler(http.server.BaseHTTPRequestHandler):
            def do_POST(self):
                self.send_response(201)
                self.send_header(
                    "Set-Cookie",
                    "bitriver_session="
                    f"{token}; Path=/; Secure; HttpOnly; SameSite=Strict",
                )
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(b'{"user":{"id":"creator"}}')

            def do_GET(self):
                primary_headers.append(self.headers.get("Authorization"))
                if self.headers.get("Authorization") not in {
                    f"Bearer {token}",
                    "Bearer explicit-test-token",
                }:
                    self.send_response(401)
                    self.end_headers()
                    return
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(b'{"status":"ready"}')

            def log_message(self, _format, *_args):
                return

        class CrossOriginHandler(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                cross_origin_headers.append(
                    self.headers.get("Authorization")
                )
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(b'{"status":"public"}')

            def log_message(self, _format, *_args):
                return

        primary = http.server.ThreadingHTTPServer(
            ("127.0.0.1", 0), PrimaryHandler
        )
        cross_origin = http.server.ThreadingHTTPServer(
            ("127.0.0.1", 0), CrossOriginHandler
        )
        threads = [
            threading.Thread(target=server.serve_forever, daemon=True)
            for server in (primary, cross_origin)
        ]
        for thread in threads:
            thread.start()
        try:
            observed_tokens = []
            base_url = f"http://127.0.0.1:{primary.server_port}"
            client = golden.ProductClient(
                base_url, 2.0, observed_tokens.append
            )
            client.request(
                "/api/auth/signup",
                method="POST",
                payload={"displayName": "Creator"},
                expected=(201,),
            )
            self.assertEqual(
                client.request("/api/status"), {"status": "ready"}
            )
            self.assertEqual(
                client.request(
                    "/api/status",
                    headers={"Authorization": "Bearer explicit-test-token"},
                ),
                {"status": "ready"},
            )
            client.request(
                f"http://127.0.0.1:{cross_origin.server_port}/public"
            )
        finally:
            primary.shutdown()
            cross_origin.shutdown()
            primary.server_close()
            cross_origin.server_close()
            for thread in threads:
                thread.join(timeout=2)

        self.assertEqual(
            primary_headers,
            [f"Bearer {token}", "Bearer explicit-test-token"],
        )
        self.assertEqual(cross_origin_headers, [None])
        self.assertEqual(observed_tokens, [token])


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
