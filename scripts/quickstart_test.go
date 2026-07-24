package scripts_test

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func testBash(t *testing.T) string {
	t.Helper()
	var candidates []string
	if configured := strings.TrimSpace(os.Getenv("BITRIVER_TEST_BASH")); configured != "" {
		candidates = append(candidates, configured)
	}
	if runtime.GOOS == "windows" {
		if programFiles := strings.TrimSpace(os.Getenv("ProgramFiles")); programFiles != "" {
			candidates = append(candidates,
				filepath.Join(programFiles, "Git", "bin", "bash.exe"),
				filepath.Join(programFiles, "Git", "usr", "bin", "bash.exe"),
			)
		}
		if programFilesX86 := strings.TrimSpace(os.Getenv("ProgramFiles(x86)")); programFilesX86 != "" {
			candidates = append(candidates,
				filepath.Join(programFilesX86, "Git", "bin", "bash.exe"),
				filepath.Join(programFilesX86, "Git", "usr", "bin", "bash.exe"),
			)
		}
	}
	if found, err := exec.LookPath("bash"); err == nil {
		candidates = append(candidates, found)
	}

	seen := make(map[string]struct{})
	var failures []string
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		candidate = filepath.Clean(candidate)
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		if _, err := os.Stat(candidate); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", candidate, err))
			continue
		}
		cmd := exec.Command(candidate, "-lc", "printf ok")
		output, err := cmd.CombinedOutput()
		if err == nil && string(output) == "ok" {
			return candidate
		}
		failures = append(failures, fmt.Sprintf("%s: %v %q", candidate, err, output))
	}
	t.Skipf("usable bash not available for quickstart wrapper tests: %s", strings.Join(failures, "; "))
	return ""
}

func shellPath(path string) string {
	clean := filepath.ToSlash(path)
	if runtime.GOOS == "windows" && len(clean) >= 2 && clean[1] == ':' {
		drive := strings.ToLower(clean[:1])
		return "/" + drive + clean[2:]
	}
	return clean
}

func copyQuickstartScripts(t *testing.T, repoRoot, tempDir string) string {
	t.Helper()
	scriptDir := filepath.Join(tempDir, "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("create script dir: %v", err)
	}
	for _, name := range []string{"quickstart.sh", "check-go-toolchain.sh"} {
		source := filepath.Join(repoRoot, "scripts", name)
		destination := filepath.Join(scriptDir, name)
		contents, err := os.ReadFile(source)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if err := os.WriteFile(destination, contents, 0o755); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return filepath.Join(scriptDir, "quickstart.sh")
}

func TestQuickstartDelegatesToCli(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)
	tempDir := t.TempDir()

	quickstartDst := copyQuickstartScripts(t, repoRoot, tempDir)
	if err := os.MkdirAll(filepath.Join(tempDir, "cmd", "bitriver"), 0o755); err != nil {
		t.Fatalf("create fake cli dir: %v", err)
	}

	logPath := filepath.Join(tempDir, "go-log.txt")
	stubBin := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(stubBin, 0o755); err != nil {
		t.Fatalf("create stub bin: %v", err)
	}
	goStubPath := filepath.Join(stubBin, "go")
	goStubBytes := []byte("#!/usr/bin/env bash\nset -euo pipefail\nif [[ \"$*\" == \"env GOVERSION\" ]]; then echo go1.26.5; exit 0; fi\necho \"$(pwd):$*\" >>\"$GO_LOG\"\n")
	if err := os.WriteFile(goStubPath, goStubBytes, 0o755); err != nil {
		t.Fatalf("write go stub: %v", err)
	}
	if runtime.GOOS == "windows" {
		goCmdStubPath := filepath.Join(stubBin, "go.cmd")
		goCmdStubBytes := []byte("@echo off\r\necho %CD%:%*>>\"%GO_LOG%\"\r\n")
		if err := os.WriteFile(goCmdStubPath, goCmdStubBytes, 0o755); err != nil {
			t.Fatalf("write go.cmd stub: %v", err)
		}
	}

	bash := testBash(t)
	cmd := exec.Command(bash, shellPath(quickstartDst))
	cmd.Dir = tempDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PATH=%s:%s", shellPath(stubBin), os.Getenv("PATH")),
		fmt.Sprintf("BITRIVER_QUICKSTART_REPO_ROOT=%s", shellPath(tempDir)),
		fmt.Sprintf("GO_LOG=%s", shellPath(logPath)),
	)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("quickstart harness failed: %v\noutput:\n%s", err, stdout.String())
	}

	logContent, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read go log: %v", err)
	}
	line := strings.TrimSpace(string(logContent))
	if !strings.Contains(line, "run ./cmd/bitriver quickstart") {
		t.Fatalf("unexpected go invocation log: %s", line)
	}
}

