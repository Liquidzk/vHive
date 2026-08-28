// analyze_figure9_10_storage computes the static storage and active-working-set
// footprints used by paper Figures 9 and 10. It deliberately follows only
// objects reachable from the selected semantic revisions; unrelated aliases,
// stale representations, and MinIO metadata are excluded.
package main

import (
	"bytes"
	"context"
	"crypto/md5" // #nosec G501 -- MD5 is the existing snapshot content identity.
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const pageSize = 4096

type workload struct {
	Profile        string `json:"profile"`
	Snapshot       string `json:"snapshot"`
	ImageInventory string `json:"image_inventory"`
}

type workloadFile struct {
	Workloads []workload `json:"workloads"`
}

type corpus struct {
	name      string
	endpoint  string
	chunkSize int
	client    *minio.Client
}

type corpusMetrics struct {
	Name                      string `json:"name"`
	Endpoint                  string `json:"endpoint"`
	ChunkSize                 int    `json:"chunk_size"`
	SemanticRevisions         int    `json:"semantic_revisions"`
	RecipeReferences          int64  `json:"recipe_references"`
	LogicalSnapshotBytes      int64  `json:"logical_snapshot_bytes"`
	SnapshotUniqueChunks      int    `json:"snapshot_unique_chunks"`
	WorkingSetUniqueChunks    int    `json:"working_set_unique_chunks"`
	SnapshotChunkBytes        int64  `json:"snapshot_chunk_bytes"`
	WorkingSetChunkBytes      int64  `json:"working_set_chunk_bytes"`
	CompressedObjectsScanned  int64  `json:"compressed_objects_scanned"`
	CompressedBytesScanned    int64  `json:"compressed_bytes_scanned"`
	ReachableCompressedFound  int    `json:"reachable_compressed_objects_found"`
	WorkingSetCompressedFound int    `json:"working_set_compressed_objects_found"`
}

type systemResult struct {
	System                   string  `json:"system"`
	Corpus                   string  `json:"corpus"`
	SnapshotBytes            int64   `json:"snapshot_bytes"`
	WorkingSetStorageBytes   int64   `json:"working_set_storage_bytes"`
	TotalStorageBytes        int64   `json:"total_storage_bytes"`
	ActiveCacheBytes         int64   `json:"active_cache_bytes"`
	StorageNormalizedToFull  float64 `json:"storage_normalized_to_full"`
	CacheNormalizedToWS      float64 `json:"cache_normalized_to_ws"`
	WorkingSetRepresentation string  `json:"working_set_representation"`
}

type report struct {
	Method               string                   `json:"method"`
	GeneratedAt          string                   `json:"generated_at"`
	SemanticRevisions    int                      `json:"semantic_revisions"`
	PageSize             int                      `json:"page_size"`
	RawFullSnapshotBytes int64                    `json:"raw_full_snapshot_bytes"`
	Compression          string                   `json:"compression"`
	PersistentMetadata   string                   `json:"persistent_metadata"`
	FullDedupBoundary    string                   `json:"full_dedup_boundary"`
	Corpora              map[string]corpusMetrics `json:"corpora"`
	Systems              []systemResult           `json:"systems"`
}

func parseRecipe(data []byte) ([]string, error) {
	if len(data)%md5.Size != 0 {
		return nil, fmt.Errorf("recipe size %d is not divisible by %d", len(data), md5.Size)
	}
	hashes := make([]string, 0, len(data)/md5.Size)
	for offset := 0; offset < len(data); offset += md5.Size {
		hashes = append(hashes, hex.EncodeToString(data[offset:offset+md5.Size]))
	}
	return hashes, nil
}

func parseWorkingSetPFNs(data []byte, recipePages int) ([]int, error) {
	records, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse working-set CSV: %w", err)
	}
	if len(records) == 0 || len(records[0]) == 0 || records[0][0] != "pfn" {
		return nil, fmt.Errorf("working-set CSV must begin with a pfn header")
	}
	pfns := make([]int, 0, len(records)-1)
	seen := make(map[int]struct{}, len(records)-1)
	for row, record := range records[1:] {
		if len(record) == 0 || strings.TrimSpace(record[0]) == "" {
			continue
		}
		value, parseErr := strconv.ParseUint(strings.TrimSpace(record[0]), 10, 64)
		if parseErr != nil {
			return nil, fmt.Errorf("parse pfn at row %d: %w", row+2, parseErr)
		}
		if value >= uint64(recipePages) {
			return nil, fmt.Errorf("pfn %d at row %d exceeds recipe page count %d", value, row+2, recipePages)
		}
		pfn := int(value)
		if _, duplicate := seen[pfn]; duplicate {
			return nil, fmt.Errorf("duplicate pfn %d", pfn)
		}
		seen[pfn] = struct{}{}
		pfns = append(pfns, pfn)
	}
	if len(pfns) == 0 {
		return nil, fmt.Errorf("working-set CSV has no pages")
	}
	return pfns, nil
}

