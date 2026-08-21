#!/usr/bin/env bash
set -euo pipefail

grpcurl=${GRPCURL:-${HOME}/vswarm/tools/bin/grpcurl}
proto_root=${PROTO_ROOT:-${HOME}/vswarm/utils/protobuf/helloworld}
endpoint=${RELAY_ENDPOINT:-127.0.0.1:18080}
image=${AES_IMAGE:-ghcr.io/leokondrashov/aes-go@sha256:50a670defc33dbbece7837c991999d9e118fa1e16bd10f9c8f06a07ecf078f07}
revision=${REVISION:-}
value=${AES_INPUT:-snapshare-aes-target}
timeout_s=${CALL_TIMEOUT:-300}

[[ -x $grpcurl ]] || { echo "missing grpcurl: $grpcurl" >&2; exit 2; }
[[ -f $proto_root/helloworld.proto ]] || { echo "missing protobuf tree: $proto_root" >&2; exit 2; }
[[ -n $revision ]] || {
  echo "set REVISION to a snapshot generated on this replacement node" >&2
  exit 2
}

relay_args="--addr=0.0.0.0:50000 --function-endpoint-url=0.0.0.0 --function-endpoint-port=50051 --function-name=aes-go --value=$value --fail-on-error-reply"
start_ns=$(date +%s%N)
response=$(timeout "$timeout_s" "$grpcurl" -plaintext -max-time "$timeout_s" \
  -import-path "$proto_root" -proto helloworld.proto \
  -H "image: $image" -H "revision: $revision" \
  -H 'args: --addr=0.0.0.0:50051' -H "relayArgs: $relay_args" \
  -d "{\"name\":\"$value\"}" "$endpoint" helloworld.Greeter/SayHello)
end_ns=$(date +%s%N)

[[ -n $response ]] || { echo "empty AES-Go response" >&2; exit 1; }
printf 'response=%s\n' "$response"
printf 'elapsed_ms=%s\n' "$(((end_ns - start_ns) / 1000000))"

for _ in $(seq 1 600); do
  if ! pgrep -x firecracker >/dev/null 2>&1; then
    sleep 1
    ! pgrep -x firecracker >/dev/null 2>&1 && break
  fi
  sleep 0.5
done
pgrep -x firecracker >/dev/null 2>&1 && {
  echo "Firecracker VM did not exit" >&2
  exit 1
}
echo AES_RESTORE=PASS
