#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  sudo ./configure_cpu_baseline.sh status
  sudo ./configure_cpu_baseline.sh apply-4ghz STATE_DIR
  sudo ./configure_cpu_baseline.sh restore STATE_DIR

apply-4ghz saves the current CPU configuration in STATE_DIR before it:
  * disables SMT;
  * leaves Intel Turbo enabled (4.0 GHz is above this host's 3.0 GHz base);
  * disables intel_pstate HWP dynamic boost;
  * selects the performance governor/EPP; and
  * requests min=max=4,000,000 kHz on every online cpufreq policy.

The requested P-state must still be verified under load with turbostat.
EOF
}

die() {
  echo "ERROR: $*" >&2
  exit 1
}

require_root() {
  [[ ${EUID} -eq 0 ]] || die "run this command with sudo"
}

read_optional() {
  local path=$1
  if [[ -r ${path} ]]; then
    tr -d '\n' < "${path}"
  else
    printf '%s' 'NA'
  fi
}

policy_dirs() {
  local policy hardware_max
  while IFS= read -r policy; do
    # Linux can retain policy directories for SMT siblings after those CPUs
    # are offlined, while removing their writable cpufreq attributes.
    [[ -r ${policy}/scaling_max_freq && -r ${policy}/cpuinfo_max_freq ]] || continue
    hardware_max=$(cat "${policy}/cpuinfo_max_freq" 2>/dev/null || true)
    [[ ${hardware_max} =~ ^[0-9]+$ ]] || continue
    printf '%s\n' "${policy}"
  done < <(find /sys/devices/system/cpu/cpufreq -maxdepth 1 -type d -name 'policy*' -print | sort -V)
}

show_status() {
  echo "timestamp=$(date -Is)"
  echo "hostname=$(hostname)"
  echo "online_cpus=$(read_optional /sys/devices/system/cpu/online)"
  echo "smt_control=$(read_optional /sys/devices/system/cpu/smt/control)"
  echo "smt_active=$(read_optional /sys/devices/system/cpu/smt/active)"
  echo "intel_pstate_no_turbo=$(read_optional /sys/devices/system/cpu/intel_pstate/no_turbo)"
  echo "intel_pstate_hwp_dynamic_boost=$(read_optional /sys/devices/system/cpu/intel_pstate/hwp_dynamic_boost)"
  echo -e 'policy\tgovernor\tepp\tmin_khz\tmax_khz\tbase_khz\tcpuinfo_min_khz\tcpuinfo_max_khz'
  local policy
  while IFS= read -r policy; do
    echo -e "$(basename "${policy}")\t$(read_optional "${policy}/scaling_governor")\t$(read_optional "${policy}/energy_performance_preference")\t$(read_optional "${policy}/scaling_min_freq")\t$(read_optional "${policy}/scaling_max_freq")\t$(read_optional "${policy}/base_frequency")\t$(read_optional "${policy}/cpuinfo_min_freq")\t$(read_optional "${policy}/cpuinfo_max_freq")"
  done < <(policy_dirs)
}

capture_state() {
  local state_dir=$1
  [[ ! -e ${state_dir}/capture.complete ]] || die "state directory was already captured: ${state_dir}"
  mkdir -p "${state_dir}"

  show_status > "${state_dir}/cpu-state.before.txt"
  lscpu --extended > "${state_dir}/lscpu-extended.before.txt"
  lscpu > "${state_dir}/lscpu.before.txt"

  {
    echo -e 'policy\tgovernor\tepp\tmin_khz\tmax_khz'
    local policy
    while IFS= read -r policy; do
      echo -e "$(basename "${policy}")\t$(read_optional "${policy}/scaling_governor")\t$(read_optional "${policy}/energy_performance_preference")\t$(read_optional "${policy}/scaling_min_freq")\t$(read_optional "${policy}/scaling_max_freq")"
    done < <(policy_dirs)
  } > "${state_dir}/policies.before.tsv"

  {
    echo "smt_control=$(read_optional /sys/devices/system/cpu/smt/control)"
    echo "no_turbo=$(read_optional /sys/devices/system/cpu/intel_pstate/no_turbo)"
    echo "hwp_dynamic_boost=$(read_optional /sys/devices/system/cpu/intel_pstate/hwp_dynamic_boost)"
  } > "${state_dir}/globals.before.env"
  date -Is > "${state_dir}/capture.complete"
}

set_policy_4ghz() {
  local policy=$1
  local target_khz=4000000
  local hardware_max
  hardware_max=$(<"${policy}/cpuinfo_max_freq")
  (( target_khz <= hardware_max )) || die "${policy}: 4.0 GHz exceeds hardware maximum ${hardware_max} kHz"

  echo performance > "${policy}/scaling_governor"
  if [[ -w ${policy}/energy_performance_preference ]]; then
    echo performance > "${policy}/energy_performance_preference"
  fi
  echo "${target_khz}" > "${policy}/scaling_max_freq"
  echo "${target_khz}" > "${policy}/scaling_min_freq"
}

