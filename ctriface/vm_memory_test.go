package ctriface

import "testing"

func TestVMMemSizeForSnapshot(t *testing.T) {
	t.Run("default without map", func(t *testing.T) {
		o := &Orchestrator{vmMemSizeMib: 512}
		got, err := o.vmMemSizeForSnapshot("revision")
		if err != nil || got != 512 {
			t.Fatalf("vmMemSizeForSnapshot = %d, %v; want 512, nil", got, err)
		}
	})

	t.Run("mapped revision", func(t *testing.T) {
		o := &Orchestrator{
			vmMemSizeMib:        512,
			vmMemSizeBySnapshot: map[string]uint32{"revision": 2048},
		}
		got, err := o.vmMemSizeForSnapshot("revision")
		if err != nil || got != 2048 {
			t.Fatalf("vmMemSizeForSnapshot = %d, %v; want 2048, nil", got, err)
		}
	})

	for name, mapping := range map[string]map[string]uint32{
		"missing revision": {"other": 2048},
		"zero memory":      {"revision": 0},
	} {
		mapping := mapping
		t.Run(name, func(t *testing.T) {
			o := &Orchestrator{vmMemSizeMib: 512, vmMemSizeBySnapshot: mapping}
			if _, err := o.vmMemSizeForSnapshot("revision"); err == nil {
				t.Fatal("vmMemSizeForSnapshot unexpectedly succeeded")
			}
		})
	}
}

func TestWithVMMemSizeBySnapshotCopiesMap(t *testing.T) {
	source := map[string]uint32{"revision": 2048}
	o := &Orchestrator{}
	WithVMMemSizeBySnapshot(source)(o)
	source["revision"] = 4096
	if got := o.vmMemSizeBySnapshot["revision"]; got != 2048 {
		t.Fatalf("orchestrator map changed through caller mutation: got %d", got)
	}
}
