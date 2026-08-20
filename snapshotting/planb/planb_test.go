package planb

import "testing"

func TestParseCodec(t *testing.T) {
	for name, want := range map[string]Codec{
		"sw_deflate":  CodecSoftwareDeflate,
		"iaa_deflate": CodecIAADeflate,
		"zstd_1":      CodecZstd1,
		"zstd_3":      CodecZstd3,
		"gzip":        CodecGzip,
	} {
		got, err := ParseCodec(name)
		if err != nil || got != want || got.String() != name {
			t.Fatalf("ParseCodec(%q) = %v, %v", name, got, err)
		}
	}
	if _, err := ParseCodec("unknown"); err == nil {
		t.Fatal("expected unknown codec error")
	}
}

func TestStubAvailabilityMatchesBuild(t *testing.T) {
	if Available() {
		t.Skip("Sabre build tag is enabled")
	}
	if _, err := Open(t.TempDir()+"/ws", Options{}); err == nil {
		t.Fatal("stub Open unexpectedly succeeded")
	}
}

func TestGzipRejectsMultiplePartitions(t *testing.T) {
	if _, err := Open(t.TempDir()+"/ws", Options{Codec: CodecGzip, PartitionCount: 2}); err == nil {
		t.Fatal("gzip unexpectedly accepted multiple partitions")
	}
}
