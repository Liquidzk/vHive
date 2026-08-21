#!/usr/bin/env bash
set -euo pipefail

STAGE=${1:-/home/ubuntu/handoff-20260822}
MINIO_ENDPOINT=${MINIO_ENDPOINT:-http://127.0.0.1:9000}
MINIO_USER=${MINIO_USER:-minio}
: "${MINIO_PASSWORD:?set MINIO_PASSWORD}"
MC_IMAGE=${MC_IMAGE:-minio/mc:RELEASE.2025-08-13T08-35-41Z}

mkdir -p "$STAGE/minio-snapshots"
rm -f "$STAGE/minio-export.complete"

sudo docker run --rm --network host \
  --user "$(id -u):$(id -g)" \
  -e MC_CONFIG_DIR=/tmp/mc \
  -e MINIO_ENDPOINT="$MINIO_ENDPOINT" \
  -e MINIO_USER="$MINIO_USER" \
  -e MINIO_PASSWORD="$MINIO_PASSWORD" \
  -v "$STAGE/minio-snapshots:/export" \
  --entrypoint /bin/sh "$MC_IMAGE" -c '
    set -eu
    mc alias set src "$MINIO_ENDPOINT" "$MINIO_USER" "$MINIO_PASSWORD" >/dev/null
    mc mirror --overwrite src/snapshots /export
  '

date -Is > "$STAGE/minio-export.complete"
