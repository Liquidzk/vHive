// MIT License
// Copyright (c) 2026 vHive team

package snapshotting

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestPersistFileAtomicConcurrentReaders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot", "recipe_file")
	const size = 256 * 1024
	if err := persistFileAtomic(path, bytes.Repeat([]byte{0}, size)); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	errCh := make(chan error, 1)
	go func() {
		defer close(done)
		var wg sync.WaitGroup
		for writer := 1; writer <= 8; writer++ {
			value := byte(writer)
			wg.Add(1)
			go func() {
				defer wg.Done()
				for iteration := 0; iteration < 20; iteration++ {
					if err := persistFileAtomic(path, bytes.Repeat([]byte{value}, size)); err != nil {
						select {
						case errCh <- err:
						default:
						}
						return
					}
				}
			}()
		}
		wg.Wait()
	}()

	for {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(data) != size {
			t.Fatalf("reader observed partial cache file: got %d bytes, want %d", len(data), size)
		}
		if !bytes.Equal(data, bytes.Repeat(data[:1], size)) {
			t.Fatal("reader observed content from overlapping cache writers")
		}

		select {
		case err := <-errCh:
			t.Fatal(err)
		case <-done:
			matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".recipe_file.tmp-*"))
			if err != nil {
				t.Fatal(err)
			}
			if len(matches) != 0 {
				t.Fatalf("temporary files remain after atomic persist: %v", matches)
			}
			return
		default:
		}
	}
}
