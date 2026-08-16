package main

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	composeProject = "bitriver-resilience"
	apiPort        = 28080
)

var longRunningServices = []string{
	"bitriver-live",
	"viewer",
	"postgres",
	"redis",
	"srs-controller",
	"srs",
	"ome",
	"transcoder",
	"transcoder-public",
}

var fixedContainerNames = []string{
	"bitriver-live",
	"bitriver-viewer",
	"bitriver-postgres",
	"bitriver-postgres-host-port",
	"bitriver-srs-controller",
	"bitriver-srs",
	"bitriver-srs-api",
	"bitriver-srs-config",
	"bitriver-ome-health-token-check",
	"bitriver-ome",
	"bitriver-transcoder",
	"bitriver-transcoder-public",
}

type signalKind string

const (
	signalAPIUnavailable    signalKind = "api-unavailable"
	signalReadyDegraded     signalKind = "readyz-503"
	signalStatusComponent   signalKind = "status-component-degraded"
	signalViewerUnavailable signalKind = "viewer-unavailable"
)

type scenarioSpec struct {
	Name            string
	Targets         []string
	RecoveryTargets []string
	Signal          signalKind
	Component       string
	ExpectedSignal  string
}

var scenarioSpecs = []scenarioSpec{
	{Name: "api", Targets: []string{"bitriver-live"}, Signal: signalAPIUnavailable, ExpectedSignal: "public API endpoint unavailable"},
	{Name: "postgres", Targets: []string{"postgres"}, Signal: signalReadyDegraded, ExpectedSignal: "/readyz returns HTTP 503"},
	{Name: "redis", Targets: []string{"redis"}, Signal: signalReadyDegraded, ExpectedSignal: "/readyz returns HTTP 503"},
	{Name: "srs_path", Targets: []string{"srs-controller", "srs"}, RecoveryTargets: []string{"srs", "srs-controller"}, Signal: signalStatusComponent, Component: "srs", ExpectedSignal: "authenticated /api/status reports srs non-ready"},
	{Name: "ovenmediaengine", Targets: []string{"ome"}, Signal: signalStatusComponent, Component: "ovenmediaengine", ExpectedSignal: "authenticated /api/status reports ovenmediaengine non-ready"},
	{Name: "transcoder", Targets: []string{"transcoder"}, Signal: signalStatusComponent, Component: "transcoder", ExpectedSignal: "authenticated /api/status reports transcoder non-ready"},
	{Name: "viewer", Targets: []string{"viewer"}, Signal: signalViewerUnavailable, ExpectedSignal: "/viewer returns an upstream failure"},
}

type commandRunner struct {
	logPath string
	secrets []string
	env     []string
}

type cappedTail struct {
	limit int
	data  []byte
}

func (w *cappedTail) Write(payload []byte) (int, error) {
	w.data = append(w.data, payload...)
	if len(w.data) > w.limit {
		w.data = append([]byte{}, w.data[len(w.data)-w.limit:]...)
	}
	return len(payload), nil
}

func (w *cappedTail) String() string {
	return string(w.data)
}

func main() {
	var reportPath string
	var retainWorkdir bool
	var waitSeconds int
	flag.StringVar(&reportPath, "report", filepath.FromSlash(".artifacts/service-resilience/report.json"), "retained JSON evidence path")
	flag.BoolVar(&retainWorkdir, "retain-workdir", false, "retain the private staged tree for debugging")
	flag.IntVar(&waitSeconds, "wait-seconds", 240, "bounded per-signal wait in seconds")
	flag.Parse()

	if waitSeconds < 15 || waitSeconds > 900 {
		fmt.Fprintln(os.Stderr, "error: --wait-seconds must be between 15 and 900")
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := execute(ctx, reportPath, retainWorkdir, time.Duration(waitSeconds)*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "service resilience rehearsal failed: %v\n", err)
		os.Exit(1)
	}
}

