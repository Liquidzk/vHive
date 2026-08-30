package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	ctrdlog "github.com/containerd/containerd/log"
	log "github.com/sirupsen/logrus"
	"github.com/vhive-serverless/vhive/ctriface"
	"github.com/vhive-serverless/vhive/metrics"
	"github.com/vhive-serverless/vhive/snapshotting"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	pkghttp "knative.dev/serving/pkg/http"
)

var (
	homeDir, _ = os.UserHomeDir()
	// snapDir = "/tmp/snapshots"
	snapDir  = homeDir + "/snapshots"
	vhiveDir = homeDir + "/vhive"
)

var (
	orch            *ctriface.Orchestrator
	snapMgr         *snapshotting.SnapshotManager
	imageMap        map[string]string
	mu              = &sync.Mutex{}
	cleaning        *bool
	baseSnap        *bool
	dropCaches      *bool
	requireSnapshot *bool
)

const (
	functionReadyTimeout = 60 * time.Second
	relayReadyTimeout    = 10 * time.Second
	readyRetryInterval   = 100 * time.Millisecond
)

func waitForTCP(ctx context.Context, address string, timeout time.Duration) error {
	readyCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	dialer := &net.Dialer{Timeout: time.Second}
	var lastErr error
	for {
		conn, err := dialer.DialContext(readyCtx, "tcp", address)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		lastErr = err

		select {
		case <-readyCtx.Done():
			return fmt.Errorf("timed out waiting for %s: %w (last dial error: %v)", address, readyCtx.Err(), lastErr)
		case <-time.After(readyRetryInterval):
		}
	}
}

// allocateLoopbackEndpoint asks the kernel for a currently unused loopback
// port. The listener is closed immediately before the auxiliary relay is
// started. Callers serialize allocation and process start with mu so two
// concurrent requests in this relay cannot select the same endpoint during
// that short hand-off window.
func allocateLoopbackEndpoint() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("allocate auxiliary relay endpoint: %w", err)
	}

	endpoint := listener.Addr().String()
	if err := listener.Close(); err != nil {
		return "", fmt.Errorf("release auxiliary relay endpoint %s: %w", endpoint, err)
	}
	return endpoint, nil
}

func parseFunctionPort(value, source string) (string, error) {
	port, err := strconv.ParseUint(value, 10, 16)
	if err != nil || port == 0 {
		return "", fmt.Errorf("invalid %s %q", source, value)
	}
	return strconv.FormatUint(port, 10), nil
}

func functionEndpointPort(directPort, relayArgs string) (string, error) {
	const defaultPort = "50051"
	if directPort != "" {
		if relayArgs != "" {
			return "", fmt.Errorf("functionPort and relayArgs are mutually exclusive")
		}
		return parseFunctionPort(directPort, "functionPort")
	}
	fields := strings.Fields(relayArgs)
	for i, field := range fields {
		var value string
		switch {
		case strings.HasPrefix(field, "--function-endpoint-port="):
			value = strings.TrimPrefix(field, "--function-endpoint-port=")
		case field == "--function-endpoint-port":
			if i+1 >= len(fields) {
				return "", fmt.Errorf("--function-endpoint-port requires a value")
			}
			value = fields[i+1]
		default:
			continue
		}

		return parseFunctionPort(value, "--function-endpoint-port")
	}
	return defaultPort, nil
}

func loadVMMemSizeMap(path string) (map[string]uint32, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read VM memory map: %w", err)
	}
	vmMemSizeBySnapshot := make(map[string]uint32)
	if err := json.Unmarshal(data, &vmMemSizeBySnapshot); err != nil {
		return nil, fmt.Errorf("parse VM memory map: %w", err)
	}
	if len(vmMemSizeBySnapshot) == 0 {
		return nil, fmt.Errorf("VM memory map is empty")
	}
	for revision, vmMemSizeMib := range vmMemSizeBySnapshot {
		if strings.TrimSpace(revision) == "" {
			return nil, fmt.Errorf("VM memory map contains an empty revision")
		}
		if vmMemSizeMib == 0 {
			return nil, fmt.Errorf("VM memory map contains zero MiB for revision %q", revision)
		}
	}
	return vmMemSizeBySnapshot, nil
}

func grpcStatus(header http.Header) string {
	status := header.Get("Grpc-Status")
	if status == "" {
		status = header.Get(http.TrailerPrefix + "Grpc-Status")
	}
	return status
}

