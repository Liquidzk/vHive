#!/usr/bin/env bash
# Run on the single IAA node after run_fourfn_planb_learning.sh succeeds.
# It clones each frozen revision in MinIO, encodes the exact same private bytes
# with the historical Go gzip, SW-QPL, IAA and zstd-3, then compares raw and
# the four codecs over
# cache-empty MinIO and local-cache restores. The pre-run local cache is moved
# aside and restored on every exit path.
set -euo pipefail

stamp=${STAMP:-20260819-fourfn-planb-codec-preflight-r2}
variant_stamp=${VARIANT_STAMP:-20260819-r2}
repetitions=${REPETITIONS:-10}
result_root=${RESULT_ROOT:-/home/ubuntu/snapshare-fourfn-eval/results/$stamp}
learning_root=${LEARNING_ROOT:-/home/ubuntu/snapshare-fourfn-eval/results/20260819-fourfn-planb-learning-r2}
snapshot_root=/home/ubuntu/snapshots
preserved_root=$result_root/preserved-before-run
helper=/home/ubuntu/snapshare-fourfn-eval/remote_fourfn_relay.sh
encoder=${ENCODER:-/home/ubuntu/snapshare-fourfn-eval/planb_encode_partitions}
relay_binary=${RELAY_BINARY:-./relay-planb-iaa-v9}
restore_relay_binary=${RESTORE_RELAY_BINARY:-./relay-planb-iaa-v9}
planb_partitions=${PLANB_PARTITIONS:-1}
planb_jobs=${PLANB_JOBS:-1}
grpcurl=/home/ubuntu/vswarm/tools/bin/grpcurl
proto_root=/home/ubuntu/vswarm/utils/protobuf/helloworld
endpoint=127.0.0.1:18080
mongo_uri=mongodb://10.0.0.11:27017
image_tag=restore-reconnect-f268d11-r2-20260818-esgz
call_timeout=300

default_workloads="image-rotate-go image-rotate-python video-processing-python video-analytics-standalone-python"
default_codecs="raw gzip sw_deflate iaa_deflate zstd_3"
read -r -a workloads <<<"${WORKLOAD_LIST:-$default_workloads}"
read -r -a codecs <<<"${CODEC_LIST:-$default_codecs}"

declare -A images=(
  [image-rotate-go]="ghcr.io/liquidzk/image-rotate-go:$image_tag"
  [image-rotate-python]="ghcr.io/liquidzk/image-rotate-python:$image_tag"
  [video-processing-python]="ghcr.io/liquidzk/video-processing-python:$image_tag"
  [video-analytics-standalone-python]="ghcr.io/liquidzk/video-analytics-standalone-python:$image_tag"
)
declare -A vm_mib=(
  [image-rotate-go]=512
  [image-rotate-python]=512
  [video-processing-python]=2048
  [video-analytics-standalone-python]=4096
)
declare -A input=(
  [image-rotate-go]=img11.jpg
  [image-rotate-python]=img5.jpg
  [video-processing-python]=video2.mp4
  [video-analytics-standalone-python]=video2.mp4
)
declare -A function_args=(
  [image-rotate-go]="--addr=0.0.0.0:50051 --db_addr=$mongo_uri --connection_mode=per-invocation --mongo_timeout=30s"
  [image-rotate-python]="--addr=0.0.0.0 --port=50051 --db_addr=$mongo_uri --connection_mode=per-invocation --mongo_timeout_ms=30000"
  [video-processing-python]="--addr=0.0.0.0 --port=50051 --db_addr=$mongo_uri --connection_mode=per-invocation --mongo_timeout_ms=30000 --default_video=default.mp4 --num_frames=10"
  [video-analytics-standalone-python]="--addr=0.0.0.0 --port=50051 --db_addr=$mongo_uri --connection_mode=per-invocation --mongo_timeout_ms=30000 --default_video=default.mp4 --num_frames=10"
)
declare -A codec_slug=(
  [raw]=raw
  [gzip]=gzip
  [sw_deflate]=sw-deflate
  [iaa_deflate]=iaa-deflate
  [zstd_3]=zstd-3
)

