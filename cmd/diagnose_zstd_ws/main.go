// diagnose_zstd_ws separates object retrieval, streaming Zstandard decoding,
// destination writes, and checksum validation for one coalesced WS object.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/vhive-serverless/vhive/snapshotting/zstdstream"
	"github.com/vhive-serverless/vhive/storage"
	"golang.org/x/sys/unix"
)

type fixedSliceWriter struct {
	data []byte
	pos  int
}

func (writer *fixedSliceWriter) Write(data []byte) (int, error) {
	if len(data) > len(writer.data)-writer.pos {
		return 0, io.ErrShortBuffer
	}
	copy(writer.data[writer.pos:], data)
	writer.pos += len(data)
	return len(data), nil
}

type sample struct {
	Run                int    `json:"run"`
	Mode               string `json:"mode"`
	DecoderConcurrency int    `json:"decoder_concurrency,omitempty"`
	RangeWorkers       int    `json:"range_workers,omitempty"`
	CompressedBytes    int64  `json:"compressed_bytes"`
	RawBytes           int64  `json:"raw_bytes"`
	ManifestUS         int64  `json:"manifest_us,omitempty"`
	ParseUS            int64  `json:"parse_us,omitempty"`
	DestinationMmapUS  int64  `json:"destination_mmap_us,omitempty"`
	CacheAllocateUS    int64  `json:"cache_allocate_us,omitempty"`
	OpenRangeUS        int64  `json:"open_range_us,omitempty"`
	DecoderInitUS      int64  `json:"decoder_init_us,omitempty"`
	ReadOrDecodeUS     int64  `json:"read_or_decode_us"`
	FinishUS           int64  `json:"finish_us,omitempty"`
	ChecksumUS         int64  `json:"checksum_us,omitempty"`
	TotalUS            int64  `json:"total_us"`
}