apply_4ghz() {
  local state_dir=$1
  if [[ -f ${state_dir}/capture.complete ]]; then
    [[ ! -f ${state_dir}/apply.complete ]] || die "4.0 GHz baseline was already applied: ${state_dir}"
    echo "Resuming incomplete apply using captured state: ${state_dir}" >&2
  else
    capture_state "${state_dir}"
  fi

  [[ -w /sys/devices/system/cpu/smt/control ]] || die "SMT control is unavailable"
  echo off > /sys/devices/system/cpu/smt/control

  # 4.0 GHz is above the 3.0 GHz base frequency on the target Xeon, so Turbo
  # must remain enabled even though opportunistic HWP dynamic boost is disabled.
  echo 0 > /sys/devices/system/cpu/intel_pstate/no_turbo
  if [[ -w /sys/devices/system/cpu/intel_pstate/hwp_dynamic_boost ]]; then
    echo 0 > /sys/devices/system/cpu/intel_pstate/hwp_dynamic_boost
  fi

  local policy
  while IFS= read -r policy; do
    set_policy_4ghz "${policy}"
  done < <(policy_dirs)

  show_status > "${state_dir}/cpu-state.after.txt"
  date -Is > "${state_dir}/apply.complete"
  show_status
}

restore_state() {
  local state_dir=$1
  [[ -f ${state_dir}/capture.complete ]] || die "no captured state in ${state_dir}"
  [[ -f ${state_dir}/globals.before.env ]] || die "missing globals.before.env"
  [[ -f ${state_dir}/policies.before.tsv ]] || die "missing policies.before.tsv"

  local saved_smt saved_no_turbo saved_hwp
  saved_smt=$(awk -F= '$1 == "smt_control" {print $2}' "${state_dir}/globals.before.env")
  saved_no_turbo=$(awk -F= '$1 == "no_turbo" {print $2}' "${state_dir}/globals.before.env")
  saved_hwp=$(awk -F= '$1 == "hwp_dynamic_boost" {print $2}' "${state_dir}/globals.before.env")

  if [[ ${saved_smt} == on || ${saved_smt} == off ]]; then
    echo "${saved_smt}" > /sys/devices/system/cpu/smt/control
  fi
  [[ ${saved_no_turbo} != NA ]] && echo "${saved_no_turbo}" > /sys/devices/system/cpu/intel_pstate/no_turbo
  if [[ ${saved_hwp} != NA && -w /sys/devices/system/cpu/intel_pstate/hwp_dynamic_boost ]]; then
    echo "${saved_hwp}" > /sys/devices/system/cpu/intel_pstate/hwp_dynamic_boost
  fi

  local name governor epp min_khz max_khz policy hardware_min hardware_max
  while IFS=$'\t' read -r name governor epp min_khz max_khz; do
    [[ ${name} != policy ]] || continue
    policy="/sys/devices/system/cpu/cpufreq/${name}"
    [[ -d ${policy} ]] || die "saved policy did not return after SMT restore: ${name}"
    hardware_min=$(<"${policy}/cpuinfo_min_freq")
    hardware_max=$(<"${policy}/cpuinfo_max_freq")
    # Open the full valid range first so either the old min or max can be
    # restored without violating the kernel's min <= max invariant.
    echo "${hardware_max}" > "${policy}/scaling_max_freq"
    echo "${hardware_min}" > "${policy}/scaling_min_freq"
    [[ ${governor} != NA ]] && echo "${governor}" > "${policy}/scaling_governor"
    if [[ ${epp} != NA && -w ${policy}/energy_performance_preference ]]; then
      echo "${epp}" > "${policy}/energy_performance_preference"
    fi
    echo "${max_khz}" > "${policy}/scaling_max_freq"
    echo "${min_khz}" > "${policy}/scaling_min_freq"
  done < "${state_dir}/policies.before.tsv"

  show_status > "${state_dir}/cpu-state.restored.txt"
  date -Is > "${state_dir}/restore.complete"
  show_status
}

main() {
  local action=${1:-}
  case "${action}" in
    status)
      show_status
      ;;
    apply-4ghz)
      require_root
      [[ $# -eq 2 ]] || die "apply-4ghz requires STATE_DIR"
      apply_4ghz "$2"
      ;;
    restore)
      require_root
      [[ $# -eq 2 ]] || die "restore requires STATE_DIR"
      restore_state "$2"
      ;;
    *)
      usage
      exit 2
      ;;
  esac
}

main "$@"
