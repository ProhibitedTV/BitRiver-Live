package scripts_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestQuickstartReconciliationAndRender(t *testing.T) {
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

	envFixture, err := os.ReadFile(filepath.Join(repoRoot, "scripts", "testdata", "quickstart", "env.partial"))
	if err != nil {
		t.Fatalf("read env fixture: %v", err)
	}
	envPath := filepath.Join(tempDir, ".env")
	if err := os.WriteFile(envPath, envFixture, 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	templateDst := filepath.Join(tempDir, "deploy", "ome")
	if err := os.MkdirAll(templateDst, 0o755); err != nil {
		t.Fatalf("create template dir: %v", err)
	}
	serverXMLBytes, err := os.ReadFile(filepath.Join(repoRoot, "scripts", "testdata", "quickstart", "Server.xml"))
	if err != nil {
		t.Fatalf("read server xml fixture: %v", err)
	}
	serverXMLPath := filepath.Join(templateDst, "Server.xml")
	if err := os.WriteFile(serverXMLPath, serverXMLBytes, 0o644); err != nil {
		t.Fatalf("write server xml: %v", err)
	}

	stubBin := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(stubBin, 0o755); err != nil {
		t.Fatalf("create stub bin: %v", err)
	}
	dockerStub := "#!/usr/bin/env bash\nif [[ $1 == compose ]]; then\n  exit 0\nelif [[ $1 == info ]]; then\n  exit 0\nfi\nexit 0\n"
	if err := os.WriteFile(filepath.Join(stubBin, "docker"), []byte(dockerStub), 0o755); err != nil {
		t.Fatalf("write docker stub: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stubBin, "curl"), []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write curl stub: %v", err)
	}

	bashScript := fmt.Sprintf(`
set -euo pipefail
PATH="%s:$PATH"
BITRIVER_QUICKSTART_TEST_MODE=1
export TMPDIR="%s"
source scripts/quickstart.sh
reconcile_env_file
render_ome_server_config
echo "__ENV_BEGIN__"
cat "$ENV_FILE"
echo "__ENV_END__"
echo "__OME_PATH__${OME_SERVER_XML_PATH}"
echo "__OME_BEGIN__"
cat "$OME_SERVER_XML_PATH"
echo "__OME_END__"
`, stubBin, tempDir)

	cmd := exec.Command("bash", "-c", bashScript)
	cmd.Dir = tempDir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout
	cmd.Env = append(os.Environ(), fmt.Sprintf("OME_TEMPLATE_PATH=%s", serverXMLPath))

	if err := cmd.Run(); err != nil {
		t.Fatalf("quickstart harness failed: %v\noutput:\n%s", err, stdout.String())
	}

	output := stdout.String()
	envContent := extractSection(output, "__ENV_BEGIN__", "__ENV_END__")
	if envContent == "" {
		t.Fatalf("failed to capture env content: output:\n%s", output)
	}
	if !strings.Contains(envContent, "BITRIVER_LIVE_IMAGE_TAG=latest") {
		t.Fatalf("expected BITRIVER_LIVE_IMAGE_TAG to be appended, got:\n%s", envContent)
	}
	if !strings.Contains(envContent, "BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD=bitriver") {
		t.Fatalf("expected BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD to be appended, got:\n%s", envContent)
	}
	if !strings.Contains(envContent, "BITRIVER_SRS_TOKEN=custom-token") {
		t.Fatalf("expected existing BITRIVER_SRS_TOKEN to be preserved, got:\n%s", envContent)
	}

	omeContent := extractSection(output, "__OME_BEGIN__", "__OME_END__")
	if omeContent == "" {
		t.Fatalf("failed to capture OME content: output:\n%s", output)
	}
	if !strings.Contains(omeContent, "<ID>ome-test-user</ID>") {
		t.Fatalf("expected OME username to be rendered, got:\n%s", omeContent)
	}
	if !strings.Contains(omeContent, "<Password>ome-test-pass</Password>") {
		t.Fatalf("expected OME password to be rendered, got:\n%s", omeContent)
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
