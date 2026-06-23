# UPF Module Validation

This document describes how to validate the UPF/UFFD memory restore path after
vHive has already been deployed with InVitro.

If a deployment is needed, use the InVitro branch at
https://github.com/Liquidzk/invitro/tree/likun/upf-validation-deploy.

Assumptions:

* The target vHive branch is already deployed on the nodes.
* Worker nodes run `./vhive -snapshots -upf`.
* A Firecracker Knative service such as `helloworld` is already available or
  can be deployed by the reviewer.
* The reviewer knows the master SSH host, worker SSH hosts, and gateway IP.

## 1. Memory Manager Unit Tests

Run the focused module tests locally:

```bash
cd "$VHIVE_ROOT"
go test ./memory/manager
```

These tests validate:

* Firecracker guest memory mapping JSON parsing.
* UFFD fd receipt over Unix socket.
* Fault-address to full-memory-file offset calculation.
* Memory manager register, prepare, activate, deactivate, and deregister flow.

If the remote node does not have Go installed, compile the test binary locally
and run it on the remote node:

```bash
cd "$VHIVE_ROOT"
go test -c ./memory/manager -o /tmp/vhive-memory-manager.test
scp /tmp/vhive-memory-manager.test <user>@<worker-host>:/tmp/vhive-memory-manager.test

ssh <user>@<worker-host> \
  '/tmp/vhive-memory-manager.test -test.v \
    -test.run "TestPrepareGuestMemoryFileAndValidateGuestMemory|TestMemoryManagerRegisterFetchPrepareDeregister|TestMemoryManagerActivateReceivesFirecrackerMappings" \
    -test.count=1 \
    -test.timeout=30s'

rm -f /tmp/vhive-memory-manager.test
ssh <user>@<worker-host> 'rm -f /tmp/vhive-memory-manager.test'
```

Expected result:

```text
PASS
```

Expected logs may include intentional negative-path messages:

```text
VM already registered with the memory manager
VM is not registered with the memory manager
Read incomplete uffd_msg
```

They are acceptable when the test exits with `PASS`.

## 2. Confirm Runtime Mode

On each worker that may host Firecracker pods:

```bash
ssh <user>@<worker-host> \
  'git -C ~/vhive log -1 --oneline; \
   pgrep -af "vhive|firecracker-containerd"; \
   ~/vhive/bin/firecracker --version'
```

The vHive process must include:

```text
./vhive -snapshots -upf
```

## 3. Create A Snapshot

Trigger the service once from the master node:

```bash
ssh <user>@<master-host> \
  'curl -i --max-time 180 http://helloworld.default.<gateway-ip>.sslip.io || true; \
   kubectl get pods -n default -o wide'
```

`curl` may return `502` for the gRPC `helloworld` workload. That is not the UPF
pass/fail signal; it only forces the VM to start.

Record the worker that hosted the pod:

```bash
ssh <user>@<master-host> \
  'kubectl get pod -n default \
    -l serving.knative.dev/service=helloworld \
    -o jsonpath="{.items[0].spec.nodeName}{\"\n\"}"'
```

Scale down or delete the pod so vHive saves a snapshot:

```bash
ssh <user>@<master-host> \
  'kubectl delete pod -n default \
    -l serving.knative.dev/service=helloworld \
    --ignore-not-found=true'
```

On the worker that hosted the pod:

```bash
ssh <user>@<snapshot-worker-host> \
  'sudo find /fccd/snapshots/helloworld-00001 \
    -maxdepth 1 -type f -printf "%f %s\n" | sort'
```

Expected files:

```text
info_file
mem_file
patch_file
snap_file
```

For the default `helloworld` VM, `mem_file` should be 512 MiB.

## 4. Verify UFFD Restore

The snapshot is local to the worker that created it. To force the next pod onto
that worker, temporarily cordon other schedulable nodes:

```bash
ssh <user>@<master-host> \
  'kubectl cordon <node-to-disable-1> <node-to-disable-2>'
```

Trigger the service again:

```bash
ssh <user>@<master-host> \
  'curl -i --max-time 180 http://helloworld.default.<gateway-ip>.sslip.io || true; \
   kubectl get pods -n default -o wide'
```

Inspect Firecracker logs on the snapshot worker:

```bash
ssh <user>@<snapshot-worker-host> \
  'tail -n 700 ~/firecracker_log.txt | \
    egrep -i "LoadSnapshot:true|SnapshotPath|Uffd|uffd.sock|successfully started the VM" || true'
```

The restore path is active when the log contains:

```text
LoadSnapshot:true
SnapshotPath:"/fccd/snapshots/helloworld-00001/snap_file"
Uffd
uffd.sock
successfully started the VM
```

Check vHive for UFFD serving errors:

```bash
ssh <user>@<snapshot-worker-host> \
  'tail -n 700 ~/vhive_log.txt | \
    egrep -i "uffd|userfault|page fault|mapping|panic|fatal|failed|error" || true'
```

There should be no UFFD mapping or page-fault serving errors. Restore scheduling
after the check:

```bash
ssh <user>@<master-host> \
  'kubectl uncordon <node-to-disable-1> <node-to-disable-2>'
```

## 5. Functional gRPC Check

Use the vHive CRI gRPC test to validate that the restored VM serves requests.

If the master has Go installed:

```bash
ssh <user>@<master-host> \
  'cd ~/vhive && go test ./cri -run TestSingleInvoke -count=1 \
    -gatewayURL <gateway-ip>.sslip.io \
    -namespace default \
    -timeout 120s'
```

If the master does not have Go installed:

```bash
cd "$VHIVE_ROOT"
go test -c ./cri -o /tmp/vhive-cri.test
scp /tmp/vhive-cri.test <user>@<master-host>:/tmp/vhive-cri.test

ssh <user>@<master-host> \
  '/tmp/vhive-cri.test -test.run TestSingleInvoke -test.count=1 \
    -gatewayURL <gateway-ip>.sslip.io \
    -namespace default \
    -test.timeout 120s'

rm -f /tmp/vhive-cri.test
ssh <user>@<master-host> 'rm -f /tmp/vhive-cri.test'
```

Expected result:

```text
PASS
```

## Pass Criteria

The UPF path is working when all are true:

* `go test ./memory/manager` passes.
* Worker vHive process includes `-snapshots -upf`.
* Snapshot files exist under `/fccd/snapshots/helloworld-00001`.
* A later VM start logs `LoadSnapshot:true`.
* The same restore request includes `Uffd` and an `uffd.sock` path.
* The restored VM logs `successfully started the VM`.
* vHive logs do not show UFFD mapping or page-fault serving errors.
* The gRPC `TestSingleInvoke` check passes.
