package scripts_test

import (
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
