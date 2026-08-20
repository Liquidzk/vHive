#!/usr/bin/env bash
# Run on the IAA node to switch the single-node SnapShare relay between
# learning and frozen codec configurations for the four MongoDB workloads.
set -euo pipefail

usage() {
  echo "usage: $0 stop | start <raw|gzip|sw_deflate|iaa_deflate|zstd_3> <vm-mib> <0|1 ws-recording> <log> [session]" >&2
  exit 2
}

action=${1:-}
relay_dir=${RELAY_DIR:-/home/ubuntu/vhive-snapshare/cmd/relay}
relay_binary=${RELAY_BINARY:-./relay-planb-iaa-v5}
planb_partitions=${PLANB_PARTITIONS:-1}
planb_jobs=${PLANB_JOBS:-1}
endpoint=127.0.0.1:18080

stop_relay() {
  local session pid
  for session in snaprelay_planb_iaa snaprelay_planb_ab_worker snaprelay_fourfn snaprelay_fourfn_learning; do
    tmux kill-session -t "$session" 2>/dev/null || true
  done
  pid=$(sudo ss -lntp 2>/dev/null | sed -n 's/.*127\.0\.0\.1:18080.*pid=\([0-9][0-9]*\).*/\1/p' | head -n 1)
  if [[ -n $pid ]]; then
    sudo kill -TERM "$pid" 2>/dev/null || true
  fi
  for _ in $(seq 1 50); do
    if ! sudo ss -lnt 2>/dev/null | grep -q '127.0.0.1:18080'; then
      return 0
    fi
    sleep 0.2
  done
  pid=$(sudo ss -lntp 2>/dev/null | sed -n 's/.*127\.0\.0\.1:18080.*pid=\([0-9][0-9]*\).*/\1/p' | head -n 1)
  if [[ -n $pid ]]; then
    sudo kill -KILL "$pid" 2>/dev/null || true
  fi
  ! sudo ss -lnt 2>/dev/null | grep -q '127.0.0.1:18080'
}

if [[ $action == stop ]]; then
  stop_relay
  exit 0
fi
[[ $action == start && $# -ge 5 && $# -le 6 ]] || usage

codec=$2
vm_mib=$3
recording=$4
relay_log=$5
session=${6:-snaprelay_fourfn}
[[ $codec == raw || $codec == gzip || $codec == sw_deflate || $codec == iaa_deflate || $codec == zstd_3 ]] || usage
[[ $vm_mib =~ ^[1-9][0-9]*$ ]] || usage
[[ $recording == 0 || $recording == 1 ]] || usage
[[ $planb_partitions =~ ^[1-9][0-9]*$ ]] || { echo 'PLANB_PARTITIONS must be positive' >&2; exit 2; }
[[ $planb_jobs =~ ^[1-9][0-9]*$ && $planb_jobs -le 255 ]] || { echo 'PLANB_JOBS must be between 1 and 255' >&2; exit 2; }

common=(
  -ss devmapper -snapshots remote -dbg -netPoolSize 2
  -vmMemSizeMib "$vm_mib" -hostIface bond0.3
  -vethPrefix 172.29 -clonePrefix 172.30 -endpoint "$endpoint"
  -chunking -chunkSize 4096 -upf -ws -wsCoalescing -lazy
  -security partial -minioCredentials '127.0.0.1:9000;minio;minio123'
)
extra=()
if [[ $codec != raw ]]; then
  relay_probe=$relay_binary
  if [[ $relay_probe != /* ]]; then
    relay_probe=$relay_dir/${relay_probe#./}
  fi
  extra=(-planBPrivateWS -planBCodec "$codec")
  if "$relay_probe" -h 2>&1 | grep -- '-planBPartitions' >/dev/null; then
    extra+=(-planBPartitions "$planb_partitions")
  elif [[ $planb_partitions != 1 ]]; then
    echo "relay binary does not support -planBPartitions: $relay_probe" >&2
    exit 2
  fi
  extra+=(-planBJobs "$planb_jobs")
fi
if [[ $recording == 1 ]]; then
  extra+=(-wsRecording)
fi

mkdir -p "$(dirname "$relay_log")"
stop_relay
command=(sudo -E "$relay_binary" "${common[@]}" "${extra[@]}")
printf -v command_q ' %q' "${command[@]}"
printf -v relay_dir_q '%q' "$relay_dir"
printf -v relay_log_q '%q' "$relay_log"
tmux new-session -d -s "$session" "cd $relay_dir_q &&$command_q 2>&1 | tee $relay_log_q"

for _ in $(seq 1 120); do
  if sudo ss -lnt 2>/dev/null | grep -q '127.0.0.1:18080'; then
    break
  fi
  sleep 0.25
done
sudo ss -lnt 2>/dev/null | grep -q '127.0.0.1:18080'

# Provenance hashes are loaded asynchronously at startup.  Do not let the
# first snapshot race their initialization.
for _ in $(seq 1 240); do
  if grep -q 'Loaded rootfs chunk hashes' "$relay_log" 2>/dev/null; then
    printf 'RELAY_READY codec=%s partitions=%s jobs=%s vm_mib=%s recording=%s session=%s log=%s\n' \
      "$codec" "$planb_partitions" "$planb_jobs" "$vm_mib" "$recording" "$session" "$relay_log"
    exit 0
  fi
  sleep 0.25
done
echo "relay did not finish provenance initialization: $relay_log" >&2
exit 1
