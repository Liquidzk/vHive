#!/usr/bin/env python3
"""Run an exploratory Welch comparison over per-restore codec timings."""

from __future__ import annotations

import argparse
import csv
import statistics
from collections import defaultdict
from pathlib import Path

from scipy import stats


Key = tuple[str, int, int]


def read_samples(path: Path) -> dict[Key, list[float]]:
    groups: dict[Key, list[float]] = defaultdict(list)
    with path.open(newline="") as handle:
        for row in csv.DictReader(handle):
            if row["codec"] != "iaa_deflate":
                continue
            key = (row["workload"], int(row["partitions"]), int(row["jobs"]))
            groups[key].append(float(row["codec_total_us"]) / 1000)
    if len(groups) != 20 or any(len(samples) != 20 for samples in groups.values()):
        counts = {key: len(samples) for key, samples in groups.items()}
        raise SystemExit(f"{path}: expected 20 variants with 20 samples each; got {counts}")
    return groups


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("old_detailed", type=Path)
    parser.add_argument("new_detailed", type=Path)
    parser.add_argument("output", type=Path)
    args = parser.parse_args()

    old = read_samples(args.old_detailed)
    new = read_samples(args.new_detailed)
    if old.keys() != new.keys():
        raise SystemExit("old/new sample keys do not match")

    records: list[dict[str, str | int]] = []
    for workload, partitions, jobs in sorted(old):
        before = old[workload, partitions, jobs]
        after = new[workload, partitions, jobs]
        test = stats.ttest_ind(after, before, equal_var=False)
        interval = test.confidence_interval(confidence_level=0.95)
        old_mean = statistics.fmean(before)
        new_mean = statistics.fmean(after)
        records.append(
            {
                "workload": workload,
                "partitions": partitions,
                "jobs": jobs,
                "samples_per_topology": len(before),
                "wq2_codec_total_ms_mean": f"{old_mean:.3f}",
                "wq8_codec_total_ms_mean": f"{new_mean:.3f}",
                "wq8_minus_wq2_ms": f"{new_mean - old_mean:.3f}",
                "wq8_vs_wq2_pct": f"{(new_mean / old_mean - 1) * 100:.3f}",
                "welch_difference_ci95_low_ms": f"{interval.low:.3f}",
                "welch_difference_ci95_high_ms": f"{interval.high:.3f}",
                "welch_p_value_uncorrected": f"{test.pvalue:.6f}",
            }
        )

    args.output.parent.mkdir(parents=True, exist_ok=True)
    with args.output.open("w", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=list(records[0]))
        writer.writeheader()
        writer.writerows(records)
    print(f"wrote {len(records)} exploratory sample comparisons to {args.output}")


if __name__ == "__main__":
    main()
