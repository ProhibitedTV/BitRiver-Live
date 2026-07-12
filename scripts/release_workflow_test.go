package scripts_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseWorkflowKeepsCredentialsJobLocal(t *testing.T) {
	workflow := readReleaseWorkflow(t)
	for _, forbidden := range []string{
		"name: release-env",
		"path: .env",
		"Download verified environment file",
		"Upload verified environment file",
		"--env-file .env --force",
	} {
		if strings.Contains(workflow, forbidden) {
			t.Fatalf("release workflow must not contain %q", forbidden)
		}
	}

	for _, required := range []string{
		`env_file="$RUNNER_TEMP/release-validation-input"`,
		`sentinel_file="$RUNNER_TEMP/release-secret-values"`,
		"name: Remove job-local production inputs",
		"if: always()",
		"name: release-contract-evidence",
		"credentialFlow\": \"job-local-ephemeral",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("release workflow missing job-local credential invariant %q", required)
		}
	}

	secretStart := strings.Index(workflow, "secret_vars=(")
	if secretStart == -1 {
		t.Fatal("release workflow missing secret_vars array")
	}
	secretRemainder := workflow[secretStart+len("secret_vars=("):]
	secretEnd := strings.Index(secretRemainder, "\n          )")
	if secretEnd == -1 {
		t.Fatal("release workflow secret_vars array is not terminated")
	}
	secretSection := secretRemainder[:secretEnd]
	for _, required := range []string{
		"BITRIVER_POSTGRES_PASSWORD",
		"BITRIVER_LIVE_ADMIN_PASSWORD",
		"BITRIVER_LIVE_METRICS_TOKEN",
		"BITRIVER_OME_API_TOKEN",
		"BITRIVER_TRANSCODER_TOKEN",
	} {
		if !strings.Contains(secretSection, required) {
			t.Fatalf("release sentinel list missing credential variable %q", required)
		}
	}
	for _, forbidden := range []string{"BITRIVER_LIVE_MODE", "BITRIVER_OME_SERVER_PORT", "BITRIVER_LIVE_IMAGE_TAG"} {
		if strings.Contains(secretSection, forbidden) {
			t.Fatalf("release sentinel list must not include ordinary value %q", forbidden)
		}
	}
}

func TestReleaseWorkflowScansAndRetainsEveryArtifactSafely(t *testing.T) {
	workflow := readReleaseWorkflow(t)
	uploads := strings.Count(workflow, "uses: actions/upload-artifact@")
	retentions := strings.Count(workflow, "retention-days:")
	if uploads == 0 || retentions != uploads {
		t.Fatalf("every upload-artifact step needs explicit retention: uploads=%d retention declarations=%d", uploads, retentions)
	}
	if strings.Count(workflow, "./scripts/scan-release-evidence.sh") < 4 {
		t.Fatalf("expected validation output, redacted evidence, downloaded artifacts, and publication payload scans")
	}
	for _, required := range []string{
		"--env-file deploy/.env.example",
		`--output "$rendered"`,
		"--inventory \"$evidence_dir/artifact-inventory.tsv\"",
		"name: release-publication-evidence",
		"downloadedArtifactScan\": \"passed",
		"publicationPayloadScan\": \"passed",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("release workflow missing artifact safety invariant %q", required)
		}
	}
}

func readReleaseWorkflow(t *testing.T) string {
	t.Helper()
	path := filepath.Join(filepath.Dir(mustGetwd(t)), ".github", "workflows", "release.yml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}
	return string(content)
}
