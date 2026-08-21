#!/usr/bin/env python3
"""Compare matched IAA-only results collected with two WQ topologies."""

from __future__ import annotations

import argparse
import csv
from pathlib import Path


KEY_FIELDS = ("workload", "partitions", "jobs")
MATCH_FIELDS = ("input_bytes", "payload_bytes", "compression_ratio")
METRICS = (
    "decode_ms_mean",
    "codec_total_ms_mean",
    "remote_e2e_ms_p50",
    "remote_e2e_ms_p95",
    "local_e2e_ms_p50",
    "local_e2e_ms_p95",
)


def read_iaa(path: Path) -> dict[tuple[str, int, int], dict[str, str]]:
    with path.open(newline="") as handle:
        rows = [row for row in csv.DictReader(handle) if row["codec"] == "IAA-DEFLATE"]
    indexed = {
        (row["workload"], int(row["partitions"]), int(row["jobs"])): row
        for row in rows
    }
    if len(rows) != 20 or len(indexed) != 20:
        raise SystemExit(f"{path}: expected 20 unique IAA variants, found {len(indexed)}")
    return indexed


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("old_csv", type=Path, help="comparison.csv for 2 WQs/device")
    parser.add_argument("new_csv", type=Path, help="comparison.csv for 8 WQs/device")
    parser.add_argument("output", type=Path)
    args = parser.parse_args()

    old = read_iaa(args.old_csv)
    new = read_iaa(args.new_csv)
    if old.keys() != new.keys():
        raise SystemExit("old/new IAA variant keys do not match")

    records: list[dict[str, str | int]] = []
    for key in sorted(old):
        before, after = old[key], new[key]
        mismatches = [field for field in MATCH_FIELDS if before[field] != after[field]]
        if mismatches:
            raise SystemExit(f"{key}: unmatched input/encoding fields: {mismatches}")
        record: dict[str, str | int] = {
            "workload": key[0],
            "partitions": key[1],
            "jobs": key[2],
            "runs_per_topology": int(after["runs"]),
            "input_bytes": after["input_bytes"],
            "payload_bytes": after["payload_bytes"],
            "compression_ratio": after["compression_ratio"],
        }
        for metric in METRICS:
            old_value, new_value = float(before[metric]), float(after[metric])
            record[f"wq2_{metric}"] = f"{old_value:.3f}"
            record[f"wq8_{metric}"] = f"{new_value:.3f}"
            record[f"wq8_vs_wq2_{metric}_pct"] = f"{(new_value / old_value - 1) * 100:.3f}"
        records.append(record)

    args.output.parent.mkdir(parents=True, exist_ok=True)
    with args.output.open("w", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=list(records[0]))
        writer.writeheader()
        writer.writerows(records)
    print(f"wrote {len(records)} matched WQ-topology comparisons to {args.output}")


if __name__ == "__main__":
    main()
