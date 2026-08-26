package main

import (
	"reflect"
	"testing"
)

func TestSelectRevisionsForProcessing(t *testing.T) {
	revisions := map[string]bool{
		"snapshare-image-go-r2":     true,
		"snapshare-image-go-r1":     true,
		"snapshare-video-python-r1": true,
	}

	all := selectRevisionsForProcessing(revisions, true)
	wantAll := []string{
		"snapshare-image-go-r1",
		"snapshare-image-go-r2",
		"snapshare-video-python-r1",
	}
	if !reflect.DeepEqual(all, wantAll) {
		t.Fatalf("all revisions mismatch: got %v want %v", all, wantAll)
	}

	representatives := selectRevisionsForProcessing(revisions, false)
	wantRepresentatives := []string{
		"snapshare-image-go-r1",
		"snapshare-video-python-r1",
	}
	if !reflect.DeepEqual(representatives, wantRepresentatives) {
		t.Fatalf("representatives mismatch: got %v want %v", representatives, wantRepresentatives)
	}
}
