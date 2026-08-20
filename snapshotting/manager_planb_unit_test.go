package snapshotting

import (
	"bytes"
	"reflect"
	"testing"
)

func TestSortPrivateWorkingSet(t *testing.T) {
	pfns := []uint64{5, 2, 3}
	content := make([]byte, len(pfns)*4096)
	for i, pfn := range pfns {
		content[i*4096] = byte(pfn)
	}
	sortedPFNs, sortedContent, err := sortPrivateWorkingSet(pfns, content)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sortedPFNs, []uint64{2, 3, 5}) {
		t.Fatalf("sorted PFNs = %v", sortedPFNs)
	}
	if got := []byte{sortedContent[0], sortedContent[4096], sortedContent[8192]}; !bytes.Equal(got, []byte{2, 3, 5}) {
		t.Fatalf("sorted page markers = %v", got)
	}
}

func TestSortPrivateWorkingSetRejectsDuplicatePFN(t *testing.T) {
	if _, _, err := sortPrivateWorkingSet([]uint64{1, 1}, make([]byte, 8192)); err == nil {
		t.Fatal("expected duplicate PFN error")
	}
}

func TestPrivateWorkingSetSize(t *testing.T) {
	size, err := privateWorkingSetSize([]byte("pfn\n2\n3\n5\n"))
	if err != nil {
		t.Fatal(err)
	}
	if size != 3*4096 {
		t.Fatalf("private working-set size = %d", size)
	}
}

func TestPackedWorkingSetSizeDeduplicatesAndAcceptsRecorderColumns(t *testing.T) {
	size, err := packedWorkingSetSize([]byte("pfn,timestamp\n5,10\n2,20\n5,30\n"))
	if err != nil {
		t.Fatal(err)
	}
	if size != 2*4096 {
		t.Fatalf("packed working-set size = %d", size)
	}
}
