package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseRecipeAndWorkingSet(t *testing.T) {
	first := md5.Sum([]byte("first"))   // #nosec G401 -- fixture content identity.
	second := md5.Sum([]byte("second")) // #nosec G401 -- fixture content identity.
	recipe := append(first[:], second[:]...)
	hashes, err := parseRecipe(recipe)
	require.NoError(t, err)
	require.Equal(t, []string{hex.EncodeToString(first[:]), hex.EncodeToString(second[:])}, hashes)

	pfns, err := parseWorkingSetPFNs([]byte("pfn\n1\n0\n"), len(hashes))
	require.NoError(t, err)
	require.Equal(t, []int{1, 0}, pfns)
}

func TestCompressedHash(t *testing.T) {
	hash := fmt.Sprintf("%032x", 42)
	actual, ok := compressedHash("_chunks_zstd_v1_l3/00/" + hash)
	require.True(t, ok)
	require.Equal(t, hash, actual)
	_, ok = compressedHash("_chunks/00/" + hash)
	require.False(t, ok)
}

func TestRecipeHashForPagePFNAcrossChunkSizes(t *testing.T) {
	recipe := []string{"first", "second"}

	got, err := recipeHashForPFN(recipe, 31, 128*1024)
	require.NoError(t, err)
	require.Equal(t, "first", got)
	got, err = recipeHashForPFN(recipe, 32, 128*1024)
	require.NoError(t, err)
	require.Equal(t, "second", got)
	got, err = recipeHashForPFN(recipe, 1, pageSize)
	require.NoError(t, err)
	require.Equal(t, "second", got)
	_, err = recipeHashForPFN(recipe, 64, 128*1024)
	require.Error(t, err)
}
