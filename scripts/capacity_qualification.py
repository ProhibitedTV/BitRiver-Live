#!/usr/bin/env python3
"""Run a bounded BitRiver Live single-host capacity qualification.

The harness targets a dedicated, already-running canonical stack through public
HTTP/RTMP surfaces. Retained evidence is versioned, candidate-bound, atomic,
and secret-safe. Host and Docker resource claims are emitted only when the
runner is explicitly co-located with the Compose host.
"""

from __future__ import annotations

import argparse
import collections
import concurrent.futures
import dataclasses
import hashlib
import json
import math
import os
import platform
import re
import secrets
import shutil
import statistics
import subprocess
import sys
import tempfile
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path
from typing import Any, Iterable

import production_golden_path as golden


SCENARIO_SCHEMA = "bitriver.capacity-scenario/v1"
REPORT_SCHEMA = "bitriver.capacity-report/v1"
MAX_SCENARIO_BYTES = 1024 * 1024
MAX_RELEASE_SET_BYTES = 10 * 1024 * 1024
MAX_PHASES = 12
MAX_PUBLISHERS = 16
MAX_VIEWERS = 512
MAX_API_RPS = 100
MAX_CHAT_RPS = 50
MAX_PHASE_SECONDS = 3600
MAX_TOTAL_SECONDS = 4 * 3600
MIN_SAMPLE_INTERVAL_SECONDS = 1.0
MAX_SAMPLE_INTERVAL_SECONDS = 60.0
MAX_MEDIA_RESPONSE_BYTES = 64 * 1024 * 1024
MAX_LATENCY_SAMPLES_PER_KIND = 20_000
MIN_ACHIEVED_WORKLOAD_RATIO = 0.80
FIRST_PARTY_IMAGE_NAMES = {
    "bitriver-live",
    "bitriver-viewer",
    "bitriver-srs-controller",
    "bitriver-transcoder",
    "bitriver-ome-config",
}
RELEASE_RE = re.compile(
    r"^v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)-"
    r"(?:[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)$"
)
SHA256_RE = re.compile(r"^[0-9a-f]{64}$")
COMMIT_RE = re.compile(r"^[0-9a-f]{40}$")
DIGEST_RE = re.compile(r"^sha256:[0-9a-f]{64}$")
METRIC_RE = re.compile(
    r"^(?P<name>[a-zA-Z_:][a-zA-Z0-9_:]*)"
    r"(?:\{(?P<labels>.*)\})?[ \t]+"
    r"(?P<value>[-+]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)"
    r"(?:[eE][-+]?[0-9]+)?|NaN|[+-]Inf)"
    r"(?:[ \t]+[0-9]+)?$"
)
LABEL_RE = re.compile(
    r'(?:^|,)\s*(?P<name>[a-zA-Z_][a-zA-Z0-9_]*)='
    r'"(?P<value>(?:\\.|[^"\\])*)"\s*'
)
BYTE_UNITS = {
    "b": 1,
    "kb": 1000,
    "mb": 1000**2,
    "gb": 1000**3,
    "tb": 1000**4,
    "kib": 1024,
    "mib": 1024**2,
    "gib": 1024**3,
    "tib": 1024**4,
}


class CapacityError(RuntimeError):
    """A bounded capacity harness refusal or qualification failure."""


@dataclasses.dataclass(frozen=True)
class CandidateIdentity:
    release: str
    release_set_sha256: str
    source_commit: str

    @classmethod
    def parse(
        cls, release: str, release_set_sha256: str, source_commit: str
    ) -> "CandidateIdentity":
        values = (
            release.strip(),
            release_set_sha256.strip().lower(),
            source_commit.strip().lower(),
        )
        if not RELEASE_RE.fullmatch(values[0]):
            raise CapacityError(
                "release identity must be a prerelease tag such as "
                "v1.2.3-rc.20"
            )
        if not SHA256_RE.fullmatch(values[1]):
            raise CapacityError("release-set identity must be 64 lowercase hex")
        if not COMMIT_RE.fullmatch(values[2]):
            raise CapacityError("source commit identity must be 40 lowercase hex")
        return cls(*values)

    def public(self) -> dict[str, str]:
        return {
            "release": self.release,
            "releaseSetSha256": self.release_set_sha256,
            "sourceCommit": self.source_commit,
        }


@dataclasses.dataclass(frozen=True)
class Phase:
    name: str
    duration_seconds: int
    publishers: int
    viewers_per_publisher: int
    api_requests_per_second: int
    chat_messages_per_second: int

    @property
    def viewers(self) -> int:
        return self.publishers * self.viewers_per_publisher


@dataclasses.dataclass(frozen=True)
class StopConditions:
    max_error_rate: float
    max_consecutive_health_failures: int
    max_host_cpu_percent: float
    max_host_memory_percent: float
    min_host_disk_free_bytes: int
    max_container_memory_percent: float
    threshold_breach_samples: int

    def public(self) -> dict[str, float | int]:
        return {
            "maxErrorRate": self.max_error_rate,
            "maxConsecutiveHealthFailures": self.max_consecutive_health_failures,
            "maxHostCPUPercent": self.max_host_cpu_percent,
            "maxHostMemoryPercent": self.max_host_memory_percent,
            "minHostDiskFreeBytes": self.min_host_disk_free_bytes,
            "maxContainerMemoryPercent": self.max_container_memory_percent,
            "thresholdBreachSamples": self.threshold_breach_samples,
        }


@dataclasses.dataclass(frozen=True)
class Scenario:
    path: Path
    payload: dict[str, Any]
    name: str
    description: str
    sample_interval_seconds: float
    phases: tuple[Phase, ...]
    stop_conditions: StopConditions
    sha256: str

    @property
    def duration_seconds(self) -> int:
        return sum(phase.duration_seconds for phase in self.phases)

    @property
    def max_publishers(self) -> int:
        return max(phase.publishers for phase in self.phases)

    @property
    def max_viewers(self) -> int:
        return max(phase.viewers for phase in self.phases)

    def public_summary(self) -> dict[str, Any]:
        return {
            "schema": SCENARIO_SCHEMA,
            "name": self.name,
            "description": self.description,
            "sha256": self.sha256,
            "sampleIntervalSeconds": self.sample_interval_seconds,
            "durationSeconds": self.duration_seconds,
            "maxPublishers": self.max_publishers,
            "maxViewers": self.max_viewers,
            "phases": [
                {
                    "name": phase.name,
                    "durationSeconds": phase.duration_seconds,
                    "publishers": phase.publishers,
                    "viewersPerPublisher": phase.viewers_per_publisher,
                    "viewers": phase.viewers,
                    "apiRequestsPerSecond": phase.api_requests_per_second,
                    "chatMessagesPerSecond": phase.chat_messages_per_second,
                }
                for phase in self.phases
            ],
            "stopConditions": self.stop_conditions.public(),
        }


