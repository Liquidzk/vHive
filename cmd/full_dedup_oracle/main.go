package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	configPath := flag.String("config", "", "corpus JSON configuration")
	outputDir := flag.String("output", "", "output directory for CSV and provenance files")
	flag.Parse()

	if *configPath == "" || *outputDir == "" {
		flag.Usage()
		os.Exit(2)
	}

	cfg, err := loadCorpusConfig(*configPath)
	if err != nil {
		fatal(err)
	}
	result, err := analyzeCorpus(cfg)
	if err != nil {
		fatal(err)
	}
	if err := writeAnalysis(*outputDir, result); err != nil {
		fatal(err)
	}

	for _, summary := range result.Summaries {
		fmt.Printf("policy=%s logical_unique_pages=%d logical_unique_bytes=%d delta_vs_current=%.3f%% fetch_pages=%d cache_hits=%d\n",
			summary.Policy,
			summary.LogicalUniquePages,
			summary.LogicalUniqueBytes,
			summary.DeltaPercentVsCurrent,
			summary.FetchPages,
			summary.CacheHits,
		)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "full_dedup_oracle:", err)
	os.Exit(1)
}
