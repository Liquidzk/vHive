# UPF Refactor Plan

This document records the staged plan for integrating vanilla Firecracker
UFFD memory mappings into `memory/manager`.

## Scope

Implement the basic full guest memory file path only:

- Receive Firecracker guest memory mappings and the UFFD fd over the existing
  Unix socket path.
- Store mappings in `SnapshotState`.
- Serve page faults by translating Firecracker fault addresses through the
  mapping data into offsets in the full guest memory file.
- Keep the existing `MemoryManager.Activate` flow.
- Do not add a standalone `uffd_handler` binary.
- Do not copy SnapShare, balloon, working-set redesign, or benchmark/runtime
  config changes from the prototype commit.

## Step 0: Inspect Current Flow

Status: complete.

Findings:

- `SnapshotState.getUFFD()` currently receives only one fd through
  `github.com/ftrvxmtrx/fd.Get`.
- `SnapshotState.servePageFault()` assumes the first fault address is the
  guest memory base and computes `offset := address - s.startAddress`.
- Firecracker's vanilla UFFD path requires using the mappings JSON instead:
  `region.Offset + (faultPageAddr - region.BaseHostVirtAddr)`.
- The prototype commit `fd7e83a745398d208c7ef3236623ee0d1e97f51c` is useful
  as a reference for mapping JSON shape and receiving fd plus body, but its
  standalone handler and experimental runtime changes should not be copied.

## Step 1: Mapping Model And Pure Offset Logic

Status: complete.

Goal: add the data model and deterministic address translation helpers.

Files expected:

- `memory/manager/snapshot_state.go` or a small new file under
  `memory/manager`
- `memory/manager/*_test.go`

Files added:

- `memory/manager/uffd_mapping.go`
- `memory/manager/uffd_mapping_test.go`

Implementation:

- Add `GuestRegionUffdMapping` with Firecracker JSON field names:
  `base_host_virt_addr`, `size`, `offset`, `page_size`.
- Add helpers to:
  - align a fault address using the region page size
  - find the mapping containing a fault page address
  - compute the full memory file offset
- Return errors for invalid mappings, especially `PageSize == 0`.

Tests first:

- one region with zero offset
- one region with non-zero offset
- multiple regions
- non-page-aligned fault address
- address outside all regions
- invalid mapping with zero page size

Validation:

```bash
go test ./memory/manager
gofmt -w memory/manager
```

## Step 2: Receive Mappings Plus UFFD FD

Status: complete.

Goal: replace the legacy receive-fd-only path with Firecracker's vanilla socket
payload.

Files expected:

- `memory/manager/snapshot_state.go`
- `memory/manager/*_test.go`

Files added or updated:

- `memory/manager/snapshot_state.go`
- `memory/manager/uffd_socket.go`
- `memory/manager/uffd_socket_test.go`

Implementation:

- Update or replace `SnapshotState.getUFFD()` so it receives:
  - mappings JSON body
  - exactly one UFFD fd from Unix socket ancillary data
- Store parsed mappings in `SnapshotState`.
- Reuse existing fd helper code where it still fits, but prefer direct
  `ReadMsgUnix` parsing if the helper cannot receive the body and fd together.
- Keep simple timeout/retry behavior similar to the current dial loop.
- Keep `MemoryManager.Activate` ordering unchanged.

Tests first:

- receive valid JSON plus a temporary file fd over a Unix socket
- verify exactly one fd is received
- verify mappings parse correctly
- verify received fd is usable
- invalid JSON returns an error
- missing fd returns an error where testable

Validation:

```bash
go test ./memory/manager
gofmt -w memory/manager
```

## Step 3: Use Mappings When Serving Page Faults

Status: complete.

Goal: make full-memory-file page fault serving use Firecracker mapping offsets.

Files expected:

- `memory/manager/snapshot_state.go`
- `memory/manager/*_test.go`

Files updated:

- `memory/manager/snapshot_state.go`
- `memory/manager/uffd_mapping.go`
- `memory/manager/uffd_mapping_test.go`

