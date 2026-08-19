package uffd_handler

import (
	"reflect"
	"testing"
)

func TestCoalesceWorkingSetCopies(t *testing.T) {
	const page = uint64(4096)
	input := []workingSetCopy{
		{dst: 3 * page, src: 12 * page, len: page},
		{dst: 1 * page, src: 10 * page, len: page},
		{dst: 2 * page, src: 11 * page, len: page},
		// Guest-contiguous but not source-contiguous: must remain separate.
		{dst: 4 * page, src: 20 * page, len: page},
		// A guest gap also starts a new range.
		{dst: 6 * page, src: 21 * page, len: page},
	}
	want := []workingSetCopy{
		{dst: 1 * page, src: 10 * page, len: 3 * page},
		{dst: 4 * page, src: 20 * page, len: page},
		{dst: 6 * page, src: 21 * page, len: page},
	}
	if got := coalesceWorkingSetCopies(input, page); !reflect.DeepEqual(got, want) {
		t.Fatalf("coalesced copies = %#v, want %#v", got, want)
	}
}

func TestCoalesceWorkingSetCopiesRejectsUnalignedLength(t *testing.T) {
	input := []workingSetCopy{{dst: 0, src: 0, len: 1}}
	if got := coalesceWorkingSetCopies(input, 4096); len(got) != 0 {
		t.Fatalf("coalesced copies = %#v, want none", got)
	}
}

func TestWorkingSetDestinationAcrossGuestRegions(t *testing.T) {
	const (
		page       = uint64(4096)
		threeGiB   = uint64(3 * 1024 * 1024 * 1024)
		oneGiB     = uint64(1024 * 1024 * 1024)
		firstHost  = uint64(0x1000000000)
		secondHost = uint64(0x2000000000)
	)
	regions := []GuestRegionUffdMapping{
		{BaseHostVirtAddr: firstHost, Size: threeGiB, Offset: 0, PageSize: page},
		{BaseHostVirtAddr: secondHost, Size: oneGiB, Offset: threeGiB, PageSize: page},
	}

	tests := []struct {
		name string
		pfn  uint64
		want uint64
		ok   bool
	}{
		{name: "first region", pfn: 7, want: firstHost + 7*page, ok: true},
		{name: "second region start", pfn: threeGiB / page, want: secondHost, ok: true},
		{name: "second region interior", pfn: threeGiB/page + 9, want: secondHost + 9*page, ok: true},
		{name: "past guest memory", pfn: (threeGiB + oneGiB) / page, ok: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := workingSetDestination(test.pfn, page, regions)
			if ok != test.ok || got != test.want {
				t.Fatalf("workingSetDestination(%d) = (%#x, %v), want (%#x, %v)", test.pfn, got, ok, test.want, test.ok)
			}
		})
	}
}
