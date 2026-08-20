//go:build sabre

package planb

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func testRoundTrip(t *testing.T, codec Codec) {
	testRoundTripWithOptions(t, Options{Codec: codec})
}

func testRoundTripWithOptions(t *testing.T, opts Options) {
	t.Helper()
	content := make([]byte, 2*1024*1024)
	for i := range content {
		content[i] = byte((i/64 + i/4096*17) % 251)
	}
	restorer, err := Open(filepath.Join(t.TempDir(), "private"), opts)
	if err != nil {
		t.Fatal(err)
	}
	defer restorer.Close()
	if err := restorer.Compress(content); err != nil {
		t.Fatal(err)
	}
	region, err := restorer.Decompress(uint64(len(content)))
	if err != nil {
		t.Fatal(err)
	}
	defer region.Free()
	if !bytes.Equal(region.Bytes(), content) {
		t.Fatal("Plan B round trip differs")
	}
}

func TestSoftwareRoundTrip(t *testing.T) {
	testRoundTrip(t, CodecSoftwareDeflate)
}

func TestSoftwareFourPartitionRoundTrip(t *testing.T) {
	testRoundTripWithOptions(t, Options{
		Codec:           CodecSoftwareDeflate,
		MaxHardwareJobs: 1,
		PartitionCount:  4,
	})
}

func TestIAARoundTrip(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("IAA work queue requires root")
	}
	testRoundTrip(t, CodecIAADeflate)
}

func TestIAAFourPartitionRoundTrip(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("IAA work queue requires root")
	}
	testRoundTripWithOptions(t, Options{
		Codec:           CodecIAADeflate,
		MaxHardwareJobs: 4,
		PartitionCount:  4,
	})
}

func TestBuildPartitions(t *testing.T) {
	parts, err := buildPartitions(10*4096, 4)
	if err != nil {
		t.Fatal(err)
	}
	wantOffsets := []uint64{0, 2 * 4096, 4 * 4096, 6 * 4096}
	wantSizes := []uint64{2 * 4096, 2 * 4096, 2 * 4096, 4 * 4096}
	for i := range parts {
		if uint64(parts[i].offset) != wantOffsets[i] || uint64(parts[i].size) != wantSizes[i] {
			t.Fatalf("partition %d = offset %d size %d, want offset %d size %d",
				i, uint64(parts[i].offset), uint64(parts[i].size), wantOffsets[i], wantSizes[i])
		}
	}
}
