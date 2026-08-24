package main

import (
	"crypto/md5"
	"encoding/csv"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestEvaluatePolicies(t *testing.T) {
	page := func(snapshot, workload, hash string, source provenance) pageRecord {
		return pageRecord{SnapshotID: snapshot, Workload: workload, RawHash: hash, Provenance: source}
	}
	inventories := []snapshotInventory{
		{
			Spec: snapshotSpec{ID: "rev-1", Workload: "fixture"},
			Pages: []pageRecord{
				page("rev-1", "fixture", "a", provenanceBaseRootfs),
				page("rev-1", "fixture", "b", provenanceImage),
				page("rev-1", "fixture", "c", provenancePrivate),
				page("rev-1", "fixture", "d", provenancePrivate),
				page("rev-1", "fixture", "c", provenancePrivate),
			},
		},
		{
			Spec: snapshotSpec{ID: "rev-2", Workload: "fixture"},
			Pages: []pageRecord{
				page("rev-2", "fixture", "a", provenanceBaseRootfs),
				page("rev-2", "fixture", "b", provenanceImage),
				page("rev-2", "fixture", "c", provenancePrivate),
				page("rev-2", "fixture", "e", provenancePrivate),
			},
		},
	}

	summaries, trace := evaluatePolicies(inventories, defaultPageSize, 0)
	wantPages := map[policy]int{policyCurrent: 6, policyNoImage: 7, policyFull: 5}
	wantHits := map[policy]int{policyCurrent: 2, policyNoImage: 1, policyFull: 3}
	for _, summary := range summaries {
		if summary.LogicalUniquePages != wantPages[summary.Policy] {
			t.Fatalf("policy %s unique pages: got %d, want %d", summary.Policy, summary.LogicalUniquePages, wantPages[summary.Policy])
		}
		if summary.FetchPages != wantPages[summary.Policy] {
			t.Fatalf("policy %s fetch pages: got %d, want %d", summary.Policy, summary.FetchPages, wantPages[summary.Policy])
		}
		if summary.CacheHits != wantHits[summary.Policy] {
			t.Fatalf("policy %s cache hits: got %d, want %d", summary.Policy, summary.CacheHits, wantHits[summary.Policy])
		}
	}
	if len(trace) != len(policies)*len(inventories) {
		t.Fatalf("trace rows: got %d, want %d", len(trace), len(policies)*len(inventories))
	}
}

func TestEvaluatePoliciesFiniteLRU(t *testing.T) {
	inventories := []snapshotInventory{
		{
			Spec: snapshotSpec{ID: "rev-1", Workload: "fixture"},
			Pages: []pageRecord{
				{SnapshotID: "rev-1", RawHash: "a", Provenance: provenanceBaseRootfs},
				{SnapshotID: "rev-1", RawHash: "b", Provenance: provenanceBaseRootfs},
				{SnapshotID: "rev-1", RawHash: "c", Provenance: provenanceBaseRootfs},
			},
		},
		{
			Spec: snapshotSpec{ID: "rev-2", Workload: "fixture"},
			Pages: []pageRecord{
				{SnapshotID: "rev-2", RawHash: "a", Provenance: provenanceBaseRootfs},
				{SnapshotID: "rev-2", RawHash: "b", Provenance: provenanceBaseRootfs},
			},
		},
	}

	summaries, trace := evaluatePolicies(inventories, defaultPageSize, 2)
	for _, summary := range summaries {
		if summary.FetchPages != 5 || summary.CacheHits != 0 || summary.CacheMisses != 5 {
			t.Fatalf("policy %s finite-cache totals: got fetch=%d hits=%d misses=%d, want 5/0/5",
				summary.Policy, summary.FetchPages, summary.CacheHits, summary.CacheMisses)
		}
	}
	for _, row := range trace {
		if row.RestoreIndex == 1 && row.ResidentPagesAfter != 2 {
			t.Fatalf("policy %s second restore resident pages: got %d, want 2", row.Policy, row.ResidentPagesAfter)
		}
	}
}

func TestLoadSnapshotInventory(t *testing.T) {
	root := t.TempDir()
	revision := "fixture-revision"
	revisionDir := filepath.Join(root, revision)
	sharedRoot := filepath.Join(root, "ws_shared")
	imageDir := filepath.Join(sharedRoot, "images")
	for _, dir := range []string{revisionDir, sharedRoot, imageDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	basePage := filledPage(0x11)
	imagePage := filledPage(0x22)
	privatePage := filledPage(0x33)
	baseHash := md5.Sum(basePage)
	imageHash := md5.Sum(imagePage)
	privateHash := md5.Sum(privatePage)

	writeHashSource(t, filepath.Join(sharedRoot, "base_rootfs_index"), filepath.Join(sharedRoot, "base_rootfs_content"), [][]byte{basePage})
	writeHashSource(t, filepath.Join(imageDir, "fixture-image_index"), filepath.Join(imageDir, "fixture-image_content"), [][]byte{imagePage})
	writeOneColumnCSV(t, filepath.Join(revisionDir, "working_set_pages"), "pfn", []string{"0", "1", "2"})
	writeOneColumnCSV(t, filepath.Join(revisionDir, "working_set_pages_index_private"), "pfn", []string{"2"})
	if err := os.WriteFile(filepath.Join(revisionDir, "working_set_pages_content_private"), privatePage, 0644); err != nil {
		t.Fatal(err)
	}

	recipe := make([]byte, 3*md5.Size)
	copy(recipe[0*md5.Size:], baseHash[:])
	copy(recipe[1*md5.Size:], imageHash[:])
	privateSalted := md5.Sum(append(privateHash[:], []byte(revision)...))
	copy(recipe[2*md5.Size:], privateSalted[:])
	if err := os.WriteFile(filepath.Join(revisionDir, "recipe_file"), recipe, 0644); err != nil {
		t.Fatal(err)
	}

	inventory, err := loadSnapshotInventory(snapshotSpec{
		ID:          revision,
		Workload:    "fixture",
		RevisionDir: revisionDir,
		SharedRoot:  sharedRoot,
		Image:       "fixture-image",
	}, defaultPageSize)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Pages) != 3 {
		t.Fatalf("page rows: got %d, want 3", len(inventory.Pages))
	}
	wantProvenance := []provenance{provenanceBaseRootfs, provenanceImage, provenancePrivate}
	wantHashes := []string{hex.EncodeToString(baseHash[:]), hex.EncodeToString(imageHash[:]), hex.EncodeToString(privateHash[:])}
	for i, page := range inventory.Pages {
		if page.Provenance != wantProvenance[i] {
			t.Fatalf("page %d provenance: got %s, want %s", i, page.Provenance, wantProvenance[i])
		}
		if page.RawHash != wantHashes[i] {
			t.Fatalf("page %d hash: got %s, want %s", i, page.RawHash, wantHashes[i])
		}
	}
	if inventory.Validation.PrivateRows != 1 || inventory.Validation.BaseRootfsRows != 1 || inventory.Validation.ImageRows != 1 {
		t.Fatalf("unexpected validation counts: %+v", inventory.Validation)
	}
}

func TestLoadSnapshotInventoryRejectsWrongPrivateSalt(t *testing.T) {
	root := t.TempDir()
	revisionDir := filepath.Join(root, "rev")
	sharedRoot := filepath.Join(root, "ws_shared")
	if err := os.MkdirAll(revisionDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sharedRoot, 0755); err != nil {
		t.Fatal(err)
	}
	writeOneColumnCSV(t, filepath.Join(revisionDir, "working_set_pages"), "pfn", []string{"0"})
	writeOneColumnCSV(t, filepath.Join(revisionDir, "working_set_pages_index_private"), "pfn", []string{"0"})
	if err := os.WriteFile(filepath.Join(revisionDir, "working_set_pages_content_private"), filledPage(0x44), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(revisionDir, "recipe_file"), make([]byte, md5.Size), 0644); err != nil {
		t.Fatal(err)
	}
	writeHashSource(t, filepath.Join(sharedRoot, "base_rootfs_index"), filepath.Join(sharedRoot, "base_rootfs_content"), nil)

	_, err := loadSnapshotInventory(snapshotSpec{ID: "rev", RevisionDir: revisionDir, SharedRoot: sharedRoot}, defaultPageSize)
	if err == nil {
		t.Fatal("expected private recipe salt validation to fail")
	}
}

func filledPage(value byte) []byte {
	page := make([]byte, defaultPageSize)
	for i := range page {
		page[i] = value
	}
	return page
}

func writeHashSource(t *testing.T, indexPath, contentPath string, pages [][]byte) {
	t.Helper()
	hashes := make([]string, 0, len(pages))
	content := make([]byte, 0, len(pages)*defaultPageSize)
	for _, page := range pages {
		sum := md5.Sum(page)
		hashes = append(hashes, hex.EncodeToString(sum[:]))
		content = append(content, page...)
	}
	writeOneColumnCSV(t, indexPath, "hash", hashes)
	if err := os.WriteFile(contentPath, content, 0644); err != nil {
		t.Fatal(err)
	}
}

func writeOneColumnCSV(t *testing.T, path, header string, values []string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := csv.NewWriter(file)
	if err := writer.Write([]string{header}); err != nil {
		t.Fatal(err)
	}
	for _, value := range values {
		if err := writer.Write([]string{value}); err != nil {
			t.Fatal(err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestResolveRelativeConfigPaths(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config.json")
	data := []byte(`{
  "page_size": 4096,
  "snapshots": [
    {"id":"one","revision_dir":"one"},
    {"id":"two","revision_dir":"two","shared_root":"shared-two"}
  ]
}`)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadCorpusConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Snapshots[0].RevisionDir != filepath.Join(root, "one") {
		t.Fatalf("revision path: got %s", cfg.Snapshots[0].RevisionDir)
	}
	if cfg.Snapshots[0].SharedRoot != filepath.Join(root, "ws_shared") {
		t.Fatalf("default shared root: got %s", cfg.Snapshots[0].SharedRoot)
	}
	if cfg.Snapshots[1].SharedRoot != filepath.Join(root, "shared-two") {
		t.Fatalf("explicit shared root: got %s", cfg.Snapshots[1].SharedRoot)
	}
}

func BenchmarkPolicyKey(b *testing.B) {
	page := pageRecord{SnapshotID: "rev", RawHash: strconv.FormatInt(123, 16), Provenance: provenancePrivate}
	for i := 0; i < b.N; i++ {
		_ = policyKey(policyFull, page)
	}
}
