#!/usr/bin/env python3
"""Build a compact, path-matched view of the fixed-CPU private-WS matrix."""

from __future__ import annotations

import argparse
import csv
from collections import defaultdict
from pathlib import Path


CODEC_LABELS = {
    "raw": "Raw",
    "gzip": "Gzip",
    "sw_deflate": "SW-QPL DEFLATE",
    "iaa_deflate": "IAA-DEFLATE",
    "zstd_3": "Zstd-3",
}


def number(row: dict[str, str], field: str) -> float | None:
    value = row[field]
    return float(value) if value else None


def fmt(value: float | None, digits: int = 3) -> str:
    return "" if value is None else f"{value:.{digits}f}"


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("result_dir", type=Path)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()

    result_dir = args.result_dir.resolve()
    output = args.output or result_dir / "comparison.csv"

    with (result_dir / "summary.csv").open(newline="") as handle:
        rows = list(csv.DictReader(handle))

    grouped: dict[tuple[str, str, int, int], list[dict[str, str]]] = defaultdict(list)
    for row in rows:
        key = (
            row["workload"],
            row["codec"],
            int(row["partitions"]),
            int(row["jobs"]),
        )
        grouped[key].append(row)

    records: list[dict[str, str | int]] = []
    for (workload, codec, partitions, jobs), pair in sorted(grouped.items()):
        by_path = {row["path"]: row for row in pair}
        if set(by_path) != {"local", "remote"}:
            raise SystemExit(f"unmatched paths for {(workload, codec, partitions, jobs)}")

        total_runs = sum(int(row["runs"]) for row in pair)

        def weighted(field: str) -> float | None:
            present = [(number(row, field), int(row["runs"])) for row in pair]
            values = [(value, runs) for value, runs in present if value is not None]
            if not values:
                return None
            return sum(value * runs for value, runs in values) / sum(runs for _, runs in values)

        remote = by_path["remote"]
        local = by_path["local"]
        records.append(
            {
                "workload": workload,
                "codec": CODEC_LABELS[codec],
                "partitions": partitions,
                "jobs": jobs,
                "runs": total_runs,
                "failures": sum(int(row["failures"]) for row in pair),
                "fallbacks": sum(int(row["fallbacks"]) for row in pair),
                "input_bytes": remote["input_bytes"],
                "payload_bytes": remote["payload_bytes"],
                "compression_ratio": remote["compression_ratio"],
                "decode_ms_mean": fmt(
                    None
                    if weighted("decompress_us_mean") is None
                    else weighted("decompress_us_mean") / 1000
                ),
                "codec_total_ms_mean": fmt(
                    None
                    if weighted("codec_total_us_mean") is None
                    else weighted("codec_total_us_mean") / 1000
                ),
                "remote_e2e_ms_p50": remote["e2e_ms_p50"],
                "remote_e2e_ms_p95": remote["e2e_ms_p95"],
                "local_e2e_ms_p50": local["e2e_ms_p50"],
                "local_e2e_ms_p95": local["e2e_ms_p95"],
            }
        )

    fieldnames = list(records[0])
    output.parent.mkdir(parents=True, exist_ok=True)
    with output.open("w", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=fieldnames)
        writer.writeheader()
        writer.writerows(records)

    expected = 5 * (5 + 3)
    if len(records) != expected:
        raise SystemExit(f"expected {expected} matched variants, found {len(records)}")
    print(f"wrote {len(records)} matched variants to {output}")


if __name__ == "__main__":
    main()