Implementation:

- Align the fault address to the mapping page boundary before computing copy
  parameters.
- Find the mapping containing the aligned page address.
- Compute source offset as:
  `region.Offset + (faultPageAddr - region.BaseHostVirtAddr)`.
- Compute the source pointer from `s.guestMem` plus that file offset.
- Preserve the existing `UFFDIO_COPY` call behavior in `installRegion`.
- Extract a small pure helper returning `srcOffset`, `dstAddr`, `copyLen`, and
  `copyMode` so tests do not require a real UFFD fd.
- Keep metrics behavior as close as possible to the current path.

Tests first:

- calculation helper returns correct copy parameters for zero-offset mapping
- calculation helper returns correct copy parameters for non-zero-offset mapping
- non-page-aligned fault is aligned before translation
- address outside mappings returns an error

Validation:

```bash
go test ./memory/manager
gofmt -w memory/manager
```

## Step 4: Cleanup And Regression Pass

Status: complete.

Goal: remove dead legacy assumptions where safe and verify the complete patch.

Files expected:

- `memory/manager/snapshot_state.go`
- `memory/manager/*_test.go`
- possibly small local helper files in `memory/manager`

Implementation:

- Remove duplicate or obsolete receive-fd-only assumptions.
- Do not leave two UPF handlers.
- Do not add `uffd_handler/`.
- Keep comments precise and minimal.
- Return errors rather than adding new `log.Fatal` paths where practical.
- Keep tests deterministic and independent of Firecracker by default.

Validation:

```bash
go test ./memory/manager
go test ./...
gofmt -w memory/manager
git diff --stat
git diff
```

Validation notes:

- `go test ./memory/manager` passes.
- `go test ./...` requires the broader vHive integration test environment. In
  this workspace it fails in existing tests that need privileged network setup,
  `/fccd/snapshots`, `/fccd/test/snapshots`, or live CRI endpoints.

## Integration Check After Manager Patch

This is separate from the isolated `memory/manager` refactor unless compilation
or runtime wiring requires it.

Items to confirm:

- Where `SnapshotStateCfg.InstanceSockAddr` should be populated for snapshot
  restore.
- Whether the current firecracker-containerd fork exposes enough API surface to
  request `MemBackend` type `Uffd` and pass the UFFD socket path.
- Whether any `ctriface` change is needed after `memory/manager` can receive
  and use Firecracker mappings.

Keep this check small and avoid modifying `cmd/snap_bench.go` or runtime log
configuration.

## Version Alignment Follow-up

Status: in progress.

Goal: align the local runtime binaries with the reference UPF implementation
without copying the standalone prototype handler.

Reference commits:

- `fd7e83a745398d208c7ef3236623ee0d1e97f51c`: adds the prototype UFFD handler.
- `2fea4fb752dd123852d23732b9bf1fcfaa0ad2ff`: bumps the Firecracker runtime
  stack used by that handler.
- `3963547a666d7c06fbe9e4281fe428d6e905fbdf`: fixes mapping-based UFFD offset
  handling in the prototype handler.

Adopt:

- `bin/firecracker` and `bin/jailer` from the reference stack
  (`v1.13.1`).
- `bin/firecracker-containerd` and `bin/containerd-shim-aws-firecracker` from
  the same reference stack (`containerd 1.6.20+unknown`).
- The mapping-based offset rule fixed in the prototype:
  `region.Offset + (faultPageAddr - region.BaseHostVirtAddr)`.
- Non-fatal UFFD loop handling for transient or shutdown-related errors such as
  `EINTR`, `EAGAIN`, and `EBADF`.

Do not adopt:

- The standalone `uffd_handler/` directory.
- `cmd/snap_bench.go` benchmark/debug changes.
- Runtime log-level changes.
- Experimental working-set, balloon, or trace behavior outside the basic full
  guest memory file path.

The current Go module still uses the integrated `memory/manager` handler and a
newer firecracker-containerd proto package whose `MemBackend` field number is
compatible with the reference binary protocol.
