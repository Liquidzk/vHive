package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
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
	orch       *ctriface.Orchestrator
	snapMgr    *snapshotting.SnapshotManager
	imageMap   map[string]string
	relayPort  = 0
	mu         = &sync.Mutex{}
	cleaning   *bool
	baseSnap   *bool
	dropCaches *bool
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
	log.Debugf("env vars: %v, args: %v", envArr, argsArr)

	var resp *ctriface.StartVMResponse
	var err error
	var snap *snapshotting.Snapshot
	var metric *metrics.Metric

	var ok bool
	if snap, err = snapMgr.AcquireSnapshot(rev); err == nil { // local case
		log.Debugf("Using snapshot for rev %s", rev)
		resp, metric, err = orch.LoadSnapshot(ctx, snap, false, false)
		if err == nil && metric != nil {
			log.Debugf("Loaded snapshot for rev %s in %v", rev, metric.Total())
			metric.PrintAll()
		}
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
		log.Debugf("Snapshot Load Result: metric: %p", metric)
		if metric != nil {
			log.Debugf("Loaded snapshot for rev %s in %v", rev, metric.Total())
			metric.PrintAll()
		}
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
	functionEndpoint := resp.GuestIP + ":50051"
	if err := waitForTCP(ctx, functionEndpoint, functionReadyTimeout); err != nil {
		log.Errorf("function readiness check failed for VM %s: %v", vmId, err)
		if stopErr := orch.StopSingleVM(ctx, vmId); stopErr != nil {
			log.Errorf("failed to stop unready VM %s: %v", vmId, stopErr)
		}
		http.Error(w, fmt.Sprintf("Function Readiness Error: %v", err), http.StatusServiceUnavailable)
		return
	}
	log.Debugf("function endpoint %s is ready", functionEndpoint)

	relayArgs := r.Header.Get("relayArgs")
	endpoint := functionEndpoint
	if relayArgs != "" {
		mu.Lock()
		relayPort++
		port := 50000 + relayPort%5000
		mu.Unlock()

		endpoint = fmt.Sprintf("localhost:%d", port)
		relayArgs = strings.Replace(relayArgs, "--addr=0.0.0.0:50000", "--addr="+endpoint, 1)
		relayArgs = strings.Replace(relayArgs, "--function-endpoint-url=0.0.0.0", "--function-endpoint-url="+resp.GuestIP, 1)
		log.Debugf("Relay args: %s", relayArgs)

		go func() {
			cmd := exec.CommandContext(
				relayCtx,
				homeDir+"/vswarm/tools/relay/server",
				strings.Split(relayArgs, " ")...,
			)

			out, err := cmd.CombinedOutput()

			log.Debugf("vswarm relay output:\n%s\n", out)

			if err != nil {
				fmt.Printf("vswarm relay error: %v\n", err)
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
	isWSRecording := flag.Bool("wsRecording", false, "Enable recording of working set pages accessed during function execution")
	planBPrivateWS := flag.Bool("planBPrivateWS", false, "Compress and restore the private working set through the optional Plan B codec")
	planBCodec := flag.String("planBCodec", "iaa_deflate", "Plan B codec: iaa_deflate, sw_deflate, zstd_1, or zstd_3")
	planBJobs := flag.Uint("planBJobs", 1, "Maximum concurrent IAA jobs for the Plan B codec")
	hostIface := flag.String("hostIface", "", "Host net-interface for the VMs to bind to for internet access")
	netPoolSize := flag.Int("netPoolSize", 10, "Amount of network configs to preallocate in a pool")
	vethPrefix := flag.String("vethPrefix", "172.17", "Prefix for IP addresses of veth devices, expected subnet is /16")
	clonePrefix := flag.String("clonePrefix", "172.18", "Prefix for node-accessible IP addresses of uVMs, expected subnet is /16")
	vmMemSizeMib := flag.Uint("vmMemSizeMib", 512, "Memory size in MiB for newly created microVMs")
	dockerCredentials := flag.String("dockerCredentials", `{"docker-credentials":{"ghcr.io":{"username":"","password":""}}}`, "Docker credentials for pulling images from inside a microVM") // https://github.com/firecracker-microvm/firecracker-containerd/blob/main/docker-credential-mmds
	minioCredentials := flag.String("minioCredentials", "10.0.1.1:9000;minio;minio123", "Minio credentials for uploading/downloading remote firecracker snapshots. Format: <minioAddr>;<minioAccessKey>;<minioSecretKey>")
	endpoint := flag.String("endpoint", "localhost:8080", "Endpoint for the relay server")
	chunkSize := flag.Uint64("chunkSize", 512*1024, "Chunk size in bytes for memory file uploads and downloads when chunking is enabled")
	cacheSize := flag.Uint64("cacheSize", 15000, "Size of the cache for memory file chunks when chunking is enabled")
	cleaning = flag.Bool("clean", false, "Clean existing snapshots after each invocation")
	dropCaches = flag.Bool("dropCaches", false, "Drop Linux page caches after each invocation teardown")
	security := flag.String("security", "none", "Snapshot security mode: none, full")
	baseSnap = flag.Bool("baseSnap", false, "Use base snapshot of booted VM for snapshot creation")
	threads := flag.Int("j", 8, "How many concurrent uploads/downloads to run when transferring snapshots")
	encryption := flag.Bool("encryption", false, "Enable snapshot encryption")
	flag.Parse()
	if *vmMemSizeMib == 0 || uint64(*vmMemSizeMib) > uint64(^uint32(0)) {
		log.Fatalf("vmMemSizeMib must be between 1 and %d", uint64(^uint32(0)))
	}
	if *planBJobs == 0 || *planBJobs > uint(^uint8(0)) {
		log.Fatalf("planBJobs must be between 1 and %d", uint(^uint8(0)))
	}
	if *planBPrivateWS && (!*isUPFEnabled || !*isLazyMode || !*isWSEnabled || !*isWSCoalescing) {
		log.Fatal("planBPrivateWS requires -upf -lazy -ws -wsCoalescing")
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
		ctriface.WithVMMemSizeMib(uint32(*vmMemSizeMib)),
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
	)
	// defer orch.Cleanup()
	snapMgr = orch.GetSnapshotManager()
	if *planBPrivateWS {
		if err := snapMgr.ConfigurePlanB(*planBCodec, uint8(*planBJobs)); err != nil {
			log.Fatalf("failed to enable Plan B private working set: %v", err)
		}
	}
	time.Sleep(1 * time.Second) // Wait for orchestrator to fully initialize

	if *baseSnap {
		orch.PrepareBaseSnapshot(context.Background())
	}

	s := &http.Server{Addr: *endpoint, Handler: h2c.NewHandler(http.HandlerFunc(handler), &http2.Server{})}
	s.ListenAndServe()
	// http.HandleFunc("/", handler)
	// http.ListenAndServe(":8080", nil)
}
