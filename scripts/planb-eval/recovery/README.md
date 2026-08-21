# Single-node IAA recovery helpers

These scripts preserve the procedure used to capture and restore the
2026-08-22 SnapShare Plan B evaluation node. Runtime binaries, OCI images,
MongoDB data, all essential experiment results/logs, and SHA-256 manifests are
stored separately in the local `IAA单节点-保存/恢复/node-handoff-20260822`
package; they are intentionally not committed to Git. Regenerable MinIO
objects, staged WS payloads, caches, and build directories are not retained.

- `collect_node_provenance.sh` records hardware, CPU, IAA, runtime, package,
  container, and binary state without stopping services.
- `export_minio_bucket.sh` exports the complete `snapshots` bucket through the
  S3 API. It does not copy MinIO's internal `xl.meta` representation.
- `pack_remote_handoff.sh` is the optional full forensic packer. The compact
  release package keeps only its function-provenance/component outputs and a
  separately filtered essential-results archive.
- `restore_node.sh` performs explicit, gated recovery stages from the local
  package. It refuses to overwrite existing MinIO, MongoDB, result, or Sabre
  state.
- `validate_handoff.sh` verifies every checksum, archive stream, Git bundle,
  script, and required hardware/provenance count before the source node is
  released.
- `smoke_aes_restore.sh` validates a restored AES-Go IAA snapshot after the
  runtime is started.
- `fast-restore.cloud-init.yaml` installs only the inexpensive preflight
  dependencies and exits early when IAA or KVM is unavailable.

The recovery script expects four IAA devices, 32 enabled shared work queues,
Ubuntu 24.04 x86-64, and the local package layout documented by that package's
README. The normal sequence on a fresh compatible node is:

```bash
./restore_node.sh verify
./restore_node.sh install
./restore_node.sh restore-eval
./restore_node.sh restore-sabre
./restore_node.sh configure-iaa
./restore_node.sh configure-cpu
./restore_node.sh start-minio
MONGO_PRIVATE_IP=10.0.0.11 ./restore_node.sh restore-mongodb
./restore_node.sh create-devmapper
HOST_IFACE=bond0.3 ./restore_node.sh start
```

`restore-eval` installs the portable local result tree, overlays the
essential result/log archive collected from the remote node, and restores the
earlier `sabre-results` plus runtime logs. Large staged
working-set payloads, generated snapshot caches, MinIO objects, and build
directories are deliberately regenerated rather than copied.

After regenerating an AES snapshot on the replacement node, validate it with
`REVISION=<new-revision> ./smoke_aes_restore.sh`. The helper deliberately has
no default revision because the old MinIO objects are not part of this compact
handoff.

`MONGO_PRIVATE_IP` and `HOST_IFACE` must be adapted to the replacement node.
The restored MinIO and MongoDB credentials are evaluation-only defaults and
must not be exposed on a public interface.
