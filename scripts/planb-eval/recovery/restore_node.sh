#!/usr/bin/env bash
set -euo pipefail

archive_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_dir=${VHIVE_DIR:-${HOME}/vhive-snapshare}
vswarm_dir=${VSWARM_DIR:-${HOME}/vswarm}
eval_dir=${EVAL_DIR:-${HOME}/snapshare-fourfn-eval}
minio_user=${MINIO_USER:-minio}
minio_password=${MINIO_PASSWORD:-minio123}
required_vhive_base=2dda717cb1bcb955dbe6a986449abf09efea8421

die() {
  echo "ERROR: $*" >&2
  exit 1
}

hardware_gate() {
  local iaa_count=0
  sudo modprobe idxd 2>/dev/null || true
  sudo modprobe kvm_intel 2>/dev/null || true
  for device_file in /sys/bus/pci/devices/*/device; do
    [[ -r $device_file ]] || continue
    [[ $(<"$device_file") == 0x0cfe ]] && iaa_count=$((iaa_count + 1))
  done
  echo "IAA_PCI_COUNT=$iaa_count"
  ((iaa_count >= 4)) || die "four Intel IAA PCI devices (0x0cfe) are required"
  [[ -c /dev/kvm ]] || die "/dev/kvm is unavailable"
}

verify_archive() {
  hardware_gate
  (
    cd "$archive_dir"
    sha256sum -c SHA256SUMS
  )
  for archive in "$archive_dir"/packages/*.tar.zst \
    "$archive_dir"/images/*.tar.zst; do
    zstd -q -t "$archive"
  done
  git bundle verify "$archive_dir/source/Liquidzk-vHive-snapshare-sabre-buffered.bundle"
  git bundle verify "$archive_dir/source/Liquidzk-vSwarm-snapshare-eval-images.bundle"
  echo ARCHIVE=PASS
}

install_artifacts() {
  hardware_gate
  export DEBIAN_FRONTEND=noninteractive
  sudo apt-get update
  sudo apt-get install -y --no-install-recommends \
    accel-config bc ca-certificates curl docker.io git iproute2 iptables jq \
    libgflags-dev liblz4-dev libzstd-dev python3 rsync \
    thin-provisioning-tools tmux zstd
  sudo systemctl enable --now docker

  zstd -dc "$archive_dir/images/docker-service-images.tar.zst" \
    | sudo docker load

  for binary in "$archive_dir"/binaries/*; do
    sudo install -m 0755 "$binary" "/usr/local/bin/$(basename "$binary")"
  done
  sudo install -d -m 0755 /etc/firecracker-containerd /etc/containerd
  sudo install -d -m 0755 /var/lib/firecracker-containerd/runtime
  sudo install -m 0644 "$archive_dir/configs/config.toml" \
    /etc/firecracker-containerd/config.toml
  sudo install -m 0644 "$archive_dir/configs/firecracker-runtime.json" \
    /etc/containerd/firecracker-runtime.json
  sudo install -m 0644 "$archive_dir/runtime/default-rootfs.img" \
    /var/lib/firecracker-containerd/runtime/default-rootfs.img
  sudo install -m 0644 "$archive_dir/runtime/hello-vmlinux.bin" \
    /var/lib/firecracker-containerd/runtime/hello-vmlinux.bin

  sudo install -d -m 0755 /usr/local/qpl /usr/local/include /usr/local/lib
  sudo cp -a "$archive_dir/build-deps/qpl/." /usr/local/qpl/
  sudo install -m 0644 "$archive_dir"/build-deps/snappy/snappy*.h /usr/local/include/
  sudo install -m 0644 "$archive_dir/build-deps/snappy/libsnappy.a" /usr/local/lib/
  sudo install -d -m 0755 /opt/sabre/firecracker/sabre /opt/sabre/firecracker/build/sabre
  sudo install -m 0644 "$archive_dir"/build-deps/sabre/*.h /opt/sabre/firecracker/sabre/
  sudo install -m 0644 "$archive_dir/build-deps/sabre/libmemory_restorator.a" \
    /opt/sabre/firecracker/build/sabre/libmemory_restorator.a
  sudo ldconfig

  if [[ ! -d $repo_dir/.git ]]; then
    git clone --branch snapshare-sabre-buffered \
      "$archive_dir/source/Liquidzk-vHive-snapshare-sabre-buffered.bundle" "$repo_dir"
    git -C "$repo_dir" remote set-url origin git@github.com:Liquidzk/vHive.git
  fi
  git -C "$repo_dir" merge-base --is-ancestor "$required_vhive_base" HEAD || \
    die "vHive checkout does not contain required base $required_vhive_base"

  if [[ ! -d $vswarm_dir/.git ]]; then
    git clone --branch snapshare-eval-images \
      "$archive_dir/source/Liquidzk-vSwarm-snapshare-eval-images.bundle" "$vswarm_dir"
    git -C "$vswarm_dir" remote set-url origin git@github.com:Liquidzk/vSwarm.git
  fi

  install -m 0644 "$archive_dir/configs/image_map.json" \
    "$repo_dir/cmd/relay/image_map.json"
  install -m 0755 "$archive_dir/binaries/relay-planb-iaa-v9" \
    "$repo_dir/cmd/relay/relay-planb-iaa-v9"
  install -m 0755 "$archive_dir/binaries/relay-planb-iaa-v10-partitions" \
    "$repo_dir/cmd/relay/relay-planb-iaa-v10-partitions"
  install -d -m 0755 "$eval_dir"
  install -m 0755 "$repo_dir"/scripts/planb-eval/*.sh "$eval_dir/"
  install -m 0755 "$repo_dir"/scripts/planb-eval/*.py "$eval_dir/"
  install -m 0755 "$archive_dir/binaries/planb_encode_partitions" "$eval_dir/"
  install -m 0755 "$archive_dir/binaries/planb_encode_gzip" "$eval_dir/"

  if [[ ! -d ${HOME}/snapshare-sabre ]]; then
    tar --zstd -xf "$archive_dir/packages/snapshare-sabre-component.tar.zst" -C "$HOME"
  fi
  if [[ ! -d ${HOME}/images ]]; then
    tar --zstd -xf "$archive_dir/packages/function-provenance-images.tar.zst" -C "$HOME"
  fi
  echo "INSTALL=PASS repo=$repo_dir commit=$(git -C "$repo_dir" rev-parse HEAD)"
}

restore_eval() {
  [[ ! -e $eval_dir/results ]] || die "$eval_dir/results already exists"
  tar --zstd -xf "$archive_dir/packages/snapshare-fourfn-eval.tar.zst" -C "$HOME"
  echo "EVAL_RESTORE=PASS root=$eval_dir"
}

restore_original_sabre() {
  [[ ! -e /opt/sabre/.git ]] || die "/opt/sabre already exists"
  sudo tar --zstd -xf "$archive_dir/packages/original-sabre-opt.tar.zst" -C /opt
  echo ORIGINAL_SABRE_RESTORE=PASS
}

configure_iaa() {
  hardware_gate
  local config="$archive_dir/provenance/accel-config-current.json"
  local missing=0
  while IFS= read -r dev; do
    [[ -e /sys/bus/dsa/devices/$dev ]] || {
      echo "Missing saved-topology device: $dev" >&2
      missing=1
    }
  done < <(jq -r '.[].dev' "$config")
  ((missing == 0)) || die "IAA topology differs; do not force-load the saved config"

  sudo accel-config load-config -f -e -c "$config"
  local queue_count
  queue_count=$(find /dev/iax -maxdepth 1 -type c -name 'wq*' 2>/dev/null | wc -l)
  [[ $queue_count -eq 32 ]] || die "expected 32 enabled IAA work queues, found $queue_count"
  local hugepages=${IAA_HUGEPAGES:-4000}
  for hp_file in /sys/devices/system/node/node*/hugepages/hugepages-2048kB/nr_hugepages; do
    echo "$hugepages" | sudo tee "$hp_file" >/dev/null
  done
  echo "IAA_CONFIG=PASS queues=$queue_count"
}

