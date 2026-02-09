package scripts_test

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

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

	quickstartSrc := filepath.Join(repoRoot, "scripts", "quickstart.sh")
	quickstartDst := filepath.Join(scriptDir, "quickstart.sh")
	scriptBytes, err := os.ReadFile(quickstartSrc)
	if err != nil {
		t.Fatalf("read quickstart: %v", err)
	}
	if err := os.WriteFile(quickstartDst, scriptBytes, 0o755); err != nil {
		t.Fatalf("write quickstart: %v", err)
	}

	verifySrc := filepath.Join(repoRoot, "scripts", "verify-ome-health-token.sh")
	verifyDst := filepath.Join(scriptDir, "verify-ome-health-token.sh")
	verifyBytes, err := os.ReadFile(verifySrc)
	if err != nil {
		t.Fatalf("read verify script: %v", err)
	}
	if err := os.WriteFile(verifyDst, verifyBytes, 0o755); err != nil {
		t.Fatalf("write verify script: %v", err)
	}

	logPath := filepath.Join(tempDir, "go-log.txt")
	stubBin := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(stubBin, 0o755); err != nil {
		t.Fatalf("create stub bin: %v", err)
	}
	goStub := "#!/usr/bin/env bash\nset -euo pipefail\necho \"$(pwd):$*\" >>\"$GO_LOG\"\n"
	if err := os.WriteFile(filepath.Join(stubBin, "go"), []byte(goStub), 0o755); err != nil {
		t.Fatalf("write go stub: %v", err)
	}

	envPath := filepath.Join(tempDir, "custom.env")
	envContents := strings.Join([]string{
		"BITRIVER_OME_API_TOKEN=token",
		"BITRIVER_OME_ACCESS_TOKEN=token",
	}, "\n") + "\n"
	if err := os.WriteFile(envPath, []byte(envContents), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}
	generatedConfigPath := filepath.Join(tempDir, "deploy", "ome", "Server.generated.xml")
	if err := os.MkdirAll(filepath.Dir(generatedConfigPath), 0o755); err != nil {
		t.Fatalf("create generated config dir: %v", err)
	}
	generatedConfig := `<Server><Managers><API><AccessToken>token</AccessToken></API></Managers></Server>`
	if err := os.WriteFile(generatedConfigPath, []byte(generatedConfig), 0o644); err != nil {
		t.Fatalf("write generated config: %v", err)
	}
	composePath := filepath.Join(tempDir, "deploy", "custom-compose.yml")
	if err := os.MkdirAll(filepath.Dir(composePath), 0o755); err != nil {
		t.Fatalf("create compose dir: %v", err)
	}

	cmd := exec.Command("bash", quickstartDst)
	cmd.Dir = tempDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PATH=%s:%s", stubBin, os.Getenv("PATH")),
		fmt.Sprintf("BITRIVER_QUICKSTART_REPO_ROOT=%s", tempDir),
		fmt.Sprintf("ENV_FILE=%s", envPath),
		fmt.Sprintf("COMPOSE_FILE=%s", composePath),
		fmt.Sprintf("GO_LOG=%s", logPath),
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

	lines := strings.Split(strings.TrimSpace(string(logContent)), "\n")
	if len(lines) != 2 {
		t.Fatalf("unexpected number of go invocations: %d\n%s", len(lines), strings.Join(lines, "\n"))
	}

	line1 := strings.TrimSpace(lines[0])
	sep1 := strings.Index(line1, ":run ")
	if sep1 == -1 {
		t.Fatalf("unexpected go invocation format: %s", line1)
	}
	invocation1 := line1[sep1+1:]
	expectedInvocation1 := fmt.Sprintf("run ./cmd/bitriver ome render --force --env-file %s", envPath)
	if invocation1 != expectedInvocation1 {
		t.Fatalf("unexpected preflight invocation:\n got: %s\nwant: %s", invocation1, expectedInvocation1)
	}

	line2 := strings.TrimSpace(lines[1])
	sep2 := strings.Index(line2, ":run ")
	if sep2 == -1 {
		t.Fatalf("unexpected go invocation format: %s", line2)
	}
	invocation2 := line2[sep2+1:]
	expectedInvocation2 := fmt.Sprintf("run ./cmd/bitriver quickstart --env-file %s --compose-file %s", envPath, composePath)
	if invocation2 != expectedInvocation2 {
		t.Fatalf("unexpected quickstart invocation:\n got: %s\nwant: %s", invocation2, expectedInvocation2)
	}
}

