package main

import (
	"container/list"
	"crypto/md5" // Content identity matches the snapshot format and the paper methodology.
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const defaultPageSize = 4096

type provenance string

const (
	provenanceBaseRootfs provenance = "base-rootfs"
	provenanceImage      provenance = "image"
	provenancePrivate    provenance = "private"
)

type policy string

const (
	policyCurrent policy = "current"
	policyNoImage policy = "no-image-sharing"
	policyFull    policy = "full-dedup"
)

var policies = []policy{policyCurrent, policyNoImage, policyFull}

type corpusConfig struct {
	PageSize           int            `json:"page_size"`
	CacheCapacityPages int            `json:"cache_capacity_pages"`
	Snapshots          []snapshotSpec `json:"snapshots"`
}

type snapshotSpec struct {
	ID          string `json:"id"`
	Workload    string `json:"workload"`
	RevisionDir string `json:"revision_dir"`
	SharedRoot  string `json:"shared_root"`
	Image       string `json:"image"`
}

type pageRecord struct {
	SnapshotID string
	Workload   string
	Occurrence int
	PFN        uint64
	RawHash    string
	Provenance provenance
}

type snapshotInventory struct {
	Spec       snapshotSpec
	Pages      []pageRecord
	Validation validationRecord
	Inputs     []inputRecord
}

type validationRecord struct {
	SnapshotID       string
	Workload         string
	WSRows           int
	UniquePFNs       int
	DuplicatePFNs    int
	PrivateRows      int
	BaseRootfsRows   int
	ImageRows        int
	UniqueRawPages   int
	RecipePages      int
	PrivateFileBytes int64
	BaseFileBytes    int64
	ImageFileBytes   int64
}

type inputRecord struct {
	SnapshotID string
	Role       string
	Path       string
	Bytes      int64
	SHA256     string
}

type summaryRecord struct {
	Policy                policy
	PageSize              int
	SnapshotCount         int
	PageOccurrences       int
	LogicalUniquePages    int
	LogicalUniqueBytes    int64
	DeltaPagesVsCurrent   int
	DeltaBytesVsCurrent   int64
	DeltaPercentVsCurrent float64
	CacheCapacityPages    int
	FetchPages            int
	FetchBytes            int64
	CacheHits             int
	CacheMisses           int
}

type traceRecord struct {
	Policy             policy
	RestoreIndex       int
	SnapshotID         string
	Workload           string
	LogicalUniquePages int
	FetchPages         int
	FetchBytes         int64
	CacheHits          int
	CacheMisses        int
	ResidentPagesAfter int
}

type analysisResult struct {
	Config      corpusConfig
	Inventories []snapshotInventory
	Summaries   []summaryRecord
	Trace       []traceRecord
}

func loadCorpusConfig(path string) (corpusConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return corpusConfig{}, fmt.Errorf("read config: %w", err)
	}

	var cfg corpusConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return corpusConfig{}, fmt.Errorf("decode config: %w", err)
	}
	if cfg.PageSize == 0 {
		cfg.PageSize = defaultPageSize
	}
	if cfg.PageSize != defaultPageSize {
		return corpusConfig{}, fmt.Errorf("page_size must be %d for the current recipe format", defaultPageSize)
	}
	if cfg.CacheCapacityPages < 0 {
		return corpusConfig{}, fmt.Errorf("cache_capacity_pages must be non-negative (zero means unbounded)")
	}
	if len(cfg.Snapshots) < 2 {
		return corpusConfig{}, fmt.Errorf("full dedup requires at least two snapshots/revisions")
	}

	configDir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return corpusConfig{}, fmt.Errorf("resolve config directory: %w", err)
	}

	seenIDs := make(map[string]bool, len(cfg.Snapshots))
	for i := range cfg.Snapshots {
		spec := &cfg.Snapshots[i]
		spec.ID = strings.TrimSpace(spec.ID)
		if spec.ID == "" {
			return corpusConfig{}, fmt.Errorf("snapshot %d has an empty id", i)
		}
		if seenIDs[spec.ID] {
			return corpusConfig{}, fmt.Errorf("duplicate snapshot id %q", spec.ID)
		}
		seenIDs[spec.ID] = true
		if spec.Workload == "" {
			spec.Workload = spec.ID
		}
		if spec.RevisionDir == "" {
			return corpusConfig{}, fmt.Errorf("snapshot %q has an empty revision_dir", spec.ID)
		}
		spec.RevisionDir = resolveConfigPath(configDir, spec.RevisionDir)
		if spec.SharedRoot == "" {
			spec.SharedRoot = filepath.Join(filepath.Dir(spec.RevisionDir), "ws_shared")
		} else {
			spec.SharedRoot = resolveConfigPath(configDir, spec.SharedRoot)
		}
		spec.Image = strings.TrimSpace(spec.Image)
	}

	return cfg, nil
}

