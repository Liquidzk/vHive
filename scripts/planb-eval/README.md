# SnapShare Plan B evaluation runners

This directory versions the scripts used for the fixed-CPU Private-WS codec
matrix. It intentionally contains no result logs, snapshots, function-input
blobs, or prebuilt binaries. The scripts retain the original lab defaults for
paths, the loopback MinIO test account, and the private MongoDB endpoint; adapt
those values when restoring onto a different topology.

The complete matrix covers AES-Go and the four MongoDB-backed workloads. It
runs raw, default-level Go Gzip, SW-QPL DEFLATE, IAA-DEFLATE, and Zstd-3 at one
partition/job, plus IAA-DEFLATE at `2/2`, `4/4`, and `8/8`. Each variant uses
matched cache-empty remote and populated-cache local restores.

## Runtime prerequisites

The runners target the single-node layout used by the evaluation and expect:

- an IAA-enabled host with QPL, Sabre's `MemoryRestorator`, KVM, containerd,
  Firecracker, the demux snapshotter, and the HTTP resolver;
- eight enabled IAA work queues;
- MinIO container `snapshare-minio` at `127.0.0.1:9000` and MongoDB at
  `10.0.0.11:27017`;
- vSwarm's `grpcurl` and protobuf tree under `/home/ubuntu/vswarm`;
- the frozen source revisions and four-function learning manifest restored
  from the separately retained recovery package;
- `/home/ubuntu/snapshots` as the local snapshot cache.

These data-plane prerequisites are deliberately not committed to Git. On the
original evaluation host they are already present. For a replacement host,
restore them from the local `IAA单节点-保存/恢复` package before running the
matrix.

## Build the two binaries

Build with the Sabre/QPL cgo environment used by the target host:

```bash
cd /home/ubuntu/vhive-snapshare
source /home/ubuntu/snapshare-sabre/go/sabre/build_env.sh

sudo -E env \
  CGO_CFLAGS="$CGO_CFLAGS" \
  CGO_CXXFLAGS="$CGO_CXXFLAGS" \
  CGO_LDFLAGS="$CGO_LDFLAGS" \
  /usr/local/go/bin/go build -tags sabre \
  -o /home/ubuntu/snapshare-fourfn-eval/planb_encode_partitions \
  ./cmd/planb_encode

sudo -E env \
  CGO_CFLAGS="$CGO_CFLAGS" \
  CGO_CXXFLAGS="$CGO_CXXFLAGS" \
  CGO_LDFLAGS="$CGO_LDFLAGS" \
  /usr/local/go/bin/go build -tags sabre \
  -o ./cmd/relay/relay-planb-iaa-v10-partitions \
  ./cmd/relay
```

## Install the runners

The scripts retain the original absolute runtime root so existing provenance
and recovery tooling continue to match:

```bash
cd /home/ubuntu/vhive-snapshare
install -d -m 0755 /home/ubuntu/snapshare-fourfn-eval
install -m 0755 scripts/planb-eval/*.sh \
  /home/ubuntu/snapshare-fourfn-eval/
install -m 0755 scripts/planb-eval/*.py \
  /home/ubuntu/snapshare-fourfn-eval/
```

## Validate and run

Apply the CPU baseline once, saving the previous settings in a fresh state
directory:

```bash
sudo /home/ubuntu/snapshare-fourfn-eval/configure_cpu_baseline.sh \
  apply-4ghz /home/ubuntu/snapshare-fourfn-eval/cpu-state-$(date +%Y%m%d-%H%M%S)
```

Always use a fresh stamp. First execute the read-only preflight:

```bash
export STAMP=$(date +%Y%m%d-%H%M%S)-fixed4g-private-matrix
PREFLIGHT_ONLY=1 \
  /home/ubuntu/snapshare-fourfn-eval/run_fixed4g_private_matrix.sh
```

Then launch the formal run in tmux:

```bash
tmux new-session -d -s "$STAMP" \
  "STAMP=$STAMP /home/ubuntu/snapshare-fourfn-eval/run_fixed4g_private_matrix.sh \
   2>&1 | tee /home/ubuntu/snapshare-fourfn-eval/results/$STAMP-tmux.log"
```

Do not reuse a completed stamp or overwrite an existing MinIO revision. The
runner checks 4-GHz/SMT-off state and IAA queues, preserves the pre-run local
snapshot cache, waits for Firecracker and vSwarm relay ports to become idle,
and restores the baseline relay on exit. A valid default run ends with 800
correct calls, 40 variants, 80 path-specific groups, zero failures, and zero
fallbacks.
