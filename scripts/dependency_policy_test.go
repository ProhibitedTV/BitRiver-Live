package scripts_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestViewerDependencyAuditsAreBlocking(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	for _, workflowPath := range []string{
		filepath.Join(".github", "workflows", "viewer-ci.yml"),
		filepath.Join(".github", "workflows", "release.yml"),
	} {
		workflow := readRepoFile(t, repoRoot, workflowPath)
		if !strings.Contains(workflow, "npm audit --audit-level=high") {
			t.Errorf("%s must run the high-severity npm audit", workflowPath)
		}
	}

	viewerWorkflow := readRepoFile(t, repoRoot, filepath.Join(".github", "workflows", "viewer-ci.yml"))
	if strings.Contains(viewerWorkflow, "continue-on-error: true") {
		t.Fatal("viewer audit must not be advisory")
	}

	mainWorkflow := readRepoFile(t, repoRoot, filepath.Join(".github", "workflows", "ci.yml"))
	if !strings.Contains(mainWorkflow, "uses: ./.github/workflows/viewer-ci.yml") {
		t.Fatal("main workflow must delegate viewer checks to the reusable workflow")
	}
	if strings.Contains(mainWorkflow, "npm audit --audit-level=high") {
		t.Fatal("main workflow must not duplicate the reusable viewer audit")
	}

	auditIndex := strings.Index(viewerWorkflow, "npm audit --audit-level=high")
	auditTail := viewerWorkflow[auditIndex:]
	if newline := strings.Index(auditTail, "\n\n"); newline >= 0 {
		auditTail = auditTail[:newline]
	}
	if strings.Contains(auditTail, "continue-on-error") {
		t.Fatal("reusable viewer workflow npm audit must not continue on error")
	}
}

func TestDependabotGroupsGoAndViewerUpdates(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	config := readRepoFile(t, repoRoot, filepath.Join(".github", "dependabot.yml"))
	for _, required := range []string{
		`package-ecosystem: "gomod"`,
		`directory: "/"`,
		"go-production-dependencies:",
		`package-ecosystem: "npm"`,
		`directory: "/web/viewer"`,
		"viewer-runtime:",
		"viewer-tooling:",
	} {
		if !strings.Contains(config, required) {
			t.Errorf("Dependabot config missing %q", required)
		}
	}
}

func TestGovulncheckPolicyFailsClosed(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	script := readRepoFile(t, repoRoot, filepath.Join("scripts", "run-govulncheck.sh"))
	for _, required := range []string{
		`GOVULNCHECK_VERSION="v1.6.0"`,
		`("id", "owner", "reason", "tracking_issue", "expires")`,
		`decoder.raw_decode(data, offset)`,
		`package.get("name")`,
		`cygpath -w "$path"`,
		`"$(python_path "$output_file")"`,
		`"$new_file_python"`,
		"expired on",
		`finding["policy"] = "disallowed-reachable"`,
	} {
		if !strings.Contains(script, required) {
			t.Errorf("govulncheck policy missing %q", required)
		}
	}
	for _, forbidden := range []string{"is_go121", "informational-stdlib-on-go1.21"} {
		if strings.Contains(script, forbidden) {
			t.Errorf("govulncheck policy retains obsolete exception %q", forbidden)
		}
	}
}

func TestDependencyExceptionPolicyIsTimeBounded(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	policy := readRepoFile(t, repoRoot, filepath.Join("docs", "dependency-policy.md"))
	for _, required := range []string{
		"Critical findings cannot receive a release exception",
		"named owner and tracking issue",
		"ISO expiry date no more than 30 days",
		"scanner rejects incomplete or expired entries",
		"Never use `continue-on-error`",
		"Browserslist 4.28.9",
		"PostCSS 8.5.28",
		"Sharp 0.35.3",
		"review it by 2026-09-18",
	} {
		if !strings.Contains(policy, required) {
			t.Errorf("dependency policy missing %q", required)
		}
	}
}