func resolveConfigPath(base, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(base, path))
}

func analyzeCorpus(cfg corpusConfig) (analysisResult, error) {
	inventories := make([]snapshotInventory, 0, len(cfg.Snapshots))
	for _, spec := range cfg.Snapshots {
		inventory, err := loadSnapshotInventory(spec, cfg.PageSize)
		if err != nil {
			return analysisResult{}, fmt.Errorf("snapshot %q: %w", spec.ID, err)
		}
		inventories = append(inventories, inventory)
	}

	summaries, trace := evaluatePolicies(inventories, cfg.PageSize, cfg.CacheCapacityPages)
	return analysisResult{
		Config:      cfg,
		Inventories: inventories,
		Summaries:   summaries,
		Trace:       trace,
	}, nil
}

func loadSnapshotInventory(spec snapshotSpec, pageSize int) (snapshotInventory, error) {
	workingSetPath := filepath.Join(spec.RevisionDir, "working_set_pages")
	recipePath := filepath.Join(spec.RevisionDir, "recipe_file")
	privateIndexPath := filepath.Join(spec.RevisionDir, "working_set_pages_index_private")
	privateContentPath := filepath.Join(spec.RevisionDir, "working_set_pages_content_private")
	baseIndexPath := filepath.Join(spec.SharedRoot, "base_rootfs_index")
	baseContentPath := filepath.Join(spec.SharedRoot, "base_rootfs_content")

	imageName, err := resolveImageName(spec.SharedRoot, spec.Image)
	if err != nil {
		return snapshotInventory{}, err
	}
	imageIndexPath := ""
	imageContentPath := ""
	if imageName != "" {
		imageIndexPath = filepath.Join(spec.SharedRoot, "images", imageName+"_index")
		imageContentPath = filepath.Join(spec.SharedRoot, "images", imageName+"_content")
	}

	wsPFNs, err := readPFNCSV(workingSetPath)
	if err != nil {
		return snapshotInventory{}, fmt.Errorf("read working set: %w", err)
	}
	privatePFNs, err := readPFNCSV(privateIndexPath)
	if err != nil {
		return snapshotInventory{}, fmt.Errorf("read private index: %w", err)
	}
	privateContent, err := os.ReadFile(privateContentPath)
	if err != nil {
		return snapshotInventory{}, fmt.Errorf("read private content: %w", err)
	}
	if len(privateContent) != len(privatePFNs)*pageSize {
		return snapshotInventory{}, fmt.Errorf("private content is %d bytes for %d index rows; expected %d",
			len(privateContent), len(privatePFNs), len(privatePFNs)*pageSize)
	}

	baseHashes, baseBytes, err := readAndValidateSharedSource(baseIndexPath, baseContentPath, pageSize, false)
	if err != nil {
		return snapshotInventory{}, fmt.Errorf("read base/rootfs source: %w", err)
	}
	imageHashes, imageBytes, err := readAndValidateSharedSource(imageIndexPath, imageContentPath, pageSize, true)
	if err != nil {
		return snapshotInventory{}, fmt.Errorf("read image source: %w", err)
	}

	recipe, err := os.ReadFile(recipePath)
	if err != nil {
		return snapshotInventory{}, fmt.Errorf("read recipe: %w", err)
	}
	if len(recipe)%md5.Size != 0 {
		return snapshotInventory{}, fmt.Errorf("recipe size %d is not divisible by %d", len(recipe), md5.Size)
	}
	recipePages := len(recipe) / md5.Size

	type privatePage struct {
		rawHash string
	}
	privateByPFN := make(map[uint64][]privatePage)
	for i, pfn := range privatePFNs {
		start := i * pageSize
		sum := md5.Sum(privateContent[start : start+pageSize])
		privateByPFN[pfn] = append(privateByPFN[pfn], privatePage{rawHash: hex.EncodeToString(sum[:])})
	}
	privateCursor := make(map[uint64]int)

	baseSet := stringSet(baseHashes)
	imageSet := stringSet(imageHashes)
	pages := make([]pageRecord, 0, len(wsPFNs))
	uniquePFNs := make(map[uint64]bool)
	uniqueRaw := make(map[string]bool)
	privateRows := 0
	baseRows := 0
	imageRows := 0

	for occurrence, pfn := range wsPFNs {
		if pfn >= uint64(recipePages) {
			return snapshotInventory{}, fmt.Errorf("working-set PFN %d exceeds recipe page count %d", pfn, recipePages)
		}
		uniquePFNs[pfn] = true

		var rawHash string
		var source provenance
		privatePages := privateByPFN[pfn]
		cursor := privateCursor[pfn]
		if cursor < len(privatePages) {
			rawHash = privatePages[cursor].rawHash
			privateCursor[pfn] = cursor + 1
			source = provenancePrivate
			privateRows++

			rawBytes, decodeErr := hex.DecodeString(rawHash)
			if decodeErr != nil {
				return snapshotInventory{}, fmt.Errorf("decode private raw hash %q: %w", rawHash, decodeErr)
			}
			expectedSalted := md5.Sum(append(rawBytes, []byte(spec.ID)...))
			recipeHash := recipeHashAt(recipe, pfn)
			if recipeHash != hex.EncodeToString(expectedSalted[:]) {
				return snapshotInventory{}, fmt.Errorf("private PFN %d recipe hash does not match revision-salted raw content hash", pfn)
			}
		} else {
			rawHash = recipeHashAt(recipe, pfn)
			switch {
			case baseSet[rawHash]:
				source = provenanceBaseRootfs
				baseRows++
			case imageSet[rawHash]:
				source = provenanceImage
				imageRows++
			default:
				return snapshotInventory{}, fmt.Errorf("PFN %d with recipe hash %s is absent from private, base/rootfs, and image sources", pfn, rawHash)
			}
		}

		uniqueRaw[rawHash] = true
		pages = append(pages, pageRecord{
			SnapshotID: spec.ID,
			Workload:   spec.Workload,
			Occurrence: occurrence,
			PFN:        pfn,
			RawHash:    rawHash,
			Provenance: source,
		})
	}

	for pfn, entries := range privateByPFN {
		if privateCursor[pfn] != len(entries) {
			return snapshotInventory{}, fmt.Errorf("private PFN %d has %d unconsumed index/content rows", pfn, len(entries)-privateCursor[pfn])
		}
	}

	inputPaths := []struct {
		role string
		path string
	}{
		{"working-set-pages", workingSetPath},
		{"recipe", recipePath},
		{"private-index", privateIndexPath},
		{"private-content", privateContentPath},
		{"base-rootfs-index", baseIndexPath},
		{"base-rootfs-content", baseContentPath},
	}
	if imageIndexPath != "" {
		if _, statErr := os.Stat(imageIndexPath); statErr == nil {
			inputPaths = append(inputPaths,
				struct{ role, path string }{"image-index", imageIndexPath},
				struct{ role, path string }{"image-content", imageContentPath},
			)
		}
	}

	inputs := make([]inputRecord, 0, len(inputPaths))
	for _, input := range inputPaths {
		record, checksumErr := checksumInput(spec.ID, input.role, input.path)
		if checksumErr != nil {
			return snapshotInventory{}, checksumErr
		}
		inputs = append(inputs, record)
	}

	return snapshotInventory{
		Spec:  spec,
		Pages: pages,
		Validation: validationRecord{
			SnapshotID:       spec.ID,
			Workload:         spec.Workload,
			WSRows:           len(wsPFNs),
			UniquePFNs:       len(uniquePFNs),
			DuplicatePFNs:    len(wsPFNs) - len(uniquePFNs),
			PrivateRows:      privateRows,
			BaseRootfsRows:   baseRows,
			ImageRows:        imageRows,
			UniqueRawPages:   len(uniqueRaw),
			RecipePages:      recipePages,
			PrivateFileBytes: int64(len(privateContent)),
			BaseFileBytes:    baseBytes,
			ImageFileBytes:   imageBytes,
		},
		Inputs: inputs,
	}, nil
}