func TestQuickstartFailsOmeAuthPreflightWhenAPITokenMissing(t *testing.T) {
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

	verifySrc := filepath.Join(repoRoot, "scripts", "verify-ome-health-token.sh")
	verifyDst := filepath.Join(scriptDir, "verify-ome-health-token.sh")
	verifyBytes, err := os.ReadFile(verifySrc)
	if err != nil {
		t.Fatalf("read verify script: %v", err)
	}
	if err := os.WriteFile(verifyDst, verifyBytes, 0o755); err != nil {
		t.Fatalf("write verify script: %v", err)
	}

	stubBin := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(stubBin, 0o755); err != nil {
		t.Fatalf("create stub bin: %v", err)
	}
	goStub := `#!/usr/bin/env bash
set -euo pipefail
if [ "$1" = "run" ] && [ "$2" = "./cmd/bitriver" ] && [ "${3:-}" = "quickstart" ]; then
  echo "quickstart must not run when preflight fails" >&2
  exit 91
fi
exit 0
`
	if err := os.WriteFile(filepath.Join(stubBin, "go"), []byte(goStub), 0o755); err != nil {
		t.Fatalf("write go stub: %v", err)
	}

	envPath := filepath.Join(tempDir, ".env")
	if err := os.WriteFile(envPath, []byte("# intentionally empty\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	cmd := exec.Command("bash", quickstartDst)
	cmd.Dir = tempDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PATH=%s:%s", stubBin, os.Getenv("PATH")),
		fmt.Sprintf("BITRIVER_QUICKSTART_REPO_ROOT=%s", tempDir),
		fmt.Sprintf("ENV_FILE=%s", envPath),
	)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout

	err = cmd.Run()
	if err == nil {
		t.Fatalf("expected quickstart to fail when BITRIVER_OME_API_TOKEN is missing")
	}
	if exitErr := (&exec.ExitError{}); errors.As(err, &exitErr) {
		if exitErr.ExitCode() == 91 {
			t.Fatalf("quickstart CLI path executed despite failed preflight:\n%s", stdout.String())
		}
	}
	out := stdout.String()
	if !strings.Contains(out, "OME auth preflight failed: BITRIVER_OME_API_TOKEN is empty") {
		t.Fatalf("expected missing BITRIVER_OME_API_TOKEN preflight error, got:\n%s", out)
	}
	if !strings.Contains(out, "BITRIVER_OME_API_TOKEN") {
		t.Fatalf("expected BITRIVER_OME_API_TOKEN guidance in output, got:\n%s", out)
	}
}

func TestQuickstartFailsOmeAuthPreflightWhenAuthModeInvalid(t *testing.T) {
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

	verifySrc := filepath.Join(repoRoot, "scripts", "verify-ome-health-token.sh")
	verifyDst := filepath.Join(scriptDir, "verify-ome-health-token.sh")
	verifyBytes, err := os.ReadFile(verifySrc)
	if err != nil {
		t.Fatalf("read verify script: %v", err)
	}
	if err := os.WriteFile(verifyDst, verifyBytes, 0o755); err != nil {
		t.Fatalf("write verify script: %v", err)
	}

	envPath := filepath.Join(tempDir, ".env")
	envContents := strings.Join([]string{
		"BITRIVER_OME_HEALTHCHECK_AUTH_MODE=digest",
		"BITRIVER_OME_API_TOKEN=token",
	}, "\n") + "\n"
	if err := os.WriteFile(envPath, []byte(envContents), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	cmd := exec.Command("bash", quickstartDst)
	cmd.Dir = tempDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("BITRIVER_QUICKSTART_REPO_ROOT=%s", tempDir),
		fmt.Sprintf("ENV_FILE=%s", envPath),
	)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout

	err = cmd.Run()
	if err == nil {
		t.Fatalf("expected quickstart to fail for unsupported auth mode")
	}
	out := stdout.String()
	if !strings.Contains(out, "BITRIVER_OME_HEALTHCHECK_AUTH_MODE must be accesstoken, basic, or none/off/disabled") {
		t.Fatalf("expected auth mode validation error, got:\n%s", out)
	}
}

func TestQuickstartFailsOmeAuthPreflightWhenBasicAuthMissingCredentials(t *testing.T) {
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

	verifySrc := filepath.Join(repoRoot, "scripts", "verify-ome-health-token.sh")
	verifyDst := filepath.Join(scriptDir, "verify-ome-health-token.sh")
	verifyBytes, err := os.ReadFile(verifySrc)
	if err != nil {
		t.Fatalf("read verify script: %v", err)
	}
	if err := os.WriteFile(verifyDst, verifyBytes, 0o755); err != nil {
		t.Fatalf("write verify script: %v", err)
	}

	envPath := filepath.Join(tempDir, ".env")
	envContents := strings.Join([]string{
		"BITRIVER_OME_HEALTHCHECK_AUTH_MODE=basic",
		"BITRIVER_OME_API_TOKEN=token",
		"BITRIVER_OME_USERNAME=ome-user",
	}, "\n") + "\n"
	if err := os.WriteFile(envPath, []byte(envContents), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	cmd := exec.Command("bash", quickstartDst)
	cmd.Dir = tempDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("BITRIVER_QUICKSTART_REPO_ROOT=%s", tempDir),
		fmt.Sprintf("ENV_FILE=%s", envPath),
	)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout

	err = cmd.Run()
	if err == nil {
		t.Fatalf("expected quickstart to fail when basic auth credentials are incomplete")
	}
	out := stdout.String()
	if !strings.Contains(out, "BITRIVER_OME_HEALTHCHECK_AUTH_MODE=basic requires BITRIVER_OME_USERNAME and BITRIVER_OME_PASSWORD") {
		t.Fatalf("expected basic credential validation error, got:\n%s", out)
	}
}

func TestQuickstartAllowsMismatchedAccessAndAPITokensWhenCanonicalTokenIsProvided(t *testing.T) {
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

	verifySrc := filepath.Join(repoRoot, "scripts", "verify-ome-health-token.sh")
	verifyDst := filepath.Join(scriptDir, "verify-ome-health-token.sh")
	verifyBytes, err := os.ReadFile(verifySrc)
	if err != nil {
		t.Fatalf("read verify script: %v", err)
	}
	if err := os.WriteFile(verifyDst, verifyBytes, 0o755); err != nil {
		t.Fatalf("write verify script: %v", err)
	}

	stubBin := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(stubBin, 0o755); err != nil {
		t.Fatalf("create stub bin: %v", err)
	}
	goStub := `#!/usr/bin/env bash
set -euo pipefail
if [ "$1" = "run" ] && [ "$2" = "./cmd/bitriver" ] && [ "${3:-}" = "ome" ] && [ "${4:-}" = "render" ]; then
  exit 0
fi
if [ "$1" = "run" ] && [ "$2" = "./cmd/bitriver" ] && [ "${3:-}" = "quickstart" ]; then
  echo "quickstart invoked"
  exit 0
fi
exit 0
`
	if err := os.WriteFile(filepath.Join(stubBin, "go"), []byte(goStub), 0o755); err != nil {
		t.Fatalf("write go stub: %v", err)
	}

	envPath := filepath.Join(tempDir, ".env")
	envContents := strings.Join([]string{
		"BITRIVER_OME_API_TOKEN=api-token",
		"BITRIVER_OME_ACCESS_TOKEN=access-token",
		"BITRIVER_OME_HEALTHCHECK_TOKEN=healthcheck-token",
	}, "\n") + "\n"
	if err := os.WriteFile(envPath, []byte(envContents), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	configDir := filepath.Join(tempDir, "deploy", "ome")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	configContents := `<Server><Managers><API><AccessToken>healthcheck-token</AccessToken></API></Managers></Server>`
	if err := os.WriteFile(filepath.Join(configDir, "Server.generated.xml"), []byte(configContents), 0o644); err != nil {
		t.Fatalf("write generated config: %v", err)
	}

	cmd := exec.Command("bash", quickstartDst)
	cmd.Dir = tempDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PATH=%s:%s", stubBin, os.Getenv("PATH")),
		fmt.Sprintf("BITRIVER_QUICKSTART_REPO_ROOT=%s", tempDir),
		fmt.Sprintf("ENV_FILE=%s", envPath),
	)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout

	err = cmd.Run()
	if err != nil {
		t.Fatalf("expected quickstart to proceed with canonical token, got error: %v\n%s", err, stdout.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "quickstart invoked") {
		t.Fatalf("expected quickstart command to run after preflight, got:\n%s", out)
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

	if !strings.Contains(string(content), "run_cli quickstart") {
		t.Fatalf("quickstart invocation not found in quickstart script")
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

	outputPath := filepath.Join(repoRoot, "deploy", "ome", "Server.generated.xml")
	original, err := os.ReadFile(outputPath)
	var originalMode os.FileMode
	if err == nil {
		stat, statErr := os.Stat(outputPath)
		if statErr != nil {
			t.Fatalf("stat generated config: %v", statErr)
		}
		originalMode = stat.Mode()
		t.Cleanup(func() {
			_ = os.WriteFile(outputPath, original, originalMode)
		})
	} else if errors.Is(err, os.ErrNotExist) {
		t.Cleanup(func() {
			_ = os.Remove(outputPath)
		})
	} else {
		t.Fatalf("read generated config: %v", err)
	}

	cmd := exec.Command("go", "run", "./cmd/bitriver", "ome", "render", "--force", "--env-file", envPath)
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
