import copy
import contextlib
import hashlib
import http.server
import io
import json
import tempfile
import threading
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest import mock

import capacity_qualification as capacity


SCRIPT_DIR = Path(__file__).resolve().parent
SCENARIO_PATH = SCRIPT_DIR / "capacity-scenarios" / "rc-qualification-small.json"
RELEASE_SET_SHA256 = "d" * 64
SOURCE_COMMIT = "a" * 40


def release_set_payload(tag="v1.2.3-rc.20", commit=SOURCE_COMMIT):
    images = []
    for index, name in enumerate(sorted(capacity.FIRST_PARTY_IMAGE_NAMES), 1):
        digest = "sha256:" + str(index) * 64
        candidate_reference = f"ghcr.io/prohibitedtv/{name}:{tag}"
        images.append(
            {
                "name": name,
                "candidateReference": candidate_reference,
                "immutableReference": (
                    candidate_reference.rsplit(":", 1)[0] + "@" + digest
                ),
                "digest": digest,
            }
        )
    return {
        "schemaVersion": "bitriver.release-set/v1",
        "candidate": {
            "tag": tag,
            "sourceCommit": commit,
            "repository": "ProhibitedTV/BitRiver-Live",
        },
        "images": images,
        "integrity": {
            "manifestSignature": {"asset": "release-set.sigstore.json"}
        },
    }


def write_release_set(root: Path, payload=None):
    path = root / "release-set.json"
    path.write_text(
        json.dumps(payload or release_set_payload(), sort_keys=True),
        encoding="utf-8",
    )
    return path, hashlib.sha256(path.read_bytes()).hexdigest()


class ReleaseSetTests(unittest.TestCase):
    def test_exact_release_set_bytes_and_inventory_are_bound(self):
        with tempfile.TemporaryDirectory() as temp:
            path, digest = write_release_set(Path(temp))
            identity = capacity.CandidateIdentity.parse(
                "v1.2.3-rc.20", digest, SOURCE_COMMIT
            )
            result = capacity.load_release_set(path, identity)
        self.assertEqual(result["status"], "verified")
        self.assertEqual(result["sha256"], digest)
        self.assertEqual(len(result["images"]), 5)
        self.assertEqual(result["runtimeImageMatch"], "not-collected")

    def test_release_set_rejects_wrong_file_hash(self):
        with tempfile.TemporaryDirectory() as temp:
            path, _ = write_release_set(Path(temp))
            identity = capacity.CandidateIdentity.parse(
                "v1.2.3-rc.20", RELEASE_SET_SHA256, SOURCE_COMMIT
            )
            with self.assertRaisesRegex(capacity.CapacityError, "SHA-256"):
                capacity.load_release_set(path, identity)

    def test_release_set_rejects_wrong_candidate_identity(self):
        payload = release_set_payload(commit="b" * 40)
        with tempfile.TemporaryDirectory() as temp:
            path, digest = write_release_set(Path(temp), payload)
            identity = capacity.CandidateIdentity.parse(
                "v1.2.3-rc.20", digest, SOURCE_COMMIT
            )
            with self.assertRaisesRegex(capacity.CapacityError, "source commit"):
                capacity.load_release_set(path, identity)

    def test_release_set_rejects_incomplete_or_inconsistent_images(self):
        cases = []
        missing = release_set_payload()
        missing["images"].pop()
        cases.append(missing)
        inconsistent = release_set_payload()
        inconsistent["images"][0]["immutableReference"] += "bad"
        cases.append(inconsistent)
        wrong_tag = release_set_payload()
        wrong_tag["images"][0]["candidateReference"] = (
            "ghcr.io/prohibitedtv/bitriver-live:v1.2.3-rc.19"
        )
        cases.append(wrong_tag)
        for payload in cases:
            with self.subTest(payload=payload["images"]):
                with tempfile.TemporaryDirectory() as temp:
                    path, digest = write_release_set(Path(temp), payload)
                    identity = capacity.CandidateIdentity.parse(
                        "v1.2.3-rc.20", digest, SOURCE_COMMIT
                    )
                    with self.assertRaises(capacity.CapacityError):
                        capacity.load_release_set(path, identity)


