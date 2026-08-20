//go:build sabre

package planb

/*
#cgo CXXFLAGS: -std=c++20
#cgo LDFLAGS: -lstdc++ -lm
#include <stdlib.h>
#include "sabre_wrapper.h"
*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"
)

type sabreRestorer struct {
	ctx            *C.snapshare_sabre_ctx
	path           string
	partitionCount uint32
}

func Available() bool { return true }

func openImpl(path string, opts Options) (restorerImpl, error) {
	if opts.Codec == CodecGzip {
		return openGzipImpl(path)
	}
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	codec, err := codecToC(opts.Codec)
	if err != nil {
		return nil, err
	}
	jobs := opts.MaxHardwareJobs
	if jobs == 0 {
		jobs = 1
	}
	ctx := C.snapshare_sabre_open(cPath, codec, C.uint8_t(jobs), boolToInt(opts.StaticHuffman))
	if ctx == nil {
		return nil, fmt.Errorf("open Plan B snapshot %q with codec %s: IAA requires a configured work queue and relay privileges", path, opts.Codec)
	}
	return &sabreRestorer{ctx: ctx, path: path, partitionCount: opts.PartitionCount}, nil
}

func (r *sabreRestorer) compress(content []byte) error {
	parts, err := buildPartitions(len(content), r.partitionCount)
	if err != nil {
		return fmt.Errorf("partition Plan B snapshot %q: %w", r.path, err)
	}
	if C.snapshare_sabre_compress(
		r.ctx,
		(*C.uint8_t)(unsafe.Pointer(&content[0])),
		&parts[0],
		C.size_t(len(parts)),
	) != 0 {
		return fmt.Errorf("compress Plan B snapshot %q", r.path)
	}
	runtime.KeepAlive(content)
	runtime.KeepAlive(parts)
	return nil
}

func buildPartitions(contentBytes int, count uint32) ([]C.snapshare_partition, error) {
	const pageSize = 4096
	if contentBytes <= 0 {
		return nil, fmt.Errorf("content is empty")
	}
	if count == 0 {
		return nil, fmt.Errorf("partition count is zero")
	}
	if count == 1 {
		return []C.snapshare_partition{{offset: 0, size: C.uint64_t(contentBytes)}}, nil
	}
	if contentBytes%pageSize != 0 {
		return nil, fmt.Errorf("%d bytes is not page-aligned", contentBytes)
	}

	pages := uint64(contentBytes / pageSize)
	if uint64(count) > pages {
		return nil, fmt.Errorf("%d partitions exceed %d working-set pages", count, pages)
	}
	pagesPerPart := pages / uint64(count)
	remainderPages := pages % uint64(count)
	parts := make([]C.snapshare_partition, int(count))
	offset := uint64(0)
	for i := range parts {
		partPages := pagesPerPart
		if i == len(parts)-1 {
			partPages += remainderPages
		}
		partBytes := partPages * pageSize
		parts[i] = C.snapshare_partition{
			offset: C.uint64_t(offset),
			size:   C.uint64_t(partBytes),
		}
		offset += partBytes
	}
	return parts, nil
}

func (r *sabreRestorer) decompress(expectedBytes uint64) (*Region, error) {
	var out *C.uint8_t
	if C.snapshare_sabre_decompress(r.ctx, C.size_t(expectedBytes), &out) != 0 {
		return nil, fmt.Errorf("decompress Plan B snapshot %q", r.path)
	}
	bytes := unsafe.Slice((*byte)(unsafe.Pointer(out)), int(expectedBytes))
	return newRegion(bytes, func() {
		C.snapshare_sabre_free_region(out, C.size_t(expectedBytes))
	}), nil
}

func (r *sabreRestorer) metrics() Metrics {
	var metrics C.snapshare_metrics
	C.snapshare_sabre_metrics(r.ctx, &metrics)
	return Metrics{
		GetPartitionInfo:      int64(metrics.get_partition_info),
		MmapSnapshot:          int64(metrics.mmap_snapshot),
		MmapDecompressionBuff: int64(metrics.mmap_decompression_buff),
		Decompress:            int64(metrics.decompress),
		MemRestoreTotal:       int64(metrics.mem_restore_total),
	}
}

func (r *sabreRestorer) close() {
	if r.ctx != nil {
		C.snapshare_sabre_close(r.ctx)
		r.ctx = nil
	}
}

func codecToC(codec Codec) (C.snapshare_codec, error) {
	switch codec {
	case CodecSoftwareDeflate:
		return C.SNAPSHARE_CODEC_SW_DEFLATE, nil
	case CodecIAADeflate:
		return C.SNAPSHARE_CODEC_IAA_DEFLATE, nil
	case CodecZstd1:
		return C.SNAPSHARE_CODEC_ZSTD_1, nil
	case CodecZstd3:
		return C.SNAPSHARE_CODEC_ZSTD_3, nil
	default:
		return 0, fmt.Errorf("unsupported Plan B codec %d", codec)
	}
}

func boolToInt(value bool) C.int {
	if value {
		return 1
	}
	return 0
}