func TestQuickstartFailsWhenCliSourcesMissing(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)
	tempDir := t.TempDir()

	quickstartDst := copyQuickstartScripts(t, repoRoot, tempDir)

	bash := testBash(t)
	cmd := exec.Command(bash, shellPath(quickstartDst))
	cmd.Dir = tempDir
	cmd.Env = append(os.Environ(), fmt.Sprintf("BITRIVER_QUICKSTART_REPO_ROOT=%s", shellPath(tempDir)))
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected quickstart to fail when cmd/bitriver is missing")
	}
	if !strings.Contains(string(output), "expected Go CLI sources") {
		t.Fatalf("expected missing source error, got:\n%s", output)
	}
}

func TestQuickstartOmeRenderingRunsByDefault(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)

	quickstartPath := filepath.Join(repoRoot, "scripts", "quickstart.sh")
	content, err := os.ReadFile(quickstartPath)
	if err != nil {
		t.Fatalf("read quickstart: %v", err)
	}

	if !strings.Contains(string(content), "go run ./cmd/bitriver quickstart") {
		t.Fatalf("quickstart Go CLI invocation not found in quickstart script")
	}
}

func TestUnixWrapperStartUsesCliQuickstart(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)

	wrapperPath := filepath.Join(repoRoot, "scripts", "bitriver-live-wrapper.sh")
	content, err := os.ReadFile(wrapperPath)
	if err != nil {
		t.Fatalf("read wrapper: %v", err)
	}

	wrapper := string(content)
	if !strings.Contains(wrapper, `"$binary_path" quickstart --compose-file "$compose_file" --env-file "$env_file"`) {
		t.Fatalf("expected unix wrapper to invoke CLI quickstart with compose/env files")
	}
	if strings.Contains(wrapper, "compose_cmd up -d") {
		t.Fatalf("expected unix wrapper start path not to call docker compose up directly")
	}
}

func TestPowerShellWrapperStartUsesCliQuickstart(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)

	wrapperPath := filepath.Join(repoRoot, "scripts", "bitriver-live-wrapper.ps1")
	content, err := os.ReadFile(wrapperPath)
	if err != nil {
		t.Fatalf("read wrapper: %v", err)
	}

	wrapper := string(content)
	if !strings.Contains(wrapper, "& $binary quickstart --compose-file $composeFile --env-file $envFilePath") {
		t.Fatalf("expected PowerShell wrapper to invoke CLI quickstart with compose/env files")
	}
	if strings.Contains(wrapper, "Invoke-Compose up -d") {
		t.Fatalf("expected PowerShell wrapper start path not to call docker compose up directly")
	}
}