func execute(ctx context.Context, reportPath string, retainWorkdir bool, signalTimeout time.Duration) (returnErr error) {
	repoRoot, err := repositoryRoot(ctx)
	if err != nil {
		return err
	}
	reportPath, err = resolveReportPath(repoRoot, reportPath)
	if err != nil {
		return err
	}
	rootEnvPath := filepath.Join(repoRoot, ".env")
	operatorOMEPath := filepath.Join(repoRoot, "deploy", "ome", "Server.generated.xml")
	if _, err := os.Stat(rootEnvPath); err != nil {
		return fmt.Errorf("private root environment is required: %w", err)
	}
	envHashBefore, err := fileSHA256(rootEnvPath)
	if err != nil {
		return fmt.Errorf("hash operator environment: %w", err)
	}
	omeHashBefore, err := fileSHA256(operatorOMEPath)
	if err != nil {
		return fmt.Errorf("hash operator OME configuration: %w", err)
	}
	secretSentinels, err := privateSentinels(rootEnvPath)
	if err != nil {
		return err
	}

	commit, err := commandText(ctx, repoRoot, nil, "git", "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("resolve source commit: %w", err)
	}
	commit = strings.TrimSpace(commit)
	if len(commit) != 40 {
		return fmt.Errorf("unexpected source commit identity")
	}

	if err := preflightPorts(); err != nil {
		return err
	}
	baseTemp := filepath.Join(repoRoot, ".tmp")
	if err := os.MkdirAll(baseTemp, 0o755); err != nil {
		return fmt.Errorf("create private work root: %w", err)
	}
	workdir, err := os.MkdirTemp(baseTemp, "service-resilience-")
	if err != nil {
		return fmt.Errorf("create private work directory: %w", err)
	}
	stageRoot := filepath.Join(workdir, "tree")
	runner := &commandRunner{
		logPath: filepath.Join(workdir, "commands.log"),
		secrets: secretSentinels,
		env:     cleanEnvironment(os.Environ()),
	}
	composeBase := []string{
		"compose",
		"--project-name", composeProject,
		"--env-file", filepath.Join(stageRoot, ".env"),
		"-f", filepath.Join(stageRoot, "deploy", "docker-compose.yml"),
	}
	stackStarted := false
	cleanupComplete := false
	defer func() {
		if !cleanupComplete && stackStarted {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			_ = runner.compose(cleanupCtx, stageRoot, composeBase, "down", "-v", "--remove-orphans", "--timeout", "20")
			cancel()
		}
		if !retainWorkdir {
			_ = os.RemoveAll(workdir)
		}
	}()

	fmt.Printf("[resilience] source commit %s\n", commit[:12])
	if err := runner.run(ctx, repoRoot, "docker", "version"); err != nil {
		return fmt.Errorf("Docker preflight: %w", err)
	}
	if err := preflightContainers(ctx, repoRoot, runner); err != nil {
		return err
	}
	if err := extractTrackedTree(ctx, repoRoot, stageRoot); err != nil {
		return err
	}
	if err := stagePrivateEnvironment(rootEnvPath, filepath.Join(stageRoot, ".env"), commit); err != nil {
		return err
	}
	if err := prepareRuntimeStorage(stageRoot); err != nil {
		return err
	}

	fmt.Println("[resilience] validating and building isolated Compose stack")
	if err := runner.compose(ctx, stageRoot, composeBase, "config", "--quiet"); err != nil {
		return fmt.Errorf("render staged Compose contract: %w", err)
	}
	if err := runner.compose(ctx, stageRoot, composeBase, "build"); err != nil {
		return fmt.Errorf("build staged Compose stack: %w", err)
	}
	stackStarted = true
	if err := runner.compose(ctx, stageRoot, composeBase, "up", "--no-build", "--pull", "never", "-d"); err != nil {
		return fmt.Errorf("start staged Compose stack: %w", err)
	}

	client, err := newAPIClient(fmt.Sprintf("http://127.0.0.1:%d", apiPort))
	if err != nil {
		return err
	}
	baselineCtx, cancelBaseline := context.WithTimeout(ctx, 5*time.Minute)
	if _, err := waitFor(baselineCtx, time.Second, func(probeCtx context.Context) bool {
		return readyRecovered(client.observe(probeCtx, "/readyz"))
	}); err != nil {
		cancelBaseline()
		return fmt.Errorf("baseline readiness: %w", err)
	}
	if _, err := waitFor(baselineCtx, time.Second, func(probeCtx context.Context) bool {
		return pageRecovered(client.observe(probeCtx, "/viewer"))
	}); err != nil {
		cancelBaseline()
		return fmt.Errorf("baseline viewer: %w", err)
	}
	fixture, err := client.createFixture(baselineCtx)
	if err != nil {
		cancelBaseline()
		return err
	}
	secretSentinels = append(secretSentinels, fixture.Email, fixture.Password)
	runner.secrets = secretSentinels
	for _, component := range []string{"srs", "ovenmediaengine", "transcoder"} {
		component := component
		if _, err := waitFor(baselineCtx, time.Second, func(probeCtx context.Context) bool {
			return statusComponent(client.observe(probeCtx, "/api/status"), component, true)
		}); err != nil {
			cancelBaseline()
			return fmt.Errorf("baseline %s status: %w", component, err)
		}
	}
	if _, err := client.verifyDurableState(baselineCtx, fixture); err != nil {
		cancelBaseline()
		return fmt.Errorf("baseline durable state: %w", err)
	}
	cancelBaseline()

	report := newReport(commit)
	for _, spec := range scenarioSpecs {
		fmt.Printf("[resilience] injecting %s outage\n", spec.Name)
		evidence, err := runScenario(ctx, runner, stageRoot, composeBase, client, fixture, spec, signalTimeout)
		if err != nil {
			return fmt.Errorf("scenario %s: %w", spec.Name, err)
		}
		report.Scenarios = append(report.Scenarios, evidence)
		fmt.Printf("[resilience] %s recovered in %.3fs\n", spec.Name, evidence.RecoverySeconds)
	}

	fmt.Println("[resilience] tearing down isolated Compose stack")
	cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), 2*time.Minute)
	if err := runner.compose(cleanupCtx, stageRoot, composeBase, "down", "-v", "--remove-orphans", "--timeout", "20"); err != nil {
		cancelCleanup()
		return fmt.Errorf("tear down staged Compose stack: %w", err)
	}
	if err := assertProjectRemoved(cleanupCtx, repoRoot, runner); err != nil {
		cancelCleanup()
		return err
	}
	cancelCleanup()
	cleanupComplete = true
	stackStarted = false

	envHashAfter, err := fileSHA256(rootEnvPath)
	if err != nil {
		return fmt.Errorf("rehash operator environment: %w", err)
	}
	omeHashAfter, err := fileSHA256(operatorOMEPath)
	if err != nil {
		return fmt.Errorf("rehash operator OME configuration: %w", err)
	}
	report.Isolation.OperatorEnvironmentUnchanged = envHashAfter == envHashBefore
	report.Isolation.OperatorOMEConfigUnchanged = omeHashAfter == omeHashBefore
	report.Isolation.TeardownComplete = cleanupComplete
	if err := writeReport(reportPath, report, secretSentinels); err != nil {
		return fmt.Errorf("publish resilience evidence: %w", err)
	}
	fmt.Printf("[resilience] passed; report: %s\n", reportPath)
	return nil
}

