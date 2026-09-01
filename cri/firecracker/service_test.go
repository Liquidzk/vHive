// MIT License
//
// Copyright (c) 2026 vHive team

package firecracker

import "testing"

func TestIsStockMultiContainerSiblingName(t *testing.T) {
	tests := []struct {
		name      string
		container string
		want      bool
	}{
		{name: "first user container", container: "user-container-0", want: true},
		{name: "second user container", container: "user-container-1", want: true},
		{name: "explicit relay container", container: "relay", want: true},
		{name: "explicit function container", container: "function", want: true},
		{name: "vm placeholder", container: "user-container", want: false},
		{name: "queue proxy", container: "queue-proxy", want: false},
		{name: "empty metadata", container: "", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isStockMultiContainerSiblingName(tc.container); got != tc.want {
				t.Fatalf("isStockMultiContainerSiblingName(%q) = %v, want %v", tc.container, got, tc.want)
			}
		})
	}
}