class ScenarioTests(unittest.TestCase):
    def load_payload(self):
        return json.loads(SCENARIO_PATH.read_text(encoding="utf-8"))

    def write_payload(self, root: Path, payload):
        path = root / "scenario.json"
        path.write_text(json.dumps(payload), encoding="utf-8")
        return path

    def test_checked_in_scenario_is_bounded_and_complete(self):
        scenario = capacity.load_scenario(SCENARIO_PATH)
        self.assertEqual(scenario.name, "rc-qualification-small")
        self.assertEqual(
            [phase.name for phase in scenario.phases],
            ["warm-up", "steady-state", "spike", "soak"],
        )
        self.assertEqual(scenario.duration_seconds, 240)
        self.assertEqual(scenario.max_publishers, 2)
        self.assertEqual(scenario.max_viewers, 12)
        self.assertRegex(scenario.sha256, r"^[0-9a-f]{64}$")

    def test_hash_is_canonical_across_json_formatting(self):
        payload = self.load_payload()
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            first = self.write_payload(root, payload)
            expected = capacity.load_scenario(first).sha256
            first.write_text(
                json.dumps(payload, sort_keys=True, indent=7) + "\n",
                encoding="utf-8",
            )
            self.assertEqual(capacity.load_scenario(first).sha256, expected)

    def test_unknown_fields_and_missing_stop_condition_are_rejected(self):
        payload = self.load_payload()
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            extra = copy.deepcopy(payload)
            extra["unsafeOverride"] = True
            with self.assertRaisesRegex(capacity.CapacityError, "unknown keys"):
                capacity.load_scenario(self.write_payload(root, extra))

            missing = copy.deepcopy(payload)
            del missing["stopConditions"]["maxErrorRate"]
            with self.assertRaisesRegex(capacity.CapacityError, "missing keys"):
                capacity.load_scenario(self.write_payload(root, missing))

    def test_workload_and_duration_hard_caps_are_rejected(self):
        payload = self.load_payload()
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            viewers = copy.deepcopy(payload)
            viewers["phases"][0]["publishers"] = capacity.MAX_PUBLISHERS
            viewers["phases"][0]["viewersPerPublisher"] = capacity.MAX_VIEWERS
            with self.assertRaisesRegex(capacity.CapacityError, "hard cap"):
                capacity.load_scenario(self.write_payload(root, viewers))

            duration = copy.deepcopy(payload)
            duration["phases"] = [
                {
                    **duration["phases"][0],
                    "name": f"phase-{index}",
                    "durationSeconds": capacity.MAX_PHASE_SECONDS,
                }
                for index in range(5)
            ]
            with self.assertRaisesRegex(capacity.CapacityError, "duration"):
                capacity.load_scenario(self.write_payload(root, duration))

    def test_duplicate_phase_names_are_rejected(self):
        payload = self.load_payload()
        payload["phases"][1]["name"] = payload["phases"][0]["name"]
        with tempfile.TemporaryDirectory() as temp:
            with self.assertRaisesRegex(capacity.CapacityError, "duplicate phase"):
                capacity.load_scenario(
                    self.write_payload(Path(temp), payload)
                )


class CandidateIdentityTests(unittest.TestCase):
    def test_prerelease_identity_is_normalized(self):
        identity = capacity.CandidateIdentity.parse(
            " v1.2.3-rc.20 ", RELEASE_SET_SHA256.upper(), SOURCE_COMMIT.upper()
        )
        self.assertEqual(identity.release, "v1.2.3-rc.20")
        self.assertEqual(identity.release_set_sha256, RELEASE_SET_SHA256)
        self.assertEqual(identity.source_commit, SOURCE_COMMIT)

    def test_stable_tag_and_short_hashes_are_rejected(self):
        with self.assertRaisesRegex(capacity.CapacityError, "prerelease"):
            capacity.CandidateIdentity.parse(
                "v1.2.3", RELEASE_SET_SHA256, SOURCE_COMMIT
            )
        with self.assertRaisesRegex(capacity.CapacityError, "64 lowercase"):
            capacity.CandidateIdentity.parse("v1.2.3-rc.20", "abc", SOURCE_COMMIT)
        with self.assertRaisesRegex(capacity.CapacityError, "40 lowercase"):
            capacity.CandidateIdentity.parse(
                "v1.2.3-rc.20", RELEASE_SET_SHA256, "abc"
            )