func runScenario(ctx context.Context, runner *commandRunner, stageRoot string, composeBase []string, client *apiClient, fixture fixtureIdentity, spec scenarioSpec, signalTimeout time.Duration) (scenarioEvidence, error) {
	evidence := scenarioEvidence{
		Name:           spec.Name,
		Targets:        append([]string{}, spec.Targets...),
		ExpectedSignal: spec.ExpectedSignal,
	}
	commandCtx, cancelCommand := context.WithTimeout(ctx, 2*time.Minute)
	stopArgs := append([]string{"stop", "--timeout", "20"}, spec.Targets...)
	if err := runner.compose(commandCtx, stageRoot, composeBase, stopArgs...); err != nil {
		cancelCommand()
		return evidence, fmt.Errorf("stop targets: %w", err)
	}
	cancelCommand()

	degradeCtx, cancelDegrade := context.WithTimeout(ctx, signalTimeout)
	degradeDuration, err := waitFor(degradeCtx, time.Second, func(probeCtx context.Context) bool {
		observation := client.observe(probeCtx, signalPath(spec.Signal))
		switch spec.Signal {
		case signalAPIUnavailable, signalViewerUnavailable:
			return unavailable(observation)
		case signalReadyDegraded:
			return readyDegraded(observation)
		case signalStatusComponent:
			return statusComponent(observation, spec.Component, false)
		default:
			return false
		}
	})
	cancelDegrade()
	if err != nil {
		return evidence, fmt.Errorf("observe expected degradation: %w", err)
	}
	evidence.DegradationObserved = true
	evidence.DegradationSeconds = seconds(degradeDuration)

	recoveryTargets := spec.RecoveryTargets
	if len(recoveryTargets) == 0 {
		recoveryTargets = spec.Targets
	}
	recoveryStarted := time.Now()
	startCtx, cancelStart := context.WithTimeout(ctx, 2*time.Minute)
	startArgs := append([]string{"start"}, recoveryTargets...)
	if err := runner.compose(startCtx, stageRoot, composeBase, startArgs...); err != nil {
		cancelStart()
		return evidence, fmt.Errorf("start targets: %w", err)
	}
	cancelStart()

	recoverCtx, cancelRecover := context.WithTimeout(ctx, signalTimeout)
	_, err = waitFor(recoverCtx, time.Second, func(probeCtx context.Context) bool {
		observation := client.observe(probeCtx, signalPath(spec.Signal))
		switch spec.Signal {
		case signalAPIUnavailable, signalReadyDegraded:
			return readyRecovered(observation)
		case signalViewerUnavailable:
			return pageRecovered(observation)
		case signalStatusComponent:
			return statusComponent(observation, spec.Component, true)
		default:
			return false
		}
	})
	cancelRecover()
	if err != nil {
		return evidence, fmt.Errorf("observe expected recovery: %w", err)
	}
	evidence.RecoverySeconds = seconds(time.Since(recoveryStarted))

	invariantCtx, cancelInvariant := context.WithTimeout(ctx, time.Minute)
	evidence.DurableState, err = client.verifyDurableState(invariantCtx, fixture)
	cancelInvariant()
	if err != nil {
		return evidence, err
	}
	first, err := collectRestartCounts(ctx, runner, stageRoot, composeBase)
	if err != nil {
		return evidence, err
	}
	select {
	case <-ctx.Done():
		return evidence, ctx.Err()
	case <-time.After(5 * time.Second):
	}
	second, err := collectRestartCounts(ctx, runner, stageRoot, composeBase)
	if err != nil {
		return evidence, err
	}
	evidence.RestartCountsStable = restartCountsStable(first, second)
	if !evidence.RestartCountsStable {
		return evidence, fmt.Errorf("restart counts grew during the stabilization window")
	}
	evidence.Result = "passed"
	return evidence, nil
}