func recipeHashForPFN(recipe []string, pfn, chunkSize int) (string, error) {
	if chunkSize < pageSize || chunkSize%pageSize != 0 {
		return "", fmt.Errorf("invalid chunk size %d", chunkSize)
	}
	chunkIndex := pfn / (chunkSize / pageSize)
	if pfn < 0 || chunkIndex >= len(recipe) {
		return "", fmt.Errorf("pfn %d exceeds %d recipe chunks of %d bytes", pfn, len(recipe), chunkSize)
	}
	return recipe[chunkIndex], nil
}

func newCorpus(name, endpoint string, chunkSize int, accessKey, secretKey string) (corpus, error) {
	if chunkSize < pageSize || chunkSize%pageSize != 0 {
		return corpus{}, fmt.Errorf("invalid chunk size %d for %s", chunkSize, name)
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		return corpus{}, err
	}
	return corpus{name: name, endpoint: endpoint, chunkSize: chunkSize, client: client}, nil
}

func download(ctx context.Context, c corpus, bucket, key string) ([]byte, error) {
	object, err := c.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer object.Close()
	var output bytes.Buffer
	if _, err := output.ReadFrom(object); err != nil {
		return nil, fmt.Errorf("read %s/%s: %w", c.name, key, err)
	}
	return output.Bytes(), nil
}

func objectSize(ctx context.Context, c corpus, bucket, key string) (int64, error) {
	info, err := c.client.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return 0, fmt.Errorf("stat %s/%s: %w", c.name, key, err)
	}
	return info.Size, nil
}

func compressedHash(key string) (string, bool) {
	parts := strings.Split(key, "/")
	if len(parts) != 3 || parts[0] != "_chunks_zstd_v1_l3" || len(parts[1]) != 2 || len(parts[2]) != md5.Size*2 {
		return "", false
	}
	if _, err := hex.DecodeString(parts[2]); err != nil {
		return "", false
	}
	return parts[2], true
}