class ParserTests(unittest.TestCase):
    def test_network_urls_reject_files_credentials_and_ambiguous_targets(self):
        self.assertEqual(
            capacity.require_network_url(
                "https://media.example/live/index.m3u8?token=opaque",
                "media",
                {"http", "https"},
                allow_query=True,
            ),
            "https://media.example/live/index.m3u8?token=opaque",
        )
        for value in (
            "file:///etc/passwd",
            "https://user:secret@media.example/live.m3u8",
            "https://media.example/live.m3u8#fragment",
        ):
            with self.subTest(value=value), self.assertRaises(
                capacity.CapacityError
            ):
                capacity.require_network_url(value, "media", {"http", "https"})

    def test_master_playlist_rejects_non_network_variant_before_fetch(self):
        master = "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1\nfile:///etc/passwd\n"
        with mock.patch.object(capacity.golden, "fetch_text", return_value=master) as fetch:
            with self.assertRaisesRegex(capacity.CapacityError, "media playlist URL"):
                capacity.resolve_safe_media_playlist(
                    "https://media.example/master.m3u8", 1
                )
        fetch.assert_called_once()

    def test_prometheus_parser_preserves_labels_and_numbers(self):
        samples = capacity.parse_prometheus(
            '# HELP ignored docs\n'
            'bitriver_http_requests_total{method="GET",path="/api/a\\\"b",status="200"} 12\n'
            "bitriver_transcoder_active_jobs 2.5\n"
        )
        self.assertEqual(len(samples), 2)
        self.assertEqual(samples[0]["labels"]["path"], '/api/a"b')
        self.assertEqual(samples[0]["value"], 12)
        self.assertEqual(samples[1]["labels"], {})
        self.assertEqual(samples[1]["value"], 2.5)

    def test_prometheus_parser_rejects_malformed_labels(self):
        with self.assertRaisesRegex(capacity.CapacityError, "labels"):
            capacity.parse_prometheus('metric{bad="one",broken} 1\n')

        with self.assertRaisesRegex(capacity.CapacityError, "non-finite"):
            capacity.parse_prometheus("metric NaN\n")

    def test_percent_and_byte_parsers_are_strict(self):
        self.assertEqual(capacity.parse_percent("12.5%"), 12.5)
        self.assertEqual(capacity.parse_byte_size("1.5 GiB"), int(1.5 * 1024**3))
        self.assertEqual(
            capacity.parse_byte_pair("512MiB / 2GiB"),
            (512 * 1024**2, 2 * 1024**3),
        )
        for invalid in ("12", "-1%", "NaN%"):
            with self.assertRaises(capacity.CapacityError):
                capacity.parse_percent(invalid)
        with self.assertRaises(capacity.CapacityError):
            capacity.parse_byte_size("12 parsecs")

    def test_latency_summary_uses_nearest_rank_percentiles(self):
        summary = capacity.latency_summary([1, 2, 3, 4, 100])
        self.assertEqual(summary["count"], 5)
        self.assertEqual(summary["p50Ms"], 3)
        self.assertEqual(summary["p95Ms"], 100)
        self.assertEqual(summary["maxMs"], 100)
        self.assertEqual(capacity.latency_summary([]), {"count": 0})

    def test_host_description_does_not_retain_operator_path(self):
        with tempfile.TemporaryDirectory() as temp:
            description = capacity.HostSampler(Path(temp)).describe()
        self.assertEqual(description["dataFilesystem"], "operator-supplied")
        self.assertNotIn(temp, json.dumps(description))


class StopEvaluatorTests(unittest.TestCase):
    def conditions(self, breach_samples=2):
        return capacity.StopConditions(
            max_error_rate=0.05,
            max_consecutive_health_failures=3,
            max_host_cpu_percent=90,
            max_host_memory_percent=90,
            min_host_disk_free_bytes=5 * 1024**3,
            max_container_memory_percent=95,
            threshold_breach_samples=breach_samples,
        )

    def test_persistent_thresholds_trigger_and_recovery_resets_counter(self):
        evaluator = capacity.StopEvaluator(self.conditions())
        high = {
            "healthy": True,
            "workload": {"attempts": 10, "errors": 1},
            "host": {
                "cpuPercent": 95,
                "memoryPercent": 50,
                "diskFreeBytes": 10 * 1024**3,
            },
            "containers": [{"memoryPercent": 96}],
        }
        self.assertEqual(evaluator.evaluate(high), [])
        reasons = evaluator.evaluate(high)
        self.assertTrue(any("error rate" in reason for reason in reasons))
        self.assertTrue(any("hostCPU" in reason for reason in reasons))
        self.assertTrue(any("container memory" in reason for reason in reasons))

        low = {
            "healthy": True,
            "workload": {"attempts": 10, "errors": 0},
            "host": {"cpuPercent": 30, "memoryPercent": 40, "diskFreeBytes": 10 * 1024**3},
            "containers": [{"memoryPercent": 20}],
        }
        self.assertEqual(evaluator.evaluate(low), [])
        self.assertEqual(evaluator.evaluate(high), [])

    def test_health_failures_trigger_without_threshold_grace(self):
        evaluator = capacity.StopEvaluator(self.conditions(breach_samples=3))
        self.assertEqual(evaluator.evaluate({"healthy": False}), [])
        self.assertEqual(evaluator.evaluate({"healthy": False}), [])
        reasons = evaluator.evaluate({"healthy": False})
        self.assertEqual(len(reasons), 1)
        self.assertIn("3 consecutive", reasons[0])

    def test_missing_optional_collectors_do_not_invent_breaches(self):
        evaluator = capacity.StopEvaluator(self.conditions(breach_samples=1))
        self.assertEqual(
            evaluator.evaluate(
                {"healthy": True, "workload": {"attempts": 0, "errors": 0}}
            ),
            [],
        )


