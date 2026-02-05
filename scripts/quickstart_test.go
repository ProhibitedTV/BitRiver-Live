package scripts_test

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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
	expected := []string{
		fmt.Sprintf("%s:run ./cmd/bitriver quickstart --env-file %s --compose-file %s", tempDir, envPath, composePath),
	}

	if !reflect.DeepEqual(lines, expected) {
		t.Fatalf("unexpected go invocations:\n%s", strings.Join(lines, "\n"))
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
	if parsed.Bind.Managers.API.Port != "9000" || parsed.Bind.Managers.API.TLSPort != "9443" || parsed.Bind.Managers.API.WorkerCount != "1" {
		t.Fatalf("expected <Bind><Managers><API> listener fields to remain present, got port=%q tls=%q workers=%q", parsed.Bind.Managers.API.Port, parsed.Bind.Managers.API.TLSPort, parsed.Bind.Managers.API.WorkerCount)
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
