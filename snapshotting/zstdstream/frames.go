// Package zstdstream implements a seekable, independently framed Zstandard
// representation.  Each frame can be fetched and decoded without waiting for
// earlier frames, which lets SnapShare overlap parallel object-store reads
// with decompression while still reconstructing one contiguous byte range.
package zstdstream

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/klauspost/compress/zstd"
)

const (
	FormatVersion = 1
	CodecZstd     = "zstd"
)

// Frame describes one independent Zstandard frame inside the payload object.
// RawOffset is also the destination offset when the working set is restored.
type Frame struct {
	Index            int    `json:"index"`
	RawOffset        int64  `json:"raw_offset"`
	RawSize          int64  `json:"raw_size"`
	CompressedOffset int64  `json:"compressed_offset"`
	CompressedSize   int64  `json:"compressed_size"`
	RawSHA256        string `json:"raw_sha256"`
}

// Manifest is stored separately from the concatenated frame payload.  The
// explicit offsets allow range GETs to finish out of order safely.
type Manifest struct {
	Version        int     `json:"version"`
	Codec          string  `json:"codec"`
	Level          int     `json:"level"`
	FrameSize      int64   `json:"frame_size"`
	RawSize        int64   `json:"raw_size"`
	CompressedSize int64   `json:"compressed_size"`
	Frames         []Frame `json:"frames"`
}

// OpenRange opens exactly length compressed bytes starting at offset.  The
// returned reader may deliver the bytes incrementally from remote storage.
type OpenRange func(offset, length int64) (io.ReadCloser, error)

func MarshalManifest(manifest *Manifest) ([]byte, error) {
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	return json.MarshalIndent(manifest, "", "  ")
}

func ParseManifest(data []byte) (*Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("decode zstd frame manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func (manifest *Manifest) Validate() error {
	if manifest == nil {
		return fmt.Errorf("zstd frame manifest is nil")
	}
	if manifest.Version != FormatVersion {
		return fmt.Errorf("unsupported zstd frame manifest version %d", manifest.Version)
	}
	if manifest.Codec != CodecZstd {
		return fmt.Errorf("unsupported frame codec %q", manifest.Codec)
	}
	if manifest.FrameSize <= 0 {
		return fmt.Errorf("frame size must be positive")
	}
	if manifest.RawSize < 0 || manifest.CompressedSize < 0 {
		return fmt.Errorf("manifest sizes must be non-negative")
	}
	if manifest.RawSize == 0 {
		if manifest.CompressedSize != 0 || len(manifest.Frames) != 0 {
			return fmt.Errorf("empty payload must not contain frames")
		}
		return nil
	}
	if len(manifest.Frames) == 0 {
		return fmt.Errorf("non-empty payload has no frames")
	}

	var rawOffset int64
	var compressedOffset int64
	for i, frame := range manifest.Frames {
		if frame.Index != i {
			return fmt.Errorf("frame %d has index %d", i, frame.Index)
		}
		if frame.RawOffset != rawOffset {
			return fmt.Errorf("frame %d raw offset %d does not follow %d", i, frame.RawOffset, rawOffset)
		}
		if frame.CompressedOffset != compressedOffset {
			return fmt.Errorf("frame %d compressed offset %d does not follow %d", i, frame.CompressedOffset, compressedOffset)
		}
		if frame.RawSize <= 0 || frame.RawSize > manifest.FrameSize {
			return fmt.Errorf("frame %d has invalid raw size %d", i, frame.RawSize)
		}
		if frame.CompressedSize <= 0 {
			return fmt.Errorf("frame %d has invalid compressed size %d", i, frame.CompressedSize)
		}
		if len(frame.RawSHA256) != sha256.Size*2 {
			return fmt.Errorf("frame %d has invalid raw SHA-256 length", i)
		}
		if _, err := hex.DecodeString(frame.RawSHA256); err != nil {
			return fmt.Errorf("frame %d has invalid raw SHA-256: %w", i, err)
		}
		rawOffset += frame.RawSize
		compressedOffset += frame.CompressedSize
	}
	if rawOffset != manifest.RawSize {
		return fmt.Errorf("frame raw extent %d does not match manifest size %d", rawOffset, manifest.RawSize)
	}
	if compressedOffset != manifest.CompressedSize {
		return fmt.Errorf("frame compressed extent %d does not match manifest size %d", compressedOffset, manifest.CompressedSize)
	}
	return nil
}

// Encode splits raw into independent Zstandard frames and concatenates them.
// Compression is an offline operation, so keeping one encoder here avoids
// adding concurrency that would make conversion provenance harder to control.
func Encode(raw []byte, frameSize int64, level int) ([]byte, *Manifest, error) {
	if frameSize <= 0 {
		return nil, nil, fmt.Errorf("frame size must be positive")
	}
	encoder, err := zstd.NewWriter(nil,
		zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(level)),
		zstd.WithEncoderCRC(true),
		zstd.WithEncoderConcurrency(1),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create zstd encoder: %w", err)
	}
	defer encoder.Close()

	manifest := &Manifest{
		Version:   FormatVersion,
		Codec:     CodecZstd,
		Level:     level,
		FrameSize: frameSize,
		RawSize:   int64(len(raw)),
	}
	if len(raw) == 0 {
		return []byte{}, manifest, nil
	}

	payload := make([]byte, 0, len(raw))
	for rawOffset := int64(0); rawOffset < int64(len(raw)); rawOffset += frameSize {
		rawEnd := rawOffset + frameSize
		if rawEnd > int64(len(raw)) {
			rawEnd = int64(len(raw))
		}
		block := raw[rawOffset:rawEnd]
		compressedOffset := int64(len(payload))
		payload = encoder.EncodeAll(block, payload)
		sum := sha256.Sum256(block)
		manifest.Frames = append(manifest.Frames, Frame{
			Index:            len(manifest.Frames),
			RawOffset:        rawOffset,
			RawSize:          int64(len(block)),
			CompressedOffset: compressedOffset,
			CompressedSize:   int64(len(payload)) - compressedOffset,
			RawSHA256:        hex.EncodeToString(sum[:]),
		})
	}
	manifest.CompressedSize = int64(len(payload))
	if err := manifest.Validate(); err != nil {
		return nil, nil, fmt.Errorf("validate encoded zstd frames: %w", err)
	}
	return payload, manifest, nil
}