func parseConcurrencies(value string) ([]int, error) {
	var result []int
	seen := make(map[int]bool)
	for _, field := range strings.Split(value, ",") {
		concurrency, err := strconv.Atoi(strings.TrimSpace(field))
		if err != nil || concurrency < 1 {
			return nil, fmt.Errorf("invalid decoder concurrency %q", field)
		}
		if !seen[concurrency] {
			seen[concurrency] = true
			result = append(result, concurrency)
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("decoder concurrency list is empty")
	}
	return result, nil
}

func loadManifest(ctx context.Context, store *storage.MinioStorage, key string) ([]byte, *zstdstream.Manifest, time.Duration, time.Duration, error) {
	started := time.Now()
	data, err := store.DownloadObject(key)
	manifestElapsed := time.Since(started)
	if err != nil {
		return nil, nil, 0, 0, err
	}
	started = time.Now()
	manifest, err := zstdstream.ParseManifest(data)
	return data, manifest, manifestElapsed, time.Since(started), err
}

func openPayload(ctx context.Context, store *storage.MinioStorage, key string, size int64) (io.ReadCloser, time.Duration, error) {
	started := time.Now()
	reader, err := store.OpenObjectRange(ctx, key, 0, size)
	return reader, time.Since(started), err
}

func fetchOnly(ctx context.Context, store *storage.MinioStorage, payloadKey string, manifest *zstdstream.Manifest, run int) ([]byte, sample, error) {
	totalStarted := time.Now()
	allocateStarted := time.Now()
	payload := make([]byte, manifest.CompressedSize)
	allocateElapsed := time.Since(allocateStarted)
	reader, openElapsed, err := openPayload(ctx, store, payloadKey, manifest.CompressedSize)
	if err != nil {
		return nil, sample{}, err
	}
	readStarted := time.Now()
	_, readErr := io.ReadFull(reader, payload)
	readElapsed := time.Since(readStarted)
	closeErr := reader.Close()
	if readErr != nil {
		return nil, sample{}, readErr
	}
	if closeErr != nil {
		return nil, sample{}, closeErr
	}
	return payload, sample{
		Run:             run,
		Mode:            "fetch-only",
		CompressedBytes: manifest.CompressedSize,
		RawBytes:        manifest.RawSize,
		CacheAllocateUS: allocateElapsed.Microseconds(),
		OpenRangeUS:     openElapsed.Microseconds(),
		ReadOrDecodeUS:  readElapsed.Microseconds(),
		TotalUS:         time.Since(totalStarted).Microseconds(),
	}, nil
}

func decodeFromReader(reader io.Reader, manifest *zstdstream.Manifest, concurrency int, destination []byte) (init, output, finish, checksum time.Duration, err error) {
	started := time.Now()
	decoder, err := zstd.NewReader(reader, zstd.WithDecoderConcurrency(concurrency))
	init = time.Since(started)
	if err != nil {
		return init, 0, 0, 0, err
	}
	defer decoder.Close()

	started = time.Now()
	_, err = io.ReadFull(decoder, destination)
	output = time.Since(started)
	if err != nil {
		return init, output, 0, 0, err
	}
	started = time.Now()
	var extra [1]byte
	n, finishErr := decoder.Read(extra[:])
	finish = time.Since(started)
	if finishErr != io.EOF || n != 0 {
		if finishErr == nil {
			finishErr = fmt.Errorf("decoded more than %d bytes", manifest.RawSize)
		}
		return init, output, finish, 0, finishErr
	}
	started = time.Now()
	sum := sha256.Sum256(destination)
	checksum = time.Since(started)
	if hex.EncodeToString(sum[:]) != manifest.Frames[0].RawSHA256 {
		return init, output, finish, checksum, fmt.Errorf("raw SHA-256 mismatch")
	}
	return init, output, finish, checksum, nil
}

func memoryDecode(payload []byte, manifest *zstdstream.Manifest, concurrency, run int) (sample, error) {
	totalStarted := time.Now()
	started := time.Now()
	destination, err := unix.Mmap(-1, 0, int(manifest.RawSize), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_PRIVATE|unix.MAP_ANON)
	mmapElapsed := time.Since(started)
	if err != nil {
		return sample{}, err
	}
	defer unix.Munmap(destination)
	init, output, finish, checksum, err := decodeFromReader(bytes.NewReader(payload), manifest, concurrency, destination)
	return sample{
		Run:                run,
		Mode:               "memory-decode",
		DecoderConcurrency: concurrency,
		CompressedBytes:    manifest.CompressedSize,
		RawBytes:           manifest.RawSize,
		DestinationMmapUS:  mmapElapsed.Microseconds(),
		DecoderInitUS:      init.Microseconds(),
		ReadOrDecodeUS:     output.Microseconds(),
		FinishUS:           finish.Microseconds(),
		ChecksumUS:         checksum.Microseconds(),
		TotalUS:            time.Since(totalStarted).Microseconds(),
	}, err
}

func streamDecode(ctx context.Context, store *storage.MinioStorage, payloadKey string, manifest *zstdstream.Manifest, concurrency, run int) (sample, error) {
	totalStarted := time.Now()
	started := time.Now()
	destination, err := unix.Mmap(-1, 0, int(manifest.RawSize), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_PRIVATE|unix.MAP_ANON)
	mmapElapsed := time.Since(started)
	if err != nil {
		return sample{}, err
	}
	defer unix.Munmap(destination)
	started = time.Now()
	compressedCache := make([]byte, manifest.CompressedSize)
	cacheElapsed := time.Since(started)
	reader, openElapsed, err := openPayload(ctx, store, payloadKey, manifest.CompressedSize)
	if err != nil {
		return sample{}, err
	}
	defer reader.Close()
	capture := &fixedSliceWriter{data: compressedCache}
	init, output, finish, checksum, err := decodeFromReader(io.TeeReader(reader, capture), manifest, concurrency, destination)
	return sample{
		Run:                run,
		Mode:               "stream-decode",
		DecoderConcurrency: concurrency,
		CompressedBytes:    manifest.CompressedSize,
		RawBytes:           manifest.RawSize,
		DestinationMmapUS:  mmapElapsed.Microseconds(),
		CacheAllocateUS:    cacheElapsed.Microseconds(),
		OpenRangeUS:        openElapsed.Microseconds(),
		DecoderInitUS:      init.Microseconds(),
		ReadOrDecodeUS:     output.Microseconds(),
		FinishUS:           finish.Microseconds(),
		ChecksumUS:         checksum.Microseconds(),
		TotalUS:            time.Since(totalStarted).Microseconds(),
	}, err
}

func fetchRanges(ctx context.Context, store *storage.MinioStorage, payloadKey string, manifest *zstdstream.Manifest, workers, run int) (sample, error) {
	if workers > len(manifest.Frames) {
		workers = len(manifest.Frames)
	}
	totalStarted := time.Now()
	started := time.Now()
	payload := make([]byte, manifest.CompressedSize)
	allocateElapsed := time.Since(started)
	jobs := make(chan zstdstream.Frame)
	errCh := make(chan error, 1)
	var wait sync.WaitGroup
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for frame := range jobs {
				reader, err := store.OpenObjectRange(ctx, payloadKey, frame.CompressedOffset, frame.CompressedSize)
				if err == nil {
					_, err = io.ReadFull(reader, payload[frame.CompressedOffset:frame.CompressedOffset+frame.CompressedSize])
					closeErr := reader.Close()
					if err == nil {
						err = closeErr
					}
				}
				if err != nil {
					select {
					case errCh <- err:
					default:
					}
					return
				}
			}
		}()
	}
	for _, frame := range manifest.Frames {
		jobs <- frame
	}
	close(jobs)
	wait.Wait()
	select {
	case err := <-errCh:
		return sample{}, err
	default:
	}
	return sample{
		Run:             run,
		Mode:            "framed-fetch-only",
		RangeWorkers:    workers,
		CompressedBytes: manifest.CompressedSize,
		RawBytes:        manifest.RawSize,
		CacheAllocateUS: allocateElapsed.Microseconds(),
		ReadOrDecodeUS:  time.Since(totalStarted).Microseconds(),
		TotalUS:         time.Since(totalStarted).Microseconds(),
	}, nil
}

