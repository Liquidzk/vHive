// reframe_zstd_ws converts coalesced working-set objects from a multi-frame
// Zstandard representation to one ordered Zstandard stream.  It intentionally
// leaves recipes, page lists, snapshot metadata, and lazy chunk objects alone.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/vhive-serverless/vhive/snapshotting/zstdstream"
)

type snapshotList struct {
	Workloads []struct {
		Snapshot string `json:"snapshot"`
	} `json:"workloads"`
}

type snapshotReport struct {
	Snapshot           string `json:"snapshot"`
	ObjectStem         string `json:"object_stem"`
	RawBytes           int64  `json:"raw_bytes"`
	OldCompressedBytes int64  `json:"old_compressed_bytes"`
	NewCompressedBytes int64  `json:"new_compressed_bytes"`
	OldFrames          int    `json:"old_frames"`
	NewFrames          int    `json:"new_frames"`
	AliasesUpdated     int    `json:"aliases_updated"`
	RawSHA256          string `json:"raw_sha256"`
}

type runReport struct {
	Endpoint       string           `json:"endpoint"`
	Bucket         string           `json:"bucket"`
	BackupBucket   string           `json:"backup_bucket"`
	ObjectStem     string           `json:"object_stem"`
	AliasTag       string           `json:"alias_tag,omitempty"`
	AliasSlots     int              `json:"alias_slots"`
	ZstdLevel      int              `json:"zstd_level"`
	StartedAt      string           `json:"started_at"`
	CompletedAt    string           `json:"completed_at"`
	Snapshots      []snapshotReport `json:"snapshots"`
	TotalRawBytes  int64            `json:"total_raw_bytes"`
	TotalOldBytes  int64            `json:"total_old_compressed_bytes"`
	TotalNewBytes  int64            `json:"total_new_compressed_bytes"`
	AliasesUpdated int              `json:"aliases_updated"`
}

func readObject(ctx context.Context, client *minio.Client, bucket, key string) ([]byte, error) {
	object, err := client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer object.Close()
	data, err := io.ReadAll(object)
	if err != nil {
		return nil, fmt.Errorf("read %s/%s: %w", bucket, key, err)
	}
	return data, nil
}

func objectExists(ctx context.Context, client *minio.Client, bucket, key string) (bool, error) {
	_, err := client.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err == nil {
		return true, nil
	}
	response := minio.ToErrorResponse(err)
	if response.Code == "NoSuchKey" || response.Code == "NoSuchObject" || response.Code == "NotFound" {
		return false, nil
	}
	return false, err
}

func copyObject(ctx context.Context, client *minio.Client, srcBucket, srcKey, dstBucket, dstKey string) error {
	_, err := client.CopyObject(ctx,
		minio.CopyDestOptions{Bucket: dstBucket, Object: dstKey},
		minio.CopySrcOptions{Bucket: srcBucket, Object: srcKey},
	)
	if err != nil {
		return fmt.Errorf("copy %s/%s to %s/%s: %w", srcBucket, srcKey, dstBucket, dstKey, err)
	}
	return nil
}

func putObject(ctx context.Context, client *minio.Client, bucket, key string, data []byte, contentType string) error {
	_, err := client.PutObject(ctx, bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("put %s/%s: %w", bucket, key, err)
	}
	return nil
}

func ensureBackup(ctx context.Context, client *minio.Client, sourceBucket, backupBucket, key string) error {
	exists, err := objectExists(ctx, client, backupBucket, key)
	if err != nil {
		return fmt.Errorf("stat backup %s/%s: %w", backupBucket, key, err)
	}
	if exists {
		return nil
	}
	return copyObject(ctx, client, sourceBucket, key, backupBucket, key)
}

