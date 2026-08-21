#!/usr/bin/env bash
set -euo pipefail

STAGE=${1:-/home/ubuntu/handoff-20260822}
OUT="$STAGE/provenance"
mkdir -p "$OUT"

date -Is > "$OUT/collected-at.txt"
hostnamectl > "$OUT/hostnamectl.txt"
uname -a > "$OUT/uname.txt"
cp /etc/os-release "$OUT/os-release"
cat /proc/cmdline > "$OUT/proc-cmdline.txt"
lscpu > "$OUT/lscpu.txt"
lspci -nn > "$OUT/lspci-nn.txt"
lspci -nn -d 8086:0cfe > "$OUT/lspci-iaa.txt"
lsblk -o NAME,KNAME,TYPE,SIZE,FSTYPE,MOUNTPOINTS,MODEL,SERIAL > "$OUT/lsblk.txt"
df -hT > "$OUT/df-hT.txt"
findmnt > "$OUT/findmnt.txt"
free -h > "$OUT/free-h.txt"

sudo accel-config list > "$OUT/accel-config-current.json"
find /dev/iax -maxdepth 1 -type c -printf '%f\n' 2>/dev/null | sort > "$OUT/iaa-device-nodes.txt"
{
  printf 'online='; cat /sys/devices/system/cpu/online
  printf 'smt_active='; cat /sys/devices/system/cpu/smt/active
  printf 'smt_control='; cat /sys/devices/system/cpu/smt/control
  printf 'intel_pstate_no_turbo='; cat /sys/devices/system/cpu/intel_pstate/no_turbo
  printf 'intel_pstate_hwp_dynamic_boost='; cat /sys/devices/system/cpu/intel_pstate/hwp_dynamic_boost
  for policy in /sys/devices/system/cpu/cpufreq/policy*; do
    min=$(cat "$policy/scaling_min_freq" 2>/dev/null) || continue
    max=$(cat "$policy/scaling_max_freq" 2>/dev/null) || continue
    governor=$(cat "$policy/scaling_governor" 2>/dev/null) || continue
    epp=$(cat "$policy/energy_performance_preference" 2>/dev/null) || continue
    printf '%s min=%s max=%s governor=%s epp=%s\n' \
      "${policy##*/}" \
      "$min" "$max" "$governor" "$epp"
  done
} > "$OUT/cpu-state.txt"

dpkg-query -W -f='${binary:Package}\t${Version}\n' | sort > "$OUT/dpkg-packages.tsv"
go version > "$OUT/go-version.txt" 2>&1 || true
gcc --version > "$OUT/gcc-version.txt" 2>&1 || true
cmake --version > "$OUT/cmake-version.txt" 2>&1 || true
docker --version > "$OUT/docker-version.txt" 2>&1 || true
sudo docker images --no-trunc --digests > "$OUT/docker-images.txt"
sudo docker ps -a --no-trunc > "$OUT/docker-containers.txt"
for container in snapshare-minio snapshare-mongodb; do
  sudo docker inspect "$container" --format \
    '{{.Name}} image={{.Config.Image}} image_id={{.Image}} command={{json .Config.Cmd}} mounts={{json .Mounts}}' \
    >> "$OUT/service-containers.txt"
done

sudo git -c safe.directory=/opt/sabre -C /opt/sabre rev-parse HEAD > "$OUT/sabre-head.txt"
sudo git -c safe.directory=/opt/sabre -C /opt/sabre status --short --branch > "$OUT/sabre-status.txt"
sudo git -c safe.directory=/opt/sabre -C /opt/sabre submodule status > "$OUT/sabre-submodules.txt"

{
  find /home/ubuntu/vhive-snapshare/bin -maxdepth 1 -type f -perm -111 -print0
  find /home/ubuntu/vhive-snapshare/cmd/relay -maxdepth 1 -type f -perm -111 -print0
  find /home/ubuntu/snapshare-fourfn-eval -maxdepth 1 -type f -perm -111 -print0
  find /var/lib/firecracker-containerd/runtime -maxdepth 2 -type f -print0
  find /etc/firecracker-containerd -maxdepth 2 -type f -print0
} | sort -z | xargs -0 sha256sum > "$OUT/runtime-binary-sha256.txt"

sudo dmsetup ls --tree > "$OUT/dmsetup-tree.txt" 2>&1 || true
sudo losetup --list > "$OUT/losetup.txt" 2>&1 || true
grep -R . /sys/kernel/mm/hugepages/hugepages-*/nr_hugepages > "$OUT/hugepages.txt" 2>&1 || true
ip -brief address > "$OUT/ip-address.txt"
ip route > "$OUT/ip-route.txt"
