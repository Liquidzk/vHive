package main

import (
	"os"
	"path/filepath"
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

func TestLoadRevisionAllowlist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "revisions.txt")
	if err := os.WriteFile(path, []byte("# tier\naes-go-1\nvideo-processing-2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := loadRevisionAllowlist(path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"aes-go-1": true, "video-processing-2": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("allowlist mismatch: got %v want %v", got, want)
	}

	for _, contents := range []string{"", "revision\nrevision\n", "bad/revision\n"} {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadRevisionAllowlist(path); err == nil {
			t.Fatalf("loadRevisionAllowlist(%q) unexpectedly succeeded", contents)
		}
	}
}
