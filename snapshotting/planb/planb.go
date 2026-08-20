// Package planb provides the optional buffered private-working-set codec used
// by SnapShare. Normal vHive builds get a stub; builds tagged "sabre" bind to
// Sabre/QPL and can select Intel IAA explicitly.
package planb

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
)

var ErrUnavailable = errors.New("Plan B codec is unavailable; rebuild with -tags sabre")

type Codec uint8

const (
	CodecSoftwareDeflate Codec = iota
	CodecIAADeflate
	CodecZstd1
	CodecZstd3
)

func ParseCodec(value string) (Codec, error) {
	switch value {
	case "sw_deflate":
		return CodecSoftwareDeflate, nil
	case "iaa_deflate":
		return CodecIAADeflate, nil
	case "zstd_1":
		return CodecZstd1, nil
	case "zstd_3":
		return CodecZstd3, nil
	default:
		return 0, fmt.Errorf("unsupported Plan B codec %q", value)
	}
}

func (c Codec) String() string {
	switch c {
	case CodecSoftwareDeflate:
		return "sw_deflate"
	case CodecIAADeflate:
		return "iaa_deflate"
	case CodecZstd1:
		return "zstd_1"
	case CodecZstd3:
		return "zstd_3"
	default:
		return fmt.Sprintf("unknown_%d", c)
	}
}

type Options struct {
	Codec           Codec
	MaxHardwareJobs uint8
	PartitionCount  uint32
	StaticHuffman   bool
}

type Metrics struct {
	GetPartitionInfo      int64
	MmapSnapshot          int64
	MmapDecompressionBuff int64
	Decompress            int64
	MemRestoreTotal       int64
}

type restorerImpl interface {
	compress([]byte) error
	decompress(uint64) (*Region, error)
	metrics() Metrics
	close()
}

type Restorer struct {
	impl restorerImpl
}

func Open(path string, opts Options) (*Restorer, error) {
	if path == "" {
		return nil, errors.New("Plan B snapshot path is empty")
	}
	if opts.MaxHardwareJobs == 0 {
		opts.MaxHardwareJobs = 1
	}
	if opts.PartitionCount == 0 {
		opts.PartitionCount = 1
	}
	impl, err := openImpl(path, opts)
	if err != nil {
		return nil, err
	}
	r := &Restorer{impl: impl}
	runtime.SetFinalizer(r, (*Restorer).Close)
	return r, nil
}

// Compress writes one or more independently decodable streams. Guest PFNs
// remain in SnapShare's separate private index; only the packed private bytes
// and their page-aligned codec partitions enter this layer.
func (r *Restorer) Compress(content []byte) error {
	if r == nil || r.impl == nil {
		return errors.New("Plan B restorer is closed")
	}
	if len(content) == 0 {
		return errors.New("Plan B private working set is empty")
	}
	return r.impl.compress(content)
}

func (r *Restorer) Decompress(expectedBytes uint64) (*Region, error) {
	if r == nil || r.impl == nil {
		return nil, errors.New("Plan B restorer is closed")
	}
	if expectedBytes == 0 {
		return nil, errors.New("Plan B expected size is zero")
	}
	return r.impl.decompress(expectedBytes)
}

func (r *Restorer) Metrics() Metrics {
	if r == nil || r.impl == nil {
		return Metrics{}
	}
	return r.impl.metrics()
}

func (r *Restorer) Close() {
	if r != nil && r.impl != nil {
		r.impl.close()
		r.impl = nil
		runtime.SetFinalizer(r, nil)
	}
}

// Region is an mmap-owned decompression buffer. Bytes stays valid until Free.
type Region struct {
	bytes []byte
	free  func()
	once  sync.Once
}

func newRegion(bytes []byte, free func()) *Region {
	r := &Region{bytes: bytes, free: free}
	runtime.SetFinalizer(r, (*Region).Free)
	return r
}

func (r *Region) Bytes() []byte {
	if r == nil {
		return nil
	}
	return r.bytes
}

func (r *Region) Free() {
	if r == nil {
		return
	}
	r.once.Do(func() {
		if r.free != nil {
			r.free()
		}
		r.bytes = nil
		r.free = nil
		runtime.SetFinalizer(r, nil)
	})
}