func analyzeCorpus(ctx context.Context, c corpus, bucket string, workloads []workload) (corpusMetrics, error) {
	snapshotWanted := make(map[string]struct{})
	workingSetWanted := make(map[string]struct{})
	var references int64
	for _, workload := range workloads {
		recipeData, err := download(ctx, c, bucket, path.Join(workload.Snapshot, "recipe_file"))
		if err != nil {
			return corpusMetrics{}, err
		}
		recipe, err := parseRecipe(recipeData)
		if err != nil {
			return corpusMetrics{}, fmt.Errorf("%s/%s: %w", c.name, workload.Snapshot, err)
		}
		wsData, err := download(ctx, c, bucket, path.Join(workload.Snapshot, "working_set_pages"))
		if err != nil {
			return corpusMetrics{}, err
		}
		pagesPerChunk := c.chunkSize / pageSize
		pfns, err := parseWorkingSetPFNs(wsData, len(recipe)*pagesPerChunk)
		if err != nil {
			return corpusMetrics{}, fmt.Errorf("%s/%s: %w", c.name, workload.Snapshot, err)
		}
		references += int64(len(recipe))
		for _, hash := range recipe {
			snapshotWanted[hash] = struct{}{}
		}
		for _, pfn := range pfns {
			hash, hashErr := recipeHashForPFN(recipe, pfn, c.chunkSize)
			if hashErr != nil {
				return corpusMetrics{}, fmt.Errorf("%s/%s: %w", c.name, workload.Snapshot, hashErr)
			}
			workingSetWanted[hash] = struct{}{}
		}
	}

	metrics := corpusMetrics{
		Name:                   c.name,
		Endpoint:               c.endpoint,
		ChunkSize:              c.chunkSize,
		SemanticRevisions:      len(workloads),
		RecipeReferences:       references,
		LogicalSnapshotBytes:   references * int64(c.chunkSize),
		SnapshotUniqueChunks:   len(snapshotWanted),
		WorkingSetUniqueChunks: len(workingSetWanted),
	}
	foundSnapshot := make(map[string]struct{}, len(snapshotWanted))
	foundWS := make(map[string]struct{}, len(workingSetWanted))
	objects := c.client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix:    "_chunks_zstd_v1_l3/",
		Recursive: true,
	})
	for object := range objects {
		if object.Err != nil {
			return corpusMetrics{}, fmt.Errorf("list compressed chunks for %s: %w", c.name, object.Err)
		}
		hash, valid := compressedHash(object.Key)
		if !valid {
			return corpusMetrics{}, fmt.Errorf("unexpected compressed object key %q in %s", object.Key, c.name)
		}
		metrics.CompressedObjectsScanned++
		metrics.CompressedBytesScanned += object.Size
		if _, wanted := snapshotWanted[hash]; wanted {
			metrics.SnapshotChunkBytes += object.Size
			foundSnapshot[hash] = struct{}{}
		}
		if _, wanted := workingSetWanted[hash]; wanted {
			metrics.WorkingSetChunkBytes += object.Size
			foundWS[hash] = struct{}{}
		}
	}
	metrics.ReachableCompressedFound = len(foundSnapshot)
	metrics.WorkingSetCompressedFound = len(foundWS)
	if len(foundSnapshot) != len(snapshotWanted) {
		return corpusMetrics{}, fmt.Errorf("%s compressed store covers %d/%d reachable hashes", c.name, len(foundSnapshot), len(snapshotWanted))
	}
	if len(foundWS) != len(workingSetWanted) {
		return corpusMetrics{}, fmt.Errorf("%s compressed store covers %d/%d WS hashes", c.name, len(foundWS), len(workingSetWanted))
	}
	return metrics, nil
}

func sumPayloads(ctx context.Context, c corpus, bucket string, workloads []workload, object string) (int64, error) {
	var total int64
	for _, workload := range workloads {
		size, err := objectSize(ctx, c, bucket, path.Join(workload.Snapshot, object))
		if err != nil {
			return 0, err
		}
		total += size
	}
	return total, nil
}

func sumSharedSources(ctx context.Context, c corpus, bucket string, workloads []workload, shareImages bool) (int64, error) {
	base, err := objectSize(ctx, c, bucket, "ws_shared/base_rootfs/content")
	if err != nil {
		return 0, err
	}
	total := base
	if !shareImages {
		return total, nil
	}
	images := make(map[string]struct{})
	for _, workload := range workloads {
		if workload.ImageInventory != "" {
			images[workload.ImageInventory] = struct{}{}
		}
	}
	ordered := make([]string, 0, len(images))
	for image := range images {
		ordered = append(ordered, image)
	}
	sort.Strings(ordered)
	for _, image := range ordered {
		size, err := objectSize(ctx, c, bucket, path.Join("ws_shared/images", image, "content"))
		if err != nil {
			return 0, err
		}
		total += size
	}
	return total, nil
}