func TestPowerShellQuickstartPropagatesCliExitCodes(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)

	scriptPath := filepath.Join(repoRoot, "scripts", "quickstart.ps1")
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read quickstart.ps1: %v", err)
	}

	script := string(content)
	if !strings.Contains(script, "$goPath = (Get-Command go -ErrorAction Stop).Source") {
		t.Fatalf("expected PowerShell quickstart wrapper to resolve the absolute go.exe path before launching the CLI")
	}
	if !strings.Contains(script, "SetEnvironmentVariable('GOTOOLCHAIN', 'local', 'Process')") {
		t.Fatalf("expected PowerShell quickstart wrapper to inspect and build with the installed local Go toolchain")
	}
	if !strings.Contains(script, "$buildOutput = @(& $goPath @buildArgs 2>&1)") {
		t.Fatalf("expected PowerShell quickstart wrapper to compile the CLI directly with the resolved go.exe path")
	}
	if !strings.Contains(script, "$cliOutput = @(& $tempBinary @CliArgs 2>&1)") {
		t.Fatalf("expected PowerShell quickstart wrapper to invoke the compiled CLI without leaking build-only Go environment settings")
	}
	if !strings.Contains(script, "$processGOCACHE = [System.Environment]::GetEnvironmentVariable('GOCACHE', 'Process')") {
		t.Fatalf("expected PowerShell quickstart wrapper to capture the process GOCACHE value before launching the CLI")
	}
	if !strings.Contains(script, "bitriver-live-go-build-cache") {
		t.Fatalf("expected PowerShell quickstart wrapper to provide a writable fallback GOCACHE location")
	}
	if !strings.Contains(script, "SetEnvironmentVariable('GOCACHE', $goCacheRoot, 'Process')") {
		t.Fatalf("expected PowerShell quickstart wrapper to set GOCACHE before launching the CLI")
	}
	if !strings.Contains(script, "SetEnvironmentVariable('GOCACHE', $processGOCACHE, 'Process')") {
		t.Fatalf("expected PowerShell quickstart wrapper to restore the original GOCACHE value after compiling the CLI")
	}
	if !strings.Contains(script, "SetEnvironmentVariable('GOPROXY', $processGOPROXY, 'Process')") {
		t.Fatalf("expected PowerShell quickstart wrapper to restore GOPROXY before running Docker-backed CLI stages")
	}
	if strings.Contains(script, "SetEnvironmentVariable('PATH', $null, 'Process')") {
		t.Fatalf("expected PowerShell quickstart wrapper not to clear PATH after setting Path on case-insensitive Windows")
	}
	if !strings.Contains(script, "$exitCode = $LASTEXITCODE") {
		t.Fatalf("expected PowerShell quickstart wrapper to inspect the child process exit code")
	}
	if !strings.Contains(script, "flag: help requested") {
		t.Fatalf("expected PowerShell quickstart wrapper to allow the validate-only help path")
	}
	if !strings.Contains(script, "exit $exitCode") {
		t.Fatalf("expected PowerShell quickstart wrapper to return non-zero CLI exit codes")
	}
}

func TestWindowsDockerDesktopProofUsesCanonicalQuickstart(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)

	scriptPath := filepath.Join(repoRoot, "scripts", "verify-windows-docker.ps1")
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read verify-windows-docker.ps1: %v", err)
	}

	script := string(content)
	for _, expected := range []string{
		"docker' -Arguments @('version', '--format', '{{.Server.Os}}|{{.Server.Arch}}|{{.Server.Version}}')",
		"docker' -Arguments @('info', '--format', '{{.OperatingSystem}}|{{.OSType}}|{{.Architecture}}')",
		"'compose', '--env-file', $envPath, '-f', $composePath, 'config', '--quiet'",
		"$runtimeEnvPath = New-EvaluationEnvFile -SourcePath $envPath",
		"SetEnvironmentVariable('GOTOOLCHAIN', 'local', 'Process')",
		"& $quickstartPath --env-file $runtimeEnvPath --compose-file $composePath --image-source build",
		"@('/healthz', '/readyz', '/viewer', '/admin')",
		"Cleanup (PowerShell): `$env:BITRIVER_SRS_PUBLIC_RTMP_BASE_URL='rtmp://localhost:1935/live'; `$env:BITRIVER_OME_PUBLIC_LLHLS_BASE_URL='http://localhost:8080/live'",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("expected Windows Docker Desktop proof script to contain %q", expected)
		}
	}
	if strings.Contains(script, "docker compose up") {
		t.Fatal("expected Windows proof script to delegate startup to the canonical quickstart instead of duplicating Compose orchestration")
	}
}

