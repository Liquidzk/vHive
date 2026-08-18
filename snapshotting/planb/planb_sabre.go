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
	ctx  *C.snapshare_sabre_ctx
	path string
}

func Available() bool { return true }

func openImpl(path string, opts Options) (restorerImpl, error) {
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
	return &sabreRestorer{ctx: ctx, path: path}, nil
}

func (r *sabreRestorer) compress(content []byte) error {
	part := C.snapshare_partition{offset: 0, size: C.uint64_t(len(content))}
	if C.snapshare_sabre_compress(r.ctx, (*C.uint8_t)(unsafe.Pointer(&content[0])), &part, 1) != 0 {
		return fmt.Errorf("compress Plan B snapshot %q", r.path)
	}
	runtime.KeepAlive(content)
	return nil
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
