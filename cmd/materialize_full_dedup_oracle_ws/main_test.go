package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"reflect"
	"testing"
)

func TestParseRecipe(t *testing.T) {
	first := md5.Sum([]byte("first")) // #nosec G401 -- test fixture for the existing content identity.
	second := md5.Sum([]byte("second"))
	recipe := append(append([]byte{}, first[:]...), second[:]...)

	got, err := parseRecipe(recipe)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{fmt.Sprintf("%x", first), fmt.Sprintf("%x", second)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseRecipe() = %v, want %v", got, want)
	}

	if _, err := parseRecipe(append(recipe, 0)); err == nil {
		t.Fatal("parseRecipe accepted a truncated digest")
	}
}

func TestParseWorkingSetPFNsPreservesProfileOrder(t *testing.T) {
	got, err := parseWorkingSetPFNs([]byte("pfn\n3\n1\n4\n"), 5)
	if err != nil {
		t.Fatal(err)
	}
	want := []int{3, 1, 4}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseWorkingSetPFNs() = %v, want %v", got, want)
	}
}

func TestParseWorkingSetPFNsRejectsDuplicateAndOutOfRange(t *testing.T) {
	for _, fixture := range [][]byte{
		[]byte("pfn\n1\n1\n"),
		[]byte("pfn\n5\n"),
		[]byte("wrong\n1\n"),
	} {
		if _, err := parseWorkingSetPFNs(fixture, 5); err == nil {
			t.Fatalf("accepted invalid fixture %q", bytes.TrimSpace(fixture))
		}
	}
}

func TestUniqueSorted(t *testing.T) {
	got := uniqueSorted([]string{"b", "a", "b", "c", "a"})
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("uniqueSorted() = %v, want %v", got, want)
	}
}

func TestCanonicalHashes(t *testing.T) {
	first := "00112233445566778899aabbccddeeff"
	second := "ffeeddccbbaa99887766554433221100"
	got, seen, err := canonicalHashes([]string{chunkObject(second), chunkObject(first)})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{first, second}
	if !reflect.DeepEqual(got, want) || len(seen) != 2 {
		t.Fatalf("canonicalHashes() = %v/%d, want %v/2", got, len(seen), want)
	}

	for _, keys := range [][]string{
		{"_chunks/00/not-a-hash"},
		{chunkObject(first), chunkObject(first)},
		{"_chunks/ff/" + first},
	} {
		if _, _, err := canonicalHashes(keys); err == nil {
			t.Fatalf("accepted invalid canonical keys %v", keys)
		}
	}
}

func TestParseHashIndexedSource(t *testing.T) {
	first := bytes.Repeat([]byte{0x11}, pageSize)
	second := bytes.Repeat([]byte{0x22}, pageSize)
	firstHash := fmt.Sprintf("%x", md5.Sum(first)) // #nosec G401 -- fixture for the existing identity.
	secondHash := fmt.Sprintf("%x", md5.Sum(second))
	index := []byte(fmt.Sprintf("hash\n%s\n%s\n", firstHash, secondHash))
	content := append(append([]byte{}, first...), second...)

	got, err := parseHashIndexedSource(index, content)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got[firstHash], first) || !bytes.Equal(got[secondHash], second) {
		t.Fatal("parsed source content differs from fixture")
	}

	if _, err := parseHashIndexedSource([]byte("wrong\n"), content); err == nil {
		t.Fatal("accepted a source without a hash header")
	}
	badContent := append([]byte{}, content...)
	badContent[0] ^= 0xff
	if _, err := parseHashIndexedSource(index, badContent); err == nil {
		t.Fatal("accepted a hash/content mismatch")
	}
}