func signalPath(signal signalKind) string {
	if signal == signalViewerUnavailable {
		return "/viewer"
	}
	if signal == signalStatusComponent {
		return "/api/status"
	}
	return "/readyz"
}

func seconds(value time.Duration) float64 {
	return float64(value.Round(time.Millisecond)) / float64(time.Second)
}

func collectRestartCounts(ctx context.Context, runner *commandRunner, stageRoot string, composeBase []string) (map[string]int, error) {
	counts := make(map[string]int, len(longRunningServices))
	for _, service := range longRunningServices {
		serviceCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		id, err := runner.composeText(serviceCtx, stageRoot, composeBase, "ps", "-q", service)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("resolve %s container: %w", service, err)
		}
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, fmt.Errorf("service %s has no running container", service)
		}
		inspectCtx, cancelInspect := context.WithTimeout(ctx, 20*time.Second)
		value, err := runner.text(inspectCtx, stageRoot, "docker", "inspect", "--format", "{{.RestartCount}}", id)
		cancelInspect()
		if err != nil {
			return nil, fmt.Errorf("inspect %s restart count: %w", service, err)
		}
		count, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("parse %s restart count", service)
		}
		counts[service] = count
	}
	return counts, nil
}

func repositoryRoot(ctx context.Context) (string, error) {
	root, err := commandText(ctx, "", nil, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}
	return filepath.Clean(strings.TrimSpace(root)), nil
}

