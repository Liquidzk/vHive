#!/usr/bin/env bash
# Run selected IAA partition/job points from the five-workload Private-WS
# matrix with the accelerator configured to eight WQs per device.
# Keep the validated 4.0 GHz, SMT-off CPU baseline. Run this script in tmux.
set -euo pipefail

root=${ROOT:-/home/ubuntu/snapshare-fourfn-eval}
stamp=${STAMP:-20260821-fixed4g-iaa-wq8-matrix-r1}
repetitions=${REPETITIONS:-10}
read -r -a iaa_settings <<<"${IAA_SETTINGS:-1:1 2:2 4:4 8:8}"
settings_count=${#iaa_settings[@]}
result_root=${RESULT_ROOT:-$root/results/$stamp}
aes_runner=$root/run_aes_go_planb_ab.sh
fourfn_runner=$root/run_fourfn_planb_ab.sh
aes_preflight=$root/run_aes_go_partition_sweep.sh
fourfn_preflight=$root/run_video_processing_partition_sweep.sh
parser=$root/parse_fourfn_planb_ab.py
summarizer=$root/summarize_iaa_wq8_matrix.py
encoder=${ENCODER:-$root/planb_encode_partitions}
relay_binary=${RELAY_BINARY:-/home/ubuntu/vhive-snapshare/cmd/relay/relay-planb-iaa-v10-partitions}
restore_relay_binary=${RESTORE_RELAY_BINARY:-./relay-planb-iaa-v9}
cpu_tool=$root/configure_cpu_baseline.sh
learning_root=${LEARNING_ROOT:-$root/results/20260819-fourfn-planb-learning-r2}
outer_log=$result_root/launcher.log

[[ $repetitions =~ ^[1-9][0-9]*$ ]] || { echo 'REPETITIONS must be positive' >&2; exit 2; }
[[ $settings_count -gt 0 ]] || { echo 'IAA_SETTINGS must not be empty' >&2; exit 2; }
declare -A seen_settings=()
for setting in "${iaa_settings[@]}"; do
  IFS=: read -r partitions jobs extra <<<"$setting"
  [[ -z ${extra:-} && $partitions =~ ^[1-9][0-9]*$ && $jobs =~ ^[1-9][0-9]*$ ]] || {
    echo "invalid IAA setting: $setting (expected PARTITIONS:JOBS)" >&2
    exit 2
  }
  [[ $jobs -le $partitions ]] || {
    echo "invalid IAA setting: $setting (jobs cannot exceed partitions)" >&2
    exit 2
  }
  [[ ! -v seen_settings[$setting] ]] || { echo "duplicate IAA setting: $setting" >&2; exit 2; }
  seen_settings[$setting]=1
done
for required in "$aes_runner" "$fourfn_runner" "$aes_preflight" "$fourfn_preflight" \
  "$parser" "$summarizer" "$encoder" "$relay_binary" "$cpu_tool"; do
  [[ -x $required || $required == *.py && -f $required ]] || {
    echo "missing required file: $required" >&2
    exit 2
  }
done
[[ -s $learning_root/manifest.csv ]] || { echo "missing learning manifest: $learning_root/manifest.csv" >&2; exit 2; }
[[ ! -f $result_root/complete.marker ]] || { echo "matrix is already complete: $result_root" >&2; exit 1; }

log() {
  printf '%s %s\n' "$(date -Is)" "$*" | tee -a "$outer_log"
}

capture_cpu() {
  local destination=$1
  sudo "$cpu_tool" status >"$destination"
}

preflight() {
  local suffix=$1
  local preflight_stamp="${stamp}-preflight-${suffix}-$$"
  STAMP="$preflight_stamp-aes" PREFLIGHT_ONLY=1 \
    EXPECTED_IAA_WQ_COUNT=32 \
    ENCODER="$encoder" RELAY_BINARY="$relay_binary" \
    "$aes_preflight"
  STAMP="$preflight_stamp-fourfn" PREFLIGHT_ONLY=1 \
    EXPECTED_IAA_WQ_COUNT=32 \
    ENCODER="$encoder" RELAY_BINARY="$relay_binary" \
    LEARNING_ROOT="$learning_root" "$fourfn_preflight"
}

if [[ ${PREFLIGHT_ONLY:-0} == 1 ]]; then
  preflight outer-only
  echo "fixed-CPU matrix preflight passed: stamp=$stamp repetitions=$repetitions settings=${iaa_settings[*]}"
  exit 0
fi

mkdir -p "$result_root"

finish() {
  local status=$?
  trap - EXIT
  capture_cpu "$result_root/cpu-state.exit.txt" || true
  {
    echo "end=$(date -Is)"
    echo "status=$status"
    df -h / /home
  } >>"$result_root/provenance.txt"
  exit "$status"
}
trap finish EXIT

if [[ ! -f $result_root/provenance.txt ]]; then
  {
    echo "stamp=$stamp"
    echo "start=$(date -Is)"
    echo "host=$(hostname)"
    echo "repetitions=$repetitions"
    echo "workloads=aes-go image-rotate-go image-rotate-python video-processing-python video-analytics-standalone-python"
    echo "codecs=iaa_deflate"
    echo "iaa_settings=${iaa_settings[*]}"
    echo "wq_topology=4 devices, 8 engines/device, 8 shared WQs/device, size 16"
    echo "expected_calls=$((5 * settings_count * 2 * repetitions))"
    echo "expected_variants=$((5 * settings_count))"
    uname -a
    lscpu
    df -h / /home
    sha256sum "$aes_runner" "$fourfn_runner" "$aes_preflight" \
      "$fourfn_preflight" "$parser" "$summarizer" "$encoder" \
      "$relay_binary" "$cpu_tool"
    git -C /home/ubuntu/vhive-snapshare rev-parse HEAD || true
    git -C /home/ubuntu/vhive-snapshare status --short || true
  } >"$result_root/provenance.txt"
  capture_cpu "$result_root/cpu-state.before.txt"
  sudo accel-config list -i >"$result_root/accel-config.before.json"
fi

printf 'stamp=%s\nrepetitions=%s\nexpected_calls=%s\nexpected_variants=%s\niaa_settings=%s\nvariant_suffix=%s\n' \
  "$stamp" "$repetitions" "$((5 * settings_count * 2 * repetitions))" "$((5 * settings_count))" \
  "${iaa_settings[*]}" "${VARIANT_SUFFIX:-}" \
  >"$result_root/matrix.env"

{
  echo "launch=$(date -Is)"
  echo "variant_suffix=${VARIANT_SUFFIX:-}"
  sha256sum "$aes_runner" "$fourfn_runner" "$aes_preflight" \
    "$fourfn_preflight" "$parser" "$summarizer" "$encoder" \
    "$relay_binary" "$cpu_tool"
} >>"$result_root/launch-attempts.log"

preflight initial | tee -a "$outer_log"

run_child() {
  local family=$1 partitions=$2 jobs=$3 codecs=$4 expected_rows=$5
  local child="${family}-p${partitions}j${jobs}"
  local child_root=$result_root/$child
  # A retry suffix creates immutable replacement MinIO revisions after an
  # audited partial child is moved aside. Completed children are still reused.
  local variant_stamp="${stamp}-${child}${VARIANT_SUFFIX:-}"
  local status rows failures

  if [[ -f $child_root/complete.marker ]]; then
    rows=$(awk 'END { print NR-1 }' "$child_root/calls.csv")
    failures=$(awk -F, 'NR > 1 && ($9 != 0 || $10 != 1) { n++ } END { print n+0 }' "$child_root/calls.csv")
    [[ $rows -eq $expected_rows && $failures -eq 0 ]] || {
      echo "invalid completed child=$child rows=$rows expected=$expected_rows failures=$failures" >&2
      return 1
    }
    log "skip completed child=$child rows=$rows"
    return 0
  fi
  [[ ! -e $child_root ]] || {
    echo "partial child requires audit before retry: $child_root" >&2
    return 1
  }

  preflight "$child" | tee -a "$outer_log"
  log "begin child=$child codecs=[$codecs] expected_rows=$expected_rows"
  set +e
  if [[ $family == aes ]]; then
    STAMP="${stamp}-${child}" \
    VARIANT_STAMP="$variant_stamp" \
    RESULT_ROOT="$child_root" \
    REPETITIONS="$repetitions" \
    CODEC_LIST="$codecs" \
    PLANB_PARTITIONS="$partitions" \
    PLANB_JOBS="$jobs" \
    ENCODER="$encoder" \
    RELAY_BINARY="$relay_binary" \
    RESTORE_RELAY_BINARY="$restore_relay_binary" \
      "$aes_runner" 2>&1 | tee "$result_root/$child.console.log"
  else
    STAMP="${stamp}-${child}" \
    VARIANT_STAMP="$variant_stamp" \
    RESULT_ROOT="$child_root" \
    LEARNING_ROOT="$learning_root" \
    REPETITIONS="$repetitions" \
    CODEC_LIST="$codecs" \
    PLANB_PARTITIONS="$partitions" \
    PLANB_JOBS="$jobs" \
    ENCODER="$encoder" \
    RELAY_BINARY="$relay_binary" \
    RESTORE_RELAY_BINARY="$restore_relay_binary" \
      "$fourfn_runner" 2>&1 | tee "$result_root/$child.console.log"
  fi
  status=${PIPESTATUS[0]}
  set -e
  [[ $status -eq 0 ]] || {
    log "failed child=$child status=$status"
    return "$status"
  }

  python3 "$parser" "$child_root" | tee -a "$outer_log"
  rows=$(awk 'END { print NR-1 }' "$child_root/calls.csv")
  failures=$(awk -F, 'NR > 1 && ($9 != 0 || $10 != 1) { n++ } END { print n+0 }' "$child_root/calls.csv")
  [[ $rows -eq $expected_rows && $failures -eq 0 ]] || {
    log "invalid child=$child rows=$rows expected=$expected_rows failures=$failures"
    return 1
  }
  capture_cpu "$child_root/cpu-state.after.txt"
  date -Is >"$child_root/complete.marker"
  log "complete child=$child rows=$rows failures=$failures"
}

# Only IAA-DEFLATE is affected by the WQ topology change. Re-running the other
# codecs would add cost without producing a useful matched comparison.
for setting in "${iaa_settings[@]}"; do
  IFS=: read -r partitions jobs <<<"$setting"
  run_child aes "$partitions" "$jobs" iaa_deflate $((2 * repetitions))
  run_child fourfn "$partitions" "$jobs" iaa_deflate $((4 * 2 * repetitions))
done

python3 "$summarizer" "$result_root" | tee -a "$outer_log"
capture_cpu "$result_root/cpu-state.after.txt"
date -Is >"$result_root/complete.marker"
log "FIXED4G_IAA_WQ8_MATRIX_COMPLETE result_root=$result_root"