func resolveImageName(sharedRoot, configured string) (string, error) {
	if configured != "" {
		return configured, nil
	}
	entries, err := os.ReadDir(filepath.Join(sharedRoot, "images"))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("list shared image sources: %w", err)
	}
	names := make(map[string]bool)
	for _, entry := range entries {
		name := entry.Name()
		switch {
		case strings.HasSuffix(name, "_index"):
			names[strings.TrimSuffix(name, "_index")] = true
		case strings.HasSuffix(name, "_content"):
			names[strings.TrimSuffix(name, "_content")] = true
		}
	}
	if len(names) == 0 {
		return "", nil
	}
	if len(names) != 1 {
		values := make([]string, 0, len(names))
		for name := range names {
			values = append(values, name)
		}
		sort.Strings(values)
		return "", fmt.Errorf("multiple shared image sources %v; set snapshots[].image explicitly", values)
	}
	for name := range names {
		return name, nil
	}
	return "", nil
}

func readPFNCSV(path string) ([]uint64, error) {
	values, err := readSingleColumnCSV(path, "pfn")
	if err != nil {
		return nil, err
	}
	pfns := make([]uint64, 0, len(values))
	for row, value := range values {
		pfn, parseErr := strconv.ParseUint(value, 10, 64)
		if parseErr != nil {
			return nil, fmt.Errorf("row %d has invalid PFN %q: %w", row+2, value, parseErr)
		}
		pfns = append(pfns, pfn)
	}
	return pfns, nil
}

