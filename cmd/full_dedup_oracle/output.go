package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func writeAnalysis(outputDir string, result analysisResult) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := writePagesCSV(filepath.Join(outputDir, "pages.csv"), result.Inventories); err != nil {
		return err
	}
	if err := writeValidationCSV(filepath.Join(outputDir, "validation.csv"), result.Inventories); err != nil {
		return err
	}
	if err := writeInputsCSV(filepath.Join(outputDir, "inputs.csv"), result.Inventories); err != nil {
		return err
	}
	if err := writeSummaryCSV(filepath.Join(outputDir, "summary.csv"), result.Summaries); err != nil {
		return err
	}
	if err := writeTraceCSV(filepath.Join(outputDir, "trace.csv"), result.Trace); err != nil {
		return err
	}
	manifest := struct {
		GeneratedAt string       `json:"generated_at"`
		Method      string       `json:"method"`
		Config      corpusConfig `json:"config"`
	}{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Method:      "logical 4-KiB page-content oracle; current and no-image isolate non-shareable hashes per revision; full-dedup globally unions raw hashes",
		Config:      result.Config,
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(outputDir, "manifest.json"), data, 0644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

func writePagesCSV(path string, inventories []snapshotInventory) error {
	return writeCSV(path,
		[]string{"snapshot_id", "workload", "occurrence", "pfn", "raw_hash", "provenance"},
		func(writer *csv.Writer) error {
			for _, inventory := range inventories {
				for _, page := range inventory.Pages {
					if err := writer.Write([]string{
						page.SnapshotID,
						page.Workload,
						strconv.Itoa(page.Occurrence),
						strconv.FormatUint(page.PFN, 10),
						page.RawHash,
						string(page.Provenance),
					}); err != nil {
						return err
					}
				}
			}
			return nil
		})
}

func writeValidationCSV(path string, inventories []snapshotInventory) error {
	return writeCSV(path,
		[]string{"snapshot_id", "workload", "ws_rows", "unique_pfns", "duplicate_pfns", "private_rows", "base_rootfs_rows", "image_rows", "unique_raw_pages", "recipe_pages", "private_file_bytes", "base_file_bytes", "image_file_bytes"},
		func(writer *csv.Writer) error {
			for _, inventory := range inventories {
				row := inventory.Validation
				if err := writer.Write([]string{
					row.SnapshotID,
					row.Workload,
					strconv.Itoa(row.WSRows),
					strconv.Itoa(row.UniquePFNs),
					strconv.Itoa(row.DuplicatePFNs),
					strconv.Itoa(row.PrivateRows),
					strconv.Itoa(row.BaseRootfsRows),
					strconv.Itoa(row.ImageRows),
					strconv.Itoa(row.UniqueRawPages),
					strconv.Itoa(row.RecipePages),
					strconv.FormatInt(row.PrivateFileBytes, 10),
					strconv.FormatInt(row.BaseFileBytes, 10),
					strconv.FormatInt(row.ImageFileBytes, 10),
				}); err != nil {
					return err
				}
			}
			return nil
		})
}

func writeInputsCSV(path string, inventories []snapshotInventory) error {
	return writeCSV(path,
		[]string{"snapshot_id", "role", "path", "bytes", "sha256"},
		func(writer *csv.Writer) error {
			for _, inventory := range inventories {
				for _, input := range inventory.Inputs {
					if err := writer.Write([]string{input.SnapshotID, input.Role, input.Path, strconv.FormatInt(input.Bytes, 10), input.SHA256}); err != nil {
						return err
					}
				}
			}
			return nil
		})
}

func writeSummaryCSV(path string, summaries []summaryRecord) error {
	return writeCSV(path,
		[]string{"policy", "page_size", "snapshot_count", "page_occurrences", "logical_unique_pages", "logical_unique_bytes", "delta_pages_vs_current", "delta_bytes_vs_current", "delta_percent_vs_current", "cache_capacity_pages", "fetch_pages", "fetch_bytes", "cache_hits", "cache_misses"},
		func(writer *csv.Writer) error {
			for _, row := range summaries {
				if err := writer.Write([]string{
					string(row.Policy),
					strconv.Itoa(row.PageSize),
					strconv.Itoa(row.SnapshotCount),
					strconv.Itoa(row.PageOccurrences),
					strconv.Itoa(row.LogicalUniquePages),
					strconv.FormatInt(row.LogicalUniqueBytes, 10),
					strconv.Itoa(row.DeltaPagesVsCurrent),
					strconv.FormatInt(row.DeltaBytesVsCurrent, 10),
					strconv.FormatFloat(row.DeltaPercentVsCurrent, 'f', 6, 64),
					strconv.Itoa(row.CacheCapacityPages),
					strconv.Itoa(row.FetchPages),
					strconv.FormatInt(row.FetchBytes, 10),
					strconv.Itoa(row.CacheHits),
					strconv.Itoa(row.CacheMisses),
				}); err != nil {
					return err
				}
			}
			return nil
		})
}

func writeTraceCSV(path string, trace []traceRecord) error {
	return writeCSV(path,
		[]string{"policy", "restore_index", "snapshot_id", "workload", "logical_unique_pages", "fetch_pages", "fetch_bytes", "cache_hits", "cache_misses", "resident_pages_after"},
		func(writer *csv.Writer) error {
			for _, row := range trace {
				if err := writer.Write([]string{
					string(row.Policy),
					strconv.Itoa(row.RestoreIndex),
					row.SnapshotID,
					row.Workload,
					strconv.Itoa(row.LogicalUniquePages),
					strconv.Itoa(row.FetchPages),
					strconv.FormatInt(row.FetchBytes, 10),
					strconv.Itoa(row.CacheHits),
					strconv.Itoa(row.CacheMisses),
					strconv.Itoa(row.ResidentPagesAfter),
				}); err != nil {
					return err
				}
			}
			return nil
		})
}

func writeCSV(path string, header []string, writeRows func(*csv.Writer) error) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("write %s header: %w", path, err)
	}
	if err := writeRows(writer); err != nil {
		return fmt.Errorf("write %s rows: %w", path, err)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush %s: %w", path, err)
	}
	return nil
}
