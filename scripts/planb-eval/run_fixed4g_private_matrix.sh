#!/usr/bin/env bash
# Run the complete five-workload Private-WS codec/IAA-partition matrix under
# the validated 4.0 GHz, SMT-off CPU baseline. Run this script in tmux.
set -euo pipefail

root=${ROOT:-/home/ubuntu/snapshare-fourfn-eval}
stamp=${STAMP:-20260820-fixed4g-private-matrix-r1}
repetitions=${REPETITIONS:-10}
result_root=${RESULT_ROOT:-$root/results/$stamp}
aes_runner=$root/run_aes_go_planb_ab.sh
fourfn_runner=$root/run_fourfn_planb_ab.sh
aes_preflight=$root/run_aes_go_partition_sweep.sh
fourfn_preflight=$root/run_video_processing_partition_sweep.sh
parser=$root/parse_fourfn_planb_ab.py
summarizer=$root/summarize_fixed_cpu_matrix.py
encoder=${ENCODER:-$root/planb_encode_partitions}
relay_binary=${RELAY_BINARY:-/home/ubuntu/vhive-snapshare/cmd/relay/relay-planb-iaa-v10-partitions}
restore_relay_binary=${RESTORE_RELAY_BINARY:-./relay-planb-iaa-v9}
cpu_tool=$root/configure_cpu_baseline.sh
learning_root=${LEARNING_ROOT:-$root/results/20260819-fourfn-planb-learning-r2}
outer_log=$result_root/launcher.log

[[ $repetitions =~ ^[1-9][0-9]*$ ]] || { echo 'REPETITIONS must be positive' >&2; exit 2; }
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
    ENCODER="$encoder" RELAY_BINARY="$relay_binary" \
    "$aes_preflight"
  STAMP="$preflight_stamp-fourfn" PREFLIGHT_ONLY=1 \
    ENCODER="$encoder" RELAY_BINARY="$relay_binary" \
    LEARNING_ROOT="$learning_root" "$fourfn_preflight"
}

if [[ ${PREFLIGHT_ONLY:-0} == 1 ]]; then
  preflight outer-only
  echo "fixed-CPU matrix preflight passed: stamp=$stamp repetitions=$repetitions"
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
    echo "p1_codecs=raw gzip sw_deflate iaa_deflate zstd_3"
    echo "additional_iaa_settings=2:2 4:4 8:8"
    echo "expected_calls=$((5 * 8 * 2 * repetitions))"
    echo "expected_variants=$((5 * 8))"
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

printf 'stamp=%s\nrepetitions=%s\nexpected_calls=%s\nexpected_variants=%s\nvariant_suffix=%s\n' \
  "$stamp" "$repetitions" "$((5 * 8 * 2 * repetitions))" "$((5 * 8))" "${VARIANT_SUFFIX:-}" \
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

# The p1/j1 children cover all historical codecs. The remaining children add
# only IAA-DEFLATE at the requested matched partition/job settings.
run_child aes 1 1 'raw gzip sw_deflate iaa_deflate zstd_3' $((5 * 2 * repetitions))
run_child fourfn 1 1 'raw gzip sw_deflate iaa_deflate zstd_3' $((4 * 5 * 2 * repetitions))
for setting in 2 4 8; do
  run_child aes "$setting" "$setting" iaa_deflate $((2 * repetitions))
  run_child fourfn "$setting" "$setting" iaa_deflate $((4 * 2 * repetitions))
done

python3 "$summarizer" "$result_root" | tee -a "$outer_log"
capture_cpu "$result_root/cpu-state.after.txt"
date -Is >"$result_root/complete.marker"
log "FIXED4G_PRIVATE_MATRIX_COMPLETE result_root=$result_root"