func extractTrackedTree(ctx context.Context, repoRoot, destination string) error {
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return fmt.Errorf("create staged tree: %w", err)
	}
	archivePath := filepath.Join(filepath.Dir(destination), "tracked-tree.tar")
	defer os.Remove(archivePath)
	if _, err := commandText(ctx, repoRoot, nil, "git", "archive", "--format=tar", "--output="+archivePath, "HEAD"); err != nil {
		return fmt.Errorf("create tracked-tree archive: %w", err)
	}
	archive, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open tracked-tree archive: %w", err)
	}
	defer archive.Close()
	reader := tar.NewReader(archive)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read git archive: %w", err)
		}
		cleanName := filepath.Clean(filepath.FromSlash(header.Name))
		if cleanName == "." || filepath.IsAbs(cleanName) || strings.HasPrefix(cleanName, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("git archive contains unsafe path")
		}
		target := filepath.Join(destination, cleanName)
		if !pathWithin(destination, target) {
			return fmt.Errorf("git archive escaped staged tree")
		}
		switch header.Typeflag {
		case tar.TypeXHeader, tar.TypeXGlobalHeader:
			// Metadata-only PAX entries carry Git archive identity and have no
			// filesystem representation in the staged tree.
			continue
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)&0o777); err != nil {
				return fmt.Errorf("create staged directory: %w", err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("create staged parent: %w", err)
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(header.Mode)&0o777)
			if err != nil {
				return fmt.Errorf("create staged file: %w", err)
			}
			_, copyErr := io.Copy(file, reader)
			closeErr := file.Close()
			if copyErr != nil || closeErr != nil {
				return fmt.Errorf("write staged file: %v %v", copyErr, closeErr)
			}
		default:
			return fmt.Errorf("git archive contains unsupported entry type %d at %q", header.Typeflag, header.Name)
		}
	}
	return nil
}

func pathWithin(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}

func resolveReportPath(repoRoot, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return "", fmt.Errorf("report path is required")
	}
	if !filepath.IsAbs(requested) {
		requested = filepath.Join(repoRoot, requested)
	}
	resolved := filepath.Clean(requested)
	if pathWithin(repoRoot, resolved) && !pathWithin(filepath.Join(repoRoot, ".artifacts"), resolved) {
		return "", fmt.Errorf("in-repository reports must be written below .artifacts")
	}
	return resolved, nil
}

func stagePrivateEnvironment(source, destination, commit string) error {
	payload, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read private environment: %w", err)
	}
	overrides := []string{
		"BITRIVER_DEPLOY_IMAGE_SOURCE=build",
		"BITRIVER_IMAGE_NAMESPACE=bitriver-resilience",
		"BITRIVER_LIVE_IMAGE_TAG=local",
		"BITRIVER_VIEWER_IMAGE_TAG=local",
		"BITRIVER_SRS_CONTROLLER_IMAGE_TAG=local",
		"BITRIVER_OME_CONFIG_IMAGE_TAG=local",
		"BITRIVER_TRANSCODER_IMAGE_TAG=local",
		"BITRIVER_RELEASE_COMMIT=" + commit,
		"BITRIVER_LIVE_ALLOW_SELF_SIGNUP=true",
		"BITRIVER_LIVE_PORT=28080",
		"BITRIVER_SRS_RTMP_PORT=21935",
		"BITRIVER_SRS_CONTROLLER_PORT=21986",
		"BITRIVER_OME_HTTP_PORT=28081",
		"BITRIVER_OME_API=http://ome:28081",
		"BITRIVER_OME_HTTP_TLS_PORT=28082",
		"BITRIVER_OME_LLHLS_HOST_PORT=28083",
		"BITRIVER_OME_LLHLS_TLS_PORT=28443",
		"BITRIVER_OME_SIGNALLING_PORT=29000",
		"BITRIVER_OME_SERVER_PORT=29000",
		"BITRIVER_OME_SERVER_TLS_PORT=29443",
		"BITRIVER_OME_RELAY_PORT=23478",
		"BITRIVER_OME_ICE_PORT_RANGE=21000-21009",
		"BITRIVER_TRANSCODER_HOST_PORT=29001",
		"BITRIVER_TRANSCODER_PUBLIC_PORT=29080",
		"BITRIVER_SRS_PUBLIC_RTMP_BASE_URL=rtmp://127.0.0.1:21935/live",
		"BITRIVER_OME_PUBLIC_LLHLS_BASE_URL=http://127.0.0.1:28083/app",
		"BITRIVER_TRANSCODER_PUBLIC_BASE_URL=http://127.0.0.1:29080",
		"NEXT_PUBLIC_VIEWER_URL=http://127.0.0.1:28080/viewer",
	}
	for _, key := range []string{
		"BITRIVER_LIVE_IMAGE_DIGEST",
		"BITRIVER_VIEWER_IMAGE_DIGEST",
		"BITRIVER_SRS_CONTROLLER_IMAGE_DIGEST",
		"BITRIVER_OME_CONFIG_IMAGE_DIGEST",
		"BITRIVER_TRANSCODER_IMAGE_DIGEST",
		"BITRIVER_SRS_IMAGE_DIGEST",
		"BITRIVER_OME_IMAGE_DIGEST",
		"BITRIVER_POSTGRES_IMAGE_DIGEST",
		"BITRIVER_REDIS_IMAGE_DIGEST",
		"BITRIVER_NGINX_IMAGE_DIGEST",
		"BITRIVER_DEBIAN_IMAGE_DIGEST",
		"BITRIVER_ALPINE_3_IMAGE_DIGEST",
		"BITRIVER_ALPINE_3_19_IMAGE_DIGEST",
	} {
		overrides = append(overrides, key+"=")
	}
	if len(payload) > 0 && payload[len(payload)-1] != '\n' {
		payload = append(payload, '\n')
	}
	payload = append(payload, []byte("\n# Disposable service-resilience overrides.\n"+strings.Join(overrides, "\n")+"\n")...)
	if err := os.WriteFile(destination, payload, 0o600); err != nil {
		return fmt.Errorf("write staged private environment: %w", err)
	}
	return nil
}

