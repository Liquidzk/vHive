package zstdstream

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeOutOfOrder(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	raw := make([]byte, 3*1024*1024+177)
	_, err := rng.Read(raw)
	require.NoError(t, err)
	copy(raw[512*1024:1024*1024], bytes.Repeat([]byte("zstd-streaming"), 32768))

	payload, manifest, err := Encode(raw, 256*1024, 3)
	require.NoError(t, err)
	require.Greater(t, len(manifest.Frames), 4)

	destination := make([]byte, len(raw))
	completion := make([]int, 0, len(manifest.Frames))
	var completionMu sync.Mutex
	open := func(offset, length int64) (io.ReadCloser, error) {
		frameIndex := -1
		for _, frame := range manifest.Frames {
			if frame.CompressedOffset == offset {
				frameIndex = frame.Index
				break
			}
		}
		if frameIndex < 0 {
			return nil, fmt.Errorf("unknown frame offset %d", offset)
		}
		// Earlier frames wait longer, forcing completion order to differ from
		// logical order while all writes remain disjoint.
		delay := time.Duration(len(manifest.Frames)-frameIndex) * time.Millisecond
		section := payload[offset : offset+length]
		return &delayedReader{
			Reader: bytes.NewReader(section),
			delay:  delay,
			onClose: func() {
				completionMu.Lock()
				completion = append(completion, frameIndex)
				completionMu.Unlock()
			},
		}, nil
	}

	require.NoError(t, Decode(context.Background(), manifest, open, destination, 4))
	require.Equal(t, raw, destination)
	require.Len(t, completion, len(manifest.Frames))
	require.NotEqual(t, manifest.Frames[0].Index, completion[0])
}

func TestDecodeRejectsCorruption(t *testing.T) {
	raw := bytes.Repeat([]byte("compressible working set page"), 8192)
	payload, manifest, err := Encode(raw, 64*1024, 3)
	require.NoError(t, err)
	payload[len(payload)/2] ^= 0xff

	open := func(offset, length int64) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(payload[offset : offset+length])), nil
	}
	err = Decode(context.Background(), manifest, open, make([]byte, len(raw)), 3)
	require.Error(t, err)
}

func TestManifestRoundTripAndValidation(t *testing.T) {
	payload, manifest, err := Encode([]byte("hello framed zstd"), 4096, 3)
	require.NoError(t, err)
	require.NotEmpty(t, payload)

	data, err := MarshalManifest(manifest)
	require.NoError(t, err)
	parsed, err := ParseManifest(data)
	require.NoError(t, err)
	require.Equal(t, manifest, parsed)

	parsed.Frames[0].RawOffset = 1
	require.Error(t, parsed.Validate())
}

type delayedReader struct {
	*bytes.Reader
	delay   time.Duration
	onClose func()
	once    sync.Once
}

func (reader *delayedReader) Read(p []byte) (int, error) {
	reader.once.Do(func() { time.Sleep(reader.delay) })
	return reader.Reader.Read(p)
}

func (reader *delayedReader) Close() error {
	if reader.onClose != nil {
		reader.onClose()
	}
	return nil
}
