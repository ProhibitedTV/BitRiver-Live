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
	ci := strings.ReplaceAll(
		readRepoFile(t, repoRoot, filepath.Join(".github", "workflows", "ci.yml")),
		"\r\n",
		"\n",
	)

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

func TestCIPathFiltersUseCompleteGitDiff(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	ci := strings.ReplaceAll(
		readRepoFile(t, repoRoot, filepath.Join(".github", "workflows", "ci.yml")),
		"\r\n",
		"\n",
	)

	checkoutStart := strings.Index(ci, "  changed-files:\n")
	filterStart := strings.Index(ci, "        uses: dorny/paths-filter@")
	filtersStart := strings.Index(ci, "          filters: |\n")
	if checkoutStart < 0 || filterStart <= checkoutStart || filtersStart <= filterStart {
		t.Fatal("CI workflow is missing the changed-file checkout or pinned path filter")
	}
	changedFilesJob := ci[checkoutStart:filterStart]
	filterInputs := ci[filterStart:filtersStart]
	if !strings.Contains(changedFilesJob, "          fetch-depth: 0\n") {
		t.Fatal("changed-file routing must check out complete history for git diff")
	}
	if !strings.Contains(filterInputs, "          token: ''\n") {
		t.Fatal("pull-request path filtering must use git diff instead of the 3,000-file REST API result")
	}
}

func TestCIAggregateMergeGateOwnsEveryChildResult(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	ci := strings.ReplaceAll(
		readRepoFile(t, repoRoot, filepath.Join(".github", "workflows", "ci.yml")),
		"\r\n",
		"\n",
	)
	gateStart := strings.Index(ci, "  merge-gate:\n")
	if gateStart < 0 {
		t.Fatal("CI workflow is missing the aggregate merge-gate job")
	}
	gate := ci[gateStart:]

	for _, required := range []string{
		"    name: Merge gate\n",
		"    if: always()\n",
		"          fetch-depth: 0\n",
		"        continue-on-error: true\n",
		`jq -r '.pull_request.body // ""' "$GITHUB_EVENT_PATH"`,
		"            --strict-if-risky | tee \"$RUNNER_TEMP/pr-release-scorecard.txt\"",
		"        run: ./scripts/check-ci-merge-gate.sh\n",
		"uses: actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a # v7.0.1",
		"${{ runner.temp }}/merge-gate-summary.md",
		"          if-no-files-found: error\n",
	} {
		if !strings.Contains(gate, required) {
			t.Errorf("aggregate merge gate missing %q", required)
		}
	}

	jobs := []string{
		"secret-guard",
		"changed-files",
		"verify",
		"go-verify",
		"quickstart-smoke",
		"viewer-ci",
		"shellcheck",
		"docs-consistency",
		"monitoring-config",
		"go-workflow-consistency",
		"wizard-release",
		"image-scan",
	}
	for _, job := range jobs {
		if !strings.Contains(gate, "      - "+job+"\n") {
			t.Errorf("aggregate merge gate needs list is missing %s", job)
		}
		if !strings.Contains(gate, "${{ needs."+job+".result }}") {
			t.Errorf("aggregate merge gate does not evaluate %s result", job)
		}
	}

	for _, output := range []string{
		"verify_changed",
		"go_changed",
		"deploy_changed",
		"viewer_changed",
		"monitoring_changed",
		"docs_changed",
		"shell_changed",
		"image_scan_changed",
		"quickstart_changed",
		"wizard_release_changed",
		"go_workflow_changed",
	} {
		if !strings.Contains(gate, "needs.changed-files.outputs."+output) {
			t.Errorf("aggregate merge gate does not consume %s", output)
		}
	}
}

func TestQuickstartEntrypointMatrixUsesRepositoryGoToolchain(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	workflow := readRepoFile(t, repoRoot, filepath.Join(".github", "workflows", "quickstart-smoke.yml"))
	checkout := strings.Index(workflow, "name: Check out repository")
	setupGo := strings.Index(workflow, "name: Set up Go toolchain")
	validate := strings.Index(workflow, "name: Validate quickstart.sh help and static checks")
	if checkout < 0 || setupGo < 0 || validate < 0 {
		t.Fatal("quickstart entrypoint workflow is missing checkout, Go setup, or validation")
	}
	if !(checkout < setupGo && setupGo < validate) {
		t.Fatal("quickstart entrypoint matrix must set up the repository Go toolchain after checkout and before validation")
	}
	if count := strings.Count(workflow, "uses: ./.github/actions/setup-go"); count != 2 {
		t.Fatalf("quickstart workflow shared Go setup count=%d, want 2", count)
	}
}