configure_cpu() {
  [[ -x $eval_dir/configure_cpu_baseline.sh ]] || die "run install first"
  local state_dir=${CPU_STATE_DIR:-${HOME}/cpu-state-before-fixed4g-$(date +%Y%m%d-%H%M%S)}
  sudo "$eval_dir/configure_cpu_baseline.sh" apply-4ghz "$state_dir"
  sudo "$eval_dir/configure_cpu_baseline.sh" status
}

import_minio() {
  command -v docker >/dev/null || die "run install first"
  if sudo docker container inspect snapshare-minio >/dev/null 2>&1; then
    die "container snapshare-minio already exists; inspect it instead of overwriting"
  fi
  local import_root=${MINIO_IMPORT_DIR:-${HOME}/iaa-handoff-import}
  mkdir -p "$import_root"
  if [[ ! -f $import_root/.minio-extract-complete ]]; then
    [[ ! -e $import_root/minio-snapshots ]] || \
      die "partial MinIO extraction exists at $import_root/minio-snapshots"
    tar --zstd -xf "$archive_dir/packages/minio-snapshots.tar.zst" -C "$import_root"
    touch "$import_root/.minio-extract-complete"
  fi

  sudo install -d -m 0755 /var/lib/snapshare-minio
  sudo docker run -d --name snapshare-minio --restart unless-stopped \
    --network host \
    -e "MINIO_ROOT_USER=$minio_user" \
    -e "MINIO_ROOT_PASSWORD=$minio_password" \
    -v /var/lib/snapshare-minio:/data \
    quay.io/minio/minio:RELEASE.2024-12-18T13-15-44Z \
    server /data --address 127.0.0.1:9000 \
    --console-address 127.0.0.1:9001 >/dev/null
  for _ in $(seq 1 60); do
    curl --fail --silent http://127.0.0.1:9000/minio/health/ready >/dev/null 2>&1 && break
    sleep 1
  done
  curl --fail --silent http://127.0.0.1:9000/minio/health/ready >/dev/null || \
    die "MinIO did not become ready"

  sudo docker run --rm --network host \
    --user "$(id -u):$(id -g)" \
    -e MC_CONFIG_DIR=/tmp/mc \
    -e MINIO_USER="$minio_user" \
    -e MINIO_PASSWORD="$minio_password" \
    -v "$import_root/minio-snapshots:/import:ro" \
    --entrypoint /bin/sh minio/mc:RELEASE.2025-08-13T08-35-41Z -c '
      set -eu
      mc alias set dst http://127.0.0.1:9000 "$MINIO_USER" "$MINIO_PASSWORD" >/dev/null
      mc mb --ignore-existing dst/snapshots >/dev/null
      mc mirror --overwrite /import dst/snapshots
      mc stat dst/snapshots/planb-aes-go-private-iaa-deflate-20260821-fixed4g-iaa-wq8-diagonal16-r1-aes-p16j16/recipe_file >/dev/null
      mc stat dst/snapshots/planb-video-processing-python-4k-ab-iaa-deflate-20260821-fixed4g-iaa-wq8-diagonal16-r1-fourfn-p16j16/recipe_file >/dev/null
    '
  echo MINIO_IMPORT=PASS
}

