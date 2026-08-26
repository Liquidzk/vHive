package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/vhive-serverless/vhive/snapshotting"
)

type recipeTestStorage struct {
	objects map[string][]byte
}

func (s *recipeTestStorage) UploadObject(string, io.Reader, int64) error { return nil }

func (s *recipeTestStorage) DownloadObject(key string) ([]byte, error) {
	data, ok := s.objects[key]
	if !ok {
		return nil, fmt.Errorf("missing object %s", key)
	}
	return bytes.Clone(data), nil
}

func (s *recipeTestStorage) Exists(key string) (bool, error) {
	_, ok := s.objects[key]
	return ok, nil
}

func (s *recipeTestStorage) ListObjects(string, bool) ([]string, error) { return nil, nil }
func (s *recipeTestStorage) UploadFile(string, string) error            { return nil }

func (s *recipeTestStorage) DownloadFile(key, destination string) error {
	data, err := s.DownloadObject(key)
	if err != nil {
		return err
	}
	return os.WriteFile(destination, data, 0644)
}

func TestMaterializeLazyRecipe(t *testing.T) {
	base := t.TempDir()
	snap := snapshotting.NewSnapshot("revision-a", base, "image")
	if err := snap.CreateSnapDir(); err != nil {
		t.Fatal(err)
	}
	want := []byte("recipe-bytes")
	store := &recipeTestStorage{objects: map[string][]byte{
		filepath.Join(snap.GetId(), "recipe_file"): want,
	}}

	if err := materializeLazyRecipe(store, snap); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(snap.GetRecipeFilePath())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("recipe mismatch: got %q want %q", got, want)
	}
}

func TestMaterializeLazyRecipePropagatesMissingObject(t *testing.T) {
	base := t.TempDir()
	snap := snapshotting.NewSnapshot("revision-missing", base, "image")
	if err := snap.CreateSnapDir(); err != nil {
		t.Fatal(err)
	}
	store := &recipeTestStorage{objects: map[string][]byte{}}
	if err := materializeLazyRecipe(store, snap); err == nil {
		t.Fatal("expected missing recipe to fail")
	}
}
