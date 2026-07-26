package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIUsesReusableWorkflowSources(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	ci := readRepoFile(t, repoRoot, filepath.Join(".github", "workflows", "ci.yml"))

	reusable := []string{
		"quickstart-smoke.yml",
		"viewer-ci.yml",
		"shellcheck.yml",
		"docs-consistency.yml",
		"monitoring-config.yml",
		"go-workflow-consistency.yml",
		"wizard-release.yml",
		"image-scan.yml",
	}
	for _, name := range reusable {
		call := "uses: ./.github/workflows/" + name
		if !strings.Contains(ci, call) {
			t.Errorf("CI orchestrator must call reusable workflow %q", call)
		}
		workflow := readRepoFile(t, repoRoot, filepath.Join(".github", "workflows", name))
		if !strings.Contains(workflow, "workflow_call:") {
			t.Errorf("%s must accept workflow_call", name)
		}
	}

	for _, duplicate := range []string{
		"npm audit --audit-level=high",
		"TRIVY_VERSION=",
		"./scripts/check-monitoring-config.sh",
		"./scripts/check-doc-installer-language.sh",
		"bash scripts/test-wizard-release.sh",
	} {
		if strings.Contains(ci, duplicate) {
			t.Errorf("CI orchestrator retains duplicated workflow implementation %q", duplicate)
		}
	}

	if !strings.Contains(ci, "run_compose_smoke: false") {
		t.Fatal("CI must let the unified Ubuntu gate own Compose smoke")
	}
}

func TestReusableWorkflowChangesSelectTheirCIJobs(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	ci := readRepoFile(t, repoRoot, filepath.Join(".github", "workflows", "ci.yml"))
	for _, workflow := range []string{
		"quickstart-smoke.yml",
		"viewer-ci.yml",
		"shellcheck.yml",
		"docs-consistency.yml",
		"monitoring-config.yml",
		"wizard-release.yml",
		"image-scan.yml",
	} {
		if count := strings.Count(ci, ".github/workflows/"+workflow); count < 2 {
			t.Errorf("%s must appear in its path filter and reusable call, found %d references", workflow, count)
		}
	}
	for _, action := range []string{
		".github/actions/setup-go/action.yml",
		".github/actions/setup-node-viewer/action.yml",
	} {
		if !strings.Contains(ci, action) {
			t.Errorf("CI path filters must select checks when %s changes", action)
		}
	}
}

func TestSetupActionsDoNotOwnCheckout(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	for _, relativePath := range []string{
		filepath.Join(".github", "actions", "setup-go", "action.yml"),
		filepath.Join(".github", "actions", "setup-node-viewer", "action.yml"),
	} {
		action := readRepoFile(t, repoRoot, relativePath)
		if strings.Contains(action, "actions/checkout@") {
			t.Errorf("%s must not perform a hidden second checkout", relativePath)
		}
		if strings.Contains(action, "checkout-fetch-depth") {
			t.Errorf("%s retains obsolete checkout ownership input", relativePath)
		}
	}
}

func TestReusableImageScanUsesCurrentBoundedInstaller(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	workflow := readRepoFile(t, repoRoot, filepath.Join(".github", "workflows", "image-scan.yml"))
	for _, required := range []string{
		"TRIVY_VERSION=0.70.0",
		"--retry 5",
		"--retry-all-errors",
		"test -s \"$archive\"",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("image scan workflow missing %q", required)
		}
	}
}

func TestMonitoringValidationSelectsContainerToolsExplicitly(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	script := readRepoFile(t, repoRoot, filepath.Join("scripts", "check-monitoring-config.sh"))
	if count := strings.Count(script, "--entrypoint /bin/promtool"); count != 2 {
		t.Fatalf("monitoring validation promtool entrypoint count=%d, want 2", count)
	}
	if count := strings.Count(script, "--entrypoint /bin/amtool"); count != 1 {
		t.Fatalf("monitoring validation amtool entrypoint count=%d, want 1", count)
	}
	for _, required := range []string{
		"umask 077",
		"monitoring-config-validation-only",
		`promtool check config "$PROM_CONFIG_VALIDATION"`,
		`cygpath -w "$1"`,
		"MSYS_NO_PATHCONV=1 docker run",
		`source=$PROM_CONFIG_MOUNT,target=/etc/prometheus/prometheus.yml,readonly`,
		`source=$PROM_RULES_MOUNT,target=/etc/prometheus/rules/prometheus-alerts.yml,readonly`,
		`source=$PROM_TOKEN_MOUNT,target=/etc/prometheus/metrics.token,readonly`,
		`--user "$(id -u):$(id -g)"`,
		`source=$ALERT_CONFIG_MOUNT,target=/etc/alertmanager/alertmanager.yml,readonly`,
		`if [[ ! -e "$COMPOSE_ENV" && ! -L "$COMPOSE_ENV" ]]`,
		`cp "$ROOT_DIR/deploy/.env.example" "$COMPOSE_ENV"`,
		`COMPOSE_ENV_CREATED=1`,
		`if [[ "$COMPOSE_ENV_CREATED" == "1" ]]`,
		`rm -f "$COMPOSE_ENV"`,
		`docker compose --env-file "$ROOT_DIR/deploy/.env.example"`,
	} {
		if !strings.Contains(script, required) {
			t.Errorf("monitoring validation missing non-secret token fixture %q", required)
		}
	}
	for _, stale := range []string{
		"prom/prometheus:v2.51.2 promtool",
		"prom/alertmanager:v0.27.0 amtool",
	} {
		if strings.Contains(script, stale) {
			t.Errorf("monitoring validation retains image-default entrypoint seam %q", stale)
		}
	}
}

func TestAlertmanagerRendererExportsValidationDefaults(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "alertmanager.yml")
	missingEnvPath := filepath.Join(tempDir, "missing.env")
	renderScript := filepath.Join(repoRoot, "scripts", "render-alertmanager-config.sh")

	cmd := exec.Command(
		testBash(t),
		shellPath(renderScript),
		"--env-file",
		shellPath(missingEnvPath),
		"--output",
		shellPath(outputPath),
	)
	cmd.Env = append(os.Environ(),
		"BITRIVER_ALERTMANAGER_DEFAULT_WEBHOOK_URL=",
		"BITRIVER_ALERTMANAGER_DEFAULT_WEBHOOK_TOKEN=",
		"BITRIVER_ALERTMANAGER_CRITICAL_WEBHOOK_URL=",
		"BITRIVER_ALERTMANAGER_CRITICAL_WEBHOOK_TOKEN=",
		"BITRIVER_ALERTMANAGER_AUTH_WEBHOOK_URL=",
		"BITRIVER_ALERTMANAGER_AUTH_WEBHOOK_TOKEN=",
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("render Alertmanager validation defaults: %v\n%s", err, output)
	}

	renderedBytes, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read rendered Alertmanager config: %v", err)
	}
	rendered := string(renderedBytes)
	for _, required := range []string{
		"http://example.invalid/default",
		"replace-default-token",
		"http://example.invalid/critical",
		"replace-critical-token",
		"http://example.invalid/auth",
		"replace-auth-token",
	} {
		if !strings.Contains(rendered, required) {
			t.Errorf("rendered Alertmanager config missing exported default %q", required)
		}
	}
}