restore_mongodb() {
  command -v docker >/dev/null || die "run install first"
  if sudo docker container inspect snapshare-mongodb >/dev/null 2>&1; then
    die "container snapshare-mongodb already exists; inspect it instead of overwriting"
  fi
  local bind_ips=127.0.0.1
  [[ -z ${MONGO_PRIVATE_IP:-} ]] || bind_ips="$bind_ips,$MONGO_PRIVATE_IP"
  sudo docker volume create snapshare-mongodb-db >/dev/null
  sudo docker volume create snapshare-mongodb-config >/dev/null
  sudo docker run -d --name snapshare-mongodb --restart unless-stopped \
    --network host \
    -v snapshare-mongodb-db:/data/db \
    -v snapshare-mongodb-config:/data/configdb \
    vhiveease/mongodb:latest --bind_ip "$bind_ips" >/dev/null
  for _ in $(seq 1 60); do
    timeout 1 bash -c '</dev/tcp/127.0.0.1/27017' 2>/dev/null && break
    sleep 1
  done
  timeout 1 bash -c '</dev/tcp/127.0.0.1/27017' 2>/dev/null || \
    die "MongoDB did not become ready"
  sudo docker exec -i snapshare-mongodb mongorestore --archive --gzip \
    < "$archive_dir/mongodb/snapshare-mongodb.archive.gz"
  sudo docker exec snapshare-mongodb mongo --quiet --eval \
    'db.getMongo().getDBNames().forEach(function(n){print(n)})'
  echo "MONGODB_RESTORE=PASS bind_ips=$bind_ips"
}

create_devmapper() {
  [[ -d $repo_dir/.git ]] || die "run install first"
  df -h /var/lib/firecracker-containerd 2>/dev/null || df -h /
  "$repo_dir/scripts/create_devmapper.sh"
  sudo dmsetup info fc-dev-thinpool >/dev/null
  echo DEVMAPPER=PASS
}

