//go:build sabre

// planb_encode creates the two Plan B files for one already-packed private
// working set. It lets all codecs encode the exact same frozen input before a
// controlled restore comparison.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"

	"github.com/vhive-serverless/vhive/snapshotting/planb"
)

func main() {
	codecName := flag.String("codec", "", "gzip, sw_deflate, iaa_deflate, zstd_1, or zstd_3")
	inputPath := flag.String("input", "", "packed private working-set content")
	outputBase := flag.String("output-base", "", "output base; writes .snapshot and .partitions")
	partitionCount := flag.Uint("partitions", 1, "number of independently compressed working-set partitions")
	jobs := flag.Uint("jobs", 1, "maximum concurrent IAA jobs")
	flag.Parse()

	if *codecName == "" || *inputPath == "" || *outputBase == "" ||
		*partitionCount == 0 || uint64(*partitionCount) > uint64(^uint32(0)) ||
		*jobs == 0 || *jobs > 255 {
		flag.Usage()
		os.Exit(2)
	}
	codec, err := planb.ParseCodec(*codecName)
	if err != nil {
		fatal(err)
	}
	content, err := os.ReadFile(*inputPath)
	if err != nil {
		fatal(err)
	}
	if len(content) == 0 || len(content)%4096 != 0 {
		fatal(fmt.Errorf("input length %d is not a positive multiple of 4096", len(content)))
	}

	restorer, err := planb.Open(*outputBase, planb.Options{
		Codec:           codec,
		MaxHardwareJobs: uint8(*jobs),
		PartitionCount:  uint32(*partitionCount),
	})
	if err != nil {
		fatal(err)
	}
	defer restorer.Close()
	if err := restorer.Compress(content); err != nil {
		fatal(err)
	}

	payloadPath := *outputBase + ".snapshot"
	partitionsPath := *outputBase + ".partitions"
	payload, err := os.ReadFile(payloadPath)
	if err != nil {
		fatal(err)
	}
	partitions, err := os.ReadFile(partitionsPath)
	if err != nil {
		fatal(err)
	}
	inputSHA := sha256.Sum256(content)
	payloadSHA := sha256.Sum256(payload)
	partitionsSHA := sha256.Sum256(partitions)
	fmt.Printf(
		"PLANB_ENCODED codec=%s partitions=%d jobs=%d input_bytes=%d payload_bytes=%d ratio=%.6f input_sha256=%s payload_sha256=%s partitions_sha256=%s output_base=%s\n",
		codec.String(), *partitionCount, *jobs, len(content), len(payload), float64(len(content))/float64(len(payload)),
		hex.EncodeToString(inputSHA[:]), hex.EncodeToString(payloadSHA[:]),
		hex.EncodeToString(partitionsSHA[:]), *outputBase,
	)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "planb_encode:", err)
	os.Exit(1)
}
