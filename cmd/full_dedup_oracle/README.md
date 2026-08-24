# Full Dedup oracle

This command computes the unsafe Full Dedup upper bound from existing
SplitSnap `partial` working-set artifacts. It is read-only: it does not contact
MinIO, mutate snapshots, or change the restore path.

The three logical policies use the same page inventory:

- `current`: base/rootfs/image hashes are global; private hashes are scoped to
  one revision;
- `no-image-sharing`: only base/rootfs hashes are global;
- `full-dedup`: all raw page-content hashes are global, regardless of
  provenance or revision.

Private recipe hashes are revision-salted, so the command recomputes MD5 from
the private content file and verifies the salt against the recipe. Shared
index hashes are also checked against their content files. MD5 is used only to
match the existing snapshot content-identity format and paper methodology; it
is not treated as a security primitive.

## Input

```json
{
  "page_size": 4096,
  "cache_capacity_pages": 0,
  "snapshots": [
    {
      "id": "snapshot-revision-a",
      "workload": "image-rotate-go",
      "revision_dir": "/path/to/preload/snapshot-revision-a",
      "shared_root": "/path/to/preload/ws_shared",
      "image": "image-rotate-go"
    },
    {
      "id": "snapshot-revision-b",
      "workload": "image-rotate-go",
      "revision_dir": "/path/to/preload/snapshot-revision-b",
      "shared_root": "/path/to/preload/ws_shared",
      "image": "image-rotate-go"
    }
  ]
}
```

Relative paths are resolved relative to the configuration file. A zero cache
capacity means an unbounded logical page cache; a positive value enables a
deterministic LRU simulation. Snapshot order is restore order.

## Run

```bash
go run ./cmd/full_dedup_oracle \
  -config /path/to/corpus.json \
  -output /path/to/results
```

Outputs:

- `pages.csv`: occurrence-level raw page hash and provenance inventory;
- `validation.csv`: per-snapshot format and classification checks;
- `inputs.csv`: source paths, sizes, and SHA-256 checksums;
- `summary.csv`: logical footprint and cache-simulation totals;
- `trace.csv`: one row per policy and restore;
- `manifest.json`: resolved configuration and method boundary.

`logical_unique_bytes` and `fetch_bytes` are 4-KiB page-content oracle
metrics. They intentionally do not claim to be physical coalesced-object bytes
or measured network traffic. A live baseline requires a separate gate because
changing object granularity would confound the policy comparison.
