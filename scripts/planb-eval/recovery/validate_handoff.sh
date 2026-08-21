#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
cd "$root"

sha256sum --quiet -c SHA256SUMS
(cd packages && sha256sum --quiet -c SHA256SUMS)
(cd images && sha256sum --quiet -c SHA256SUMS)
(cd mongodb && sha256sum --quiet -c SHA256SUMS)

for archive in packages/*.tar.zst; do
  zstd -q -t "$archive"
  tar --zstd -tf "$archive" >/dev/null
done
for archive in images/*.tar.zst; do
  zstd -q -t "$archive"
  zstd -dc "$archive" | tar -tf - >/dev/null
done
tar -tf images/firecracker-eval-images.oci.tar >/dev/null
gzip -t mongodb/snapshare-mongodb.archive.gz

bundle_repo=$(mktemp -d)
trap 'rm -rf -- "$bundle_repo"' EXIT
git -C "$bundle_repo" init -q
git -C "$bundle_repo" bundle verify \
  "$root/source/Liquidzk-vHive-snapshare-sabre-buffered.bundle"
git -C "$bundle_repo" bundle verify \
  "$root/source/Liquidzk-vSwarm-snapshare-eval-images.bundle"

bash -n restore_node.sh smoke_aes_restore.sh scripts/*.sh validate_handoff.sh

[[ $(find provenance -maxdepth 1 -type f | wc -l) -ge 32 ]]
[[ $(wc -l < provenance/iaa-device-nodes.txt) -eq 32 ]]
[[ $(grep -c '^policy' provenance/cpu-state.txt) -eq 24 ]]
grep -q '^iaa_settings=1:1 2:2 4:4 8:8 16:16$' \
  eval/snapshare-eval-portable/results/sabre/20260821-fixed4g-iaa-wq8-diagonal16-r1/matrix.env
grep -q '^included_entries=9481$' \
  provenance/remote-results-essential-inventory.txt
grep -q '^excluded_rebuildable_files=1521$' \
  provenance/remote-results-essential-inventory.txt
grep -q '^included_entries=566$' \
  provenance/remote-sabre-results-essential-inventory.txt
grep -q '^excluded_rebuildable_files=62$' \
  provenance/remote-sabre-results-essential-inventory.txt

echo HANDOFF_VALIDATION=PASS
