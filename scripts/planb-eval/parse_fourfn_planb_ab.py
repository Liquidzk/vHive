#!/usr/bin/env python3
"""Parse four-function private- or full-WS Plan B A/B results."""

from __future__ import annotations

import argparse
import csv
import math
import re
import statistics
from collections import defaultdict
from pathlib import Path


NUMBER = r"([0-9]+(?:\.[0-9]+)?(?:e[+-]?[0-9]+)?)"
PATTERNS = {
    "download_us": re.compile(rf"Downloaded snapshot for rev .* in {NUMBER}", re.MULTILINE),
    "decompress_us": re.compile(r"Plan B (?:private|full) WS decompressed: .* decompress_us=(\d+)"),
    "codec_total_us": re.compile(r"Plan B (?:private|full) WS decompressed: .* total_us=(\d+)"),
    "preinserted_pages": re.compile(r"Pre-inserting working set of (\d+) pages"),
    "private_pages": re.compile(r"private page count: (\d+)"),
    "uffd_copies": re.compile(r"UFFD copy operations: (\d+)"),
    "load_snapshot_us": re.compile(rf"Loaded snapshot for rev .* in {NUMBER}", re.MULTILINE),
    "working_set_content_us": re.compile(rf"^GetWorkingSetContent:\s+{NUMBER}$", re.MULTILINE),
    "load_vmm_us": re.compile(rf"^LoadVMM:\s+{NUMBER}$", re.MULTILINE),
    "post_prefill_faults": re.compile(r"Handled (\d+) page faults"),
}


def optional_number(pattern: re.Pattern[str], text: str, convert=float):
    match = pattern.search(text)
    return "" if match is None else convert(match.group(1))


def go_duration_ms(value: str) -> float:
    units = {"h": 3_600_000, "m": 60_000, "s": 1_000, "ms": 1, "µs": 0.001, "ns": 0.000001}
    tokens = re.findall(r"([0-9]+(?:\.[0-9]+)?)(h|ms|µs|ns|m|s)", value)
    if not tokens:
        raise ValueError(f"invalid Go duration: {value!r}")
    return sum(float(number) * units[unit] for number, unit in tokens)


def duration_after(pattern: str, text: str):
    duration = r"((?:[0-9]+(?:\.[0-9]+)?(?:h|ms|µs|ns|m|s))+)"
    match = re.search(pattern + duration, text)
    return "" if match is None else go_duration_ms(match.group(1))


def percentile(values: list[float], fraction: float) -> float:
    ordered = sorted(values)
    if len(ordered) == 1:
        return ordered[0]
    position = (len(ordered) - 1) * fraction
    lower, upper = math.floor(position), math.ceil(position)
    if lower == upper:
        return ordered[lower]
    return ordered[lower] + (ordered[upper] - ordered[lower]) * (position - lower)


def summarize(values: list[float]) -> dict[str, float | int | str]:
    if not values:
        return {key: "" for key in ("n", "mean", "median", "p95", "stdev", "min", "max")}
    return {
        "n": len(values),
        "mean": statistics.fmean(values),
        "median": statistics.median(values),
        "p95": percentile(values, 0.95),
        "stdev": statistics.stdev(values) if len(values) > 1 else 0.0,
        "min": min(values),
        "max": max(values),
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("result_root", type=Path)
    args = parser.parse_args()
    root = args.result_root.resolve()

    with (root / "calls.csv").open(newline="") as source:
        calls = list(csv.DictReader(source))

    detailed: list[dict[str, object]] = []
    for call in calls:
        if not call.get("scope"):
            call["scope"] = "private"
        segment_path = Path(call["segment"])
        if not segment_path.is_file():
            segment_path = (
                root
                / "runs"
                / call["workload"]
                / call["codec"]
                / call["path"]
                / f"relay-segment-{call['rep']}.log"
            )
        text = segment_path.read_text(errors="replace")
        row: dict[str, object] = dict(call)
        row.update(
            invocation_ms=duration_after(r"Invocation to .* completed in ", text),
            remote_download_us=optional_number(PATTERNS["download_us"], text),
            decompress_us=optional_number(PATTERNS["decompress_us"], text, int),
            codec_total_us=optional_number(PATTERNS["codec_total_us"], text, int),
            ws_install_ms=duration_after(r"\((?:Split|Monolithic) WS\) Pre-inserting .* in ", text),
            preinserted_pages=optional_number(PATTERNS["preinserted_pages"], text, int),
            private_pages=optional_number(PATTERNS["private_pages"], text, int),
            uffd_copies=optional_number(PATTERNS["uffd_copies"], text, int),
            load_snapshot_us=optional_number(PATTERNS["load_snapshot_us"], text),
            working_set_content_us=optional_number(PATTERNS["working_set_content_us"], text),
            load_vmm_us=optional_number(PATTERNS["load_vmm_us"], text),
            post_prefill_faults=optional_number(PATTERNS["post_prefill_faults"], text, int),
            used_remote=int("Using remote snapshot for rev" in text),
            codec_fallback=int("falling back to raw content" in text),
        )
        detailed.append(row)

    detail_fields = list(detailed[0]) if detailed else []
    with (root / "detailed.csv").open("w", newline="") as output:
        writer = csv.DictWriter(output, fieldnames=detail_fields)
        writer.writeheader()
        writer.writerows(detailed)

    metrics = (
        "e2e_ms",
        "mongo_setup_us",
        "invocation_ms",
        "remote_download_us",
        "decompress_us",
        "codec_total_us",
        "ws_install_ms",
        "preinserted_pages",
        "load_snapshot_us",
        "working_set_content_us",
        "load_vmm_us",
        "post_prefill_faults",
    )
    groups: dict[tuple[str, str, str, str], list[dict[str, object]]] = defaultdict(list)
    for row in detailed:
        groups[(str(row["workload"]), str(row["scope"]), str(row["codec"]), str(row["path"]))].append(row)

    summary_rows: list[dict[str, object]] = []
    for (workload, scope, codec, path), rows in sorted(groups.items()):
        base: dict[str, object] = {
            "workload": workload,
            "scope": scope,
            "codec": codec,
            "path": path,
            "calls": len(rows),
            "failures": sum(int(row["status"] != "0" or row["correct"] != "1") for row in rows),
            "fallbacks": sum(int(row["codec_fallback"]) for row in rows),
        }
        for metric in metrics:
            values = [float(row[metric]) for row in rows if row[metric] != ""]
            for statistic, value in summarize(values).items():
                base[f"{metric}_{statistic}"] = value
        summary_rows.append(base)

    summary_fields = list(summary_rows[0]) if summary_rows else []
    with (root / "summary.csv").open("w", newline="") as output:
        writer = csv.DictWriter(output, fieldnames=summary_fields)
        writer.writeheader()
        writer.writerows(summary_rows)

    print(f"parsed calls={len(detailed)} groups={len(summary_rows)} root={root}")


if __name__ == "__main__":
    main()
