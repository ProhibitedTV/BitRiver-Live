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

func TestQuickstartReconciliation(t *testing.T) {
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
echo "__ENV_BEGIN__"
cat "$ENV_FILE"
echo "__ENV_END__"
`, stubBin, tempDir)

	cmd := exec.Command("bash", "-c", bashScript)
	cmd.Dir = tempDir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout

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
	if !strings.Contains(envContent, "BITRIVER_OME_BIND=0.0.0.0") {
		t.Fatalf("expected BITRIVER_OME_BIND to be appended, got:\n%s", envContent)
	}
	if !strings.Contains(envContent, "BITRIVER_SRS_TOKEN=custom-token") {
		t.Fatalf("expected existing BITRIVER_SRS_TOKEN to be preserved, got:\n%s", envContent)
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

	lines := strings.Split(string(content), "\n")
	guardLine := "if [[ ${BITRIVER_OME_CUSTOM_CONFIG:-} == \"1\" ]]; then"

	guardActive := false
	found := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		switch trimmed {
		case guardLine:
			guardActive = true
			continue
		case "fi":
			guardActive = false
			continue
		case "render_ome_config":
			if guardActive {
				t.Fatalf("render_ome_config should run even when BITRIVER_OME_CUSTOM_CONFIG is unset")
			}
			found = true
		}
	}

	if !found {
		t.Fatalf("render_ome_config invocation not found in quickstart")
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

func extractSection(output, startMarker, endMarker string) string {
	start := strings.Index(output, startMarker)
	end := strings.Index(output, endMarker)
	if start == -1 || end == -1 || end <= start {
		return ""
	}
	section := output[start+len(startMarker) : end]
	return strings.TrimSpace(section)
}