func framedDecode(ctx context.Context, store *storage.MinioStorage, payloadKey string, payload []byte, manifest *zstdstream.Manifest, workers, run int, remote bool) (sample, error) {
	if workers > len(manifest.Frames) {
		workers = len(manifest.Frames)
	}
	totalStarted := time.Now()
	started := time.Now()
	destination, err := unix.Mmap(-1, 0, int(manifest.RawSize), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_PRIVATE|unix.MAP_ANON)
	mmapElapsed := time.Since(started)
	if err != nil {
		return sample{}, err
	}
	defer unix.Munmap(destination)
	var compressedCache []byte
	var cacheElapsed time.Duration
	if remote {
		started = time.Now()
		compressedCache = make([]byte, manifest.CompressedSize)
		cacheElapsed = time.Since(started)
	}
	open := func(offset, length int64) (io.ReadCloser, error) {
		if !remote {
			return io.NopCloser(bytes.NewReader(payload[offset : offset+length])), nil
		}
		reader, err := store.OpenObjectRange(ctx, payloadKey, offset, length)
		if err != nil {
			return nil, err
		}
		capture := &fixedSliceWriter{data: compressedCache[offset : offset+length]}
		return &struct {
			io.Reader
			io.Closer
		}{Reader: io.TeeReader(reader, capture), Closer: reader}, nil
	}
	started = time.Now()
	err = zstdstream.Decode(ctx, manifest, open, destination, workers)
	decodeElapsed := time.Since(started)
	mode := "framed-memory-decode"
	if remote {
		mode = "framed-stream-decode"
	}
	return sample{
		Run:               run,
		Mode:              mode,
		RangeWorkers:      workers,
		CompressedBytes:   manifest.CompressedSize,
		RawBytes:          manifest.RawSize,
		DestinationMmapUS: mmapElapsed.Microseconds(),
		CacheAllocateUS:   cacheElapsed.Microseconds(),
		ReadOrDecodeUS:    decodeElapsed.Microseconds(),
		TotalUS:           time.Since(totalStarted).Microseconds(),
	}, err
}