class WorkloadTests(unittest.TestCase):
    def test_stats_delta_and_summary_preserve_error_and_byte_counts(self):
        stats = capacity.WorkloadStats()
        stats.record("api", 10, byte_count=100)
        before = stats.snapshot()
        stats.record("api", 20, failed=True)
        stats.record("viewerMedia", 30, byte_count=2048)
        delta = capacity.WorkloadStats.delta(before, stats.snapshot())
        summary = capacity.summarize_workload(delta)
        self.assertEqual(summary["api"]["attempts"], 1)
        self.assertEqual(summary["api"]["errors"], 1)
        self.assertEqual(summary["viewerMedia"]["bytes"], 2048)
        self.assertEqual(summary["total"]["attempts"], 2)
        self.assertEqual(summary["total"]["errorRate"], 0.5)

    def test_latency_observations_are_memory_bounded_and_disclosed(self):
        with mock.patch.object(capacity, "MAX_LATENCY_SAMPLES_PER_KIND", 2):
            stats = capacity.WorkloadStats()
            before = stats.snapshot()
            for duration in (10, 20, 30):
                stats.record("api", duration)
            summary = capacity.summarize_workload(
                capacity.WorkloadStats.delta(before, stats.snapshot())
            )
        self.assertEqual(summary["api"]["attempts"], 3)
        self.assertEqual(summary["api"]["latency"]["count"], 2)
        self.assertTrue(summary["api"]["latencySamplesTruncated"])
        self.assertTrue(summary["total"]["latencySamplesTruncated"])

    def test_phase_rejects_underdelivery_and_aggregate_errors(self):
        phase = capacity.Phase("steady", 10, 1, 2, 2, 1)
        adequate = {
            "viewerPlaylist": {"attempts": 16, "errorRate": 0},
            "api": {"attempts": 16, "errorRate": 0},
            "chat": {"attempts": 8, "errorRate": 0},
            "total": {"errorRate": 0.05},
        }
        self.assertEqual(capacity.phase_workload_failures(phase, adequate, 0.05), [])
        inadequate = copy.deepcopy(adequate)
        inadequate["api"]["attempts"] = 15
        inadequate["chat"]["errorRate"] = 0.051
        inadequate["total"]["errorRate"] = 0.051
        failures = capacity.phase_workload_failures(phase, inadequate, 0.05)
        self.assertTrue(any("api delivered" in failure for failure in failures))
        self.assertTrue(any("chat error rate" in failure for failure in failures))
        self.assertTrue(any("error rate" in failure for failure in failures))

    def test_prometheus_summary_keeps_only_capacity_aggregates(self):
        summary = capacity.summarize_prometheus(
            capacity.parse_prometheus(
                'bitriver_http_requests_total{method="GET",path="/a",status="200"} 9\n'
                'bitriver_http_requests_total{method="GET",path="/b",status="503"} 1\n'
                "bitriver_active_streams 2\n"
                'bitriver_ingest_health{service="ome",status="degraded"} -1\n'
            )
        )
        self.assertEqual(summary["totals"]["httpRequests"], 10)
        self.assertEqual(summary["totals"]["http5xx"], 1)
        self.assertEqual(summary["totals"]["activeStreams"], 2)
        self.assertEqual(summary["degradedServices"], ["ome"])

    def test_paced_worker_records_operation_failure(self):
        stop = threading.Event()
        stats = capacity.WorkloadStats()

        def fail_once():
            stop.set()
            raise RuntimeError("expected")

        capacity.paced_worker(stop, 1, stats, "api", fail_once)
        bucket = stats.snapshot()["api"]
        self.assertEqual(bucket["attempts"], 1)
        self.assertEqual(bucket["errors"], 1)

    def test_run_always_cleans_publishers_after_phase_failure(self):
        fake = SimpleNamespace(
            configure_collectors=mock.Mock(),
            preflight=mock.Mock(),
            provision=mock.Mock(),
            scenario=SimpleNamespace(phases=(object(),)),
            run_phase=mock.Mock(side_effect=capacity.CapacityError("stop")),
            cleanup=mock.Mock(),
        )
        with self.assertRaisesRegex(capacity.CapacityError, "stop"):
            capacity.CapacityHarness.run(fake)
        fake.cleanup.assert_called_once_with()


