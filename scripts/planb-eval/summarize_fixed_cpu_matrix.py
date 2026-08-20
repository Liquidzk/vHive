#!/usr/bin/env python3
"""Aggregate the five-workload fixed-CPU Private-WS matrix."""

from __future__ import annotations

import csv
import math
import statistics
import sys
from collections import defaultdict
from pathlib import Path


WORKLOADS = {
    "aes-go",
    "image-rotate-go",
    "image-rotate-python",
    "video-processing-python",
    "video-analytics-standalone-python",
}
CONFIGS = {
    ("raw", 1, 1),
    ("gzip", 1, 1),
    ("sw_deflate", 1, 1),
    ("zstd_3", 1, 1),
    ("iaa_deflate", 1, 1),
    ("iaa_deflate", 2, 2),
    ("iaa_deflate", 4, 4),
    ("iaa_deflate", 8, 8),
}
METRICS = (
    "e2e_ms",
    "mongo_setup_us",
    "invocation_ms",
    "remote_download_us",
    "decompress_us",
    "codec_total_us",
    "ws_install_ms",
)


def percentile(values: list[float], fraction: float) -> float:
    ordered = sorted(values)
    position = (len(ordered) - 1) * fraction
    lower, upper = math.floor(position), math.ceil(position)
    if lower == upper:
        return ordered[lower]
    return ordered[lower] + (ordered[upper] - ordered[lower]) * (position - lower)


def formatted(value: float) -> str:
    return f"{value:.3f}"


def read_env(path: Path) -> dict[str, str]:
    result = {}
    for line in path.read_text().splitlines():
        key, value = line.split("=", 1)
        result[key] = value
    return result


def write_rows(path: Path, rows: list[dict[str, str]]) -> None:
    if not rows:
        raise SystemExit(f"refusing to write empty aggregate: {path}")
    fields: list[str] = []
    for row in rows:
        for field in row:
            if field not in fields:
                fields.append(field)
    with path.open("w", newline="") as output:
        writer = csv.DictWriter(output, fieldnames=fields, extrasaction="ignore")
        writer.writeheader()
        writer.writerows(rows)


def main() -> None:
    if len(sys.argv) != 2:
        raise SystemExit("usage: summarize_fixed_cpu_matrix.py RESULT_ROOT")
    root = Path(sys.argv[1]).resolve()
    metadata = read_env(root / "matrix.env")
    repetitions = int(metadata["repetitions"])
    expected_calls = int(metadata["expected_calls"])
    expected_variants = int(metadata["expected_variants"])

    all_calls: list[dict[str, str]] = []
    all_variants: list[dict[str, str]] = []
    all_detailed: list[dict[str, str]] = []
    children = sorted(path for path in root.iterdir() if path.is_dir() and (path / "complete.marker").is_file())
    for child in children:
        for filename, destination in (
            ("calls.csv", all_calls),
            ("variants.csv", all_variants),
            ("detailed.csv", all_detailed),
        ):
            with (child / filename).open(newline="") as source:
                for row in csv.DictReader(source):
                    row["matrix_child"] = child.name
                    destination.append(row)

    if len(children) != 8:
        raise SystemExit(f"expected 8 completed children, found {len(children)}")
    if len(all_calls) != expected_calls:
        raise SystemExit(f"expected {expected_calls} calls, found {len(all_calls)}")
    if len(all_variants) != expected_variants:
        raise SystemExit(f"expected {expected_variants} variants, found {len(all_variants)}")
    if len(all_detailed) != expected_calls:
        raise SystemExit(f"expected {expected_calls} detailed rows, found {len(all_detailed)}")

    variant_keys = {
        (row["workload"], row["codec"], int(row["partitions"]), int(row["jobs"]))
        for row in all_variants
    }
    expected_variant_keys = {
        (workload, codec, partitions, jobs)
        for workload in WORKLOADS
        for codec, partitions, jobs in CONFIGS
    }
    if variant_keys != expected_variant_keys:
        missing = sorted(expected_variant_keys - variant_keys)
        extra = sorted(variant_keys - expected_variant_keys)
        raise SystemExit(f"variant matrix mismatch: missing={missing} extra={extra}")

    failures = [row for row in all_calls if row["status"] != "0" or row["correct"] != "1"]
    if failures:
        raise SystemExit(f"matrix contains {len(failures)} failed/incorrect calls")

    variants = {
        (row["workload"], row["codec"], int(row["partitions"]), int(row["jobs"])): row
        for row in all_variants
    }
    groups: dict[tuple[str, str, int, int, str], list[dict[str, str]]] = defaultdict(list)
    for row in all_detailed:
        key = (
            row["workload"],
            row["codec"],
            int(row["partitions"]),
            int(row["jobs"]),
            row["path"],
        )
        groups[key].append(row)

    expected_group_keys = {
        (workload, codec, partitions, jobs, path)
        for workload in WORKLOADS
        for codec, partitions, jobs in CONFIGS
        for path in ("remote", "local")
    }
    if set(groups) != expected_group_keys:
        raise SystemExit("summary group matrix does not match the requested configurations")

    summary: list[dict[str, str]] = []
    for (workload, codec, partitions, jobs, path), rows in sorted(groups.items()):
        if len(rows) != repetitions:
            raise SystemExit(
                f"{workload}/{codec}/{partitions}/{jobs}/{path}: "
                f"expected {repetitions} calls, found {len(rows)}"
            )
        variant = variants[(workload, codec, partitions, jobs)]
        result = {
            "workload": workload,
            "codec": codec,
            "partitions": str(partitions),
            "jobs": str(jobs),
            "path": path,
            "runs": str(len(rows)),
            "failures": "0",
            "fallbacks": str(sum(int(row.get("codec_fallback", "0")) for row in rows)),
            "input_bytes": variant["input_bytes"],
            "payload_bytes": variant["payload_bytes"],
            "compression_ratio": variant["ratio"],
        }
        for metric in METRICS:
            values = [float(row[metric]) for row in rows if row.get(metric, "") != ""]
            result[f"{metric}_mean"] = formatted(statistics.fmean(values)) if values else ""
            result[f"{metric}_p50"] = formatted(percentile(values, 0.50)) if values else ""
            result[f"{metric}_p95"] = formatted(percentile(values, 0.95)) if values else ""
        summary.append(result)

    if any(row["fallbacks"] != "0" for row in summary):
        raise SystemExit("matrix contains one or more codec fallbacks")

    write_rows(root / "calls.csv", all_calls)
    write_rows(root / "variants.csv", all_variants)
    write_rows(root / "detailed.csv", all_detailed)
    write_rows(root / "summary.csv", summary)
    print(
        f"FIXED4G_MATRIX_SUMMARY calls={len(all_calls)} variants={len(all_variants)} "
        f"groups={len(summary)} failures=0 fallbacks=0 root={root}"
    )


if __name__ == "__main__":
    main()
