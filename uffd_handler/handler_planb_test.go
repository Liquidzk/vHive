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