func TestWorkflowComposeFixturesIncludePublicMediaURLs(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)
	assignments := []string{
		"BITRIVER_SRS_PUBLIC_RTMP_BASE_URL=rtmp://example.com/live",
		"BITRIVER_OME_PUBLIC_LLHLS_BASE_URL=https://example.com/live",
	}
	for _, relativePath := range []string{
		filepath.Join(".github", "workflows", "ci.yml"),
		filepath.Join(".github", "workflows", "image-scan.yml"),
	} {
		contents, readErr := os.ReadFile(filepath.Join(repoRoot, relativePath))
		if readErr != nil {
			t.Fatalf("read %s: %v", relativePath, readErr)
		}
		workflow := string(contents)
		for _, assignment := range assignments {
			if !strings.Contains(workflow, assignment) {
				t.Fatalf("%s must include %q in its Compose fixture", relativePath, assignment)
			}
		}
	}

	releasePath := filepath.Join(repoRoot, ".github", "workflows", "release.yml")
	releaseContents, err := os.ReadFile(releasePath)
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}
	for _, variable := range []string{
		"BITRIVER_SRS_PUBLIC_RTMP_BASE_URL: ${{ secrets.BITRIVER_SRS_PUBLIC_RTMP_BASE_URL }}",
		"BITRIVER_OME_PUBLIC_LLHLS_BASE_URL: ${{ secrets.BITRIVER_OME_PUBLIC_LLHLS_BASE_URL }}",
	} {
		if !strings.Contains(string(releaseContents), variable) {
			t.Fatalf("release workflow must forward %q", variable)
		}
	}
}

func TestTranscoderPublicNginxMapsDocumentedHLSPrefix(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)
	paths := []string{
		filepath.Join(repoRoot, "deploy", "nginx", "transcoder-public.conf"),
		filepath.Join(repoRoot, "deploy", "helm", "bitriver-live", "files", "transcoder-public.conf"),
	}
	for _, path := range paths {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		config := string(contents)
		if !strings.Contains(config, "location /hls/") || !strings.Contains(config, "alias /work/public/") {
			t.Fatalf("%s must map the documented /hls public base to /work/public", path)
		}
	}
}

func TestOmeLLHLSPublisherAllowsBrowserPlayback(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)
	paths := []string{
		filepath.Join(repoRoot, "deploy", "ome", "Server.xml"),
		filepath.Join(repoRoot, "deploy", "ome", "Server.generated.xml"),
		filepath.Join(repoRoot, "deploy", "helm", "bitriver-live", "templates", "configmap-ome.yaml"),
	}
	llhlsCORS := regexp.MustCompile(`(?s)<LLHLS>\s*<ChunkDuration>[^<]+</ChunkDuration>.*?<CrossDomains>\s*<Url>\*</Url>\s*</CrossDomains>\s*</LLHLS>`)
	for _, path := range paths {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !llhlsCORS.Match(contents) {
			t.Fatalf("%s must allow browser CORS on the application LL-HLS publisher", path)
		}
	}
}

func TestComposeMountsOmeConfigByDefault(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)

	composePath := filepath.Join(repoRoot, "deploy", "docker-compose.yml")
	content, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read compose: %v", err)
	}

	if !strings.Contains(string(content), "Server.generated.xml") {
		t.Fatalf("base compose file should mount generated OME Server.xml by default")
	}
	if !strings.Contains(string(content), "BITRIVER_OME_LLHLS_ORIGIN: ${BITRIVER_OME_LLHLS_ORIGIN:-http://ome:${BITRIVER_OME_LLHLS_PORT:-8080}}") {
		t.Fatalf("base compose file should route same-origin /live playback to the internal OME LL-HLS listener")
	}
	if !strings.Contains(string(content), "BITRIVER_TRANSCODE_LADDER: ${BITRIVER_TRANSCODE_LADDER:-}") {
		t.Fatalf("base compose file should pass the configured rendition ladder into the API container")
	}
}

