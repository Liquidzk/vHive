package snapshotting

import (
	"bytes"
	"os"
	"testing"
)

func TestPrepareBaseSnapshotChunksForRevision(t *testing.T) {
	original := baseSnapChunks
	t.Cleanup(func() { baseSnapChunks = original })
	baseSnapChunks = map[[16]byte]bool{}

	base := NewSnapshot("base-2048", t.TempDir(), "")
	if err := base.CreateSnapDir(); err != nil {
		t.Fatal(err)
	}
	hashA := bytes.Repeat([]byte{0x11}, 16)
	hashB := bytes.Repeat([]byte{0x22}, 16)
	if err := os.WriteFile(base.GetRecipeFilePath(), append(hashA, hashB...), 0o600); err != nil {
		t.Fatal(err)
	}

	mgr := &SnapshotManager{
		chunking: true,
		snapshots: map[string]*Snapshot{
			"base-2048": base,
		},
	}
	mgr.PrepareBaseSnapshotChunksForRevision("base-2048")
	if len(baseSnapChunks) != 2 {
		t.Fatalf("base chunk count = %d; want 2", len(baseSnapChunks))
	}
	var keyA, keyB [16]byte
	copy(keyA[:], hashA)
	copy(keyB[:], hashB)
	if !baseSnapChunks[keyA] || !baseSnapChunks[keyB] {
		t.Fatalf("base chunk classification missing expected hashes: %#v", baseSnapChunks)
	}
}
