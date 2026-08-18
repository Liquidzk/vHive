package main

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestExportPrivateWorkingSetSortsAndDeduplicates(t *testing.T) {
	dir := t.TempDir()
	memoryPath := filepath.Join(dir, "mem_file")
	indexPath := filepath.Join(dir, "working_set_pages")
	contentPath := filepath.Join(dir, "working_set_pages_content_private")
	privateIndexPath := filepath.Join(dir, "working_set_pages_index_private")

	memory := make([]byte, 6*pageSize)
	for pfn := 0; pfn < 6; pfn++ {
		for i := 0; i < pageSize; i++ {
			memory[pfn*pageSize+i] = byte(pfn)
		}
	}
	if err := os.WriteFile(memoryPath, memory, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexPath, []byte("pfn\n5\n2\n3\n2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	summary, err := exportPrivateWorkingSet(memoryPath, indexPath, contentPath, privateIndexPath, false)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Pages != 3 || summary.Partitions != 2 || summary.Bytes != 3*pageSize {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	content, err := os.ReadFile(contentPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := []byte{content[0], content[pageSize], content[2*pageSize]}; !reflect.DeepEqual(got, []byte{2, 3, 5}) {
		t.Fatalf("unexpected page order: %v", got)
	}
	index, err := os.Open(privateIndexPath)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	records, err := csv.NewReader(index).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"pfn"}, {"2"}, {"3"}, {"5"}}
	if !reflect.DeepEqual(records, want) {
		t.Fatalf("index = %v, want %v", records, want)
	}
}

func TestExportPrivateWorkingSetRejectsOutOfRangePFN(t *testing.T) {
	dir := t.TempDir()
	memoryPath := filepath.Join(dir, "mem_file")
	indexPath := filepath.Join(dir, "working_set_pages")
	if err := os.WriteFile(memoryPath, make([]byte, pageSize), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexPath, []byte("pfn\n1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := exportPrivateWorkingSet(
		memoryPath,
		indexPath,
		filepath.Join(dir, "content"),
		filepath.Join(dir, "index"),
		false,
	); err == nil {
		t.Fatal("expected out-of-range PFN error")
	}
}