func main() {
	workloadsPath := flag.String("workloads", "", "17-workload JSON configuration")
	bucket := flag.String("bucket", "snapshots", "MinIO bucket")
	accessKey := flag.String("minioAccessKey", "minio", "MinIO access key")
	secretKey := flag.String("minioSecretKey", "minio123", "MinIO secret key")
	full128URL := flag.String("full128URL", "", "Chunks corpus endpoint")
	full4kURL := flag.String("full4kURL", "", "Pages/WS corpus endpoint")
	noImageURL := flag.String("noImageURL", "", "No-image corpus endpoint")
	splitSnapURL := flag.String("splitSnapURL", "", "SplitSnap corpus endpoint")
	fullDedupURL := flag.String("fullDedupURL", "", "Full Dedup corpus endpoint")
	reportPath := flag.String("report", "", "optional JSON report path")
	flag.Parse()

	if *workloadsPath == "" || *full128URL == "" || *full4kURL == "" || *noImageURL == "" || *splitSnapURL == "" || *fullDedupURL == "" {
		fmt.Fprintln(os.Stderr, "-workloads and all five corpus endpoints are required")
		os.Exit(2)
	}
	configData, err := os.ReadFile(*workloadsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read workloads: %v\n", err)
		os.Exit(1)
	}
	var config workloadFile
	if err := json.Unmarshal(configData, &config); err != nil {
		fmt.Fprintf(os.Stderr, "parse workloads: %v\n", err)
		os.Exit(1)
	}
	if len(config.Workloads) == 0 {
		fmt.Fprintln(os.Stderr, "workload configuration is empty")
		os.Exit(1)
	}
	seenSnapshots := make(map[string]struct{}, len(config.Workloads))
	for _, workload := range config.Workloads {
		if workload.Profile == "" || workload.Snapshot == "" || workload.ImageInventory == "" {
			fmt.Fprintln(os.Stderr, "every workload requires profile, snapshot, and image_inventory")
			os.Exit(1)
		}
		if _, duplicate := seenSnapshots[workload.Snapshot]; duplicate {
			fmt.Fprintf(os.Stderr, "duplicate semantic snapshot %s\n", workload.Snapshot)
			os.Exit(1)
		}
		seenSnapshots[workload.Snapshot] = struct{}{}
	}

	endpoints := []struct {
		name, url string
		chunkSize int
	}{
		{"full-128k", *full128URL, 128 * 1024},
		{"full-4k", *full4kURL, pageSize},
		{"no-image-4k", *noImageURL, pageSize},
		{"partial-4k", *splitSnapURL, pageSize},
		{"full-dedup-4k", *fullDedupURL, pageSize},
	}
	corpora := make(map[string]corpus, len(endpoints))
	for _, endpoint := range endpoints {
		value, err := newCorpus(endpoint.name, endpoint.url, endpoint.chunkSize, *accessKey, *secretKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create %s client: %v\n", endpoint.name, err)
			os.Exit(1)
		}
		corpora[endpoint.name] = value
	}

	ctx := context.Background()
	metrics := make(map[string]corpusMetrics, len(corpora))
	for _, endpoint := range endpoints {
		fmt.Fprintf(os.Stderr, "scan corpus=%s endpoint=%s\n", endpoint.name, endpoint.url)
		value, err := analyzeCorpus(ctx, corpora[endpoint.name], *bucket, config.Workloads)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		metrics[endpoint.name] = value
	}

	rawFullBytes := metrics["full-4k"].LogicalSnapshotBytes
	for name, value := range metrics {
		if value.LogicalSnapshotBytes != rawFullBytes {
			fmt.Fprintf(os.Stderr, "corpus %s describes %d logical bytes; expected %d\n", name, value.LogicalSnapshotBytes, rawFullBytes)
			os.Exit(1)
		}
	}

	fullWSPayload, err := sumPayloads(ctx, corpora["full-4k"], *bucket, config.Workloads, "working_set_pages_content.zstd.frames")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	noImagePrivate, err := sumPayloads(ctx, corpora["no-image-4k"], *bucket, config.Workloads, "working_set_pages_content_private.zstd.frames")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	noImageShared, err := sumSharedSources(ctx, corpora["no-image-4k"], *bucket, config.Workloads, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	splitPrivate, err := sumPayloads(ctx, corpora["partial-4k"], *bucket, config.Workloads, "working_set_pages_content_private.zstd.frames")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	splitShared, err := sumSharedSources(ctx, corpora["partial-4k"], *bucket, config.Workloads, true)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	type systemSpec struct {
		name, corpus, representation string
		wsBytes                      int64
		cacheBytes                   int64
	}
	specs := []systemSpec{
		{"Chunks", "full-128k", "reachable independently compressed 128-KiB chunks", 0, metrics["full-128k"].WorkingSetChunkBytes},
		{"Pages", "full-4k", "reachable independently compressed 4-KiB pages", 0, metrics["full-4k"].WorkingSetChunkBytes},
		{"WS", "full-4k", "per-revision coalesced full WS in Zstd-3 frames", fullWSPayload, fullWSPayload},
		{"No-image", "no-image-4k", "shared base/rootfs plus per-revision private/image WS in Zstd-3 frames", noImageShared + noImagePrivate, noImageShared + noImagePrivate},
		{"SplitSnap", "partial-4k", "shared base/rootfs/images plus per-revision private WS in Zstd-3 frames", splitShared + splitPrivate, splitShared + splitPrivate},
		{"Full Dedup", "full-dedup-4k", "global unique exact-content WS pages in independent Zstd-3 representation", metrics["full-dedup-4k"].WorkingSetChunkBytes, metrics["full-dedup-4k"].WorkingSetChunkBytes},
	}
	wsBaseline := fullWSPayload
	if wsBaseline <= 0 || rawFullBytes <= 0 {
		fmt.Fprintln(os.Stderr, "invalid normalization denominator")
		os.Exit(1)
	}
	systems := make([]systemResult, 0, len(specs))
	for _, spec := range specs {
		snapshotBytes := metrics[spec.corpus].SnapshotChunkBytes
		total := snapshotBytes + spec.wsBytes
		systems = append(systems, systemResult{
			System:                   spec.name,
			Corpus:                   spec.corpus,
			SnapshotBytes:            snapshotBytes,
			WorkingSetStorageBytes:   spec.wsBytes,
			TotalStorageBytes:        total,
			ActiveCacheBytes:         spec.cacheBytes,
			StorageNormalizedToFull:  float64(total) / float64(rawFullBytes),
			CacheNormalizedToWS:      float64(spec.cacheBytes) / float64(wsBaseline),
			WorkingSetRepresentation: spec.representation,
		})
	}

	result := report{
		Method:               "static reachable-object analysis over exactly the 17 semantic workload revisions; no invocation trace is used",
		GeneratedAt:          time.Now().UTC().Format(time.RFC3339),
		SemanticRevisions:    len(config.Workloads),
		PageSize:             pageSize,
		RawFullSnapshotBytes: rawFullBytes,
		Compression:          "Zstd level 3 for independently compressed chunks/pages and 1-MiB framed coalesced working sets",
		PersistentMetadata:   "recipe/info/snap/index/manifest metadata and MinIO internal data are excluded, matching the original notebook's payload-only accounting",
		FullDedupBoundary:    "snapshot pages and active WS pages are globally unioned by exact raw-content hash; the separately reported WS term follows the original Figure 9 accounting and is not the transient SplitSnap-format performance-oracle view",
		Corpora:              metrics,
		Systems:              systems,
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal report: %v\n", err)
		os.Exit(1)
	}
	if *reportPath != "" {
		if err := os.WriteFile(*reportPath, append(data, '\n'), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "write report: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Println(string(data))
}
