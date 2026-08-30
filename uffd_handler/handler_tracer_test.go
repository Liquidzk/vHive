package uffd_handler

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"sync"
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestPageFaultTracerWritesEachPFNOnceUnderConcurrency(t *testing.T) {
	tracePath := filepath.Join(t.TempDir(), "working_set_pages")
	tracer, err := NewPageFaultTracer(tracePath, log.New().WithField("test", t.Name()))
	if err != nil {
		t.Fatalf("create page fault tracer: %v", err)
	}

	const goroutines = 16
	const uniquePFNs = 128
	var wg sync.WaitGroup
	for worker := 0; worker < goroutines; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pfn := uint64(0); pfn < uniquePFNs; pfn++ {
				tracer.TracePageFault(pfn*4096, pfn, "pagefault", false, true)
			}
		}()
	}
	wg.Wait()
	if err := tracer.Close(); err != nil {
		t.Fatalf("close page fault tracer: %v", err)
	}

	file, err := os.Open(tracePath)
	if err != nil {
		t.Fatalf("open trace: %v", err)
	}
	defer file.Close()
	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("parse trace: %v", err)
	}
	if len(records) != uniquePFNs+1 {
		t.Fatalf("trace rows = %d, want %d", len(records), uniquePFNs+1)
	}
	if len(records[0]) != 1 || records[0][0] != "pfn" {
		t.Fatalf("unexpected trace header: %v", records[0])
	}
	seen := make(map[string]struct{}, uniquePFNs)
	for _, record := range records[1:] {
		if len(record) != 1 {
			t.Fatalf("unexpected trace record: %v", record)
		}
		if _, duplicate := seen[record[0]]; duplicate {
			t.Fatalf("duplicate PFN %s", record[0])
		}
		seen[record[0]] = struct{}{}
	}
}