func decodePayload(manifestData, payload []byte) ([]byte, *zstdstream.Manifest, error) {
	manifest, err := zstdstream.ParseManifest(manifestData)
	if err != nil {
		return nil, nil, err
	}
	if manifest.RawSize > int64(^uint(0)>>1) {
		return nil, nil, fmt.Errorf("raw size %d is not addressable", manifest.RawSize)
	}
	if int64(len(payload)) != manifest.CompressedSize {
		return nil, nil, fmt.Errorf("payload size %d does not match manifest %d", len(payload), manifest.CompressedSize)
	}
	destination := make([]byte, int(manifest.RawSize))
	open := func(offset, length int64) (io.ReadCloser, error) {
		if offset < 0 || length <= 0 || offset+length > int64(len(payload)) {
			return nil, fmt.Errorf("invalid payload range offset=%d length=%d", offset, length)
		}
		return io.NopCloser(bytes.NewReader(payload[offset : offset+length])), nil
	}
	if err := zstdstream.Decode(context.Background(), manifest, open, destination, 8); err != nil {
		return nil, nil, err
	}
	return destination, manifest, nil
}

func main() {
	endpoint := flag.String("minioURL", "", "MinIO endpoint without scheme")
	accessKey := flag.String("minioAccessKey", "minio", "MinIO access key")
	secretKey := flag.String("minioSecretKey", "minio123", "MinIO secret key")
	bucket := flag.String("bucket", "snapshots", "source and active snapshot bucket")
	backupBucket := flag.String("backupBucket", "", "immutable backup bucket for the original framed canonical objects")
	workloadsPath := flag.String("workloads", "", "JSON file containing workloads[].snapshot")
	objectStem := flag.String("objectStem", "", "working_set_pages_content or working_set_pages_content_private")
	aliasTag := flag.String("aliasTag", "", "cold alias tag; required when aliasSlots is positive")
	aliasSlots := flag.Int("aliasSlots", 0, "number of aliases per canonical snapshot to update")
	zstdLevel := flag.Int("zstdLevel", 3, "Zstandard compression level")
	reportPath := flag.String("report", "", "output JSON report")
	flag.Parse()

	if *endpoint == "" || *backupBucket == "" || *workloadsPath == "" || *reportPath == "" {
		fmt.Fprintln(os.Stderr, "minioURL, backupBucket, workloads, and report are required")
		os.Exit(2)
	}
	if *objectStem != "working_set_pages_content" && *objectStem != "working_set_pages_content_private" {
		fmt.Fprintln(os.Stderr, "objectStem must be working_set_pages_content or working_set_pages_content_private")
		os.Exit(2)
	}
	if *aliasSlots < 0 || (*aliasSlots > 0 && *aliasTag == "") {
		fmt.Fprintln(os.Stderr, "aliasSlots must be non-negative and aliasTag is required for aliases")
		os.Exit(2)
	}

	workloadsData, err := os.ReadFile(*workloadsPath)
	if err != nil {
		panic(err)
	}
	var workloads snapshotList
	if err := json.Unmarshal(workloadsData, &workloads); err != nil {
		panic(err)
	}
	if len(workloads.Workloads) == 0 {
		panic("workload list is empty")
	}
	seen := make(map[string]bool)
	for _, workload := range workloads.Workloads {
		if workload.Snapshot == "" || seen[workload.Snapshot] {
			panic("snapshot names must be non-empty and unique")
		}
		seen[workload.Snapshot] = true
	}

	client, err := minio.New(*endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(*accessKey, *secretKey, ""),
		Secure: false,
	})
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, *backupBucket)
	if err != nil {
		panic(err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, *backupBucket, minio.MakeBucketOptions{}); err != nil {
			panic(err)
		}
	}

	report := runReport{
		Endpoint:     *endpoint,
		Bucket:       *bucket,
		BackupBucket: *backupBucket,
		ObjectStem:   *objectStem,
		AliasTag:     *aliasTag,
		AliasSlots:   *aliasSlots,
		ZstdLevel:    *zstdLevel,
		StartedAt:    time.Now().Format(time.RFC3339Nano),
	}

	for _, workload := range workloads.Workloads {
		prefix := workload.Snapshot + "/" + *objectStem
		manifestKey := prefix + ".zstd.json"
		payloadKey := prefix + ".zstd.frames"
		if err := ensureBackup(ctx, client, *bucket, *backupBucket, payloadKey); err != nil {
			panic(err)
		}
		if err := ensureBackup(ctx, client, *bucket, *backupBucket, manifestKey); err != nil {
			panic(err)
		}

		oldManifestData, err := readObject(ctx, client, *backupBucket, manifestKey)
		if err != nil {
			panic(err)
		}
		oldPayload, err := readObject(ctx, client, *backupBucket, payloadKey)
		if err != nil {
			panic(err)
		}
		raw, oldManifest, err := decodePayload(oldManifestData, oldPayload)
		if err != nil {
			panic(fmt.Errorf("decode original %s: %w", workload.Snapshot, err))
		}
		newPayload, newManifest, err := zstdstream.EncodeSingle(raw, *zstdLevel)
		if err != nil {
			panic(err)
		}
		newManifestData, err := zstdstream.MarshalManifest(newManifest)
		if err != nil {
			panic(err)
		}
		verifiedRaw, verifiedManifest, err := decodePayload(newManifestData, newPayload)
		if err != nil {
			panic(fmt.Errorf("verify new %s: %w", workload.Snapshot, err))
		}
		if len(verifiedManifest.Frames) != 1 || !bytes.Equal(raw, verifiedRaw) {
			panic(fmt.Errorf("single-stream verification failed for %s", workload.Snapshot))
		}

		// The manifest is the commit marker and is always replaced last.
		if err := putObject(ctx, client, *bucket, payloadKey, newPayload, "application/octet-stream"); err != nil {
			panic(err)
		}
		if err := putObject(ctx, client, *bucket, manifestKey, newManifestData, "application/json"); err != nil {
			panic(err)
		}

		aliasesUpdated := 0
		for slot := 0; slot < *aliasSlots; slot++ {
			alias := fmt.Sprintf("%s-%s-%d", workload.Snapshot, *aliasTag, slot)
			commitKey := alias + "/snap_file"
			aliasExists, err := objectExists(ctx, client, *bucket, commitKey)
			if err != nil {
				panic(err)
			}
			if !aliasExists {
				panic(fmt.Errorf("alias commit marker is missing: %s", commitKey))
			}
			aliasPayloadKey := alias + "/" + *objectStem + ".zstd.frames"
			aliasManifestKey := alias + "/" + *objectStem + ".zstd.json"
			if err := copyObject(ctx, client, *bucket, payloadKey, *bucket, aliasPayloadKey); err != nil {
				panic(err)
			}
			if err := copyObject(ctx, client, *bucket, manifestKey, *bucket, aliasManifestKey); err != nil {
				panic(err)
			}
			aliasesUpdated++
		}

		sum := sha256.Sum256(raw)
		entry := snapshotReport{
			Snapshot:           workload.Snapshot,
			ObjectStem:         *objectStem,
			RawBytes:           int64(len(raw)),
			OldCompressedBytes: int64(len(oldPayload)),
			NewCompressedBytes: int64(len(newPayload)),
			OldFrames:          len(oldManifest.Frames),
			NewFrames:          len(newManifest.Frames),
			AliasesUpdated:     aliasesUpdated,
			RawSHA256:          hex.EncodeToString(sum[:]),
		}
		report.Snapshots = append(report.Snapshots, entry)
		report.TotalRawBytes += entry.RawBytes
		report.TotalOldBytes += entry.OldCompressedBytes
		report.TotalNewBytes += entry.NewCompressedBytes
		report.AliasesUpdated += aliasesUpdated
		fmt.Printf("REFRAME snapshot=%s old_frames=%d new_frames=%d raw_bytes=%d old_bytes=%d new_bytes=%d aliases=%d\n",
			entry.Snapshot, entry.OldFrames, entry.NewFrames, entry.RawBytes,
			entry.OldCompressedBytes, entry.NewCompressedBytes, entry.AliasesUpdated)
	}

	report.CompletedAt = time.Now().Format(time.RFC3339Nano)
	reportData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(*reportPath, append(reportData, '\n'), 0644); err != nil {
		panic(err)
	}
	fmt.Printf("REFRAME_COMPLETE snapshots=%d aliases=%d raw_bytes=%d old_bytes=%d new_bytes=%d backup_bucket=%s\n",
		len(report.Snapshots), report.AliasesUpdated, report.TotalRawBytes,
		report.TotalOldBytes, report.TotalNewBytes, strings.TrimSpace(*backupBucket))
}
