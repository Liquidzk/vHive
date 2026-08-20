#!/usr/bin/env bash
# Sweep independently compressed Private-WS partitions and IAA in-flight job
# limits on the frozen AES-Go snapshot. PLANB_SETTINGS accepts independent
# partition:job pairs; PARTITION_COUNTS keeps the original matched shorthand.
set -euo pipefail

root=${ROOT:-/home/ubuntu/snapshare-fourfn-eval}
stamp=${STAMP:-20260820-aes-go-iaa-partition-sweep-r1}
repetitions=${REPETITIONS:-10}
runner=$root/run_aes_go_planb_ab.sh
parser=$root/summarize_aes_partition_sweep.py
encoder=${ENCODER:-$root/planb_encode_partitions}
relay_binary=${RELAY_BINARY:-/home/ubuntu/vhive-snapshare/cmd/relay/relay-planb-iaa-v10-partitions}
restore_relay_binary=${RESTORE_RELAY_BINARY:-./relay-planb-iaa-v9}
result_root=$root/results/$stamp

declare -a sweep_settings=()
if [[ -n ${PLANB_SETTINGS:-} ]]; then
  read -r -a sweep_settings <<<"$PLANB_SETTINGS"
else
  read -r -a partition_counts <<<"${PARTITION_COUNTS:-1 2 4 8}"
  for count in "${partition_counts[@]}"; do
    sweep_settings+=("$count:$count")
  done
fi

[[ $repetitions =~ ^[1-9][0-9]*$ ]] || { echo 'REPETITIONS must be positive' >&2; exit 2; }
[[ -x $runner && -x $encoder && -x $relay_binary && -f $parser ]] || {
  echo 'missing runner, parser, encoder, or relay binary' >&2
  exit 2
}
declare -A seen_settings=()
for setting in "${sweep_settings[@]}"; do
  IFS=: read -r partition_count job_count extra <<<"$setting"
  [[ -z ${extra:-} && $partition_count =~ ^[1-9][0-9]*$ && $partition_count -le 4294967295 ]] || {
    echo "invalid partition count in setting: $setting" >&2
    exit 2
  }
  [[ $job_count =~ ^[1-9][0-9]*$ && $job_count -le 255 ]] || {
    echo "invalid job count in setting: $setting" >&2
    exit 2
  }
  [[ -z ${seen_settings[$setting]:-} ]] || {
    echo "duplicate partition/job setting: $setting" >&2
    exit 2
  }
  seen_settings[$setting]=1
done
[[ ! -e $result_root ]] || { echo "refusing to reuse result root: $result_root" >&2; exit 1; }

active_policy_dirs() {
  local policy hardware_max
  for policy in /sys/devices/system/cpu/cpufreq/policy*; do
    [[ -d $policy ]] || continue
    hardware_max=$(cat "$policy/cpuinfo_max_freq" 2>/dev/null || true)
    [[ $hardware_max =~ ^[0-9]+$ ]] || continue
    printf '%s\n' "$policy"
  done
}

policy_mismatch_count() {
  local leaf=$1 expected=$2 policy value mismatches=0
  for policy in "${cpu_policies[@]}"; do
    value=$(cat "$policy/$leaf" 2>/dev/null || true)
    [[ $value == "$expected" ]] || ((mismatches += 1))
  done
  printf '%s\n' "$mismatches"
}

mapfile -t cpu_policies < <(active_policy_dirs)
for policy in "${cpu_policies[@]}"; do
  printf '%s\n' performance | sudo tee "$policy/scaling_governor" >/dev/null
  printf '%s\n' performance | sudo tee "$policy/energy_performance_preference" >/dev/null
done