func TestQuickstartSmokePreservesContainerEnvironmentPathsOnWindows(t *testing.T) {
	content, err := os.ReadFile("test-quickstart.sh")
	if err != nil {
		t.Fatalf("read quickstart smoke script: %v", err)
	}
	script := string(content)
	if !strings.Contains(script, `[[ "${OSTYPE:-}" == msys* || "${OSTYPE:-}" == cygwin* ]]`) {
		t.Fatal("quickstart smoke should detect Git Bash and Cygwin before native Docker invocations")
	}
	if !strings.Contains(script, `export MSYS2_ENV_CONV_EXCL="*"`) {
		t.Fatal("quickstart smoke should preserve container paths such as /healthz in Compose environment values")
	}
	if strings.Contains(script, `export MSYS2_ARG_CONV_EXCL="*"`) || strings.Contains(script, `export MSYS_NO_PATHCONV=1`) {
		t.Fatal("quickstart smoke must retain argument conversion for native Docker temp-file paths")
	}
	linuxOverride := extractSection(script, `cat >"$COMPOSE_SMOKE_OVERRIDE" <<YAML`, "\nYAML\nelse")
	if linuxOverride == "" {
		t.Fatal("expected Linux quickstart smoke override")
	}
	if strings.Contains(linuxOverride, `transcoder:
    user:`) {
		t.Fatal("Linux smoke must retain the transcoder image UID for its isolated named /work volume")
	}
	if !strings.Contains(linuxOverride, `transcoder:
    volumes:
      - bitriver-smoke-transcoder:/work`) {
		t.Fatal("Linux smoke should mount the isolated media volume without replacing the transcoder image user")
	}
	if strings.Contains(script, "\n    docker inspect \"$container_id\"\n") {
		t.Fatal("quickstart health diagnostics must not dump container configuration or environment values")
	}
	if !strings.Contains(script, "error={{json .State.Error}}") {
		t.Fatal("quickstart health diagnostics should retain a state-only container error summary")
	}
}

func TestProductionGoldenPathWorkflowOwnsRealComposeLifecycle(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)

	read := func(path string) string {
		t.Helper()
		content, readErr := os.ReadFile(filepath.Join(repoRoot, path))
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		return string(content)
	}

	compatibilityEntrypoint := read("scripts/test-ingest-e2e.sh")
	if strings.Contains(compatibilityEntrypoint, "go test ./internal/storage") {
		t.Fatal("ingest E2E compatibility entrypoint must not claim a storage-only test as product acceptance")
	}
	for _, required := range []string{
		"test-production-golden-path.sh",
		"--stack quickstart",
		"--client",
		"BITRIVER_GOLDEN_PATH_ARTIFACT_DIR",
	} {
		if !strings.Contains(compatibilityEntrypoint, required) {
			t.Fatalf("ingest E2E compatibility entrypoint missing %q", required)
		}
	}

	storageGuard := read("scripts/test-ingest-storage.sh")
	if !strings.Contains(storageGuard, "go test ./internal/storage") || !strings.Contains(storageGuard, "TestIngestPipelineEndToEnd") {
		t.Fatal("cheap storage/controller ingest guard should remain separately callable")
	}

	workflow := read(".github/workflows/ingest-e2e.yml")
	for _, required := range []string{
		"name: Production golden path",
		"timeout-minutes: 30",
		"run: ./scripts/test-production-golden-path.sh --stack quickstart --client docker",
		"production-golden-path.json",
		"if: always()",
		"actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("production golden-path workflow missing %q", required)
		}
	}

	testAll := read("scripts/test-all.sh")
	if !strings.Contains(testAll, "BITRIVER_TEST_ALL_PRODUCTION_GOLDEN_PATH") {
		t.Fatal("test-all should expose an accurately named production golden-path control")
	}
	if !strings.Contains(testAll, `skip_step "Quickstart smoke" "the production golden path owns the same canonical quickstart lifecycle."`) {
		t.Fatal("test-all should avoid a duplicate quickstart when the product gate is enabled")
	}
}

func TestQuickstartSmokeGeneratedEnvUsesNonDefaultHostPort(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)

	scriptPath := filepath.Join(repoRoot, "scripts", "test-quickstart.sh")
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read test-quickstart: %v", err)
	}

	script := string(content)
	envSection := extractSection(script, `cat >"$ENV_FILE" <<'ENV'`, "\nENV\nfi")
	if envSection == "" {
		t.Fatalf("expected generated env heredoc in test-quickstart.sh")
	}
	if strings.Contains(envSection, "BITRIVER_LIVE_PORT=8080") {
		t.Fatalf("generated smoke env should avoid publishing the API on common host port 8080")
	}
	if !strings.Contains(envSection, "BITRIVER_LIVE_PORT=18080") {
		t.Fatalf("generated smoke env should publish the API on host port 18080")
	}
	if !strings.Contains(envSection, "BITRIVER_LIVE_ADDR=:8080") {
		t.Fatalf("generated smoke env should keep the API listening on container port 8080")
	}
	if !strings.Contains(envSection, "NEXT_PUBLIC_VIEWER_URL=http://localhost:18080/viewer") {
		t.Fatalf("generated smoke viewer URL should follow the smoke host API port")
	}
	if !strings.Contains(script, `grep_healthcheck "bitriver-live" "http://localhost:8080/healthz"`) {
		t.Fatalf("compose healthcheck validation should continue to assert the in-container API port")
	}
}