class CommandSafetyTests(unittest.TestCase):
    def test_live_mode_requires_explicit_dedicated_environment_confirmation(self):
        with tempfile.TemporaryDirectory() as temp:
            stderr = io.StringIO()
            with contextlib.redirect_stderr(stderr):
                status = capacity.main(
                    [
                        "--scenario",
                        str(SCENARIO_PATH),
                        "--artifact-dir",
                        temp,
                        "--release",
                        "v1.2.3-rc.20",
                        "--release-set-sha256",
                        RELEASE_SET_SHA256,
                        "--source-commit",
                        SOURCE_COMMIT,
                    ]
                )
        self.assertEqual(status, 1)
        self.assertIn("--confirm-dedicated-environment", stderr.getvalue())

    def test_live_bootstrap_failure_writes_honest_failed_evidence(self):
        scenario = capacity.load_scenario(SCENARIO_PATH)
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            release_set_file, release_set_hash = write_release_set(root)
            args = SimpleNamespace(
                artifact_dir=root / "evidence",
                release_set_file=release_set_file,
                ffmpeg="missing-ffmpeg",
                stage_timeout=2.0,
                collector_mode="remote",
            )
            with mock.patch.object(
                capacity.golden,
                "command_path",
                side_effect=capacity.CapacityError("ffmpeg is unavailable"),
            ), self.assertRaisesRegex(capacity.CapacityError, "unavailable"):
                capacity.run_live(
                    args,
                    capacity.CandidateIdentity.parse(
                        "v1.2.3-rc.20", release_set_hash, SOURCE_COMMIT
                    ),
                    scenario,
                )
            report = json.loads(
                (args.artifact_dir / "capacity-report.json").read_text(
                    encoding="utf-8"
                )
            )
        self.assertEqual(report["status"], "failed")
        self.assertEqual(report["failure"], "ffmpeg is unavailable")
        self.assertEqual(
            report["collectors"]["application"]["status"], "unavailable"
        )