func TestStandaloneGoVerificationRestoresDependencyNetworkForCompose(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	workflow := readRepoFile(t, repoRoot, filepath.Join(".github", "workflows", "go-unit-tests.yml"))
	start := strings.Index(workflow, "- name: Run verification gate")
	end := strings.Index(workflow, "# ci-contract: allow-duplicate")
	if start < 0 || end <= start {
		t.Fatal("standalone Go workflow is missing the bounded Ubuntu verification step")
	}
	verificationStep := workflow[start:end]
	for _, required := range []string{
		"GOPROXY: https://proxy.golang.org,direct",
		"GOSUMDB: sum.golang.org",
		"run: ./scripts/verify.sh",
	} {
		if !strings.Contains(verificationStep, required) {
			t.Errorf("standalone Go verification step missing %q", required)
		}
	}
}

func TestGeneratedSRSAssetsUseStableLineEndings(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	attributes := readRepoFile(t, repoRoot, ".gitattributes")
	for _, required := range []string{
		"deploy/srs/conf/srs.conf text eol=lf",
		"deploy/helm/bitriver-live/files/srs.conf text eol=lf",
	} {
		if !strings.Contains(attributes, required) {
			t.Errorf(".gitattributes missing generated SRS line-ending invariant %q", required)
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

func TestReusableImageScanUsesVerifiedModuleProxyForComposeBuilds(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	workflow := readRepoFile(t, repoRoot, filepath.Join(".github", "workflows", "image-scan.yml"))
	start := strings.Index(workflow, "- name: Build local images from the Compose stack")
	end := strings.Index(workflow, "- name: Collect Compose images")
	if start < 0 || end <= start {
		t.Fatal("image scan workflow is missing the Compose image build step")
	}
	buildStep := workflow[start:end]
	for _, required := range []string{
		"GOPROXY: https://proxy.golang.org,direct",
		"GOSUMDB: sum.golang.org",
		"docker compose --env-file .ci.env -f deploy/docker-compose.yml build",
	} {
		if !strings.Contains(buildStep, required) {
			t.Errorf("image scan Compose build step missing %q", required)
		}
	}

	verify := readRepoFile(t, repoRoot, filepath.Join("scripts", "verify.sh"))
	if !strings.Contains(verify, "env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off") {
		t.Fatal("host Go verification must remain offline")
	}
}

func TestVerifySelectsPythonRunnerForMarkdownChecks(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	verify := readRepoFile(t, repoRoot, filepath.Join("scripts", "verify.sh"))
	for _, required := range []string{
		"PYTHON_RUNNER=()",
		"PYTHON_RUNNER=(python3)",
		"PYTHON_RUNNER=(py -3)",
		"PYTHON_RUNNER=(python)",
		`run_step "Markdown local-link checker tests" "${PYTHON_RUNNER[@]}" -m unittest scripts/check_doc_links_test.py`,
		`run_step "Markdown local-link check" "${PYTHON_RUNNER[@]}" scripts/check_doc_links.py`,
	} {
		if !strings.Contains(verify, required) {
			t.Errorf("verify.sh Python runner wiring missing %q", required)
		}
	}
}

func TestReusableImageScanProvesViewerOnNativeArm64(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	workflow := readRepoFile(t, repoRoot, filepath.Join(".github", "workflows", "image-scan.yml"))
	for _, required := range []string{
		"viewer-arm64:",
		"runs-on: ubuntu-24.04-arm",
		"timeout-minutes: 20",
		`test "$(uname -m)" = "aarch64"`,
		"docker build --platform linux/arm64",
		`--format '{{.Architecture}}'`,
		"docker run --rm --entrypoint uname bitriver-viewer:arm64-ci -m",
		"docker run --rm --entrypoint node bitriver-viewer:arm64-ci --version",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("image scan workflow missing native viewer arm64 invariant %q", required)
		}
	}
	if strings.Contains(workflow, "setup-qemu") || strings.Contains(workflow, "Set up QEMU") {
		t.Fatal("image scan workflow must not emulate the native viewer arm64 proof")
	}

	ci := strings.ReplaceAll(
		readRepoFile(t, repoRoot, filepath.Join(".github", "workflows", "ci.yml")),
		"\r\n",
		"\n",
	)
	filterStart := strings.Index(ci, "            image_scan_changed:\n")
	filterEnd := strings.Index(ci, "            quickstart_changed:\n")
	if filterStart == -1 || filterEnd <= filterStart {
		t.Fatal("CI workflow is missing the image-scan path filter")
	}
	if !strings.Contains(ci[filterStart:filterEnd], "'.github/workflows/release.yml'") {
		t.Fatal("release workflow changes must trigger native viewer arm64 proof")
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