governor_bad=$(policy_mismatch_count scaling_governor performance)
epp_bad=$(policy_mismatch_count energy_performance_preference performance)
min_freq_bad=$(policy_mismatch_count scaling_min_freq 4000000)
max_freq_bad=$(policy_mismatch_count scaling_max_freq 4000000)
active_policy_count=${#cpu_policies[@]}
smt_active=$(cat /sys/devices/system/cpu/smt/active 2>/dev/null || echo unavailable)
no_turbo=$(cat /sys/devices/system/cpu/intel_pstate/no_turbo 2>/dev/null || echo unavailable)
hwp_dynamic_boost=$(cat /sys/devices/system/cpu/intel_pstate/hwp_dynamic_boost 2>/dev/null || echo unavailable)
queue_count=$(find /dev/iax -maxdepth 1 -type c -name 'wq*' 2>/dev/null | wc -l)
[[ $active_policy_count -eq 24 && $governor_bad -eq 0 && $epp_bad -eq 0 \
  && $min_freq_bad -eq 0 && $max_freq_bad -eq 0 && $smt_active == 0 \
  && $no_turbo == 0 && $hwp_dynamic_boost == 0 && $queue_count -eq 8 ]] || {
  echo "preflight failed: active_policies=$active_policy_count governor_bad=$governor_bad epp_bad=$epp_bad min_freq_bad=$min_freq_bad max_freq_bad=$max_freq_bad smt_active=$smt_active no_turbo=$no_turbo hwp_dynamic_boost=$hwp_dynamic_boost iaa_work_queues=$queue_count" >&2
  exit 2
}
if [[ ${PREFLIGHT_ONLY:-0} == 1 ]]; then
  echo "preflight passed: active_policies=$active_policy_count frequency_khz=4000000 smt_active=$smt_active iaa_work_queues=$queue_count"
  exit 0
fi

mkdir -p "$result_root"
outer_log=$result_root/launcher.log
{
  echo "stamp=$stamp"
  echo "start=$(date -Is)"
  echo "host=$(hostname)"
  echo "repetitions=$repetitions"
  echo "planb_settings=${sweep_settings[*]}"
  echo "governor_bad=$governor_bad"
  echo "epp_bad=$epp_bad"
  echo "active_cpu_policies=$active_policy_count"
  echo "scaling_min_freq_bad=$min_freq_bad"
  echo "scaling_max_freq_bad=$max_freq_bad"
  echo "smt_active=$smt_active"
  echo "intel_pstate_no_turbo=$no_turbo"
  echo "intel_pstate_hwp_dynamic_boost=$hwp_dynamic_boost"
  echo "iaa_work_queues=$queue_count"
  uname -a
  lscpu
  sha256sum "$runner" "$parser" "$encoder" "$relay_binary"
} >"$result_root/provenance.txt"

for setting in "${sweep_settings[@]}"; do
  IFS=: read -r partition_count job_count <<<"$setting"
  child_root=$result_root/p${partition_count}j${job_count}
  child_variant=${stamp}-p${partition_count}j${job_count}
  printf '%s begin partitions=%s jobs=%s\n' "$(date -Is)" "$partition_count" "$job_count" | tee -a "$outer_log"
  STAMP=${stamp}-p${partition_count}j${job_count} \
  VARIANT_STAMP="$child_variant" \
  RESULT_ROOT="$child_root" \
  REPETITIONS="$repetitions" \
  CODEC_LIST=iaa_deflate \
  PLANB_PARTITIONS="$partition_count" \
  PLANB_JOBS="$job_count" \
  ENCODER="$encoder" \
  RELAY_BINARY="$relay_binary" \
  RESTORE_RELAY_BINARY="$restore_relay_binary" \
    "$runner" 2>&1 | tee -a "$outer_log"
  printf '%s complete partitions=%s jobs=%s\n' "$(date -Is)" "$partition_count" "$job_count" | tee -a "$outer_log"
done

python3 "$parser" "$result_root"
{
  echo "end=$(date -Is)"
  echo "governor_bad_after=$(policy_mismatch_count scaling_governor performance)"
  echo "epp_bad_after=$(policy_mismatch_count energy_performance_preference performance)"
  echo "scaling_min_freq_bad_after=$(policy_mismatch_count scaling_min_freq 4000000)"
  echo "scaling_max_freq_bad_after=$(policy_mismatch_count scaling_max_freq 4000000)"
} >>"$result_root/provenance.txt"
echo "AES_PARTITION_SWEEP_COMPLETE result_root=$result_root"
