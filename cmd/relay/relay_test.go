package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadVMMemSizeMap(t *testing.T) {
	t.Run("empty path disables mapping", func(t *testing.T) {
		got, err := loadVMMemSizeMap("")
		if err != nil || got != nil {
			t.Fatalf("loadVMMemSizeMap empty = %#v, %v; want nil, nil", got, err)
		}
	})

	t.Run("valid map", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "memory.json")
		if err := os.WriteFile(path, []byte(`{"aes-go-1":512,"video-analytics-1":4096}`), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := loadVMMemSizeMap(path)
		if err != nil {
			t.Fatal(err)
		}
		if got["aes-go-1"] != 512 || got["video-analytics-1"] != 4096 || len(got) != 2 {
			t.Fatalf("unexpected VM memory map: %#v", got)
		}
	})

	for _, contents := range []string{`{}`, `{"revision":0}`, `{"":512}`, `[]`} {
		contents := contents
		t.Run("reject_"+contents, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "memory.json")
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadVMMemSizeMap(path); err == nil {
				t.Fatalf("loadVMMemSizeMap(%s) unexpectedly succeeded", contents)
			}
		})
	}
}

func TestFunctionEndpointPort(t *testing.T) {
	tests := []struct {
		name      string
		direct    string
		args      string
		want      string
		wantError bool
	}{
		{name: "default", args: "--function-name=aes-go", want: "50051"},
		{name: "direct", direct: "3550", want: "3550"},
		{name: "equals", args: "--function-endpoint-port=7000 --function-name=currencyservice", want: "7000"},
		{name: "separate", args: "--function-endpoint-port 8080 --function-name=emailservice", want: "8080"},
		{name: "missing", args: "--function-endpoint-port", wantError: true},
		{name: "zero", args: "--function-endpoint-port=0", wantError: true},
		{name: "not-number", args: "--function-endpoint-port=http", wantError: true},
		{name: "direct-zero", direct: "0", wantError: true},
		{name: "direct-not-number", direct: "http", wantError: true},
		{name: "ambiguous", direct: "50051", args: "--function-name=aes-go", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := functionEndpointPort(test.direct, test.args)
			if test.wantError {
				if err == nil {
					t.Fatalf("functionEndpointPort(%q, %q) unexpectedly succeeded with %q", test.direct, test.args, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("functionEndpointPort(%q, %q): %v", test.direct, test.args, err)
			}
			if got != test.want {
				t.Fatalf("functionEndpointPort(%q, %q) = %q, want %q", test.direct, test.args, got, test.want)
			}
		})
	}
}

func TestWaitForTCPReady(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	if err := waitForTCP(context.Background(), listener.Addr().String(), time.Second); err != nil {
		t.Fatalf("waitForTCP returned an error for a listening endpoint: %v", err)
	}
}

func TestWaitForTCPTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}

	if err := waitForTCP(context.Background(), address, 20*time.Millisecond); err == nil {
		t.Fatal("waitForTCP unexpectedly succeeded for a closed endpoint")
	}
}

func TestAllocateLoopbackEndpoint(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on occupied endpoint: %v", err)
	}
	defer occupied.Close()

	endpoint, err := allocateLoopbackEndpoint()
	if err != nil {
		t.Fatalf("allocateLoopbackEndpoint: %v", err)
	}
	if endpoint == occupied.Addr().String() {
		t.Fatalf("allocated occupied endpoint %s", endpoint)
	}

	listener, err := net.Listen("tcp", endpoint)
	if err != nil {
		t.Fatalf("allocated endpoint %s is not bindable: %v", endpoint, err)
	}
	listener.Close()
}

func TestGRPCStatus(t *testing.T) {
	tests := []struct {
		name   string
		header http.Header
		want   string
	}{
		{name: "missing", header: http.Header{}, want: ""},
		{name: "response header", header: http.Header{"Grpc-Status": []string{"0"}}, want: "0"},
		{name: "trailer", header: http.Header{http.TrailerPrefix + "Grpc-Status": []string{"13"}}, want: "13"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := grpcStatus(test.header); got != test.want {
				t.Fatalf("grpcStatus() = %q, want %q", got, test.want)
			}
		})
	}
}
