package scripts_test

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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

func TestOmeConfigRenderingHandlesBindAsIp(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)

	tempDir := t.TempDir()
	templatePath := filepath.Join(repoRoot, "deploy", "ome", "Server.xml")
	outputPath := filepath.Join(tempDir, "Server.generated.xml")

	renderer := filepath.Join(repoRoot, "scripts", "render_ome_config.py")
	cmd := exec.Command("python3", renderer,
		"--template", templatePath,
		"--output", outputPath,
		"--bind", "0.0.0.0",
		"--tcp-relay", "*:3478",
		"--ice-candidate", "*:10000-10009/udp",
		"--port", "8081",
		"--tls-port", "8082",
		"--username", "admin",
		"--password", "password",
		"--api-token", "token")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("python render failed: %v; stderr: %s", err, stderr.String())
	}

	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	var parsed struct {
		IP   string `xml:"IP"`
		Bind struct {
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
	if parsed.Bind.Managers.API.Port != "8081" || parsed.Bind.Managers.API.TLSPort != "8082" {
		t.Fatalf("expected API ports to be rewritten, got %s/%s", parsed.Bind.Managers.API.Port, parsed.Bind.Managers.API.TLSPort)
	}
}

func TestOmeConfigRenderingEscapesXml(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "Server.generated.xml")
	templatePath := filepath.Join(t.TempDir(), "Server.template.xml")

	template := strings.TrimSpace(`<?xml version="1.0" encoding="utf-8"?>
<Server>
    <IP>*</IP>
    <Bind>
        <Address>0.0.0.0</Address>
        <Port>1935</Port>
        <TLSPort>2935</TLSPort>
    </Bind>
    <IceCandidates>
        <TcpRelay>*:3478</TcpRelay>
        <IceCandidate>*:10000-10009/udp</IceCandidate>
    </IceCandidates>
    <AccessTokens>
        <AccessToken>token</AccessToken>
    </AccessTokens>
    <Modules>
        <Control>
            <Authentication>
                <User>
                    <ID>admin</ID>
                    <Password>password</Password>
                </User>
            </Authentication>
        </Control>
    </Modules>
</Server>`) + "\n"

	if err := os.WriteFile(templatePath, []byte(template), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)
	renderer := filepath.Join(repoRoot, "scripts", "render_ome_config.py")

	cmd := exec.Command("python3", renderer,
		"--template", templatePath,
		"--output", outputPath,
		"--bind", "0.0.0.0",
		"--tcp-relay", "*:3478",
		"--ice-candidate", "*:10000-10009/udp",
		"--port", "9000",
		"--tls-port", "9443",
		"--username", "admin<&",
		"--password", `pass<&>'"`,
		"--api-token", "token")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("ome config render failed: %v; stderr: %s", err, stderr.String())
	}

	rendered, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read rendered output: %v", err)
	}

	contents := string(rendered)
	if !strings.Contains(contents, "admin&lt;&amp;") {
		t.Fatalf("expected username to be escaped, got:\n%s", contents)
	}
	if !strings.Contains(contents, "pass&lt;&amp;&gt;&apos;&quot;") {
		t.Fatalf("expected password to be escaped, got:\n%s", contents)
	}
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
