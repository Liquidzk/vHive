package main

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

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