func readHashCSV(path string) ([]string, error) {
	values, err := readSingleColumnCSV(path, "hash")
	if err != nil {
		return nil, err
	}
	for row, value := range values {
		if len(value) != md5.Size*2 {
			return nil, fmt.Errorf("row %d has invalid MD5 hash length %d", row+2, len(value))
		}
		if _, decodeErr := hex.DecodeString(value); decodeErr != nil {
			return nil, fmt.Errorf("row %d has invalid MD5 hash %q: %w", row+2, value, decodeErr)
		}
		values[row] = strings.ToLower(value)
	}
	return values, nil
}

func readSingleColumnCSV(path, header string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 || len(records[0]) != 1 || records[0][0] != header {
		return nil, fmt.Errorf("expected one-column CSV with %q header", header)
	}
	values := make([]string, 0, len(records)-1)
	for row := 1; row < len(records); row++ {
		if len(records[row]) != 1 || strings.TrimSpace(records[row][0]) == "" {
			return nil, fmt.Errorf("row %d must contain exactly one non-empty value", row+1)
		}
		values = append(values, strings.TrimSpace(records[row][0]))
	}
	return values, nil
}

func readAndValidateSharedSource(indexPath, contentPath string, pageSize int, optional bool) ([]string, int64, error) {
	if indexPath == "" || contentPath == "" {
		if optional {
			return nil, 0, nil
		}
		return nil, 0, fmt.Errorf("shared source paths are empty")
	}
	_, indexErr := os.Stat(indexPath)
	_, contentErr := os.Stat(contentPath)
	if os.IsNotExist(indexErr) && os.IsNotExist(contentErr) && optional {
		return nil, 0, nil
	}
	if indexErr != nil {
		return nil, 0, fmt.Errorf("stat index: %w", indexErr)
	}
	if contentErr != nil {
		return nil, 0, fmt.Errorf("stat content: %w", contentErr)
	}

	hashes, err := readHashCSV(indexPath)
	if err != nil {
		return nil, 0, err
	}
	content, err := os.ReadFile(contentPath)
	if err != nil {
		return nil, 0, err
	}
	if len(content) != len(hashes)*pageSize {
		return nil, 0, fmt.Errorf("content is %d bytes for %d hashes; expected %d", len(content), len(hashes), len(hashes)*pageSize)
	}
	seen := make(map[string]bool, len(hashes))
	for i, expected := range hashes {
		if seen[expected] {
			return nil, 0, fmt.Errorf("shared index repeats hash %s", expected)
		}
		seen[expected] = true
		start := i * pageSize
		sum := md5.Sum(content[start : start+pageSize])
		actual := hex.EncodeToString(sum[:])
		if actual != expected {
			return nil, 0, fmt.Errorf("content page %d hashes to %s, index says %s", i, actual, expected)
		}
	}
	return hashes, int64(len(content)), nil
}

func recipeHashAt(recipe []byte, pfn uint64) string {
	start := int(pfn) * md5.Size
	return hex.EncodeToString(recipe[start : start+md5.Size])
}

func stringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

func checksumInput(snapshotID, role, path string) (inputRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return inputRecord{}, fmt.Errorf("open %s for checksum: %w", path, err)
	}
	defer file.Close()

	hash := sha256.New()
	bytes, err := io.Copy(hash, file)
	if err != nil {
		return inputRecord{}, fmt.Errorf("checksum %s: %w", path, err)
	}
	return inputRecord{
		SnapshotID: snapshotID,
		Role:       role,
		Path:       path,
		Bytes:      bytes,
		SHA256:     hex.EncodeToString(hash.Sum(nil)),
	}, nil
}

