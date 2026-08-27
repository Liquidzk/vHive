package main

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestFunctionEndpointPort(t *testing.T) {
	tests := []struct {
		name      string
		args      string
		want      string
		wantError bool
	}{
		{name: "default", args: "--function-name=aes-go", want: "50051"},
		{name: "equals", args: "--function-endpoint-port=7000 --function-name=currencyservice", want: "7000"},
		{name: "separate", args: "--function-endpoint-port 8080 --function-name=emailservice", want: "8080"},
		{name: "missing", args: "--function-endpoint-port", wantError: true},
		{name: "zero", args: "--function-endpoint-port=0", wantError: true},
		{name: "not-number", args: "--function-endpoint-port=http", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := functionEndpointPort(test.args)
			if test.wantError {
				if err == nil {
					t.Fatalf("functionEndpointPort(%q) unexpectedly succeeded with %q", test.args, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("functionEndpointPort(%q): %v", test.args, err)
			}
			if got != test.want {
				t.Fatalf("functionEndpointPort(%q) = %q, want %q", test.args, got, test.want)
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