func dropLinuxPageCaches() {
	if err := exec.Command("sync").Run(); err != nil {
		log.Warnf("failed to sync before dropping page caches: %v", err)
		return
	}

	if err := os.WriteFile("/proc/sys/vm/drop_caches", []byte("3\n"), 0o644); err != nil {
		log.Warnf("failed to drop page caches (requires privileges): %v", err)
		return
	}

	log.Debug("dropped Linux page caches")
}

// statusRecorder wraps http.ResponseWriter to capture the status code
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	log.Debugf("request received, image %s, revision %s", r.Header.Get("image"), r.Header.Get("revision"))
	startTime := time.Now()

	ctx := context.Background()
	relayCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	image := r.Header.Get("image")
	if mapped, ok := imageMap[image]; ok {
		image = mapped
	}
	rev := r.Header.Get("revision")
	if rev == "" {
		rev = "default"
	} else {
		rev = strings.Join(strings.Split(rev, "-")[:len(strings.Split(rev, "-"))-2], "-") // remove the unique suffix added by Knative
	}
	env := r.Header.Get("env")
	envArr := []string{}
	if env != "" {
		envArr = strings.Split(env, "|")
	}
	args := r.Header.Get("args")
	argsArr := []string{}
	if args != "" {
		argsArr = strings.Split(args, " ")
	}
	relayArgs := r.Header.Get("relayArgs")
	functionPort, parseErr := functionEndpointPort(r.Header.Get("functionPort"), relayArgs)
	if parseErr != nil {
		http.Error(w, fmt.Sprintf("Invalid Relay Args: %v", parseErr), http.StatusBadRequest)
		return
	}
	log.Debugf("env vars: %v, args: %v", envArr, argsArr)

	var resp *ctriface.StartVMResponse
	var err error
	var snap *snapshotting.Snapshot
	var metric *metrics.Metric

	var ok bool
	if snap, err = snapMgr.AcquireSnapshot(rev); err == nil { // local case
		log.Debugf("Using snapshot for rev %s", rev)
		resp, metric, err = orch.LoadSnapshot(ctx, snap, false, false)
		if err != nil || metric == nil {
			log.Errorf("LoadSnapshot error is %v; metric: %p", err, metric)
			http.Error(w, fmt.Sprintf("Snapshot Load Error, metric: %p", metric), http.StatusInternalServerError)
			return
		}
		log.Debugf("Loaded snapshot for rev %s in %v", rev, metric.Total())
		metric.PrintAll()
	} else if ok, err = snapMgr.SnapshotExists(rev); err == nil && ok { // remote case
		log.Debugf("Using remote snapshot for rev %s", rev)
		startDownload := time.Now()
		snap, err = snapMgr.DownloadSnapshot(rev)
		if err != nil {
			log.Errorf("DownloadSnapshot error is %v", err)
		}
		if snap == nil {
			log.Errorf("DownloadSnapshot snap is nil without error!")
		}
		downloadDelay := time.Since(startDownload)
		log.Debugf("Downloaded snapshot for rev %s in %v", rev, downloadDelay.Microseconds())
		if err != nil || snap == nil {
			http.Error(w, fmt.Sprintf("Snapshot Download Error, snap: %p", snap), http.StatusInternalServerError)
			return
		}
		resp, metric, err = orch.LoadSnapshot(ctx, snap, false, false)
		if err != nil {
			log.Errorf("LoadSnapshot error is %v", err)
			http.Error(w, fmt.Sprintf("Snapshot Load Error, metric: %p", metric), http.StatusInternalServerError)
			return
		}
		if metric == nil {
			log.Error("LoadSnapshot returned a nil metric without an error")
			http.Error(w, "Snapshot Load Error, nil metric", http.StatusInternalServerError)
			return
		}
		log.Debugf("Snapshot Load Result: metric: %p", metric)
		log.Debugf("Loaded snapshot for rev %s in %v", rev, metric.Total())
		metric.PrintAll()
	} else if *requireSnapshot {
		if err != nil {
			log.Errorf("Snapshot lookup failed for required revision %s: %v", rev, err)
			http.Error(w, fmt.Sprintf("Required Snapshot Lookup Error: %v", err), http.StatusServiceUnavailable)
		} else {
			log.Errorf("Required snapshot is missing for revision %s", rev)
			http.Error(w, fmt.Sprintf("Required Snapshot Missing: %s", rev), http.StatusServiceUnavailable)
		}
		return
	} else if *baseSnap { // start from base snapshot case
		log.Debugf("No snapshot for rev %s, starting from base snapshot", rev)
		resp, err = orch.StartWithBaseSnapshot(ctx, image, envArr, argsArr)
	} else { // boot case
		log.Debugf("No snapshot for rev %s, starting from image", rev)
		resp, _, err = orch.StartVMWithEnvironment(ctx, image, envArr, argsArr)
	}
	if err != nil {
		http.Error(w, fmt.Sprintf("Server Error: %v", err), http.StatusInternalServerError)
		log.Errorf("Start VM error: %v", err)
		// cancel()
		return
	}

	vmId := resp.VMID

	log.Debugf("created VM with ID %s and IP %s for revision %s", resp.VMID, resp.GuestIP, r.Header.Get("revision"))
	functionEndpoint := net.JoinHostPort(resp.GuestIP, functionPort)
	endpoint := functionEndpoint
	if relayArgs != "" {
		// The auxiliary vSwarm relay cannot establish its downstream gRPC
		// connection until the restored function is listening.  Keep this
		// guard only for that compatibility path.  The paper's request path
		// carries the function RPC directly and must be proxied immediately;
		// an unconditional readiness poll adds a 100-ms quantization delay and
		// changes the measured execution path.
		if err := waitForTCP(ctx, functionEndpoint, functionReadyTimeout); err != nil {
			log.Errorf("function readiness check failed for VM %s: %v", vmId, err)
			if stopErr := orch.StopSingleVM(ctx, vmId); stopErr != nil {
				log.Errorf("failed to stop unready VM %s: %v", vmId, stopErr)
			}
			http.Error(w, fmt.Sprintf("Function Readiness Error: %v", err), http.StatusServiceUnavailable)
			return
		}
		log.Debugf("function endpoint %s is ready", functionEndpoint)

		// The outer relay is restarted for every formal-evaluation point. A
		// process-local counter therefore reused ports 50001..50060 across
		// points and could collide with an auxiliary relay that was still
		// exiting. Ask the kernel for a currently free port instead, and keep
		// selection plus process start serialized until the child has inherited
		// its arguments.
		mu.Lock()
		endpoint, err = allocateLoopbackEndpoint()
		if err != nil {
			mu.Unlock()
			log.Errorf("failed to allocate auxiliary relay endpoint for VM %s: %v", vmId, err)
			if stopErr := orch.StopSingleVM(ctx, vmId); stopErr != nil {
				log.Errorf("failed to stop VM %s after relay endpoint allocation failure: %v", vmId, stopErr)
			}
			http.Error(w, fmt.Sprintf("Relay Endpoint Error: %v", err), http.StatusServiceUnavailable)
			return
		}

		relayArgs = strings.Replace(relayArgs, "--addr=0.0.0.0:50000", "--addr="+endpoint, 1)
		relayArgs = strings.Replace(relayArgs, "--function-endpoint-url=0.0.0.0", "--function-endpoint-url="+resp.GuestIP, 1)
		log.Debugf("Relay args: %s", relayArgs)

		cmd := exec.CommandContext(
			relayCtx,
			homeDir+"/vswarm/tools/relay/server",
			strings.Split(relayArgs, " ")...,
		)
		var relayOutput bytes.Buffer
		cmd.Stdout = &relayOutput
		cmd.Stderr = &relayOutput
		startErr := cmd.Start()
		mu.Unlock()
		if startErr != nil {
			log.Errorf("failed to start auxiliary relay for VM %s at %s: %v", vmId, endpoint, startErr)
			if stopErr := orch.StopSingleVM(ctx, vmId); stopErr != nil {
				log.Errorf("failed to stop VM %s after relay start failure: %v", vmId, stopErr)
			}
			http.Error(w, fmt.Sprintf("Relay Start Error: %v", startErr), http.StatusServiceUnavailable)
			return
		}
		go func() {
			waitErr := cmd.Wait()
			log.Debugf("vswarm relay output:\n%s\n", relayOutput.String())
			if waitErr != nil {
				fmt.Printf("vswarm relay error: %v\n", waitErr)
			}
		}()

		if err := waitForTCP(ctx, endpoint, relayReadyTimeout); err != nil {
			log.Errorf("vswarm relay readiness check failed for VM %s: %v", vmId, err)
			if stopErr := orch.StopSingleVM(ctx, vmId); stopErr != nil {
				log.Errorf("failed to stop VM %s after relay readiness failure: %v", vmId, stopErr)
			}
			http.Error(w, fmt.Sprintf("Relay Readiness Error: %v", err), http.StatusServiceUnavailable)
			return
		}
		log.Debugf("vswarm relay endpoint %s is ready", endpoint)
	}

	log.Debugf("Sending invocation to %s", vmId)

	proxy := pkghttp.NewHeaderPruningReverseProxy(endpoint, pkghttp.NoHostOverride, []string{}, false /* use HTTP */)
	proxy.Transport = &http2.Transport{
		AllowHTTP: true,
		DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr)
		},
	}
	recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	proxy.ServeHTTP(recorder, r)
	invocationGRPCStatus := grpcStatus(recorder.Header())
	invocationOK := recorder.status >= http.StatusOK && recorder.status < http.StatusMultipleChoices &&
		(invocationGRPCStatus == "" || invocationGRPCStatus == "0")

	log.Debugf("Invocation to %s completed in %v with HTTP status %d and gRPC status %q", vmId, time.Since(startTime), recorder.status, invocationGRPCStatus)

	go func() {
		log.Debugf("removing %s", vmId)
		if snap == nil && err == nil && invocationOK {
			snap, err = snapMgr.InitSnapshot(rev, image)
			if err != nil && strings.Contains(err.Error(), "Snapshot") && strings.Contains(err.Error(), "already exists") {
				return
			}
			orch.PauseVM(ctx, vmId)
			orch.CreateSnapshot(ctx, vmId, snap)
			snapMgr.CommitSnapshot(rev)
			if err := snapMgr.UploadSnapshot(rev); err != nil {
				log.Errorf("upload error: %v", err)
			}
			log.Debugf("finished snapshotting %s", vmId)
		} else if snap == nil && !invocationOK {
			log.Warnf("not snapshotting VM %s after failed invocation (HTTP status %d, gRPC status %q)", vmId, recorder.status, invocationGRPCStatus)
		}
		orch.StopSingleVM(ctx, vmId)
		if *cleaning {
			snapMgr.DeleteSnapshot(rev)
		}
		if *dropCaches {
			dropLinuxPageCaches()
		}
		// cancel()
	}()
}