func TestQuickstartSmokeSuppliesPublicMediaDefaultsForExistingEnv(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)

	content, err := os.ReadFile(filepath.Join(repoRoot, "scripts", "test-quickstart.sh"))
	if err != nil {
		t.Fatalf("read test-quickstart: %v", err)
	}

	script := string(content)
	for _, required := range []string{
		`export BITRIVER_SRS_PUBLIC_RTMP_BASE_URL="${BITRIVER_SRS_PUBLIC_RTMP_BASE_URL:-rtmp://localhost:1935/live}"`,
		`export BITRIVER_OME_PUBLIC_LLHLS_BASE_URL="${BITRIVER_OME_PUBLIC_LLHLS_BASE_URL:-http://localhost:18080/live}"`,
		`export BITRIVER_TRANSCODER_PUBLIC_BASE_URL="${BITRIVER_TRANSCODER_PUBLIC_BASE_URL:-http://localhost:9080/hls}"`,
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("quickstart smoke must supply missing public media value %q when reusing an existing env", required)
		}
	}
}

func TestComposeOMEHealthcheckUsesUnauthenticatedRootEndpoint(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)

	composePath := filepath.Join(repoRoot, "deploy", "docker-compose.yml")
	content, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read compose: %v", err)
	}

	compose := string(content)
	if !strings.Contains(compose, `http://localhost:${BITRIVER_OME_HTTP_PORT:-8081}/`) {
		t.Fatalf("expected OME healthcheck to probe unauthenticated root endpoint")
	}
	if strings.Contains(compose, `/v1/health`) {
		t.Fatalf("expected OME healthcheck not to probe authenticated /v1/health endpoint")
	}
	if strings.Contains(compose, `Authorization: Bearer $$token`) {
		t.Fatalf("expected OME healthcheck to avoid auth headers when using public liveness endpoint")
	}
}

func TestDockerfileUsesVerifiedProductionModuleGraph(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)

	dockerfilePath := filepath.Join(repoRoot, "Dockerfile")
	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}

	dockerfile := string(content)
	if strings.Contains(dockerfile, "go mod edit -dropreplace=") {
		t.Fatal("production dependency policy must not use a hand-maintained replacement list")
	}
	for _, required := range []string{
		"go run ./cmd/tools/production-module --output go.production.mod",
		"go mod download -modfile=go.production.mod all",
		"GOFLAGS=\"-buildvcs=false -modfile=/src/go.production.mod\" ./scripts/check-postgres-pgx.sh postgres",
		"go run ./cmd/tools/verify-production-binary --require-module github.com/jackc/pgx/v5 /out/bitriver-live",
	} {
		if !strings.Contains(dockerfile, required) {
			t.Fatalf("expected production Dockerfile invariant %q", required)
		}
	}
}

