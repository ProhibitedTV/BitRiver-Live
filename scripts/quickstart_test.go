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

func TestQuickstartDelegatesToCli(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)
	tempDir := t.TempDir()

	scriptDir := filepath.Join(tempDir, "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("create script dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tempDir, "cmd", "bitriver"), 0o755); err != nil {
		t.Fatalf("create fake cli dir: %v", err)
	}

	quickstartSrc := filepath.Join(repoRoot, "scripts", "quickstart.sh")
	quickstartDst := filepath.Join(scriptDir, "quickstart.sh")
	scriptBytes, err := os.ReadFile(quickstartSrc)
	if err != nil {
		t.Fatalf("read quickstart: %v", err)
	}
	if err := os.WriteFile(quickstartDst, scriptBytes, 0o755); err != nil {
		t.Fatalf("write quickstart: %v", err)
	}

	logPath := filepath.Join(tempDir, "go-log.txt")
	stubBin := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(stubBin, 0o755); err != nil {
		t.Fatalf("create stub bin: %v", err)
	}
	goStubPath := filepath.Join(stubBin, "go")
	goStubBytes := []byte("#!/usr/bin/env bash\nset -euo pipefail\necho \"$(pwd):$*\" >>\"$GO_LOG\"\n")
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

	scriptDir := filepath.Join(tempDir, "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("create script dir: %v", err)
	}
	quickstartSrc := filepath.Join(repoRoot, "scripts", "quickstart.sh")
	quickstartDst := filepath.Join(scriptDir, "quickstart.sh")
	scriptBytes, err := os.ReadFile(quickstartSrc)
	if err != nil {
		t.Fatalf("read quickstart: %v", err)
	}
	if err := os.WriteFile(quickstartDst, scriptBytes, 0o755); err != nil {
		t.Fatalf("write quickstart: %v", err)
	}

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
	if !strings.Contains(script, "Start-Process -FilePath $goPath") {
		t.Fatalf("expected PowerShell quickstart wrapper to invoke the CLI through Start-Process using the resolved go.exe path")
	}
	if !strings.Contains(script, "$processPath = [System.Environment]::GetEnvironmentVariable('Path', 'Process')") {
		t.Fatalf("expected PowerShell quickstart wrapper to capture the process Path value before normalizing environment casing")
	}
	if !strings.Contains(script, "$processPATH = [System.Environment]::GetEnvironmentVariable('PATH', 'Process')") {
		t.Fatalf("expected PowerShell quickstart wrapper to capture the process PATH value before normalizing environment casing")
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
		t.Fatalf("expected PowerShell quickstart wrapper to restore the original GOCACHE value after Start-Process")
	}
	if !strings.Contains(script, "SetEnvironmentVariable('Path', $normalizedPath, 'Process')") {
		t.Fatalf("expected PowerShell quickstart wrapper to preserve a canonical Path value for the child process")
	}
	if !strings.Contains(script, "SetEnvironmentVariable('PATH', $null, 'Process')") {
		t.Fatalf("expected PowerShell quickstart wrapper to drop the duplicate PATH key before Start-Process")
	}
	if !strings.Contains(script, "SetEnvironmentVariable('PATH', $processPATH, 'Process')") {
		t.Fatalf("expected PowerShell quickstart wrapper to restore the original PATH value after Start-Process")
	}
	if !strings.Contains(script, "$process.ExitCode") {
		t.Fatalf("expected PowerShell quickstart wrapper to inspect the child process exit code")
	}
	if !strings.Contains(script, "flag: help requested") {
		t.Fatalf("expected PowerShell quickstart wrapper to allow the validate-only help path")
	}
	if !strings.Contains(script, "exit $exitCode") {
		t.Fatalf("expected PowerShell quickstart wrapper to return non-zero CLI exit codes")
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

func TestDockerfileDropsStubbedPuddleInRealPgxMode(t *testing.T) {
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
	if !strings.Contains(dockerfile, `go mod edit -dropreplace=github.com/jackc/pgx/v5;`) {
		t.Fatalf("expected real pgx mode to drop the local pgx replacement")
	}
	if !strings.Contains(dockerfile, `go mod edit -dropreplace=github.com/jackc/puddle/v2;`) {
		t.Fatalf("expected real pgx mode to drop the local puddle replacement")
	}
	if !strings.Contains(dockerfile, `go mod edit -dropreplace=golang.org/x/text;`) {
		t.Fatalf("expected real pgx mode to drop the local x/text replacement")
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