class LiveHarnessTests(unittest.TestCase):
    class FakePublisher:
        def __init__(self):
            self.stopped = False

        def poll(self):
            return 0 if self.stopped else None

    def test_one_phase_public_surface_run_cleans_publisher_and_writes_evidence(self):
        state = {"signups": 0, "chats": 0, "active_publishers": 0}

        class Handler(http.server.BaseHTTPRequestHandler):
            def send_json(self, status, payload):
                body = json.dumps(payload).encode("utf-8")
                self.send_response(status)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            def do_GET(self):
                path = self.path.split("?", 1)[0]
                if path in {"/readyz", "/healthz"}:
                    self.send_json(200, {"status": "ok"})
                elif path == "/metrics":
                    if self.headers.get("Authorization") != "Bearer metrics-test-token":
                        self.send_json(403, {"error": "forbidden"})
                        return
                    body = (
                        f"bitriver_active_streams {state['active_publishers']}\n"
                        f"bitriver_transcoder_active_jobs {state['active_publishers']}\n"
                        'bitriver_http_requests_total{method="GET",path="/readyz",status="200"} 5\n'
                    ).encode("utf-8")
                    self.send_response(200)
                    self.send_header("Content-Type", "text/plain")
                    self.send_header("Content-Length", str(len(body)))
                    self.end_headers()
                    self.wfile.write(body)
                elif path == "/api/channels":
                    self.send_json(200, {"items": []})
                elif path.endswith("/playback"):
                    port = self.server.server_address[1]
                    self.send_json(
                        200,
                        {
                            "live": True,
                            "playback": {
                                "renditions": [
                                    {
                                        "name": "1080p",
                                        "manifestUrl": f"http://127.0.0.1:{port}/master.m3u8",
                                    }
                                ]
                            },
                        },
                    )
                elif path == "/master.m3u8":
                    body = b"#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=2500000\nmedia.m3u8\n"
                    self.send_response(200)
                    self.send_header("Content-Length", str(len(body)))
                    self.end_headers()
                    self.wfile.write(body)
                elif path == "/media.m3u8":
                    body = b"#EXTM3U\n#EXT-X-MEDIA-SEQUENCE:1\n#EXTINF:2,\nsegment.ts\n"
                    self.send_response(200)
                    self.send_header("Content-Length", str(len(body)))
                    self.end_headers()
                    self.wfile.write(body)
                elif path == "/segment.ts":
                    body = b"synthetic-media-segment"
                    self.send_response(200)
                    self.send_header("Content-Length", str(len(body)))
                    self.end_headers()
                    self.wfile.write(body)
                else:
                    self.send_json(404, {"error": "not found"})

            def do_POST(self):
                length = int(self.headers.get("Content-Length", "0"))
                if length:
                    self.rfile.read(length)
                if self.path == "/api/auth/signup":
                    state["signups"] += 1
                    self.send_response(201)
                    self.send_header("Content-Type", "application/json")
                    self.send_header(
                        "Set-Cookie",
                        f"bitriver_session=session-{state['signups']}; Secure; Path=/",
                    )
                    body = json.dumps(
                        {"user": {"id": f"user-{state['signups']}"}}
                    ).encode("utf-8")
                    self.send_header("Content-Length", str(len(body)))
                    self.end_headers()
                    self.wfile.write(body)
                elif self.path == "/api/channels":
                    self.send_json(
                        201,
                        {"id": "channel-one", "streamKey": "private-stream-key"},
                    )
                elif self.path.endswith("/chat"):
                    state["chats"] += 1
                    self.send_json(201, {"id": f"chat-{state['chats']}"})
                else:
                    self.send_json(404, {"error": "not found"})

            def log_message(self, _format, *_args):
                return

        server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        server_thread = threading.Thread(target=server.serve_forever, daemon=True)
        server_thread.start()
        fake_publishers = []
        try:
            with tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                payload = json.loads(SCENARIO_PATH.read_text(encoding="utf-8"))
                payload["sampleIntervalSeconds"] = 1
                payload["phases"] = [
                    {
                        "name": "warm-up",
                        "durationSeconds": 1,
                        "publishers": 1,
                        "viewersPerPublisher": 1,
                        "apiRequestsPerSecond": 1,
                        "chatMessagesPerSecond": 1,
                    }
                ]
                scenario_path = root / "scenario.json"
                scenario_path.write_text(json.dumps(payload), encoding="utf-8")
                scenario = capacity.load_scenario(scenario_path)
                metrics_file = root / "metrics-token"
                metrics_file.write_text("metrics-test-token\n", encoding="utf-8")
                sentinel_file = root / "sentinels"
                sentinel_file.write_text("", encoding="utf-8")
                release_set_file, release_set_hash = write_release_set(root)
                args = SimpleNamespace(
                    ffmpeg="ffmpeg",
                    stage_timeout=2.0,
                    http_timeout=2.0,
                    base_url=f"http://127.0.0.1:{server.server_address[1]}",
                    rtmp_base_url="rtmp://127.0.0.1/live",
                    media_host_override="",
                    metrics_bearer_file=metrics_file,
                    release_set_file=release_set_file,
                    sentinel_file=sentinel_file,
                    collector_mode="remote",
                    compose_project="fixture",
                    data_path=None,
                    artifact_dir=root / "evidence",
                )

                def start_publisher(*_args, **_kwargs):
                    publisher = self.FakePublisher()
                    fake_publishers.append(publisher)
                    state["active_publishers"] += 1
                    return publisher

                def stop_publisher(process, _sentinels):
                    if not process.stopped:
                        process.stopped = True
                        state["active_publishers"] -= 1
                    return ""

                with mock.patch.object(
                    capacity.golden, "command_path", return_value="ffmpeg"
                ), mock.patch.object(
                    capacity.golden,
                    "start_live_publisher",
                    side_effect=start_publisher,
                ), mock.patch.object(
                    capacity.golden, "stop_process", side_effect=stop_publisher
                ):
                    output = capacity.run_live(
                        args,
                        capacity.CandidateIdentity.parse(
                            "v1.2.3-rc.20", release_set_hash, SOURCE_COMMIT
                        ),
                        scenario,
                    )
                report = json.loads(output.read_text(encoding="utf-8"))
                self.assertEqual(report["status"], "passed")
                self.assertEqual(
                    report["candidate"]["releaseSet"]["status"], "verified"
                )
                self.assertEqual(
                    report["final"]["metrics"]["totals"]["activeStreams"], 0
                )
                invalid_final = copy.deepcopy(report)
                invalid_final["final"]["metrics"]["totals"]["activeStreams"] = 1
                with self.assertRaisesRegex(capacity.CapacityError, "zero final"):
                    capacity.validate_report(invalid_final)
                self.assertEqual(report["phases"][0]["status"], "passed")
                self.assertEqual(report["samples"][0]["phase"], "warm-up")
                self.assertGreaterEqual(report["phases"][0]["workload"]["total"]["attempts"], 1)
                self.assertNotIn("private-stream-key", output.read_text(encoding="utf-8"))
        finally:
            server.shutdown()
            server.server_close()
            server_thread.join(timeout=2)
        self.assertTrue(fake_publishers)
        self.assertTrue(all(publisher.stopped for publisher in fake_publishers))