func main() {
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.TextFormatter{TimestampFormat: "2006-01-02T15:04:05.999", FullTimestamp: true})

	snapshotter := flag.String("ss", "devmapper", "snapshotter name")
	debug := flag.Bool("dbg", false, "Enable debug logging")

	isSaveMemory := flag.Bool("ms", false, "Enable memory saving")
	snapshotMode := flag.String("snapshots", "disabled", "Use VM snapshots when adding function instances, valid options: disabled, local, remote")
	cacheSnaps := flag.Bool("cacheSnaps", true, "Keep remote snapshots cached localy for future use")
	isUPFEnabled := flag.Bool("upf", false, "Enable user-level page faults guest memory management")
	isChunkingEnabled := flag.Bool("chunking", false, "Enable chunking for memory file uploads and downloads")
	isMetricsMode := flag.Bool("metrics", false, "Calculate UPF metrics")
	pinnedFuncNum := flag.Int("hn", 0, "Number of functions pinned in memory (IDs from 0 to X)")
	isLazyMode := flag.Bool("lazy", false, "Enable lazy serving mode when UPFs are enabled")
	isWSEnabled := flag.Bool("ws", false, "Enable working set pulling for UPFs in lazy mode")
	isWSCoalescing := flag.Bool("wsCoalescing", false, "Enable coalescing of working set pulls for multiple UPF-enabled VMs")
	isWSCompression := flag.Bool("wsCompression", false, "Store coalesced private/full working sets as independently framed Zstd")
	isChunkCompression := flag.Bool("chunkCompression", false, "Store each snapshot chunk as an independent Zstd frame")
	zstdLevel := flag.Int("zstdLevel", snapshotting.DefaultZstdLevel, "Zstd compression level")
	zstdFrameSize := flag.Int64("zstdFrameSize", snapshotting.DefaultZstdFrameSize, "Uncompressed bytes per independent WS Zstd frame; must be 4-KiB aligned")
	zstdFetchers := flag.Int("zstdFetchers", snapshotting.DefaultZstdFetchers, "Maximum concurrent Zstd frame range GET/decode workers")
	isWSRecording := flag.Bool("wsRecording", false, "Enable recording of working set pages accessed during function execution")
	hostIface := flag.String("hostIface", "", "Host net-interface for the VMs to bind to for internet access")
	netPoolSize := flag.Int("netPoolSize", 10, "Amount of network configs to preallocate in a pool")
	vethPrefix := flag.String("vethPrefix", "172.17", "Prefix for IP addresses of veth devices, expected subnet is /16")
	clonePrefix := flag.String("clonePrefix", "172.18", "Prefix for node-accessible IP addresses of uVMs, expected subnet is /16")
	dnsNameservers := flag.String("dnsNameservers", "", "Comma-separated DNS nameservers for microVMs; empty uses Kubernetes DNS discovery with the existing fallback")
	vmMemSizeMib := flag.Uint("vmMemSizeMib", 512, "Memory size in MiB for newly created microVMs")
	vmMemSizeMapPath := flag.String("vmMemSizeMap", "", "JSON map from snapshot revision to VM memory size in MiB; mapped restores fail closed on missing entries")
	dockerCredentials := flag.String("dockerCredentials", `{"docker-credentials":{"ghcr.io":{"username":"","password":""}}}`, "Docker credentials for pulling images from inside a microVM") // https://github.com/firecracker-microvm/firecracker-containerd/blob/main/docker-credential-mmds
	minioCredentials := flag.String("minioCredentials", "10.0.1.1:9000;minio;minio123", "Minio credentials for uploading/downloading remote firecracker snapshots. Format: <minioAddr>;<minioAccessKey>;<minioSecretKey>")
	endpoint := flag.String("endpoint", "localhost:8080", "Endpoint for the relay server")
	chunkSize := flag.Uint64("chunkSize", 512*1024, "Chunk size in bytes for memory file uploads and downloads when chunking is enabled")
	cacheSize := flag.Uint64("cacheSize", 15000, "Size of the cache for memory file chunks when chunking is enabled")
	cleaning = flag.Bool("clean", false, "Clean existing snapshots after each invocation")
	dropCaches = flag.Bool("dropCaches", false, "Drop Linux page caches after each invocation teardown")
	security := flag.String("security", snapshotting.SecurityModeNone,
		"Snapshot security mode: none, full-dedup, partial, no-image-sharing, full")
	baseSnap = flag.Bool("baseSnap", false, "Use base snapshot of booted VM for snapshot creation")
	requireSnapshot = flag.Bool("requireSnapshot", false, "Fail instead of creating a VM when the requested snapshot is missing")
	threads := flag.Int("j", 8, "How many concurrent uploads/downloads to run when transferring snapshots")
	encryption := flag.Bool("encryption", false, "Enable snapshot encryption")
	flag.Parse()
	if *vmMemSizeMib == 0 || uint64(*vmMemSizeMib) > uint64(^uint32(0)) {
		log.Fatalf("vmMemSizeMib must be between 1 and %d", uint64(^uint32(0)))
	}
	vmMemSizeBySnapshot, err := loadVMMemSizeMap(*vmMemSizeMapPath)
	if err != nil {
		log.Fatalf("invalid vmMemSizeMap: %v", err)
	}
	if len(vmMemSizeBySnapshot) > 0 && *baseSnap {
		log.Fatal("-vmMemSizeMap cannot be combined with -baseSnap; the map is for pre-existing mixed-memory snapshots")
	}
	*security = snapshotting.NormalizeSecurityMode(*security)
	if !snapshotting.IsValidSecurityMode(*security) {
		log.Fatalf("invalid snapshot security mode %q; expected one of: none, full-dedup, partial, no-image-sharing, full", *security)
	}
	if *security == snapshotting.SecurityModeFullDedup && *isWSCoalescing {
		log.Fatal("full-dedup requires -wsCoalescing=false: coalesced private working-set objects are revision-scoped and would not implement full deduplication")
	}
	if *isWSCompression && !*isWSCoalescing {
		log.Fatal("-wsCompression requires -wsCoalescing")
	}
	if *isChunkCompression && !*isChunkingEnabled {
		log.Fatal("-chunkCompression requires -chunking")
	}
	guestDNS := make([]string, 0)
	for _, candidate := range strings.Split(*dnsNameservers, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if net.ParseIP(candidate) == nil {
			log.Fatalf("invalid microVM DNS nameserver %q", candidate)
		}
		guestDNS = append(guestDNS, candidate)
	}

	imageMap = make(map[string]string)
	data, err := os.ReadFile("image_map.json")
	if err != nil {
		log.Warnf("Could not read image map file: %v", err)
	} else {
		if err := json.Unmarshal(data, &imageMap); err != nil {
			log.Warnf("Could not parse image map JSON: %v", err)
		}
	}

	minioAddr := "localhost:9000"
	minioAccessKey := "minio"
	minioSecretKey := "minio123"
	if *minioCredentials != "" {
		parts := strings.SplitN(*minioCredentials, ";", 3)
		if len(parts) != 3 {
			log.Fatalln("Minio credentials should be in the format <minioAddr>;<minioAccessKey>;<minioSecretKey>")
			return
		}
		minioAddr = parts[0]
		minioAccessKey = parts[1]
		minioSecretKey = parts[2]
	}

	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: ctrdlog.RFC3339NanoFixed,
		FullTimestamp:   true,
	})
	//log.SetReportCaller(true) // FIXME: make sure it's false unless debugging

	log.SetOutput(os.Stdout)

	if *debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug logging is enabled")
	} else {
		log.SetLevel(log.InfoLevel)
	}

	if *isSaveMemory {
		log.Info(fmt.Sprintf("Creating orchestrator for pinned=%d functions", *pinnedFuncNum))
	}

	orch = ctriface.NewOrchestrator(
		*snapshotter,
		*hostIface,
		ctriface.WithTestModeOn(false),
		ctriface.WithSnapshotMode(*snapshotMode),
		ctriface.WithCacheSnaps(*cacheSnaps),
		ctriface.WithUPF(*isUPFEnabled),
		ctriface.WithMetricsMode(*isMetricsMode),
		ctriface.WithLazyMode(*isLazyMode),
		ctriface.WithWSPulling(*isWSEnabled),
		ctriface.WithWSCoalescing(*isWSCoalescing),
		ctriface.WithWSRecording(*isWSRecording),
		ctriface.WithChunkingEnabled(*isChunkingEnabled),
		ctriface.WithChunkSize(*chunkSize),
		ctriface.WithNetPoolSize(*netPoolSize),
		ctriface.WithVethPrefix(*vethPrefix),
		ctriface.WithClonePrefix(*clonePrefix),
		ctriface.WithDNSNameservers(guestDNS),
		ctriface.WithVMMemSizeMib(uint32(*vmMemSizeMib)),
		ctriface.WithVMMemSizeBySnapshot(vmMemSizeBySnapshot),
		ctriface.WithDockerCredentials(*dockerCredentials),
		ctriface.WithMinioAddr(minioAddr),
		ctriface.WithMinioAccessKey(minioAccessKey),
		ctriface.WithMinioSecretKey(minioSecretKey),
		ctriface.WithSnapshotsStorage(snapDir),
		ctriface.WithShimPoolSize(5),
		ctriface.WithCacheSize(*cacheSize),
		ctriface.WithSecurityMode(*security),
		ctriface.WithThreads(*threads),
		ctriface.WithEncryption(*encryption),
		ctriface.WithCleanChunks(*cleaning),
		ctriface.WithCompression(snapshotting.CompressionConfig{
			WorkingSet: *isWSCompression,
			Chunks:     *isChunkCompression,
			Codec:      snapshotting.CompressionCodecZstd,
			Level:      *zstdLevel,
			FrameSize:  *zstdFrameSize,
			Fetchers:   *zstdFetchers,
		}),
	)
	// defer orch.Cleanup()
	snapMgr = orch.GetSnapshotManager()
	log.Infof("SNAPSHARE_COMPRESSION_CONFIG ws=%t chunks=%t codec=zstd level=%d frame_size=%d fetchers=%d",
		*isWSCompression, *isChunkCompression, *zstdLevel, *zstdFrameSize, *zstdFetchers)
	log.Infof("SNAPSHARE_VM_MEMORY_CONFIG default_mib=%d mapped_revisions=%d require_snapshot=%t",
		*vmMemSizeMib, len(vmMemSizeBySnapshot), *requireSnapshot)
	time.Sleep(1 * time.Second) // Wait for orchestrator to fully initialize

	if *baseSnap {
		orch.PrepareBaseSnapshot(context.Background())
	}

	s := &http.Server{Addr: *endpoint, Handler: h2c.NewHandler(http.HandlerFunc(handler), &http2.Server{})}
	s.ListenAndServe()
	// http.HandleFunc("/", handler)
	// http.ListenAndServe(":8080", nil)
}