func TestOmeConfigRenderingOmitsUnsupportedRootBindHostTags(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)

	envContents := strings.Join([]string{
		"BITRIVER_OME_BIND=0.0.0.0",
		"BITRIVER_OME_IP=0.0.0.0",
		"BITRIVER_OME_SERVER_PORT=8081",
		"BITRIVER_OME_SERVER_TLS_PORT=8082",
		"BITRIVER_OME_USERNAME=admin",
		"BITRIVER_OME_PASSWORD=password",
		"BITRIVER_OME_API_TOKEN=token",
		"BITRIVER_OME_TCP_RELAY=*:3478",
		"BITRIVER_OME_ICE_CANDIDATE=*:10000-10009/udp",
		"BITRIVER_OME_IMAGE_TAG=0.16.0",
	}, "\n")
	output := renderOMEConfig(t, repoRoot, envContents)

	contents := string(output)
	if strings.Contains(contents, "<Application><Outputs>") || strings.Contains(contents, "<Outputs>") {
		t.Fatalf("expected rendered config to define output profiles directly under <Application><OutputProfiles>, got:\n%s", contents)
	}
	if !strings.Contains(contents, "<Application>") || !strings.Contains(contents, "<OutputProfiles>") {
		t.Fatalf("expected rendered config to include direct <Application><OutputProfiles>, got:\n%s", contents)
	}
	if regexp.MustCompile(`(?s)<Application\b[^>]*>\s*<LLHLS\b`).MatchString(contents) {
		t.Fatalf("expected rendered config to avoid deprecated <Application><LLHLS>, got:\n%s", contents)
	}
	if strings.Contains(contents, "<OutputStreams>") {
		t.Fatalf("expected rendered config to avoid deprecated <OutputStreams> wrapper, got:\n%s", contents)
	}
	if !strings.Contains(contents, "<OutputStreamName>${OriginStreamName}</OutputStreamName>") {
		t.Fatalf("expected rendered config to include <OutputStreamName>${OriginStreamName}</OutputStreamName>, got:\n%s", contents)
	}
	var parsed struct {
		IP   string `xml:"IP"`
		Bind struct {
			Address  string `xml:"Address"`
			IP       string `xml:"IP"`
			Managers struct {
				API struct {
					Port    string `xml:"Port"`
					TLSPort string `xml:"TLSPort"`
				} `xml:"API"`
			} `xml:"Managers"`
		} `xml:"Bind"`
	}

	if err := xml.Unmarshal(output, &parsed); err != nil {
		t.Fatalf("parse rendered output: %v", err)
	}

	if parsed.IP != "0.0.0.0" {
		t.Fatalf("expected root IP to be rendered, got %q", parsed.IP)
	}
	if parsed.Bind.IP != "" || parsed.Bind.Address != "" {
		t.Fatalf("expected root bind to omit host tags, got IP=%q Address=%q", parsed.Bind.IP, parsed.Bind.Address)
	}
	if parsed.Bind.Managers.API.Port != "8081" || parsed.Bind.Managers.API.TLSPort != "8082" {
		t.Fatalf("expected API ports to be rewritten, got %s/%s", parsed.Bind.Managers.API.Port, parsed.Bind.Managers.API.TLSPort)
	}
}

func TestOmeConfigRenderingPreservesXmlComments(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)

	envContents := strings.Join([]string{
		"BITRIVER_OME_BIND=0.0.0.0",
		"BITRIVER_OME_IP=0.0.0.0",
		"BITRIVER_OME_SERVER_PORT=8081",
		"BITRIVER_OME_SERVER_TLS_PORT=8082",
		"BITRIVER_OME_USERNAME=admin",
		"BITRIVER_OME_PASSWORD=password",
		"BITRIVER_OME_API_TOKEN=token",
		"BITRIVER_OME_TCP_RELAY=*:3478",
		"BITRIVER_OME_ICE_CANDIDATE=*:10000-10009/udp",
		"BITRIVER_OME_IMAGE_TAG=0.16.0",
	}, "\n")
	output := renderOMEConfig(t, repoRoot, envContents)

	comment := "<!-- Root <IP> is the canonical bind host; keep protocol sections inside <Bind>. -->"
	if !strings.Contains(string(output), comment) {
		t.Fatalf("expected comment to be preserved, got:\n%s", string(output))
	}
	var parsed struct{}
	if err := xml.Unmarshal(output, &parsed); err != nil {
		t.Fatalf("expected well-formed XML output, got error: %v", err)
	}
}

