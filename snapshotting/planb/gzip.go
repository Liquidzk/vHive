package planb

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"math"
	"os"
	"time"
)

// The original SplitSnap prototype used Go's gzip.NewWriter with its default
// compression level. Keep that exact codec choice here so the Plan B A/B can
// compare the historical software-compression baseline with IAA and zstd on
// the same packed private working set.
const gzipMetadata = "snapshare-planb-gzip-v1\n"

type gzipRestorer struct {
	path        string
	lastMetrics Metrics
}

func openGzipImpl(path string) (restorerImpl, error) {
	return &gzipRestorer{path: path}, nil
}

func (r *gzipRestorer) compress(content []byte) (retErr error) {
	payloadPath := r.path + ".snapshot"
	payload, err := os.Create(payloadPath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := payload.Close(); retErr == nil && closeErr != nil {
			retErr = closeErr
		}
	}()

	writer := gzip.NewWriter(payload)
	if _, err := writer.Write(content); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := payload.Sync(); err != nil {
		return err
	}
	if err := os.Chmod(payloadPath, 0o644); err != nil {
		return err
	}
	return os.WriteFile(r.path+".partitions", []byte(gzipMetadata), 0o644)
}

func (r *gzipRestorer) decompress(expectedBytes uint64) (*Region, error) {
	if expectedBytes > math.MaxInt64 {
		return nil, fmt.Errorf("gzip output is too large: %d bytes", expectedBytes)
	}

	totalStart := time.Now()
	metadata, err := os.ReadFile(r.path + ".partitions")
	if err != nil {
		return nil, err
	}
	if string(metadata) != gzipMetadata {
		return nil, fmt.Errorf("unsupported gzip partition metadata %q", metadata)
	}
	payload, err := os.ReadFile(r.path + ".snapshot")
	if err != nil {
		return nil, err
	}

	decompressStart := time.Now()
	reader, err := gzip.NewReader(bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	decoded, readErr := io.ReadAll(io.LimitReader(reader, int64(expectedBytes)+1))
	closeErr := reader.Close()
	decompressUS := time.Since(decompressStart).Microseconds()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if uint64(len(decoded)) != expectedBytes {
		return nil, fmt.Errorf("gzip output size is %d bytes, expected %d", len(decoded), expectedBytes)
	}

	r.lastMetrics = Metrics{
		Decompress:      decompressUS,
		MemRestoreTotal: time.Since(totalStart).Microseconds(),
	}
	return newRegion(decoded, nil), nil
}

func (r *gzipRestorer) metrics() Metrics { return r.lastMetrics }

func (r *gzipRestorer) close() {}