// Decode fetches and decompresses independent frames concurrently.  Each
// decoder writes only to its own destination range, so completion order does
// not affect correctness.  Reading from each frame reader is incremental,
// allowing network retrieval and decompression to overlap.
func Decode(ctx context.Context, manifest *Manifest, open OpenRange, destination []byte, workers int) error {
	if err := manifest.Validate(); err != nil {
		return err
	}
	if int64(len(destination)) != manifest.RawSize {
		return fmt.Errorf("destination size %d does not match raw size %d", len(destination), manifest.RawSize)
	}
	if manifest.RawSize == 0 {
		return nil
	}
	if open == nil {
		return fmt.Errorf("open range callback is nil")
	}
	if workers < 1 {
		workers = 1
	}
	if workers > len(manifest.Frames) {
		workers = len(manifest.Frames)
	}

	decodeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan Frame)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup

	decodeFrame := func(frame Frame) error {
		reader, err := open(frame.CompressedOffset, frame.CompressedSize)
		if err != nil {
			return fmt.Errorf("open compressed frame %d: %w", frame.Index, err)
		}
		defer reader.Close()

		decoder, err := zstd.NewReader(reader, zstd.WithDecoderConcurrency(1))
		if err != nil {
			return fmt.Errorf("create decoder for frame %d: %w", frame.Index, err)
		}
		defer decoder.Close()

		raw := destination[frame.RawOffset : frame.RawOffset+frame.RawSize]
		if _, err := io.ReadFull(decoder, raw); err != nil {
			return fmt.Errorf("decode frame %d: %w", frame.Index, err)
		}
		var extra [1]byte
		if n, err := decoder.Read(extra[:]); err != io.EOF || n != 0 {
			if err == nil {
				return fmt.Errorf("frame %d decoded more than %d bytes", frame.Index, frame.RawSize)
			}
			return fmt.Errorf("finish frame %d: %w", frame.Index, err)
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != frame.RawSHA256 {
			return fmt.Errorf("frame %d raw SHA-256 mismatch", frame.Index)
		}
		return nil
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-decodeCtx.Done():
					return
				case frame, ok := <-jobs:
					if !ok {
						return
					}
					if err := decodeFrame(frame); err != nil {
						select {
						case errCh <- err:
						default:
						}
						cancel()
						return
					}
				}
			}
		}()
	}

sendLoop:
	for _, frame := range manifest.Frames {
		select {
		case <-decodeCtx.Done():
			break sendLoop
		case jobs <- frame:
		}
	}
	close(jobs)
	wg.Wait()

	select {
	case err := <-errCh:
		return err
	default:
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}
