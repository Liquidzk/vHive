//go:build sabre

package planb

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func testRoundTrip(t *testing.T, codec Codec) {
	t.Helper()
	content := make([]byte, 2*1024*1024)
	for i := range content {
		content[i] = byte((i/64 + i/4096*17) % 251)
	}
	restorer, err := Open(filepath.Join(t.TempDir(), "private"), Options{Codec: codec})
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

func TestIAARoundTrip(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("IAA work queue requires root")
	}
	testRoundTrip(t, CodecIAADeflate)
}