func main() {
	endpoint := flag.String("minioURL", "", "MinIO endpoint without scheme")
	accessKey := flag.String("minioAccessKey", "minio", "MinIO access key")
	secretKey := flag.String("minioSecretKey", "minio123", "MinIO secret key")
	bucket := flag.String("bucket", "snapshots", "snapshot bucket")
	prefix := flag.String("prefix", "", "snapshot/object stem without .zstd suffix")
	runs := flag.Int("runs", 10, "recorded iterations")
	warmup := flag.Int("warmup", 3, "unrecorded iterations")
	concurrencyList := flag.String("decoderConcurrency", "1,4,8", "comma-separated decoder concurrency values")
	flag.Parse()
	if *endpoint == "" || *prefix == "" || *runs < 1 || *warmup < 0 {
		fmt.Fprintln(os.Stderr, "minioURL, prefix, positive runs, and non-negative warmup are required")
		os.Exit(2)
	}
	concurrencies, err := parseConcurrencies(*concurrencyList)
	if err != nil {
		panic(err)
	}
	client, err := minio.New(*endpoint, &minio.Options{Creds: credentials.NewStaticV4(*accessKey, *secretKey, ""), Secure: false})
	if err != nil {
		panic(err)
	}
	store, err := storage.NewMinioStorage(client, *bucket)
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	manifestKey := *prefix + ".zstd.json"
	payloadKey := *prefix + ".zstd.frames"
	encoder := json.NewEncoder(os.Stdout)
	for iteration := -*warmup; iteration < *runs; iteration++ {
		run := iteration + 1
		_, manifest, manifestElapsed, parseElapsed, err := loadManifest(ctx, store, manifestKey)
		if err != nil {
			panic(err)
		}
		payload, fetchSample, err := fetchOnly(ctx, store, payloadKey, manifest, run)
		if err != nil {
			panic(err)
		}
		fetchSample.ManifestUS = manifestElapsed.Microseconds()
		fetchSample.ParseUS = parseElapsed.Microseconds()
		if iteration >= 0 {
			if err := encoder.Encode(fetchSample); err != nil {
				panic(err)
			}
		}
		for _, concurrency := range concurrencies {
			if len(manifest.Frames) > 1 {
				fetchRangesSample, err := fetchRanges(ctx, store, payloadKey, manifest, concurrency, run)
				if err != nil {
					panic(err)
				}
				memorySample, err := framedDecode(ctx, store, payloadKey, payload, manifest, concurrency, run, false)
				if err != nil {
					panic(err)
				}
				streamSample, err := framedDecode(ctx, store, payloadKey, payload, manifest, concurrency, run, true)
				if err != nil {
					panic(err)
				}
				if iteration >= 0 {
					for _, current := range []sample{fetchRangesSample, memorySample, streamSample} {
						if err := encoder.Encode(current); err != nil {
							panic(err)
						}
					}
				}
				continue
			}
			memorySample, err := memoryDecode(payload, manifest, concurrency, run)
			if err != nil {
				panic(err)
			}
			if iteration >= 0 {
				if err := encoder.Encode(memorySample); err != nil {
					panic(err)
				}
			}
			streamSample, err := streamDecode(ctx, store, payloadKey, manifest, concurrency, run)
			if err != nil {
				panic(err)
			}
			if iteration >= 0 {
				if err := encoder.Encode(streamSample); err != nil {
					panic(err)
				}
			}
		}
	}
}