start_services() {
  hardware_gate
  [[ -d $repo_dir/.git ]] || die "run install first"
  [[ -d ${HOME}/images ]] || die "function provenance images are missing"
  sudo dmsetup info fc-dev-thinpool >/dev/null 2>&1 || die "run create-devmapper first"
  sudo docker container inspect snapshare-minio >/dev/null 2>&1 || die "run import-minio first"
  sudo docker container inspect snapshare-mongodb >/dev/null 2>&1 || die "run restore-mongodb first"
  [[ $(find /dev/iax -maxdepth 1 -type c -name 'wq*' 2>/dev/null | wc -l) -eq 32 ]] || \
    die "run configure-iaa first"

  local host_iface=${HOST_IFACE:-}
  if [[ -z $host_iface ]]; then
    host_iface=$(ip -o route show default | awk '{print $5; exit}')
  fi
  [[ -n $host_iface ]] || die "set HOST_IFACE explicitly"
  ip link show "$host_iface" >/dev/null || die "host interface $host_iface does not exist"
  mkdir -p "${HOME}/iaa-runtime-logs" "${HOME}/snapshots"

  tmux has-session -t http-address-resolver 2>/dev/null || \
    tmux new-session -d -s http-address-resolver \
      "sudo /usr/local/bin/http-address-resolver 2>&1 | tee ${HOME}/iaa-runtime-logs/http-address-resolver.log"
  tmux has-session -t demux-snapshotter 2>/dev/null || \
    tmux new-session -d -s demux-snapshotter \
      "sudo /usr/local/bin/demux-snapshotter 2>&1 | tee ${HOME}/iaa-runtime-logs/demux-snapshotter.log"
  tmux has-session -t firecracker 2>/dev/null || \
    tmux new-session -d -s firecracker \
      "sudo /usr/local/bin/firecracker-containerd --config /etc/firecracker-containerd/config.toml 2>&1 | tee ${HOME}/iaa-runtime-logs/firecracker-containerd.log"
  for _ in $(seq 1 60); do
    [[ -S /run/firecracker-containerd/containerd.sock ]] && break
    sleep 1
  done
  [[ -S /run/firecracker-containerd/containerd.sock ]] || \
    die "firecracker-containerd socket did not appear"
  sudo /usr/local/bin/firecracker-ctr \
    --address /run/firecracker-containerd/containerd.sock \
    images import "$archive_dir/images/firecracker-eval-images.oci.tar"

  local relay_dir=$repo_dir/cmd/relay
  local relay_binary=./relay-planb-iaa-v10-partitions
  local credentials="127.0.0.1:9000;$minio_user;$minio_password"
  local relay_log=${HOME}/iaa-runtime-logs/relay-planb-wq8.log
  local command=(sudo -E "$relay_binary"
    -ss devmapper -snapshots remote -dbg -netPoolSize 2 -vmMemSizeMib 4096
    -hostIface "$host_iface" -vethPrefix 172.29 -clonePrefix 172.30
    -endpoint 127.0.0.1:18080 -chunking -chunkSize 4096 -upf -ws
    -wsCoalescing -lazy -security partial -minioCredentials "$credentials"
    -planBPrivateWS -planBCodec iaa_deflate -planBPartitions 1 -planBJobs 1)
  if ! tmux has-session -t snaprelay_fourfn 2>/dev/null; then
    local command_q relay_dir_q relay_log_q
    printf -v command_q ' %q' "${command[@]}"
    printf -v relay_dir_q %q "$relay_dir"
    printf -v relay_log_q %q "$relay_log"
    tmux new-session -d -s snaprelay_fourfn \
      "cd $relay_dir_q &&$command_q 2>&1 | tee $relay_log_q"
  fi
  for _ in $(seq 1 240); do
    grep -q 'Loaded rootfs chunk hashes' "$relay_log" 2>/dev/null && break
    sleep 0.25
  done
  grep -q 'Loaded rootfs chunk hashes' "$relay_log" || \
    die "relay did not finish provenance initialization"
  echo "START=PASS endpoint=127.0.0.1:18080 host_iface=$host_iface"
}

status() {
  hardware_gate
  printf 'iaa_work_queues=%s\n' \
    "$(find /dev/iax -maxdepth 1 -type c -name 'wq*' 2>/dev/null | wc -l)"
  tmux list-sessions 2>/dev/null || true
  sudo docker ps --no-trunc 2>/dev/null || true
  sudo ss -lntp | grep -E ':(9000|27017|18080|35097)\b' || true
}

case ${1:-} in
  verify) verify_archive ;;
  install) install_artifacts ;;
  restore-eval) restore_eval ;;
  restore-sabre) restore_original_sabre ;;
  configure-iaa) configure_iaa ;;
  configure-cpu) configure_cpu ;;
  import-minio) import_minio ;;
  restore-mongodb) restore_mongodb ;;
  create-devmapper) create_devmapper ;;
  start) start_services ;;
  status) status ;;
  *)
    echo "usage: $0 {verify|install|restore-eval|restore-sabre|configure-iaa|configure-cpu|import-minio|restore-mongodb|create-devmapper|start|status}" >&2
    exit 2
    ;;
esac