class EvidenceTests(unittest.TestCase):
    def identity(self):
        return capacity.CandidateIdentity.parse(
            "v1.2.3-rc.20", RELEASE_SET_SHA256, SOURCE_COMMIT
        )

    def test_dry_run_report_is_atomic_versioned_and_candidate_bound(self):
        scenario = capacity.load_scenario(SCENARIO_PATH)
        with tempfile.TemporaryDirectory() as temp:
            output = capacity.write_dry_run(
                Path(temp), self.identity(), scenario
            )
            payload = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(payload["schema"], capacity.REPORT_SCHEMA)
            self.assertEqual(payload["status"], "planned")
            self.assertEqual(payload["mode"], "dry-run")
            self.assertEqual(payload["candidate"]["release"], "v1.2.3-rc.20")
            self.assertEqual(
                payload["candidate"]["releaseSetSha256"], RELEASE_SET_SHA256
            )
            self.assertEqual(payload["scenario"]["sha256"], scenario.sha256)
            self.assertEqual(
                payload["scenario"]["stopConditions"]["maxErrorRate"], 0.05
            )
            self.assertEqual(payload["collectors"]["host"]["status"], "unavailable")
            self.assertFalse(output.with_suffix(".json.tmp").exists())

    def test_report_validator_rejects_incomplete_identity_and_secret(self):
        scenario = capacity.load_scenario(SCENARIO_PATH)
        with tempfile.TemporaryDirectory() as temp:
            output = capacity.write_dry_run(Path(temp), self.identity(), scenario)
            payload = json.loads(output.read_text(encoding="utf-8"))
        missing_identity = copy.deepcopy(payload)
        missing_identity["candidate"]["sourceCommit"] = ""
        with self.assertRaisesRegex(capacity.CapacityError, "40 lowercase"):
            capacity.validate_report(missing_identity)

        sentinel = "private-capacity-token"
        secret = copy.deepcopy(payload)
        secret["diagnostic"] = sentinel
        with self.assertRaisesRegex(capacity.CapacityError, "private sentinel"):
            capacity.validate_report(secret, [sentinel])

    def test_secret_bearing_evidence_is_refused_before_retention(self):
        scenario = capacity.load_scenario(SCENARIO_PATH)
        sentinel = "private-stream-key/value"
        with tempfile.TemporaryDirectory() as temp:
            output = Path(temp) / "capacity-report.json"
            evidence = capacity.Evidence(
                output, self.identity(), scenario, [sentinel]
            )
            evidence.payload["mode"] = "live"
            evidence.payload["collectors"] = {
                name: {"status": "unavailable", "reason": "test fixture"}
                for name in ("application", "loadClient", "host", "docker")
            }
            evidence.payload["failure"] = f"publisher failed: {sentinel}"
            with self.assertRaisesRegex(capacity.CapacityError, "private sentinel"):
                evidence.finish("failed")
            self.assertFalse(output.exists())


if __name__ == "__main__":
    unittest.main()
