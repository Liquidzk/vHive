#!/usr/bin/env bash
set -euo pipefail

STAGE=${1:-/home/ubuntu/handoff-20260822}
PACKAGES="$STAGE/packages"
mkdir -p "$PACKAGES"

pack_user_tree() {
  local base=$1
  local name=$2
  local output=$3
  local partial="$output.partial"
  if [[ -f "$output" ]]; then
    zstd -q -t "$output"
    return
  fi
  rm -f "$partial"
  tar --numeric-owner --acls --xattrs -C "$base" -cf - "$name" \
    | zstd -T0 -3 -o "$partial"
  mv "$partial" "$output"
  zstd -q -t "$output"
}

pack_root_tree() {
  local base=$1
  local name=$2
  local output=$3
  local partial="$output.partial"
  if [[ -f "$output" ]]; then
    zstd -q -t "$output"
    return
  fi
  rm -f "$partial"
  sudo tar --numeric-owner --acls --xattrs -C "$base" -cf - "$name" \
    | zstd -T0 -3 -o "$partial"
  mv "$partial" "$output"
  zstd -q -t "$output"
}

pack_user_tree "$STAGE" minio-snapshots \
  "$PACKAGES/minio-snapshots.tar.zst"
pack_root_tree /home/ubuntu snapshare-fourfn-eval \
  "$PACKAGES/snapshare-fourfn-eval.tar.zst"
pack_user_tree /home/ubuntu vhive-snapshare \
  "$PACKAGES/vhive-snapshare-working-tree.tar.zst"
pack_user_tree /home/ubuntu snapshare-sabre \
  "$PACKAGES/snapshare-sabre-component.tar.zst"
pack_user_tree /home/ubuntu images \
  "$PACKAGES/function-provenance-images.tar.zst"
pack_root_tree /opt sabre \
  "$PACKAGES/original-sabre-opt.tar.zst"

(
  cd "$PACKAGES"
  find . -maxdepth 1 -type f -name '*.tar.zst' -printf '%f\0' \
    | sort -z | xargs -0 sha256sum > SHA256SUMS
)
date -Is > "$STAGE/package-export.complete"