[[ $repetitions =~ ^[1-9][0-9]*$ ]] || { echo "REPETITIONS must be positive" >&2; exit 2; }
[[ $planb_partitions =~ ^[1-9][0-9]*$ && $planb_partitions -le 4294967295 ]] || {
  echo "PLANB_PARTITIONS must be between 1 and 4294967295" >&2
  exit 2
}
[[ $planb_jobs =~ ^[1-9][0-9]*$ && $planb_jobs -le 255 ]] || {
  echo "PLANB_JOBS must be between 1 and 255" >&2
  exit 2
}
[[ ${#workloads[@]} -gt 0 && ${#codecs[@]} -gt 0 ]] || { echo "workload and codec lists must be non-empty" >&2; exit 2; }
for workload in "${workloads[@]}"; do
  [[ -n ${images[$workload]+x} ]] || { echo "unsupported workload: $workload" >&2; exit 2; }
done
for codec in "${codecs[@]}"; do
  [[ -n ${codec_slug[$codec]+x} ]] || { echo "unsupported codec: $codec" >&2; exit 2; }
done
[[ -x $helper && -x $encoder && -x $grpcurl ]] || { echo "missing helper, encoder, or grpcurl" >&2; exit 2; }
[[ -s $learning_root/manifest.csv ]] || { echo "learning manifest is missing" >&2; exit 2; }
learning_rows=$(awk 'END { print NR-1 }' "$learning_root/manifest.csv")
learning_failures=$(awk -F, 'NR > 1 && ($7 != 0 || $8 != 1) { n++ } END { print n+0 }' "$learning_root/manifest.csv")
[[ $learning_rows -eq 12 && $learning_failures -eq 0 ]] || {
  echo "learning gate is incomplete: rows=$learning_rows failures=$learning_failures" >&2
  exit 2
}

mkdir -p "$result_root/runs" "$result_root/staged"
calls=$result_root/calls.csv
variants=$result_root/variants.csv
launcher=$result_root/launcher.log
if [[ -e $calls || -e $preserved_root/snapshots ]]; then
  echo "refusing to reuse result root: $result_root" >&2
  exit 1
fi
printf '%s\n' 'workload,codec,path,rep,revision,start_ns,end_ns,e2e_ms,status,correct,mongo_setup_us,response,segment,relay_log,partitions,jobs' >"$calls"
printf '%s\n' 'workload,codec,source_revision,revision,input_bytes,payload_bytes,ratio,input_sha256,payload_sha256,partitions_sha256,partitions,jobs' >"$variants"

log() {
  printf '%s %s\n' "$(date -Is)" "$*" | tee -a "$launcher"
}

mc_eval() {
  sudo docker exec snapshare-minio sh -lc \
    'mc alias set eval http://127.0.0.1:9000 minio minio123 >/dev/null'
  sudo docker exec snapshare-minio mc "$@"
}

mc_pipe_file() {
  local source=$1 target=$2
  sudo docker exec snapshare-minio sh -lc \
    'mc alias set eval http://127.0.0.1:9000 minio minio123 >/dev/null'
  sudo docker exec -i snapshare-minio mc pipe "$target" <"$source" >/dev/null
}

revision_for() {
  local workload=$1 codec=$2
  printf 'planb-%s-4k-ab-%s-%s\n' "$workload" "${codec_slug[$codec]}" "$variant_stamp"
}

source_revision_for() {
  printf 'planb-%s-fourfn-20260819-r2\n' "$1"
}

stop_relay() {
  "$helper" stop
}

start_relay() {
  PLANB_PARTITIONS="$planb_partitions" PLANB_JOBS="$planb_jobs" \
    RELAY_BINARY="$relay_binary" "$helper" start "$@"
}

relay_ports_idle() {
  sudo ss -H -lnt 2>/dev/null \
    | awk '$4 ~ /:5000[0-9]$/ { found=1 } END { exit found ? 1 : 0 }'
}

wait_no_vm() {
  for _ in $(seq 1 600); do
    if ! pgrep -x firecracker >/dev/null 2>&1 && relay_ports_idle; then
      sleep 1
      if ! pgrep -x firecracker >/dev/null 2>&1 && relay_ports_idle; then
        return 0
      fi
    fi
    sleep 0.5
  done
  return 1
}

restore_original_state() {
  local status=$1
  trap - EXIT INT TERM
  log "restoring pre-run cache and frozen IAA relay status=$status"
  stop_relay || true
  if [[ -d $preserved_root/snapshots ]]; then
    sudo rm -rf -- "$snapshot_root"
    sudo mv "$preserved_root/snapshots" "$snapshot_root"
  fi
  PLANB_PARTITIONS=1 PLANB_JOBS=1 RELAY_BINARY="$restore_relay_binary" \
    "$helper" start iaa_deflate 4096 0 \
    /home/ubuntu/snapshare-fourfn-eval/logs/fourfn-after-ab.log snaprelay_fourfn || true
  exit "$status"
}
trap 'restore_original_state $?' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

prepare_variant() {
  local workload=$1 codec=$2 source_revision revision raw_file stage_dir encode_line
  local input_bytes payload_bytes ratio input_sha payload_sha partitions_sha
  source_revision=$(source_revision_for "$workload")
  revision=$(revision_for "$workload" "$codec")
  stage_dir=$result_root/staged/$workload/$codec
  mkdir -p "$stage_dir"
  raw_file=$result_root/staged/$workload/frozen/working_set_pages_content_private
  mkdir -p "$(dirname "$raw_file")"
  if [[ ! -s $raw_file ]]; then
    mc_eval stat "eval/snapshots/$source_revision/working_set_pages_index_private" >/dev/null
    mc_eval cat "eval/snapshots/$source_revision/working_set_pages_content_private" >"$raw_file"
  fi
  [[ -s $raw_file ]] || { log "missing frozen private working set workload=$workload source=$source_revision"; return 1; }

  if mc_eval stat "eval/snapshots/$revision/recipe_file" >/dev/null 2>&1; then
    log "refusing to overwrite MinIO variant=$revision"
    return 1
  fi
  mc_eval cp --recursive "eval/snapshots/$source_revision/" "eval/snapshots/$revision/" >/dev/null

  input_bytes=$(stat -c %s "$raw_file")
  input_sha=$(sha256sum "$raw_file" | awk '{print $1}')
  payload_bytes=0
  ratio=1
  payload_sha=
  partitions_sha=
  if [[ $codec != raw ]]; then
    encode_line=$(sudo -E "$encoder" -codec "$codec" -input "$raw_file" \
      -output-base "$stage_dir/working_set_pages_content_private.planb" \
      -partitions "$planb_partitions" -jobs "$planb_jobs")
    printf '%s\n' "$encode_line" >"$stage_dir/encode.log"
    payload_bytes=$(sed -n 's/.* payload_bytes=\([0-9][0-9]*\).*/\1/p' <<<"$encode_line")
    ratio=$(sed -n 's/.* ratio=\([0-9.]*\).*/\1/p' <<<"$encode_line")
    payload_sha=$(sed -n 's/.* payload_sha256=\([0-9a-f]*\).*/\1/p' <<<"$encode_line")
    partitions_sha=$(sed -n 's/.* partitions_sha256=\([0-9a-f]*\).*/\1/p' <<<"$encode_line")
    mc_pipe_file "$stage_dir/working_set_pages_content_private.planb.snapshot" \
      "eval/snapshots/$revision/working_set_pages_content_private.planb.snapshot"
    mc_pipe_file "$stage_dir/working_set_pages_content_private.planb.partitions" \
      "eval/snapshots/$revision/working_set_pages_content_private.planb.partitions"
  fi
  printf '%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\n' \
    "$workload" "$codec" "$source_revision" "$revision" "$input_bytes" \
    "$payload_bytes" "$ratio" "$input_sha" "$payload_sha" "$partitions_sha" \
    "$planb_partitions" "$planb_jobs" >>"$variants"
  log "variant ready workload=$workload codec=$codec partitions=$planb_partitions jobs=$planb_jobs revision=$revision input_bytes=$input_bytes payload_bytes=$payload_bytes ratio=$ratio"
}

reset_generated_cache() {
  stop_relay
  sudo rm -rf -- "$snapshot_root"
  sudo install -d -m 0755 -o ubuntu -g ubuntu "$snapshot_root"
}

invoke_once() {
  local workload=$1 codec=$2 path=$3 rep=$4 relay_log=$5
  local revision image args value relay_args run_dir response stderr segment
  local start_line start_ns end_ns e2e_ms status correct=0 mongo_setup_us
  revision=$(revision_for "$workload" "$codec")
  image=${images[$workload]}
  args=${function_args[$workload]}
  value=${input[$workload]}
  relay_args="--addr=0.0.0.0:50000 --function-endpoint-url=0.0.0.0 --function-endpoint-port=50051 --function-name=$workload --value=$value --fail-on-error-reply"
  run_dir=$result_root/runs/$workload/$codec/$path
  response=$run_dir/response-$rep.json
  stderr=$run_dir/response-$rep.stderr.log
  segment=$run_dir/relay-segment-$rep.log
  mkdir -p "$run_dir"
  start_line=$(wc -l <"$relay_log")
  start_ns=$(date +%s%N)
  set +e
  timeout "$call_timeout" "$grpcurl" -plaintext -max-time "$call_timeout" \
    -import-path "$proto_root" -proto helloworld.proto \
    -H "image: $image" -H "revision: $revision-a-b" \
    -H "args: $args" -H "relayArgs: $relay_args" \
    -d "{\"name\":\"$value\"}" "$endpoint" helloworld.Greeter/SayHello \
    >"$response" 2>"$stderr"
  status=$?
  set -e
  end_ns=$(date +%s%N)
  e2e_ms=$(((end_ns - start_ns) / 1000000))
  wait_no_vm
  sed -n "$((start_line + 1)),\$p" "$relay_log" >"$segment"

  if [[ $status -eq 0 ]] \
    && grep -Fq 'connection_mode: per-invocation' "$response" \
    && ! grep -Eq 'Error:|Failed|Unavailable' "$response"; then
    correct=1
  fi
  if [[ $path == remote ]]; then
    grep -q "Using remote snapshot for rev $revision" "$segment"
  else
    grep -q "Using snapshot for rev $revision" "$segment"
  fi
  if [[ $codec == raw ]]; then
    ! grep -q 'Plan B private WS decompressed' "$segment"
  else
    grep -q "Plan B private WS decompressed: revision=$revision codec=$codec" "$segment"
    ! grep -q 'falling back to raw content' "$segment"
  fi
  mongo_setup_us=$(sed -n 's/.*mongo_setup_us: \([0-9][0-9]*\).*/\1/p' "$response" | tail -n 1)
  printf '%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\n' \
    "$workload" "$codec" "$path" "$rep" "$revision" "$start_ns" "$end_ns" \
    "$e2e_ms" "$status" "$correct" "${mongo_setup_us:-}" "$response" "$segment" "$relay_log" \
    "$planb_partitions" "$planb_jobs" >>"$calls"
  log "call workload=$workload codec=$codec partitions=$planb_partitions jobs=$planb_jobs path=$path rep=$rep/$repetitions e2e_ms=$e2e_ms status=$status correct=$correct mongo_setup_us=${mongo_setup_us:-na}"
  [[ $status -eq 0 && $correct -eq 1 ]]
}

log "preparing four-function Plan B variants partitions=$planb_partitions jobs=$planb_jobs repetitions=$repetitions"
for workload in "${workloads[@]}"; do
  for codec in "${codecs[@]}"; do
    prepare_variant "$workload" "$codec"
  done
done

stop_relay
mkdir -p "$preserved_root"
sudo mv "$snapshot_root" "$preserved_root/snapshots"
sudo install -d -m 0755 -o ubuntu -g ubuntu "$snapshot_root"

for workload in "${workloads[@]}"; do
  log "begin workload=$workload"
  for codec in "${codecs[@]}"; do
    log "begin workload=$workload codec=$codec"
    for rep in $(seq 1 "$repetitions"); do
      reset_generated_cache
      relay_log=$result_root/runs/$workload/$codec/remote/relay-$rep.log
      mkdir -p "$(dirname "$relay_log")"
      start_relay "$codec" "${vm_mib[$workload]}" 0 "$relay_log" snaprelay_fourfn
      invoke_once "$workload" "$codec" remote "$rep" "$relay_log"
    done
    for rep in $(seq 1 "$repetitions"); do
      invoke_once "$workload" "$codec" local "$rep" "$relay_log"
    done
    log "complete workload=$workload codec=$codec"
  done
  log "complete workload=$workload"
done

rows=$(awk 'END { print NR-1 }' "$calls")
failures=$(awk -F, 'NR > 1 && ($9 != 0 || $10 != 1) { n++ } END { print n+0 }' "$calls")
expected_rows=$(( ${#workloads[@]} * ${#codecs[@]} * 2 * repetitions ))
log "FOURFN_PLANB_AB_COMPLETE rows=$rows expected=$expected_rows failures=$failures"
[[ $rows -eq $expected_rows && $failures -eq 0 ]]