def _mapping(value: Any, label: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise CapacityError(f"{label} must be a JSON object")
    return value


def _exact_keys(
    mapping: dict[str, Any], required: set[str], label: str
) -> None:
    missing = sorted(required - mapping.keys())
    extra = sorted(mapping.keys() - required)
    if missing:
        raise CapacityError(f"{label} is missing keys: {', '.join(missing)}")
    if extra:
        raise CapacityError(f"{label} has unknown keys: {', '.join(extra)}")


def _integer(
    mapping: dict[str, Any], key: str, minimum: int, maximum: int, label: str
) -> int:
    value = mapping.get(key)
    if isinstance(value, bool) or not isinstance(value, int):
        raise CapacityError(f"{label}.{key} must be an integer")
    if not minimum <= value <= maximum:
        raise CapacityError(
            f"{label}.{key} must be between {minimum} and {maximum}"
        )
    return value


def _number(
    mapping: dict[str, Any], key: str, minimum: float, maximum: float, label: str
) -> float:
    value = mapping.get(key)
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        raise CapacityError(f"{label}.{key} must be numeric")
    result = float(value)
    if not math.isfinite(result) or not minimum <= result <= maximum:
        raise CapacityError(
            f"{label}.{key} must be between {minimum:g} and {maximum:g}"
        )
    return result


def canonical_json_sha256(payload: dict[str, Any]) -> str:
    encoded = json.dumps(
        payload, sort_keys=True, separators=(",", ":"), ensure_ascii=True
    ).encode("utf-8")
    return hashlib.sha256(encoded).hexdigest()


def require_network_url(
    raw_url: str,
    label: str,
    schemes: set[str],
    *,
    allow_query: bool = False,
) -> str:
    """Reject local-file/credential URLs before a workload client uses them."""
    value = raw_url.strip()
    try:
        parsed = urllib.parse.urlsplit(value)
        _ = parsed.port
    except ValueError as exc:
        raise CapacityError(f"{label} has an invalid port") from exc
    if parsed.scheme.lower() not in schemes or not parsed.hostname:
        expected = "/".join(sorted(schemes))
        raise CapacityError(f"{label} must be an absolute {expected} URL")
    if parsed.username is not None or parsed.password is not None:
        raise CapacityError(f"{label} must not embed credentials")
    if parsed.fragment or (parsed.query and not allow_query):
        raise CapacityError(f"{label} contains unsupported query/fragment data")
    return value


def load_release_set(
    path: Path, identity: CandidateIdentity
) -> dict[str, Any]:
    """Bind live evidence to exact, structurally valid release-set bytes."""
    try:
        data = path.read_bytes()
    except OSError as exc:
        raise CapacityError(f"cannot read release set {path}: {exc}") from exc
    if not 1 <= len(data) <= MAX_RELEASE_SET_BYTES:
        raise CapacityError(
            f"release set size must be between 1 and {MAX_RELEASE_SET_BYTES} bytes"
        )
    digest = hashlib.sha256(data).hexdigest()
    if digest != identity.release_set_sha256:
        raise CapacityError("release-set file SHA-256 does not match declared identity")
    try:
        manifest = json.loads(data)
    except (UnicodeError, json.JSONDecodeError) as exc:
        raise CapacityError(f"cannot parse release set {path}: {exc}") from exc
    root = _mapping(manifest, "release set")
    if root.get("schemaVersion") != "bitriver.release-set/v1":
        raise CapacityError("release set has an unsupported schema")
    candidate = _mapping(root.get("candidate"), "release set candidate")
    if candidate.get("tag") != identity.release:
        raise CapacityError("release set candidate tag does not match declared identity")
    if candidate.get("sourceCommit") != identity.source_commit:
        raise CapacityError("release set source commit does not match declared identity")
    repository = candidate.get("repository")
    if not isinstance(repository, str) or not re.fullmatch(
        r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repository
    ):
        raise CapacityError("release set repository is invalid")
    raw_images = root.get("images")
    if not isinstance(raw_images, list) or len(raw_images) != 5:
        raise CapacityError("release set must contain five first-party images")
    images: list[dict[str, str]] = []
    seen_names: set[str] = set()
    for index, value in enumerate(raw_images):
        image = _mapping(value, f"release set image {index}")
        name = str(image.get("name", ""))
        reference = str(image.get("candidateReference", ""))
        immutable_reference = str(image.get("immutableReference", ""))
        image_digest = str(image.get("digest", ""))
        if not re.fullmatch(r"[a-z0-9][a-z0-9._-]{1,63}", name):
            raise CapacityError("release set image name is invalid")
        if name in seen_names:
            raise CapacityError(f"release set repeats first-party image {name}")
        seen_names.add(name)
        if not reference or "@" in reference or any(
            character.isspace() for character in reference
        ):
            raise CapacityError(f"release set image {name} reference is invalid")
        if not reference.endswith(f":{identity.release}"):
            raise CapacityError(
                f"release set image {name} does not reference the candidate tag"
            )
        if not DIGEST_RE.fullmatch(image_digest):
            raise CapacityError(f"release set image {name} digest is invalid")
        if immutable_reference != reference.rsplit(":", 1)[0] + "@" + image_digest:
            raise CapacityError(
                f"release set image {name} immutable reference is inconsistent"
            )
        images.append(
            {
                "name": name,
                "candidateReference": reference,
                "immutableReference": immutable_reference,
                "digest": image_digest,
            }
        )
    if seen_names != FIRST_PARTY_IMAGE_NAMES:
        raise CapacityError("release set first-party image names are incomplete")
    integrity = _mapping(root.get("integrity"), "release set integrity")
    signature = _mapping(
        integrity.get("manifestSignature"), "release set manifest signature"
    )
    if signature.get("asset") != "release-set.sigstore.json":
        raise CapacityError("release set lacks its Sigstore root reference")
    return {
        "status": "verified",
        "schema": "bitriver.release-set/v1",
        "sha256": digest,
        "repository": repository,
        "images": images,
        "runtimeImageMatch": "not-collected",
    }


def load_scenario(path: Path) -> Scenario:
    try:
        size = path.stat().st_size
    except OSError as exc:
        raise CapacityError(f"cannot stat scenario {path}: {exc}") from exc
    if size <= 0 or size > MAX_SCENARIO_BYTES:
        raise CapacityError(
            f"scenario size must be between 1 and {MAX_SCENARIO_BYTES} bytes"
        )
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        raise CapacityError(f"cannot parse scenario {path}: {exc}") from exc
    root = _mapping(payload, "scenario")
    _exact_keys(
        root,
        {
            "schema",
            "name",
            "description",
            "sampleIntervalSeconds",
            "phases",
            "stopConditions",
        },
        "scenario",
    )
    if root.get("schema") != SCENARIO_SCHEMA:
        raise CapacityError(f"scenario schema must be {SCENARIO_SCHEMA}")
    name = root.get("name")
    description = root.get("description")
    if not isinstance(name, str) or not re.fullmatch(r"[a-z0-9][a-z0-9-]{2,63}", name):
        raise CapacityError("scenario.name must be a 3-64 character slug")
    if not isinstance(description, str) or not 10 <= len(description.strip()) <= 240:
        raise CapacityError("scenario.description must be 10-240 characters")
    interval = _number(
        root,
        "sampleIntervalSeconds",
        MIN_SAMPLE_INTERVAL_SECONDS,
        MAX_SAMPLE_INTERVAL_SECONDS,
        "scenario",
    )
    raw_phases = root.get("phases")
    if not isinstance(raw_phases, list) or not 1 <= len(raw_phases) <= MAX_PHASES:
        raise CapacityError(f"scenario.phases must contain 1-{MAX_PHASES} phases")
    phases: list[Phase] = []
    seen_names: set[str] = set()
    for index, raw_phase in enumerate(raw_phases):
        label = f"scenario.phases[{index}]"
        phase = _mapping(raw_phase, label)
        _exact_keys(
            phase,
            {
                "name",
                "durationSeconds",
                "publishers",
                "viewersPerPublisher",
                "apiRequestsPerSecond",
                "chatMessagesPerSecond",
            },
            label,
        )
        phase_name = phase.get("name")
        if not isinstance(phase_name, str) or not re.fullmatch(
            r"[a-z][a-z0-9-]{2,31}", phase_name
        ):
            raise CapacityError(f"{label}.name must be a 3-32 character slug")
        if phase_name in seen_names:
            raise CapacityError(f"duplicate phase name: {phase_name}")
        seen_names.add(phase_name)
        parsed = Phase(
            name=phase_name,
            duration_seconds=_integer(
                phase, "durationSeconds", 1, MAX_PHASE_SECONDS, label
            ),
            publishers=_integer(
                phase, "publishers", 1, MAX_PUBLISHERS, label
            ),
            viewers_per_publisher=_integer(
                phase, "viewersPerPublisher", 0, MAX_VIEWERS, label
            ),
            api_requests_per_second=_integer(
                phase, "apiRequestsPerSecond", 0, MAX_API_RPS, label
            ),
            chat_messages_per_second=_integer(
                phase, "chatMessagesPerSecond", 0, MAX_CHAT_RPS, label
            ),
        )
        if parsed.viewers > MAX_VIEWERS:
            raise CapacityError(
                f"{label} requests {parsed.viewers} viewers; hard cap is {MAX_VIEWERS}"
            )
        phases.append(parsed)
    total_duration = sum(phase.duration_seconds for phase in phases)
    if total_duration > MAX_TOTAL_SECONDS:
        raise CapacityError(
            f"scenario duration {total_duration}s exceeds {MAX_TOTAL_SECONDS}s cap"
        )

    raw_stop = _mapping(root.get("stopConditions"), "scenario.stopConditions")
    stop_keys = {
        "maxErrorRate",
        "maxConsecutiveHealthFailures",
        "maxHostCPUPercent",
        "maxHostMemoryPercent",
        "minHostDiskFreeBytes",
        "maxContainerMemoryPercent",
        "thresholdBreachSamples",
    }
    _exact_keys(raw_stop, stop_keys, "scenario.stopConditions")
    stop = StopConditions(
        max_error_rate=_number(
            raw_stop, "maxErrorRate", 0.001, 0.25, "scenario.stopConditions"
        ),
        max_consecutive_health_failures=_integer(
            raw_stop,
            "maxConsecutiveHealthFailures",
            1,
            10,
            "scenario.stopConditions",
        ),
        max_host_cpu_percent=_number(
            raw_stop,
            "maxHostCPUPercent",
            50,
            99,
            "scenario.stopConditions",
        ),
        max_host_memory_percent=_number(
            raw_stop,
            "maxHostMemoryPercent",
            50,
            99,
            "scenario.stopConditions",
        ),
        min_host_disk_free_bytes=_integer(
            raw_stop,
            "minHostDiskFreeBytes",
            1024**3,
            1024**5,
            "scenario.stopConditions",
        ),
        max_container_memory_percent=_number(
            raw_stop,
            "maxContainerMemoryPercent",
            50,
            99.9,
            "scenario.stopConditions",
        ),
        threshold_breach_samples=_integer(
            raw_stop,
            "thresholdBreachSamples",
            1,
            10,
            "scenario.stopConditions",
        ),
    )
    return Scenario(
        path=path,
        payload=root,
        name=name,
        description=description.strip(),
        sample_interval_seconds=interval,
        phases=tuple(phases),
        stop_conditions=stop,
        sha256=canonical_json_sha256(root),
    )


def parse_percent(value: object) -> float:
    text = str(value).strip()
    if not text.endswith("%"):
        raise CapacityError(f"invalid percent value: {text}")
    try:
        result = float(text[:-1].strip())
    except ValueError as exc:
        raise CapacityError(f"invalid percent value: {text}") from exc
    if not math.isfinite(result) or result < 0:
        raise CapacityError(f"invalid percent value: {text}")
    return result


def parse_byte_size(value: object) -> int:
    text = str(value).strip()
    match = re.fullmatch(
        r"(?i)([0-9]+(?:\.[0-9]+)?)\s*(B|KB|MB|GB|TB|KiB|MiB|GiB|TiB)",
        text,
    )
    if not match:
        raise CapacityError(f"invalid byte size: {text}")
    return int(float(match.group(1)) * BYTE_UNITS[match.group(2).lower()])


def parse_byte_pair(value: object) -> tuple[int, int]:
    parts = [part.strip() for part in str(value).split("/")]
    if len(parts) != 2:
        raise CapacityError(f"invalid byte pair: {value}")
    return parse_byte_size(parts[0]), parse_byte_size(parts[1])


def _unescape_label(value: str) -> str:
    return (
        value.replace(r"\\", "\0")
        .replace(r'\"', '"')
        .replace(r"\n", "\n")
        .replace("\0", "\\")
    )


def parse_prometheus(text: str) -> list[dict[str, Any]]:
    samples: list[dict[str, Any]] = []
    for line_number, raw_line in enumerate(text.splitlines(), 1):
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        match = METRIC_RE.fullmatch(line)
        if not match:
            raise CapacityError(f"invalid Prometheus sample on line {line_number}")
        labels_text = match.group("labels") or ""
        labels: dict[str, str] = {}
        position = 0
        while position < len(labels_text):
            label_match = LABEL_RE.match(labels_text, position)
            if not label_match:
                raise CapacityError(
                    f"invalid Prometheus labels on line {line_number}"
                )
            label_name = label_match.group("name")
            if label_name in labels:
                raise CapacityError(
                    f"duplicate Prometheus label {label_name} on line {line_number}"
                )
            labels[label_name] = _unescape_label(label_match.group("value"))
            position = label_match.end()
        try:
            value = float(match.group("value"))
        except ValueError as exc:
            raise CapacityError(
                f"invalid Prometheus value on line {line_number}"
            ) from exc
        if not math.isfinite(value):
            raise CapacityError(
                f"non-finite Prometheus value on line {line_number}"
            )
        samples.append(
            {"name": match.group("name"), "labels": labels, "value": value}
        )
    return samples


def latency_summary(milliseconds: Iterable[float]) -> dict[str, float | int]:
    values = sorted(float(value) for value in milliseconds)
    if not values:
        return {"count": 0}
    if any(not math.isfinite(value) or value < 0 for value in values):
        raise CapacityError("latencies must be finite non-negative numbers")

    def percentile(fraction: float) -> float:
        index = max(0, math.ceil(fraction * len(values)) - 1)
        return round(values[index], 3)

    return {
        "count": len(values),
        "minMs": round(values[0], 3),
        "meanMs": round(statistics.fmean(values), 3),
        "p50Ms": percentile(0.50),
        "p95Ms": percentile(0.95),
        "p99Ms": percentile(0.99),
        "maxMs": round(values[-1], 3),
    }


class StopEvaluator:
    """Evaluate persistent threshold breaches without hiding missing collectors."""

    def __init__(self, conditions: StopConditions) -> None:
        self.conditions = conditions
        self.counts: dict[str, int] = {}
        self.health_failures = 0

    def _persistent(self, name: str, breached: bool) -> bool:
        self.counts[name] = self.counts.get(name, 0) + 1 if breached else 0
        return self.counts[name] >= self.conditions.threshold_breach_samples

    def evaluate(self, sample: dict[str, Any]) -> list[str]:
        reasons: list[str] = []
        healthy = sample.get("healthy")
        if healthy is False:
            self.health_failures += 1
        elif healthy is True:
            self.health_failures = 0
        if self.health_failures >= self.conditions.max_consecutive_health_failures:
            reasons.append(
                f"health failed for {self.health_failures} consecutive samples"
            )

        workload = sample.get("workload")
        if isinstance(workload, dict):
            attempts = workload.get("attempts", 0)
            errors = workload.get("errors", 0)
            if isinstance(attempts, int) and attempts > 0 and isinstance(errors, int):
                rate = errors / attempts
                if self._persistent(
                    "workloadErrorRate", rate > self.conditions.max_error_rate
                ):
                    reasons.append(
                        f"workload error rate {rate:.4f} exceeded "
                        f"{self.conditions.max_error_rate:.4f}"
                    )

        host = sample.get("host")
        if isinstance(host, dict):
            checks = (
                (
                    "hostCPU",
                    host.get("cpuPercent"),
                    self.conditions.max_host_cpu_percent,
                    ">",
                ),
                (
                    "hostMemory",
                    host.get("memoryPercent"),
                    self.conditions.max_host_memory_percent,
                    ">",
                ),
                (
                    "hostDiskFree",
                    host.get("diskFreeBytes"),
                    self.conditions.min_host_disk_free_bytes,
                    "<",
                ),
            )
            for name, value, threshold, operator in checks:
                if not isinstance(value, (int, float)):
                    continue
                breached = value > threshold if operator == ">" else value < threshold
                if self._persistent(name, breached):
                    reasons.append(
                        f"{name} {value:g} breached {operator}{threshold:g}"
                    )

        containers = sample.get("containers")
        if isinstance(containers, list):
            memory_values = [
                item.get("memoryPercent")
                for item in containers
                if isinstance(item, dict)
                and isinstance(item.get("memoryPercent"), (int, float))
            ]
            maximum = max(memory_values, default=None)
            breached = (
                maximum is not None
                and maximum > self.conditions.max_container_memory_percent
            )
            if self._persistent("containerMemory", breached):
                reasons.append(
                    f"container memory {maximum:g}% exceeded "
                    f"{self.conditions.max_container_memory_percent:g}%"
                )
        return reasons


class WorkloadStats:
    """Thread-safe load-client counters and bounded latency observations."""

    def __init__(self) -> None:
        self.lock = threading.Lock()
        self.values: dict[str, dict[str, Any]] = {}

    def record(
        self,
        kind: str,
        duration_ms: float,
        *,
        failed: bool = False,
        byte_count: int = 0,
    ) -> None:
        if duration_ms < 0 or not math.isfinite(duration_ms):
            raise CapacityError("workload duration must be finite and non-negative")
        with self.lock:
            bucket = self.values.setdefault(
                kind,
                {
                    "attempts": 0,
                    "errors": 0,
                    "bytes": 0,
                    "latenciesMs": collections.deque(
                        maxlen=MAX_LATENCY_SAMPLES_PER_KIND
                    ),
                },
            )
            bucket["attempts"] += 1
            bucket["errors"] += int(failed)
            bucket["bytes"] += max(0, int(byte_count))
            bucket["latenciesMs"].append(round(duration_ms, 3))

    def snapshot(self) -> dict[str, dict[str, Any]]:
        with self.lock:
            return {
                kind: {
                    "attempts": int(bucket["attempts"]),
                    "errors": int(bucket["errors"]),
                    "bytes": int(bucket["bytes"]),
                    "latenciesMs": list(bucket["latenciesMs"]),
                    "latencyStart": int(bucket["attempts"])
                    - len(bucket["latenciesMs"]),
                }
                for kind, bucket in self.values.items()
            }

    @staticmethod
    def delta(
        before: dict[str, dict[str, Any]],
        after: dict[str, dict[str, Any]],
    ) -> dict[str, dict[str, Any]]:
        result: dict[str, dict[str, Any]] = {}
        for kind in sorted(set(before) | set(after)):
            old = before.get(
                kind, {"attempts": 0, "errors": 0, "bytes": 0, "latenciesMs": []}
            )
            new = after.get(
                kind, {"attempts": 0, "errors": 0, "bytes": 0, "latenciesMs": []}
            )
            old_attempts = int(old.get("attempts", 0))
            new_attempts = int(new.get("attempts", 0))
            new_latencies = new.get("latenciesMs", [])
            new_start = int(
                new.get("latencyStart", max(0, new_attempts - len(new_latencies)))
            )
            sample_start = max(old_attempts, new_start)
            latency_index = max(0, sample_start - new_start)
            result[kind] = {
                "attempts": max(0, new_attempts - old_attempts),
                "errors": max(0, int(new.get("errors", 0)) - int(old.get("errors", 0))),
                "bytes": max(0, int(new.get("bytes", 0)) - int(old.get("bytes", 0))),
                "latenciesMs": list(new_latencies[latency_index:]),
                "latencySamplesTruncated": sample_start > old_attempts,
            }
        return result


def summarize_workload(
    buckets: dict[str, dict[str, Any]],
) -> dict[str, Any]:
    summary: dict[str, Any] = {}
    total_attempts = 0
    total_errors = 0
    total_bytes = 0
    for kind, bucket in sorted(buckets.items()):
        attempts = int(bucket.get("attempts", 0))
        errors = int(bucket.get("errors", 0))
        byte_count = int(bucket.get("bytes", 0))
        total_attempts += attempts
        total_errors += errors
        total_bytes += byte_count
        summary[kind] = {
            "attempts": attempts,
            "errors": errors,
            "errorRate": round(errors / attempts, 6) if attempts else 0,
            "bytes": byte_count,
            "latency": latency_summary(bucket.get("latenciesMs", [])),
            "latencySamplesTruncated": bool(
                bucket.get("latencySamplesTruncated", False)
            ),
        }
    summary["total"] = {
        "attempts": total_attempts,
        "errors": total_errors,
        "errorRate": round(total_errors / total_attempts, 6) if total_attempts else 0,
        "bytes": total_bytes,
        "latencySamplesTruncated": any(
            item.get("latencySamplesTruncated", False)
            for item in summary.values()
            if isinstance(item, dict)
        ),
    }
    return summary


def phase_workload_failures(
    phase: Phase,
    summary: dict[str, Any],
    max_error_rate: float,
) -> list[str]:
    """Reject a nominal phase that did not actually deliver its configured load."""
    expectations = {
        "viewerPlaylist": phase.viewers * phase.duration_seconds,
        "api": phase.api_requests_per_second * phase.duration_seconds,
        "chat": phase.chat_messages_per_second * phase.duration_seconds,
    }
    failures: list[str] = []
    for kind, expected in expectations.items():
        if expected <= 0:
            continue
        minimum = max(1, math.ceil(expected * MIN_ACHIEVED_WORKLOAD_RATIO))
        bucket = summary.get(kind, {})
        attempts = int(bucket.get("attempts", 0)) if isinstance(bucket, dict) else 0
        if attempts < minimum:
            failures.append(
                f"{kind} delivered {attempts}/{expected} configured attempts; "
                f"minimum is {minimum}"
            )
        rate = float(bucket.get("errorRate", 0)) if isinstance(bucket, dict) else 0
        if rate > max_error_rate:
            failures.append(
                f"{kind} error rate {rate:.4f} exceeded {max_error_rate:.4f}"
            )
    total = summary.get("total", {})
    if isinstance(total, dict):
        rate = float(total.get("errorRate", 0))
        if rate > max_error_rate:
            failures.append(
                f"phase workload error rate {rate:.4f} exceeded "
                f"{max_error_rate:.4f}"
            )
    return failures


class HostSampler:
    """Sample direct host resources without third-party Python dependencies."""

    def __init__(self, data_path: Path) -> None:
        self.data_path = data_path
        self.previous_cpu: tuple[int, int] | None = None
        self.proc_available = Path("/proc/stat").is_file() and Path(
            "/proc/meminfo"
        ).is_file()

    @staticmethod
    def _cpu_totals() -> tuple[int, int]:
        fields = Path("/proc/stat").read_text(encoding="utf-8").splitlines()[0].split()
        if not fields or fields[0] != "cpu" or len(fields) < 5:
            raise CapacityError("host /proc/stat has an invalid aggregate CPU row")
        values = [int(value) for value in fields[1:]]
        total = sum(values)
        idle = values[3] + (values[4] if len(values) > 4 else 0)
        return total, idle

    @staticmethod
    def _memory() -> tuple[int, int]:
        values: dict[str, int] = {}
        for line in Path("/proc/meminfo").read_text(encoding="utf-8").splitlines():
            name, separator, remainder = line.partition(":")
            if not separator:
                continue
            fields = remainder.strip().split()
            if not fields:
                continue
            multiplier = 1024 if len(fields) > 1 and fields[1].lower() == "kb" else 1
            values[name] = int(fields[0]) * multiplier
        total = values.get("MemTotal", 0)
        available = values.get("MemAvailable", 0)
        if total <= 0 or not 0 <= available <= total:
            raise CapacityError("host /proc/meminfo lacks valid memory totals")
        return total, available

    def describe(self) -> dict[str, Any]:
        disk = shutil.disk_usage(self.data_path)
        description: dict[str, Any] = {
            "status": "available",
            "source": "co-located runner",
            "dataFilesystem": "operator-supplied",
            "cpuCount": os.cpu_count() or 0,
            "diskTotalBytes": disk.total,
        }
        if not self.proc_available:
            description["limitations"] = [
                "CPU and memory sampling requires Linux /proc; disk remains direct"
            ]
        else:
            memory_total, _ = self._memory()
            description["memoryTotalBytes"] = memory_total
        return description

    def sample(self) -> dict[str, Any]:
        disk = shutil.disk_usage(self.data_path)
        result: dict[str, Any] = {
            "diskFreeBytes": disk.free,
            "diskUsedBytes": disk.used,
            "diskTotalBytes": disk.total,
        }
        if not self.proc_available:
            return result
        current = self._cpu_totals()
        if self.previous_cpu is not None:
            total_delta = current[0] - self.previous_cpu[0]
            idle_delta = current[1] - self.previous_cpu[1]
            if total_delta > 0:
                result["cpuPercent"] = round(
                    max(0.0, min(100.0, 100.0 * (1 - idle_delta / total_delta))),
                    3,
                )
        self.previous_cpu = current
        memory_total, memory_available = self._memory()
        result["memoryUsedBytes"] = memory_total - memory_available
        result["memoryTotalBytes"] = memory_total
        result["memoryPercent"] = round(
            100.0 * (memory_total - memory_available) / memory_total, 3
        )
        return result


class DockerSampler:
    """Collect project-scoped Docker stats from a co-located runner."""

    def __init__(self, project: str) -> None:
        if not re.fullmatch(r"[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}", project):
            raise CapacityError("Compose project must be a safe 1-64 character name")
        self.project = project
        if shutil.which("docker") is None:
            raise CapacityError("co-located Docker collection requires docker on PATH")

    def describe(self) -> dict[str, Any]:
        version = subprocess.run(
            ["docker", "version", "--format", "{{.Server.Version}}"],
            check=True,
            capture_output=True,
            text=True,
            timeout=15,
        ).stdout.strip()
        if not version:
            raise CapacityError("Docker server version was empty")
        return {
            "status": "available",
            "source": "docker stats",
            "composeProject": self.project,
            "serverVersion": version,
        }

    def _container_ids(self) -> list[str]:
        result = subprocess.run(
            [
                "docker",
                "ps",
                "--filter",
                f"label=com.docker.compose.project={self.project}",
                "--format",
                "{{.ID}}",
            ],
            check=True,
            capture_output=True,
            text=True,
            timeout=15,
        )
        ids = [line.strip() for line in result.stdout.splitlines() if line.strip()]
        if not ids:
            raise CapacityError(
                f"no running containers found for Compose project {self.project}"
            )
        return ids

    def sample(self) -> list[dict[str, Any]]:
        result = subprocess.run(
            [
                "docker",
                "stats",
                "--no-stream",
                "--format",
                "{{json .}}",
                *self._container_ids(),
            ],
            check=True,
            capture_output=True,
            text=True,
            timeout=30,
        )
        samples: list[dict[str, Any]] = []
        for line in result.stdout.splitlines():
            if not line.strip():
                continue
            try:
                item = json.loads(line)
            except json.JSONDecodeError as exc:
                raise CapacityError("docker stats returned invalid JSON") from exc
            memory_used, memory_limit = parse_byte_pair(item.get("MemUsage", ""))
            network_in, network_out = parse_byte_pair(item.get("NetIO", ""))
            block_read, block_write = parse_byte_pair(item.get("BlockIO", ""))
            samples.append(
                {
                    "name": str(item.get("Name", "unknown"))[:128],
                    "cpuPercent": parse_percent(item.get("CPUPerc", "")),
                    "memoryPercent": parse_percent(item.get("MemPerc", "")),
                    "memoryUsedBytes": memory_used,
                    "memoryLimitBytes": memory_limit,
                    "networkInBytes": network_in,
                    "networkOutBytes": network_out,
                    "blockReadBytes": block_read,
                    "blockWriteBytes": block_write,
                    "pids": int(item.get("PIDs", 0)),
                }
            )
        if not samples:
            raise CapacityError("docker stats returned no project samples")
        return samples


def fetch_bounded_bytes(url: str, timeout: float) -> tuple[int, float]:
    url = require_network_url(
        url, "media segment URL", {"http", "https"}, allow_query=True
    )
    started = time.monotonic()
    request = urllib.request.Request(url, headers={"Accept": "*/*"})
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            content_length = response.headers.get("Content-Length")
            if content_length and int(content_length) > MAX_MEDIA_RESPONSE_BYTES:
                raise CapacityError("media response exceeds the 64 MiB safety cap")
            total = 0
            while True:
                chunk = response.read(min(1024 * 1024, MAX_MEDIA_RESPONSE_BYTES + 1 - total))
                if not chunk:
                    break
                total += len(chunk)
                if total > MAX_MEDIA_RESPONSE_BYTES:
                    raise CapacityError("media response exceeded the 64 MiB safety cap")
    except (OSError, urllib.error.URLError, ValueError) as exc:
        raise CapacityError(
            f"GET {golden.sanitize_url(url)} failed: {exc}"
        ) from exc
    return total, (time.monotonic() - started) * 1000


def resolve_safe_media_playlist(
    manifest_url: str, timeout: float
) -> tuple[str, str]:
    manifest_url = require_network_url(
        manifest_url,
        "playback manifest URL",
        {"http", "https"},
        allow_query=True,
    )
    manifest = golden.fetch_text(manifest_url, timeout)
    if "#EXT-X-STREAM-INF" not in manifest:
        return manifest_url, manifest
    uris = golden.playlist_uris(manifest)
    if not uris:
        raise CapacityError("HLS master playlist contained no variants")
    media_url = require_network_url(
        urllib.parse.urljoin(manifest_url, uris[0]),
        "media playlist URL",
        {"http", "https"},
        allow_query=True,
    )
    return media_url, golden.fetch_text(media_url, timeout)


def paced_worker(
    stop: threading.Event,
    requests_per_second: int,
    stats: WorkloadStats,
    kind: str,
    operation: Any,
) -> None:
    if requests_per_second <= 0:
        return
    interval = 1.0 / requests_per_second
    next_run = time.monotonic()
    while not stop.is_set():
        started = time.monotonic()
        try:
            byte_count = int(operation())
        except Exception:
            stats.record(kind, (time.monotonic() - started) * 1000, failed=True)
        else:
            stats.record(
                kind,
                (time.monotonic() - started) * 1000,
                byte_count=byte_count,
            )
        next_run += interval
        if next_run < time.monotonic():
            next_run = time.monotonic() + interval
        delay = max(0.0, next_run - time.monotonic())
        if stop.wait(delay):
            return


def viewer_worker(
    stop: threading.Event,
    playback_url: str,
    timeout: float,
    stats: WorkloadStats,
) -> None:
    last_media_url = ""
    next_run = time.monotonic()
    while not stop.is_set():
        playlist_started = time.monotonic()
        failed_kind = "viewerPlaylist"
        try:
            media_url, manifest = resolve_safe_media_playlist(
                playback_url, timeout
            )
            stats.record(
                "viewerPlaylist",
                (time.monotonic() - playlist_started) * 1000,
                byte_count=len(manifest.encode("utf-8")),
            )
            uris = golden.playlist_uris(manifest) or golden.playlist_part_uris(manifest)
            if not uris:
                raise CapacityError("viewer playlist contained no media URI")
            segment_url = urllib.parse.urljoin(media_url, uris[-1])
            if segment_url != last_media_url:
                failed_kind = "viewerMedia"
                byte_count, duration_ms = fetch_bounded_bytes(segment_url, timeout)
                stats.record(
                    "viewerMedia", duration_ms, byte_count=byte_count
                )
                last_media_url = segment_url
        except Exception:
            stats.record(
                failed_kind,
                (time.monotonic() - playlist_started) * 1000,
                failed=True,
            )
        next_run += 1.0
        if next_run < time.monotonic():
            next_run = time.monotonic() + 1.0
        stop.wait(max(0.0, next_run - time.monotonic()))


class Evidence:
    """Atomic capacity evidence writer with exact sentinel refusal."""

    def __init__(
        self,
        output_path: Path,
        identity: CandidateIdentity,
        scenario: Scenario,
        sentinels: list[str] | None = None,
        release_set: dict[str, Any] | None = None,
    ) -> None:
        self.output_path = output_path
        self.sentinels = sentinels if sentinels is not None else []
        self.payload: dict[str, Any] = {
            "schema": REPORT_SCHEMA,
            "status": "running",
            "startedAt": golden.utc_timestamp(),
            "candidate": {
                **identity.public(),
                "releaseSet": release_set
                if release_set is not None
                else {"status": "declared-only"},
            },
            "scenario": scenario.public_summary(),
            "runner": {
                "platform": platform.system().lower(),
                "architecture": platform.machine().lower(),
                "python": platform.python_version(),
            },
            "collectors": {
                name: {
                    "status": "unavailable",
                    "reason": "collector was not configured before the run failed",
                }
                for name in ("application", "loadClient", "host", "docker")
            },
            "phases": [],
            "samples": [],
            "stop": {"triggered": False, "reasons": []},
            "unproven": [
                "physical target-host safe operating range",
                "running container match to the expected release-set image digests",
                "WebRTC and browser playback compatibility",
                "VOD upload and publication capacity activity",
                "direct Postgres pool, Redis, encoder, and dropped-frame telemetry",
                "production SLO thresholds",
                "4K or multi-host capacity",
            ],
        }

    def add_sentinel(self, value: str) -> None:
        if value and value not in self.sentinels:
            self.sentinels.append(value)

    def finish(self, status: str) -> None:
        if status not in {"passed", "failed", "planned"}:
            raise CapacityError(f"invalid evidence status: {status}")
        self.payload["status"] = status
        self.payload["finishedAt"] = golden.utc_timestamp()
        validate_report(self.payload, self.sentinels)
        self.write()

    def write(self) -> None:
        clean = golden.json_safe(self.payload, self.sentinels)
        serialized = json.dumps(clean, indent=2, sort_keys=True) + "\n"
        for sentinel in self.sentinels:
            if sentinel and (
                sentinel in serialized
                or golden.urllib.parse.quote(sentinel, safe="") in serialized
            ):
                raise CapacityError("refusing to write secret-bearing evidence")
        self.output_path.parent.mkdir(parents=True, exist_ok=True)
        temporary = self.output_path.with_suffix(self.output_path.suffix + ".tmp")
        temporary.write_text(serialized, encoding="utf-8")
        temporary.replace(self.output_path)


def validate_report(payload: dict[str, Any], sentinels: Iterable[str] = ()) -> None:
    """Validate the retained evidence envelope before its final atomic write."""
    report = _mapping(payload, "capacity report")
    if report.get("schema") != REPORT_SCHEMA:
        raise CapacityError(f"capacity report schema must be {REPORT_SCHEMA}")
    status = report.get("status")
    if status not in {"planned", "passed", "failed"}:
        raise CapacityError("capacity report status must be planned, passed, or failed")
    candidate = _mapping(report.get("candidate"), "capacity report candidate")
    CandidateIdentity.parse(
        str(candidate.get("release", "")),
        str(candidate.get("releaseSetSha256", "")),
        str(candidate.get("sourceCommit", "")),
    )
    scenario = _mapping(report.get("scenario"), "capacity report scenario")
    if scenario.get("schema") != SCENARIO_SCHEMA:
        raise CapacityError("capacity report carries the wrong scenario schema")
    if not SHA256_RE.fullmatch(str(scenario.get("sha256", ""))):
        raise CapacityError("capacity report scenario hash must be 64 lowercase hex")
    collectors = _mapping(report.get("collectors"), "capacity report collectors")
    for required in ("application", "loadClient", "host", "docker"):
        collector = _mapping(
            collectors.get(required), f"capacity report collector {required}"
        )
        if collector.get("status") not in {"planned", "available", "unavailable"}:
            raise CapacityError(f"capacity report collector {required} has invalid status")
    phases = report.get("phases")
    samples = report.get("samples")
    if not isinstance(phases, list) or not isinstance(samples, list):
        raise CapacityError("capacity report phases and samples must be arrays")
    if status == "planned":
        if report.get("mode") != "dry-run" or phases or samples:
            raise CapacityError("planned capacity evidence must be an empty dry-run")
    elif report.get("mode") != "live":
        raise CapacityError("completed capacity evidence must use live mode")
    if status == "failed" and not str(report.get("failure", "")).strip():
        raise CapacityError("failed capacity evidence requires a failure reason")
    if status == "passed":
        release_set = _mapping(
            candidate.get("releaseSet"), "capacity report release set"
        )
        if (
            release_set.get("status") != "verified"
            or release_set.get("sha256") != candidate.get("releaseSetSha256")
        ):
            raise CapacityError(
                "passed capacity evidence requires verified release-set bytes"
            )
        for required in ("application", "loadClient"):
            if collectors[required].get("status") != "available":
                raise CapacityError(
                    f"passed capacity evidence requires {required} collection"
                )
        configured_phases = scenario.get("phases")
        if not isinstance(configured_phases, list) or len(phases) != len(
            configured_phases
        ):
            raise CapacityError("passed capacity evidence must complete every phase")
        configured_names = [item.get("name") for item in configured_phases]
        completed_names = [item.get("name") for item in phases]
        if configured_names != completed_names or any(
            item.get("status") != "passed" for item in phases
        ):
            raise CapacityError("capacity evidence phase order/status is incomplete")
        sampled_phases = {
            item.get("phase") for item in samples if isinstance(item, dict)
        }
        if any(name not in sampled_phases for name in configured_names):
            raise CapacityError("passed capacity evidence lacks a sample for every phase")
        stop = _mapping(report.get("stop"), "capacity report stop")
        if stop.get("triggered") is not False:
            raise CapacityError("passed capacity evidence cannot contain a stop trigger")
        final = _mapping(report.get("final"), "capacity report final application")
        final_metrics = _mapping(
            final.get("metrics"), "capacity report final application metrics"
        )
        final_totals = _mapping(
            final_metrics.get("totals"),
            "capacity report final application metric totals",
        )
        if (
            final_totals.get("activeStreams") != 0
            or final_totals.get("activeTranscoderJobs") != 0
        ):
            raise CapacityError(
                "passed capacity evidence requires zero final streams and jobs"
            )
    if not isinstance(report.get("unproven"), list) or not report["unproven"]:
        raise CapacityError("capacity report must retain explicit unproven claims")
    serialized = json.dumps(report, sort_keys=True)
    for sentinel in sentinels:
        if sentinel and (
            sentinel in serialized
            or golden.urllib.parse.quote(sentinel, safe="") in serialized
        ):
            raise CapacityError("capacity report contains a private sentinel")


def summarize_prometheus(samples: list[dict[str, Any]]) -> dict[str, Any]:
    """Retain only bounded aggregate application metrics needed for capacity."""
    totals: dict[str, float] = {}
    degraded_services: list[str] = []
    http_errors = 0.0
    for sample in samples:
        name = str(sample.get("name", ""))
        labels = sample.get("labels", {})
        value = float(sample.get("value", 0))
        if name == "bitriver_http_requests_total":
            totals["httpRequests"] = totals.get("httpRequests", 0) + value
            status = str(labels.get("status", ""))
            if status.startswith("5"):
                http_errors += value
        elif name == "bitriver_ingest_attempts_total":
            totals["ingestAttempts"] = totals.get("ingestAttempts", 0) + value
        elif name == "bitriver_ingest_failures_total":
            totals["ingestFailures"] = totals.get("ingestFailures", 0) + value
        elif name == "bitriver_chat_events_total":
            totals["chatEvents"] = totals.get("chatEvents", 0) + value
        elif name == "bitriver_transcoder_jobs_total":
            totals["transcoderJobEvents"] = totals.get("transcoderJobEvents", 0) + value
            if labels.get("status") == "fail":
                totals["transcoderFailures"] = totals.get("transcoderFailures", 0) + value
        elif name in {"bitriver_active_streams", "bitriver_transcoder_active_jobs"}:
            totals[
                "activeStreams"
                if name == "bitriver_active_streams"
                else "activeTranscoderJobs"
            ] = value
        elif name == "bitriver_ingest_health" and value < 0:
            degraded_services.append(str(labels.get("service", "unknown"))[:64])
    totals["http5xx"] = http_errors
    return {
        "totals": {key: round(value, 6) for key, value in sorted(totals.items())},
        "degradedServices": sorted(set(degraded_services)),
    }


@dataclasses.dataclass
class PublisherFixture:
    channel_id: str
    stream_key: str
    process: subprocess.Popen[str] | None = None
    playback_url: str = ""


class CapacityHarness:
    """Coordinate fixtures, publishers, load workers, and bounded sampling."""

    def __init__(
        self,
        args: argparse.Namespace,
        scenario: Scenario,
        evidence: Evidence,
    ) -> None:
        self.args = args
        self.scenario = scenario
        self.evidence = evidence
        self.stats = WorkloadStats()
        self.fixtures: list[PublisherFixture] = []
        self.creator: golden.ProductClient | None = None
        self.viewer: golden.ProductClient | None = None
        self.viewer_id = ""
        self.ffmpeg = ""
        self.metrics_headers: dict[str, str] = {}
        self.host_sampler: HostSampler | None = None
        self.docker_sampler: DockerSampler | None = None
        self.publisher_duration = int(
            scenario.duration_seconds
            + scenario.max_publishers * args.stage_timeout
            + 120
        )
        self.evidence.payload["mode"] = "live"
        self.ffmpeg = golden.command_path(args.ffmpeg, "ffmpeg")
        self.evidence.payload["collectors"]["loadClient"] = {
            "status": "available",
            "source": "public HTTP/RTMP synthetic load",
            "coLocatedWithTarget": args.collector_mode == "co-located",
        }
        self.evidence.payload["collectors"]["host"] = {
            "status": "unavailable",
            "reason": "runner was not declared co-located with the target host",
        }
        self.evidence.payload["collectors"]["docker"] = {
            "status": "unavailable",
            "reason": "runner was not declared co-located with the Compose host",
        }

    def add_sentinel(self, value: str) -> None:
        self.evidence.add_sentinel(value)
        golden.write_sentinels(self.args.sentinel_file, self.evidence.sentinels)

    def configure_collectors(self) -> None:
        metrics_token = self.args.metrics_bearer_file.read_text(
            encoding="utf-8"
        ).strip()
        if not metrics_token:
            raise CapacityError("metrics bearer file was empty")
        self.add_sentinel(metrics_token)
        self.metrics_headers["Authorization"] = f"Bearer {metrics_token}"
        if self.args.collector_mode != "co-located":
            return
        self.host_sampler = HostSampler(self.args.data_path.resolve())
        self.docker_sampler = DockerSampler(self.args.compose_project)
        self.evidence.payload["collectors"]["host"] = self.host_sampler.describe()
        self.evidence.payload["collectors"]["docker"] = self.docker_sampler.describe()
        self.evidence.payload["unproven"].append(
            "co-located load generation is included in host resource samples"
        )

    def collect_application(self) -> tuple[bool, dict[str, Any]]:
        client = golden.ProductClient(self.args.base_url, self.args.http_timeout)
        ready = client.request(
            "/readyz", parse_json=False, expected=(200, 503)
        )
        health = client.request(
            "/healthz", parse_json=False, expected=(200, 503)
        )
        metrics_response = client.request(
            "/metrics",
            headers=self.metrics_headers,
            parse_json=False,
            expected=(200,),
        )
        metrics_text = metrics_response["body"].decode("utf-8", errors="strict")
        parsed = parse_prometheus(metrics_text)
        if not any(str(item.get("name", "")).startswith("bitriver_") for item in parsed):
            raise CapacityError("protected metrics exposed no BitRiver samples")
        healthy = ready["status"] == 200 and health["status"] == 200
        return healthy, {
            "readyStatus": ready["status"],
            "healthStatus": health["status"],
            "metrics": summarize_prometheus(parsed),
        }

    def preflight(self) -> None:
        healthy, application = self.collect_application()
        self.evidence.payload["collectors"]["application"] = {
            "status": "available",
            "source": "public /readyz, /healthz, and protected /metrics",
        }
        if not healthy:
            raise CapacityError(
                "dedicated stack must pass /readyz and /healthz before load"
            )
        self.evidence.payload["preflight"] = application

    def provision(self) -> None:
        suffix = secrets.token_hex(8)
        creator_email = f"capacity-creator-{suffix}@example.invalid"
        viewer_email = f"capacity-viewer-{suffix}@example.invalid"
        creator_password = f"Cq!{secrets.token_urlsafe(24)}"
        viewer_password = f"Cq!{secrets.token_urlsafe(24)}"
        for sentinel in (
            creator_email,
            viewer_email,
            creator_password,
            viewer_password,
        ):
            self.add_sentinel(sentinel)
        self.creator = golden.ProductClient(
            self.args.base_url, self.args.http_timeout, self.add_sentinel
        )
        self.viewer = golden.ProductClient(
            self.args.base_url, self.args.http_timeout, self.add_sentinel
        )
        golden.signup(
            self.creator,
            f"Capacity Creator {suffix}",
            creator_email,
            creator_password,
        )
        viewer_user = golden.signup(
            self.viewer,
            f"Capacity Viewer {suffix}",
            viewer_email,
            viewer_password,
        )
        self.viewer_id = golden.require_string(viewer_user, "id", "capacity viewer")
        for index in range(self.scenario.max_publishers):
            channel = golden.require_mapping(
                self.creator.request(
                    "/api/channels",
                    method="POST",
                    payload={
                        "title": f"Capacity Fixture {index + 1} {suffix}",
                        "category": "testing",
                        "tags": ["capacity-qualification", "1080p"],
                    },
                    expected=(201,),
                ),
                "capacity channel response",
            )
            channel_id = golden.require_string(channel, "id", "capacity channel")
            stream_key = golden.require_string(channel, "streamKey", "capacity channel")
            self.add_sentinel(stream_key)
            self.fixtures.append(PublisherFixture(channel_id, stream_key))
        self.evidence.payload["fixture"] = {
            "accounts": 2,
            "channels": len(self.fixtures),
            "video": "testsrc2",
            "width": 1920,
            "height": 1080,
            "frameRate": 15,
            "videoBitrateKbps": 2500,
            "audio": "sine-1000hz",
            "audioBitrateKbps": 128,
        }

    def _start_publisher(self, fixture: PublisherFixture) -> None:
        if fixture.process is not None and fixture.process.poll() is None:
            return
        rtmp_url = (
            f"{self.args.rtmp_base_url.rstrip('/')}/"
            f"{urllib.parse.quote(fixture.stream_key, safe='')}"
        )
        fixture.process = golden.start_live_publisher(
            self.ffmpeg, rtmp_url, self.publisher_duration
        )

        def playback_ready() -> str | None:
            assert fixture.process is not None
            if fixture.process.poll() is not None:
                diagnostic = golden.stop_process(
                    fixture.process, self.evidence.sentinels
                )
                raise CapacityError(
                    f"publisher exited before live state: {diagnostic}"
                )
            response = golden.require_mapping(
                golden.ProductClient(
                    self.args.base_url, self.args.http_timeout
                ).request(
                    f"/api/channels/{urllib.parse.quote(fixture.channel_id)}/playback"
                ),
                "capacity playback response",
            )
            playback = response.get("playback")
            if response.get("live") is not True or not isinstance(playback, dict):
                return None
            return golden.rewrite_media_host(
                golden.select_transcoder_manifest(playback),
                self.args.media_host_override,
            )

        fixture.playback_url = golden.bounded_poll(
            "capacity publisher live playback",
            self.args.stage_timeout,
            playback_ready,
        )

    def _stop_publisher(self, fixture: PublisherFixture) -> None:
        if fixture.process is not None:
            golden.stop_process(fixture.process, self.evidence.sentinels)
        fixture.process = None
        fixture.playback_url = ""

    def set_publisher_count(self, count: int) -> list[PublisherFixture]:
        for fixture in self.fixtures[:count]:
            self._start_publisher(fixture)
        for fixture in self.fixtures[count:]:
            self._stop_publisher(fixture)
        return self.fixtures[:count]

    def _api_operation(self) -> int:
        response = golden.ProductClient(
            self.args.base_url, self.args.http_timeout
        ).request("/api/channels?limit=50")
        return len(json.dumps(response, separators=(",", ":")).encode("utf-8"))

    def _chat_operation(self, sequence: list[int]) -> int:
        if self.viewer is None or not self.fixtures:
            raise CapacityError("chat fixture is not initialized")
        sequence[0] += 1
        response = self.viewer.request(
            f"/api/channels/{urllib.parse.quote(self.fixtures[0].channel_id)}/chat",
            method="POST",
            payload={
                "userId": self.viewer_id,
                "content": f"capacity-message-{sequence[0]}",
            },
            expected=(201,),
        )
        return len(json.dumps(response, separators=(",", ":")).encode("utf-8"))

    def _sample(
        self,
        phase: Phase,
        phase_started: float,
        interval_before: dict[str, dict[str, Any]],
    ) -> tuple[dict[str, Any], dict[str, dict[str, Any]]]:
        healthy, application = self.collect_application()
        current = self.stats.snapshot()
        interval = WorkloadStats.delta(interval_before, current)
        workload = summarize_workload(interval)
        sample: dict[str, Any] = {
            "timestamp": golden.utc_timestamp(),
            "phase": phase.name,
            "phaseElapsedMs": golden.elapsed_ms(phase_started),
            "healthy": healthy,
            "application": application,
            "workload": workload["total"],
            "workloadByType": {
                key: value for key, value in workload.items() if key != "total"
            },
            "publishers": phase.publishers,
            "viewers": phase.viewers,
        }
        totals = application["metrics"]["totals"]
        active_streams = totals.get("activeStreams")
        active_jobs = totals.get("activeTranscoderJobs")
        if active_streams != phase.publishers:
            raise CapacityError(
                f"application reported {active_streams!r} active streams during "
                f"{phase.name}; expected {phase.publishers}"
            )
        if active_jobs != phase.publishers:
            raise CapacityError(
                f"application reported {active_jobs!r} active transcoder jobs "
                f"during {phase.name}; expected {phase.publishers}"
            )
        if self.host_sampler is not None:
            sample["host"] = self.host_sampler.sample()
        if self.docker_sampler is not None:
            sample["containers"] = self.docker_sampler.sample()
        for index, fixture in enumerate(self.fixtures[: phase.publishers]):
            if fixture.process is None or fixture.process.poll() is not None:
                diagnostic = (
                    golden.stop_process(fixture.process, self.evidence.sentinels)
                    if fixture.process is not None
                    else "not running"
                )
                raise CapacityError(
                    f"publisher {index + 1} exited during {phase.name}: {diagnostic}"
                )
        return sample, current

    def run_phase(self, phase: Phase) -> None:
        active = self.set_publisher_count(phase.publishers)
        if any(not fixture.playback_url for fixture in active):
            raise CapacityError("active publisher lacks a playback URL")
        phase_record: dict[str, Any] = {
            "name": phase.name,
            "status": "running",
            "configuredDurationSeconds": phase.duration_seconds,
            "publishers": phase.publishers,
            "viewers": phase.viewers,
            "apiRequestsPerSecond": phase.api_requests_per_second,
            "chatMessagesPerSecond": phase.chat_messages_per_second,
        }
        self.evidence.payload["phases"].append(phase_record)
        self.evidence.write()
        stop = threading.Event()
        phase_started = time.monotonic()
        phase_stats_before = self.stats.snapshot()
        interval_before = phase_stats_before
        evaluator = StopEvaluator(self.scenario.stop_conditions)
        chat_sequence = [0]
        workers = phase.viewers
        workers += int(phase.api_requests_per_second > 0)
        workers += int(phase.chat_messages_per_second > 0)
        executor = concurrent.futures.ThreadPoolExecutor(
            max_workers=max(1, workers), thread_name_prefix="capacity-load"
        )
        futures: list[concurrent.futures.Future[Any]] = []
        try:
            for index in range(phase.viewers):
                playback_url = active[index % len(active)].playback_url
                futures.append(
                    executor.submit(
                        viewer_worker,
                        stop,
                        playback_url,
                        self.args.http_timeout,
                        self.stats,
                    )
                )
            if phase.api_requests_per_second:
                futures.append(
                    executor.submit(
                        paced_worker,
                        stop,
                        phase.api_requests_per_second,
                        self.stats,
                        "api",
                        self._api_operation,
                    )
                )
            if phase.chat_messages_per_second:
                futures.append(
                    executor.submit(
                        paced_worker,
                        stop,
                        phase.chat_messages_per_second,
                        self.stats,
                        "chat",
                        lambda: self._chat_operation(chat_sequence),
                    )
                )
            deadline = phase_started + phase.duration_seconds
            while time.monotonic() < deadline:
                delay = min(
                    self.scenario.sample_interval_seconds,
                    max(0.0, deadline - time.monotonic()),
                )
                if delay:
                    time.sleep(delay)
                sample, interval_before = self._sample(
                    phase, phase_started, interval_before
                )
                self.evidence.payload["samples"].append(sample)
                reasons = evaluator.evaluate(sample)
                if reasons:
                    self.evidence.payload["stop"] = {
                        "triggered": True,
                        "phase": phase.name,
                        "reasons": reasons,
                    }
                    self.evidence.write()
                    raise CapacityError("; ".join(reasons))
        except (Exception, KeyboardInterrupt):
            phase_record["status"] = "failed"
            phase_record["durationMs"] = golden.elapsed_ms(phase_started)
            raise
        finally:
            stop.set()
            executor.shutdown(wait=True, cancel_futures=True)
            for future in futures:
                if future.cancelled():
                    continue
                try:
                    future.result(timeout=0)
                except Exception:
                    pass
        phase_record["durationMs"] = golden.elapsed_ms(phase_started)
        phase_record["workload"] = summarize_workload(
            WorkloadStats.delta(phase_stats_before, self.stats.snapshot())
        )
        failures = phase_workload_failures(
            phase,
            phase_record["workload"],
            self.scenario.stop_conditions.max_error_rate,
        )
        if failures:
            phase_record["status"] = "failed"
            self.evidence.write()
            raise CapacityError("; ".join(failures))
        phase_record["status"] = "passed"
        self.evidence.write()

    def cleanup(self) -> None:
        for fixture in self.fixtures:
            self._stop_publisher(fixture)

    def run(self) -> None:
        self.configure_collectors()
        self.preflight()
        self.provision()
        try:
            for phase in self.scenario.phases:
                self.run_phase(phase)
            self.set_publisher_count(0)

            def workload_drained() -> dict[str, Any] | None:
                healthy, application = self.collect_application()
                totals = application["metrics"]["totals"]
                if (
                    healthy
                    and totals.get("activeStreams") == 0
                    and totals.get("activeTranscoderJobs") == 0
                ):
                    return application
                return None

            final_application = golden.bounded_poll(
                "capacity workload drain",
                self.args.stage_timeout,
                workload_drained,
            )
            self.evidence.payload["final"] = final_application
        finally:
            self.cleanup()


def run_live(
    args: argparse.Namespace,
    identity: CandidateIdentity,
    scenario: Scenario,
) -> Path:
    output = args.artifact_dir / "capacity-report.json"
    release_set = load_release_set(args.release_set_file, identity)
    evidence = Evidence(output, identity, scenario, release_set=release_set)
    try:
        harness = CapacityHarness(args, scenario, evidence)
        harness.run()
    except (Exception, KeyboardInterrupt) as exc:
        failure = golden.sanitize_text(exc, evidence.sentinels).strip()
        evidence.payload["failure"] = failure or "capacity run interrupted"
        evidence.finish("failed")
        raise
    evidence.finish("passed")
    return output


def write_dry_run(
    artifact_dir: Path, identity: CandidateIdentity, scenario: Scenario
) -> Path:
    output = artifact_dir / "capacity-report.json"
    evidence = Evidence(output, identity, scenario)
    evidence.payload["mode"] = "dry-run"
    evidence.payload["collectors"] = {
        "application": {"status": "planned", "source": "/metrics and probes"},
        "loadClient": {"status": "planned", "source": "public HTTP/RTMP"},
        "host": {
            "status": "unavailable",
            "reason": "dry-run does not claim co-located host resources",
        },
        "docker": {
            "status": "unavailable",
            "reason": "dry-run does not claim Compose container resources",
        },
    }
    evidence.finish("planned")
    return output


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Run or validate a bounded BitRiver capacity scenario."
    )
    parser.add_argument("--scenario", type=Path, required=True)
    parser.add_argument("--artifact-dir", type=Path, required=True)
    parser.add_argument("--release", required=True)
    parser.add_argument("--release-set-sha256", required=True)
    parser.add_argument("--source-commit", required=True)
    parser.add_argument("--release-set-file", type=Path)
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="validate, hash, and report the scenario without contacting a stack",
    )
    parser.add_argument("--base-url", default="http://localhost:18080")
    parser.add_argument("--rtmp-base-url", default="rtmp://localhost:1935/live")
    parser.add_argument("--media-host-override", default="")
    parser.add_argument("--metrics-bearer-file", type=Path)
    parser.add_argument("--sentinel-file", type=Path)
    parser.add_argument("--ffmpeg", default="ffmpeg")
    parser.add_argument("--http-timeout", type=float, default=15.0)
    parser.add_argument("--stage-timeout", type=float, default=120.0)
    parser.add_argument(
        "--collector-mode",
        choices=("remote", "co-located"),
        default="remote",
    )
    parser.add_argument("--compose-project", default="bitriver-live")
    parser.add_argument("--data-path", type=Path)
    parser.add_argument(
        "--confirm-dedicated-environment",
        action="store_true",
        help="required for live load; confirms fixtures and load may modify the target",
    )
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv or sys.argv[1:])
    try:
        identity = CandidateIdentity.parse(
            args.release, args.release_set_sha256, args.source_commit
        )
        scenario = load_scenario(args.scenario)
        require_network_url(args.base_url, "base URL", {"http", "https"})
        require_network_url(args.rtmp_base_url, "RTMP base URL", {"rtmp", "rtmps"})
        if args.dry_run:
            output = write_dry_run(args.artifact_dir, identity, scenario)
        else:
            if not args.confirm_dedicated_environment:
                raise CapacityError(
                    "live execution requires --confirm-dedicated-environment"
                )
            if args.metrics_bearer_file is None:
                raise CapacityError("live execution requires --metrics-bearer-file")
            if not args.metrics_bearer_file.is_file():
                raise CapacityError("metrics bearer file does not exist")
            if args.sentinel_file is None:
                raise CapacityError("live execution requires --sentinel-file")
            if args.release_set_file is None or not args.release_set_file.is_file():
                raise CapacityError(
                    "live execution requires an existing --release-set-file"
                )
            if args.http_timeout <= 0 or args.stage_timeout <= 0:
                raise CapacityError("HTTP and stage timeouts must be positive")
            if args.collector_mode == "co-located":
                if args.data_path is None or not args.data_path.exists():
                    raise CapacityError(
                        "co-located collection requires an existing --data-path"
                    )
            output = run_live(args, identity, scenario)
    except (CapacityError, golden.GoldenPathError) as exc:
        print(f"capacity qualification refused: {exc}", file=sys.stderr)
        return 1
    except KeyboardInterrupt:
        print("capacity qualification interrupted", file=sys.stderr)
        return 130
    print(f"capacity qualification evidence: {output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
