package main

import (
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

const pageSize = 4096

type exportSummary struct {
	Pages      int
	Partitions int
	Bytes      int64
}

func readPFNs(path string) ([]uint64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 || len(records[0]) == 0 || records[0][0] != "pfn" {
		return nil, errors.New("working-set index must start with a pfn header")
	}

	seen := make(map[uint64]struct{}, len(records)-1)
	pfns := make([]uint64, 0, len(records)-1)
	for line, record := range records[1:] {
		if len(record) == 0 || record[0] == "" {
			continue
		}
		pfn, parseErr := strconv.ParseUint(record[0], 10, 64)
		if parseErr != nil {
			return nil, fmt.Errorf("parse PFN on CSV line %d: %w", line+2, parseErr)
		}
		if _, ok := seen[pfn]; ok {
			continue
		}
		seen[pfn] = struct{}{}
		pfns = append(pfns, pfn)
	}
	if len(pfns) == 0 {
		return nil, errors.New("working-set index contains no PFNs")
	}
	sort.Slice(pfns, func(i, j int) bool { return pfns[i] < pfns[j] })
	return pfns, nil
}

func createOutput(path string, force bool) (*os.File, string, error) {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return nil, "", fmt.Errorf("output already exists: %s", path)
		} else if !os.IsNotExist(err) {
			return nil, "", err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".planb-ws-export-*")
	if err != nil {
		return nil, "", err
	}
	return tmp, tmp.Name(), nil
}

func exportPrivateWorkingSet(memoryPath, indexPath, contentOut, indexOut string, force bool) (_ exportSummary, retErr error) {
	if contentOut == indexOut || contentOut == memoryPath || indexOut == memoryPath ||
		contentOut == indexPath || indexOut == indexPath {
		return exportSummary{}, errors.New("input and output paths must be distinct")
	}

	pfns, err := readPFNs(indexPath)
	if err != nil {
		return exportSummary{}, fmt.Errorf("read working-set index: %w", err)
	}
	memory, err := os.Open(memoryPath)
	if err != nil {
		return exportSummary{}, fmt.Errorf("open guest memory: %w", err)
	}
	defer memory.Close()
	info, err := memory.Stat()
	if err != nil {
		return exportSummary{}, fmt.Errorf("stat guest memory: %w", err)
	}
	if info.Size() < pageSize {
		return exportSummary{}, fmt.Errorf("guest memory is only %d bytes", info.Size())
	}
	memoryPages := uint64(info.Size()) / pageSize
	for _, pfn := range pfns {
		if pfn >= memoryPages {
			return exportSummary{}, fmt.Errorf("PFN %d is outside %d-byte guest memory", pfn, info.Size())
		}
	}

	content, contentTmp, err := createOutput(contentOut, force)
	if err != nil {
		return exportSummary{}, err
	}
	defer func() {
		content.Close()
		if retErr != nil {
			os.Remove(contentTmp)
		}
	}()
	index, indexTmp, err := createOutput(indexOut, force)
	if err != nil {
		return exportSummary{}, err
	}
	defer func() {
		index.Close()
		if retErr != nil {
			os.Remove(indexTmp)
		}
	}()

	page := make([]byte, pageSize)
	writer := csv.NewWriter(index)
	if err := writer.Write([]string{"pfn"}); err != nil {
		return exportSummary{}, err
	}
	partitions := 1
	for i, pfn := range pfns {
		if _, err := memory.ReadAt(page, int64(pfn*pageSize)); err != nil && err != io.EOF {
			return exportSummary{}, fmt.Errorf("read PFN %d: %w", pfn, err)
		}
		if _, err := content.Write(page); err != nil {
			return exportSummary{}, fmt.Errorf("write PFN %d: %w", pfn, err)
		}
		if err := writer.Write([]string{strconv.FormatUint(pfn, 10)}); err != nil {
			return exportSummary{}, err
		}
		if i > 0 && pfn != pfns[i-1]+1 {
			partitions++
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return exportSummary{}, err
	}
	if err := content.Sync(); err != nil {
		return exportSummary{}, err
	}
	if err := index.Sync(); err != nil {
		return exportSummary{}, err
	}
	if err := content.Close(); err != nil {
		return exportSummary{}, err
	}
	if err := index.Close(); err != nil {
		return exportSummary{}, err
	}
	if err := os.Chmod(contentTmp, 0o644); err != nil {
		return exportSummary{}, err
	}
	if err := os.Chmod(indexTmp, 0o644); err != nil {
		return exportSummary{}, err
	}
	if err := os.Rename(contentTmp, contentOut); err != nil {
		return exportSummary{}, err
	}
	if err := os.Rename(indexTmp, indexOut); err != nil {
		return exportSummary{}, err
	}

	return exportSummary{Pages: len(pfns), Partitions: partitions, Bytes: int64(len(pfns) * pageSize)}, nil
}

func main() {
	memory := flag.String("memory", "", "path to the full guest memory file")
	workingSet := flag.String("working-set", "", "path to working_set_pages CSV")
	contentOut := flag.String("content-out", "", "private working-set content output")
	indexOut := flag.String("index-out", "", "private working-set PFN index output")
	force := flag.Bool("force", false, "replace existing output files")
	flag.Parse()

	if *memory == "" || *workingSet == "" || *contentOut == "" || *indexOut == "" {
		flag.Usage()
		os.Exit(2)
	}
	summary, err := exportPrivateWorkingSet(*memory, *workingSet, *contentOut, *indexOut, *force)
	if err != nil {
		fmt.Fprintln(os.Stderr, "planb_ws_export:", err)
		os.Exit(1)
	}
	fmt.Printf("PLANB_WS_EXPORT pages=%d partitions=%d bytes=%d content=%s index=%s\n",
		summary.Pages, summary.Partitions, summary.Bytes, *contentOut, *indexOut)
}