func prepareRuntimeStorage(stageRoot string) error {
	for _, path := range []string{
		filepath.Join(stageRoot, "deploy", "data"),
		filepath.Join(stageRoot, "deploy", "transcoder-data"),
		filepath.Join(stageRoot, "deploy", "srs", "conf"),
		filepath.Join(stageRoot, "deploy", "ome"),
	} {
		if err := os.MkdirAll(path, 0o777); err != nil {
			return fmt.Errorf("create staged runtime storage: %w", err)
		}
		if err := os.Chmod(path, 0o777); err != nil && runtime.GOOS != "windows" {
			return fmt.Errorf("make staged runtime storage writable: %w", err)
		}
	}
	return nil
}

func preflightPorts() error {
	tcpPorts := []int{21935, 21986, 28080, 28081, 28082, 28083, 28443, 29000, 29001, 29080, 29443, 23478}
	for _, port := range tcpPorts {
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			return fmt.Errorf("required test TCP port %d is unavailable: %w", port, err)
		}
		_ = listener.Close()
	}
	for _, port := range append([]int{23478}, integerRange(21000, 21009)...) {
		packet, err := net.ListenPacket("udp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			return fmt.Errorf("required test UDP port %d is unavailable: %w", port, err)
		}
		_ = packet.Close()
	}
	return nil
}

func integerRange(start, end int) []int {
	values := make([]int, 0, end-start+1)
	for value := start; value <= end; value++ {
		values = append(values, value)
	}
	return values
}

func preflightContainers(ctx context.Context, dir string, runner *commandRunner) error {
	projectIDs, err := runner.text(ctx, dir, "docker", "ps", "-aq", "--filter", "label=com.docker.compose.project="+composeProject)
	if err != nil {
		return fmt.Errorf("inspect prior rehearsal containers: %w", err)
	}
	if strings.TrimSpace(projectIDs) != "" {
		return fmt.Errorf("prior %s containers exist; remove them before retrying", composeProject)
	}
	projectVolumes, err := runner.text(ctx, dir, "docker", "volume", "ls", "-q", "--filter", "label=com.docker.compose.project="+composeProject)
	if err != nil {
		return fmt.Errorf("inspect prior rehearsal volumes: %w", err)
	}
	if strings.TrimSpace(projectVolumes) != "" {
		return fmt.Errorf("prior %s volumes exist; remove them before retrying", composeProject)
	}
	projectNetworks, err := runner.text(ctx, dir, "docker", "network", "ls", "-q", "--filter", "label=com.docker.compose.project="+composeProject)
	if err != nil {
		return fmt.Errorf("inspect prior rehearsal networks: %w", err)
	}
	if strings.TrimSpace(projectNetworks) != "" {
		return fmt.Errorf("prior %s networks exist; remove them before retrying", composeProject)
	}
	for _, name := range fixedContainerNames {
		ids, err := runner.text(ctx, dir, "docker", "ps", "-aq", "--filter", "name=^/"+name+"$")
		if err != nil {
			return fmt.Errorf("inspect canonical container %s: %w", name, err)
		}
		if strings.TrimSpace(ids) != "" {
			return fmt.Errorf("canonical container %s already exists; refusing to touch it", name)
		}
	}
	return nil
}

