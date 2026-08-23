package snapshotting

import (
	"crypto/md5"
	"testing"
)

func revisionHash(page []byte, revision string) [md5.Size]byte {
	hash := md5.Sum(page)
	return md5.Sum(append(hash[:], []byte(revision)...))
}

func TestSecurityModeValidation(t *testing.T) {
	valid := []string{
		SecurityModeNone,
		SecurityModePartial,
		SecurityModeNoImageSharing,
		SecurityModeFull,
		" NO-IMAGE-SHARING ",
	}
	for _, mode := range valid {
		if !IsValidSecurityMode(mode) {
			t.Fatalf("expected security mode %q to be valid", mode)
		}
	}
	if IsValidSecurityMode("unknown") {
		t.Fatal("unexpectedly accepted unknown security mode")
	}
}

func TestDeriveChunkHashBySecurityMode(t *testing.T) {
	originalImages := imageChunks
	originalRootfs := rootfsChunks
	originalBase := baseSnapChunks
	t.Cleanup(func() {
		imageChunks = originalImages
		rootfsChunks = originalRootfs
		baseSnapChunks = originalBase
	})

	rootfsPage := []byte("rootfs-page")
	basePage := []byte("base-page")
	imagePage := []byte("image-page")
	privatePage := []byte("private-page")
	rootfsHash := md5.Sum(rootfsPage)
	baseHash := md5.Sum(basePage)
	imageHash := md5.Sum(imagePage)

	rootfsChunks = map[[md5.Size]byte]bool{rootfsHash: true}
	baseSnapChunks = map[[md5.Size]byte]bool{baseHash: true}
	imageChunks = map[string]map[[md5.Size]byte]bool{
		"image-rotate-go": {imageHash: true},
	}

	revision := "revision-a"
	image := "registry.local/liquidzk/image-rotate-go:tag"
	raw := func(page []byte) [md5.Size]byte { return md5.Sum(page) }

	tests := []struct {
		name     string
		mode     string
		page     []byte
		expected [md5.Size]byte
	}{
		{name: "none-private", mode: SecurityModeNone, page: privatePage, expected: raw(privatePage)},
		{name: "partial-rootfs", mode: SecurityModePartial, page: rootfsPage, expected: raw(rootfsPage)},
		{name: "partial-base", mode: SecurityModePartial, page: basePage, expected: raw(basePage)},
		{name: "partial-image", mode: SecurityModePartial, page: imagePage, expected: raw(imagePage)},
		{name: "partial-private", mode: SecurityModePartial, page: privatePage, expected: revisionHash(privatePage, revision)},
		{name: "no-image-rootfs", mode: SecurityModeNoImageSharing, page: rootfsPage, expected: raw(rootfsPage)},
		{name: "no-image-base", mode: SecurityModeNoImageSharing, page: basePage, expected: raw(basePage)},
		{name: "no-image-image", mode: SecurityModeNoImageSharing, page: imagePage, expected: revisionHash(imagePage, revision)},
		{name: "no-image-private", mode: SecurityModeNoImageSharing, page: privatePage, expected: revisionHash(privatePage, revision)},
		{name: "full-rootfs", mode: SecurityModeFull, page: rootfsPage, expected: revisionHash(rootfsPage, revision)},
		{name: "full-image", mode: SecurityModeFull, page: imagePage, expected: revisionHash(imagePage, revision)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mgr := &SnapshotManager{securityMode: test.mode}
			actual := mgr.DeriveChunkHash(test.page, revision, image)
			if actual != test.expected {
				t.Fatalf("unexpected hash for mode %s: got %x want %x", test.mode, actual, test.expected)
			}
		})
	}
}

func TestNoImageSharingWorkingSetPolicy(t *testing.T) {
	current := &SnapshotManager{securityMode: SecurityModePartial}
	if !current.usesProvenanceClassification() || !current.sharesImagePages() {
		t.Fatal("partial mode must classify and share image working-set pages")
	}

	noImage := &SnapshotManager{securityMode: SecurityModeNoImageSharing}
	if !noImage.usesProvenanceClassification() {
		t.Fatal("no-image-sharing mode must retain provenance classification")
	}
	if noImage.sharesImagePages() {
		t.Fatal("no-image-sharing mode must place image working-set pages in the private source")
	}
}