func evaluatePolicies(inventories []snapshotInventory, pageSize, cacheCapacity int) ([]summaryRecord, []traceRecord) {
	pageOccurrences := 0
	for _, inventory := range inventories {
		pageOccurrences += len(inventory.Pages)
	}

	uniqueCounts := make(map[policy]int, len(policies))
	for _, candidate := range policies {
		keys := make(map[string]bool)
		for _, inventory := range inventories {
			for _, page := range inventory.Pages {
				keys[policyKey(candidate, page)] = true
			}
		}
		uniqueCounts[candidate] = len(keys)
	}

	trace := make([]traceRecord, 0, len(policies)*len(inventories))
	cacheTotals := make(map[policy]struct{ hits, misses int }, len(policies))
	for _, candidate := range policies {
		cache := newLRUCache(cacheCapacity)
		for restoreIndex, inventory := range inventories {
			keys := orderedUniquePolicyKeys(candidate, inventory.Pages)
			hits := 0
			misses := 0
			for _, key := range keys {
				if cache.access(key) {
					hits++
				} else {
					misses++
				}
			}
			totals := cacheTotals[candidate]
			totals.hits += hits
			totals.misses += misses
			cacheTotals[candidate] = totals
			trace = append(trace, traceRecord{
				Policy:             candidate,
				RestoreIndex:       restoreIndex,
				SnapshotID:         inventory.Spec.ID,
				Workload:           inventory.Spec.Workload,
				LogicalUniquePages: len(keys),
				FetchPages:         misses,
				FetchBytes:         int64(misses * pageSize),
				CacheHits:          hits,
				CacheMisses:        misses,
				ResidentPagesAfter: cache.len(),
			})
		}
	}

	currentPages := uniqueCounts[policyCurrent]
	summaries := make([]summaryRecord, 0, len(policies))
	for _, candidate := range policies {
		deltaPages := currentPages - uniqueCounts[candidate]
		deltaPercent := 0.0
		if currentPages > 0 {
			deltaPercent = float64(deltaPages) / float64(currentPages) * 100
		}
		totals := cacheTotals[candidate]
		summaries = append(summaries, summaryRecord{
			Policy:                candidate,
			PageSize:              pageSize,
			SnapshotCount:         len(inventories),
			PageOccurrences:       pageOccurrences,
			LogicalUniquePages:    uniqueCounts[candidate],
			LogicalUniqueBytes:    int64(uniqueCounts[candidate] * pageSize),
			DeltaPagesVsCurrent:   deltaPages,
			DeltaBytesVsCurrent:   int64(deltaPages * pageSize),
			DeltaPercentVsCurrent: deltaPercent,
			CacheCapacityPages:    cacheCapacity,
			FetchPages:            totals.misses,
			FetchBytes:            int64(totals.misses * pageSize),
			CacheHits:             totals.hits,
			CacheMisses:           totals.misses,
		})
	}
	return summaries, trace
}

func policyKey(candidate policy, page pageRecord) string {
	switch candidate {
	case policyFull:
		return "global:" + page.RawHash
	case policyCurrent:
		if page.Provenance == provenancePrivate {
			return "revision:" + page.SnapshotID + ":" + page.RawHash
		}
		return "global:" + page.RawHash
	case policyNoImage:
		if page.Provenance == provenanceBaseRootfs {
			return "global:" + page.RawHash
		}
		return "revision:" + page.SnapshotID + ":" + page.RawHash
	default:
		panic(fmt.Sprintf("unsupported policy %q", candidate))
	}
}

func orderedUniquePolicyKeys(candidate policy, pages []pageRecord) []string {
	seen := make(map[string]bool, len(pages))
	keys := make([]string, 0, len(pages))
	for _, page := range pages {
		key := policyKey(candidate, page)
		if seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	return keys
}

type lruCache struct {
	capacity int
	order    *list.List
	entries  map[string]*list.Element
}

func newLRUCache(capacity int) *lruCache {
	return &lruCache{capacity: capacity, order: list.New(), entries: make(map[string]*list.Element)}
}

func (cache *lruCache) access(key string) bool {
	if element, ok := cache.entries[key]; ok {
		cache.order.MoveToFront(element)
		return true
	}
	element := cache.order.PushFront(key)
	cache.entries[key] = element
	if cache.capacity > 0 && cache.order.Len() > cache.capacity {
		oldest := cache.order.Back()
		delete(cache.entries, oldest.Value.(string))
		cache.order.Remove(oldest)
	}
	return false
}

func (cache *lruCache) len() int {
	return cache.order.Len()
}