func TestOmeConfigRenderingUsesCanonicalAPIAccessToken(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)
	envContents := strings.Join([]string{
		"BITRIVER_OME_BIND=0.0.0.0",
		"BITRIVER_OME_IP=0.0.0.0",
		"BITRIVER_OME_SERVER_PORT=9000",
		"BITRIVER_OME_SERVER_TLS_PORT=9443",
		"BITRIVER_OME_API_TOKEN=token<&>'\"",
		"BITRIVER_OME_TCP_RELAY=*:3478",
		"BITRIVER_OME_ICE_CANDIDATE=*:10000-10009/udp",
		"BITRIVER_OME_IMAGE_TAG=0.16.0",
	}, "\n")
	rendered := renderOMEConfig(t, repoRoot, envContents)

	contents := string(rendered)
	if !strings.Contains(contents, "<Managers>") || !strings.Contains(contents, "<API>") {
		t.Fatalf("expected rendered config to include top-level <Managers><API> auth block, got:\n%s", contents)
	}
	if strings.Contains(contents, "<AccessTokens>") {
		t.Fatalf("expected canonical <AccessToken> without <AccessTokens> wrapper, got:\n%s", contents)
	}
	if strings.Contains(contents, "<Authentication>") {
		t.Fatalf("expected rendered config to omit unsupported API <Authentication> block, got:\n%s", contents)
	}
	var parsed struct {
		Managers struct {
			API struct {
				AccessToken string `xml:"AccessToken"`
				Port        string `xml:"Port"`
				TLSPort     string `xml:"TLSPort"`
				WorkerCount string `xml:"WorkerCount"`
			} `xml:"API"`
		} `xml:"Managers"`
		Bind struct {
			Managers struct {
				API struct {
					AccessToken string `xml:"AccessToken"`
					Port        string `xml:"Port"`
					TLSPort     string `xml:"TLSPort"`
					WorkerCount string `xml:"WorkerCount"`
				} `xml:"API"`
			} `xml:"Managers"`
		} `xml:"Bind"`
	}
	if err := xml.Unmarshal(rendered, &parsed); err != nil {
		t.Fatalf("expected rendered config to be parseable XML, got error: %v", err)
	}
	if parsed.Managers.API.AccessToken != "token<&>'\"" {
		t.Fatalf("expected top-level auth access token to unmarshal correctly, got %q", parsed.Managers.API.AccessToken)
	}
	if parsed.Managers.API.Port != "" || parsed.Managers.API.TLSPort != "" || parsed.Managers.API.WorkerCount != "" {
		t.Fatalf("expected top-level <Managers><API> auth block to omit listener-only fields, got port=%q tls=%q workers=%q", parsed.Managers.API.Port, parsed.Managers.API.TLSPort, parsed.Managers.API.WorkerCount)
	}
	if parsed.Bind.Managers.API.AccessToken != "" {
		t.Fatalf("expected <Bind><Managers><API> to omit auth-only <AccessToken>, got %q", parsed.Bind.Managers.API.AccessToken)
	}
	if parsed.Bind.Managers.API.Port != "8081" || parsed.Bind.Managers.API.TLSPort != "8082" || parsed.Bind.Managers.API.WorkerCount != "1" {
		t.Fatalf("expected <Bind><Managers><API> listener fields to follow BITRIVER_OME_HTTP_* defaults, got port=%q tls=%q workers=%q", parsed.Bind.Managers.API.Port, parsed.Bind.Managers.API.TLSPort, parsed.Bind.Managers.API.WorkerCount)
	}
}
func renderOMEConfig(t *testing.T, repoRoot, envContents string) []byte {
	t.Helper()

	envPath := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(envPath, []byte(envContents+"\n"), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	outputPath := filepath.Join(t.TempDir(), "Server.generated.xml")

	cmd := exec.Command("go", "run", "./cmd/bitriver", "ome", "render", "--force", "--env-file", envPath, "--output", outputPath)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"GOTOOLCHAIN=local",
		"GOPROXY=off",
		"GOSUMDB=off",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go renderer failed: %v\n%s", err, output)
	}

	rendered, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read generated output: %v", err)
	}

	return rendered
}

func extractSection(output, startMarker, endMarker string) string {
	start := strings.Index(output, startMarker)
	end := strings.Index(output, endMarker)
	if start == -1 || end == -1 || end <= start {
		return ""
	}
	section := output[start+len(startMarker) : end]
	return strings.TrimSpace(section)
}

func extractTagValue(t *testing.T, output, tag string) string {
	t.Helper()

	re := regexp.MustCompile(fmt.Sprintf(`(?s)<%s>(.*?)</%s>`, tag, tag))
	matches := re.FindStringSubmatch(output)
	if len(matches) < 2 {
		t.Fatalf("missing <%s> tag in output:\n%s", tag, output)
	}
	return matches[1]
}
