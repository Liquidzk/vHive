#!/usr/bin/env python3
"""Summarize one-workload Plan B partition/job sweeps."""

import csv
import math
import re
import statistics
import sys
from pathlib import Path


DECODE_RE = re.compile(
    r"Plan B private WS decompressed:.*?decompress_us=(\d+).*?total_us=(\d+)"
)
PARTITIONS_RE = re.compile(r"Number of partitions:\s*(\d+)")


def percentile(values, fraction):
    ordered = sorted(values)
    if not ordered:
        return math.nan
    position = (len(ordered) - 1) * fraction
    lower = math.floor(position)
    upper = math.ceil(position)
    if lower == upper:
        return float(ordered[lower])
    weight = position - lower
    return ordered[lower] * (1 - weight) + ordered[upper] * weight


def main():
    if len(sys.argv) != 2:
        raise SystemExit("usage: summarize_planb_partition_sweep.py RESULT_ROOT")
    root = Path(sys.argv[1])
    detailed = []
    variant_by_config = {}

    for child in sorted(root.glob("p*j*")):
        variants_path = child / "variants.csv"
        calls_path = child / "calls.csv"
        if not variants_path.is_file() or not calls_path.is_file():
            continue
        with variants_path.open(newline="") as source:
            variants = list(csv.DictReader(source))
        if len(variants) != 1:
            raise SystemExit(f"expected one variant in {variants_path}, got {len(variants)}")
        variant = variants[0]
        config = (int(variant["partitions"]), int(variant["jobs"]))
        if config in variant_by_config:
            raise SystemExit(f"duplicate partition/job configuration: {config}")
        variant_by_config[config] = variant

        with calls_path.open(newline="") as source:
            for row in csv.DictReader(source):
                segment_path = Path(row["segment"])
                if not segment_path.is_file():
                    segment_path = (
                        child
                        / "runs"
                        / row["workload"]
                        / row["codec"]
                        / row["path"]
                        / f"relay-segment-{row['rep']}.log"
                    )
                text = segment_path.read_text(errors="replace")
                decode_matches = DECODE_RE.findall(text)
                partition_matches = PARTITIONS_RE.findall(text)
                if len(decode_matches) != 1 or not partition_matches:
                    raise SystemExit(
                        f"missing unique Plan B metrics in {segment_path}: "
                        f"decode={len(decode_matches)} partitions={len(partition_matches)}"
                    )
                observed_partitions = int(partition_matches[-1])
                if observed_partitions != config[0]:
                    raise SystemExit(
                        f"{segment_path}: observed {observed_partitions} partitions, "
                        f"expected {config[0]}"
                    )
                decode_us, total_us = map(int, decode_matches[0])
                detailed.append(
                    {
                        "workload": row["workload"],
                        "partitions": config[0],
                        "jobs": config[1],
                        "path": row["path"],
                        "rep": int(row["rep"]),
                        "e2e_ms": int(row["e2e_ms"]),
                        "status": int(row["status"]),
                        "correct": int(row["correct"]),
                        "mongo_setup_us": int(row["mongo_setup_us"]) if row["mongo_setup_us"] else "",
                        "observed_partitions": observed_partitions,
                        "decompress_us": decode_us,
                        "restore_total_us": total_us,
                        "segment": str(segment_path),
                    }
                )

    if not detailed:
        raise SystemExit(f"no calls found under {root}")

    detailed_path = root / "detailed.csv"
    with detailed_path.open("w", newline="") as target:
        writer = csv.DictWriter(target, fieldnames=list(detailed[0]))
        writer.writeheader()
        writer.writerows(detailed)

    summary_rows = []
    for config in sorted(variant_by_config):
        variant = variant_by_config[config]
        for path in ("remote", "local"):
            group = [
                row
                for row in detailed
                if (row["partitions"], row["jobs"]) == config and row["path"] == path
            ]
            failures = sum(row["status"] != 0 or row["correct"] != 1 for row in group)
            e2e = [row["e2e_ms"] for row in group]
            mongo_ms = [row["mongo_setup_us"] / 1000 for row in group if row["mongo_setup_us"] != ""]
            decode_ms = [row["decompress_us"] / 1000 for row in group]
            restore_total_ms = [row["restore_total_us"] / 1000 for row in group]
            summary_rows.append(
                {
                    "workload": variant["workload"],
                    "partitions": config[0],
                    "jobs": config[1],
                    "path": path,
                    "runs": len(group),
                    "failures": failures,
                    "input_bytes": variant["input_bytes"],
                    "payload_bytes": variant["payload_bytes"],
                    "compression_ratio": variant["ratio"],
                    "e2e_p50_ms": f"{percentile(e2e, 0.50):.3f}",
                    "e2e_p95_ms": f"{percentile(e2e, 0.95):.3f}",
                    "mongo_setup_p50_ms": f"{percentile(mongo_ms, 0.50):.3f}",
                    "mongo_setup_p95_ms": f"{percentile(mongo_ms, 0.95):.3f}",
                    "decode_mean_ms": f"{statistics.fmean(decode_ms):.3f}",
                    "decode_p50_ms": f"{percentile(decode_ms, 0.50):.3f}",
                    "decode_p95_ms": f"{percentile(decode_ms, 0.95):.3f}",
                    "restore_total_p50_ms": f"{percentile(restore_total_ms, 0.50):.3f}",
                    "restore_total_p95_ms": f"{percentile(restore_total_ms, 0.95):.3f}",
                }
            )

    summary_path = root / "summary.csv"
    with summary_path.open("w", newline="") as target:
        writer = csv.DictWriter(target, fieldnames=list(summary_rows[0]))
        writer.writeheader()
        writer.writerows(summary_rows)
    print(f"wrote {detailed_path} ({len(detailed)} calls)")
    print(f"wrote {summary_path} ({len(summary_rows)} groups)")


if __name__ == "__main__":
    main()