func assertProjectRemoved(ctx context.Context, dir string, runner *commandRunner) error {
	ids, err := runner.text(ctx, dir, "docker", "ps", "-aq", "--filter", "label=com.docker.compose.project="+composeProject)
	if err != nil {
		return fmt.Errorf("verify rehearsal teardown: %w", err)
	}
	if strings.TrimSpace(ids) != "" {
		return fmt.Errorf("rehearsal containers remain after teardown")
	}
	volumes, err := runner.text(ctx, dir, "docker", "volume", "ls", "-q", "--filter", "label=com.docker.compose.project="+composeProject)
	if err != nil {
		return fmt.Errorf("verify rehearsal volume teardown: %w", err)
	}
	if strings.TrimSpace(volumes) != "" {
		return fmt.Errorf("rehearsal volumes remain after teardown")
	}
	networks, err := runner.text(ctx, dir, "docker", "network", "ls", "-q", "--filter", "label=com.docker.compose.project="+composeProject)
	if err != nil {
		return fmt.Errorf("verify rehearsal network teardown: %w", err)
	}
	if strings.TrimSpace(networks) != "" {
		return fmt.Errorf("rehearsal networks remain after teardown")
	}
	return nil
}

func privateSentinels(envPath string) ([]string, error) {
	payload, err := os.ReadFile(envPath)
	if err != nil {
		return nil, fmt.Errorf("read private sentinels: %w", err)
	}
	var sentinels []string
	for _, line := range strings.Split(string(payload), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		upperKey := strings.ToUpper(strings.TrimSpace(key))
		if !strings.Contains(upperKey, "PASSWORD") && !strings.Contains(upperKey, "TOKEN") &&
			!strings.Contains(upperKey, "SECRET") && !strings.Contains(upperKey, "API_KEY") &&
			!strings.Contains(upperKey, "DSN") {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if len(value) >= 8 {
			sentinels = append(sentinels, value)
		}
	}
	return uniqueStrings(sentinels), nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func cleanEnvironment(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		key, _, _ := strings.Cut(value, "=")
		upper := strings.ToUpper(key)
		if strings.HasPrefix(upper, "BITRIVER_") || strings.HasPrefix(upper, "COMPOSE_") ||
			upper == "GOPROXY" || upper == "GOSUMDB" || upper == "GOTOOLCHAIN" || upper == "GOCACHE" {
			continue
		}
		result = append(result, value)
	}
	return result
}

func (r *commandRunner) compose(ctx context.Context, dir string, base []string, args ...string) error {
	commandArgs := append(append([]string{}, base...), args...)
	return r.run(ctx, dir, "docker", commandArgs...)
}

func (r *commandRunner) composeText(ctx context.Context, dir string, base []string, args ...string) (string, error) {
	commandArgs := append(append([]string{}, base...), args...)
	return r.text(ctx, dir, "docker", commandArgs...)
}

func (r *commandRunner) run(ctx context.Context, dir, name string, args ...string) error {
	_, err := r.runCaptured(ctx, dir, name, args...)
	return err
}

func (r *commandRunner) text(ctx context.Context, dir, name string, args ...string) (string, error) {
	return r.runCaptured(ctx, dir, name, args...)
}

func (r *commandRunner) runCaptured(ctx context.Context, dir, name string, args ...string) (string, error) {
	logFile, err := os.OpenFile(r.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return "", fmt.Errorf("open private command log: %w", err)
	}
	defer logFile.Close()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = r.env
	output := &cappedTail{limit: 12000}
	cmd.Stdout = io.MultiWriter(logFile, output)
	cmd.Stderr = io.MultiWriter(logFile, output)
	err = cmd.Run()
	text := output.String()
	if err != nil {
		return "", fmt.Errorf("%s failed: %s", name, redact(text, r.secrets))
	}
	return text, nil
}

func commandText(ctx context.Context, dir string, env []string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if env != nil {
		cmd.Env = env
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w", strings.TrimSpace(string(output)), err)
	}
	return string(output), nil
}

func redact(value string, secrets []string) string {
	for _, secret := range secrets {
		if len(secret) >= 8 {
			value = strings.ReplaceAll(value, secret, "<redacted>")
		}
	}
	return strings.TrimSpace(value)
}
