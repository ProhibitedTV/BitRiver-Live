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
		"secrets.BITRIVER_",
		"secret_vars=(",
		"IMAGE_NAMESPACE: ghcr.io/bitriver-live",
	} {
		if strings.Contains(workflow, forbidden) {
			t.Fatalf("release workflow must not contain %q", forbidden)
		}
	}

	for _, required := range []string{
		`env_file="$RUNNER_TEMP/release-validation-input"`,
		`sentinel_file="$RUNNER_TEMP/release-secret-values"`,
		"prepare_release_candidate.py metadata",
		"prepare_release_candidate.py env",
		"--unpublished-first-party-digests",
		"--resolve-digests",
		`image_namespace="ghcr.io/${GITHUB_REPOSITORY_OWNER,,}"`,
		"name: Remove job-local production inputs",
		"if: always()",
		"name: release-contract-evidence",
		"credentialFlow\": \"job-local-ephemeral",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("release workflow missing job-local credential invariant %q", required)
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

func TestReleaseWorkflowBlocksPublicationOnPulledProductEvidence(t *testing.T) {
	workflow := readReleaseWorkflow(t)
	for _, required := range []string{
		"pull-only-product-gate:",
		"name: Pull-only tagged product gate",
		"- publish-images",
		"timeout-minutes: 30",
		"docker logout ghcr.io",
		"prepare_release_candidate.py images",
		"--first-party-evidence",
		"--product-loopback",
		"BITRIVER_SMOKE_IMAGE_SOURCE: pull",
		"BITRIVER_SMOKE_LIVE_MODE: production",
		"test-production-golden-path.sh",
		"--stack quickstart",
		"--client docker",
		"production-golden-path.json",
		"name: release-product-evidence",
		"retention-days: 14",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("release workflow missing pulled-product invariant %q", required)
		}
	}

	releaseStart := strings.Index(workflow, "\n  release:\n")
	if releaseStart == -1 {
		t.Fatal("release job not found")
	}
	releaseJob := workflow[releaseStart:]
	if !strings.Contains(releaseJob, "- pull-only-product-gate") {
		t.Fatal("GitHub Release creation must depend on the pulled-image product gate")
	}
}

func TestReleaseWorkflowHandlesPrereleasesWithoutMovingLatest(t *testing.T) {
	workflow := readReleaseWorkflow(t)
	for _, required := range []string{
		"publish_latest: ${{ steps.release-metadata.outputs.publish_latest }}",
		"needs.verify-env.outputs.publish_latest == 'true'",
		"org.opencontainers.image.source=https://github.com/${{ github.repository }}",
		"prerelease: ${{ needs.verify-env.outputs.is_prerelease == 'true' }}",
		"MSI_VERSION: ${{ needs.verify-env.outputs.msi_version }}",
		"NFPM_VERSION: ${{ needs.verify-env.outputs.nfpm_version }}",
		"NFPM_PRERELEASE: ${{ needs.verify-env.outputs.nfpm_prerelease }}",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("release workflow missing prerelease invariant %q", required)
		}
	}
	if strings.Contains(workflow, "${{ env.IMAGE_NAMESPACE }}/${{ matrix.image_name }}:latest") {
		t.Fatal("release workflow must not publish latest unconditionally")
	}
}

func TestWindowsMSIUsesCanonicalReleaseAssets(t *testing.T) {
	workflow := readReleaseWorkflow(t)
	for _, required := range []string{
		"name: Stage canonical Windows release assets",
		"stage-release-assets.sh",
		`--output "$launcher_root/share/bitriver-live"`,
		"heat.exe",
		"-cg ReleaseAssets",
		"-dr RELEASEASSETSDIR",
		"-dProductVersion=$env:MSI_VERSION",
		"bitriver-release-assets.wixobj",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("Windows MSI workflow missing canonical-asset invariant %q", required)
		}
	}
	for _, forbidden := range []string{
		"Copy-Item deploy/docker-compose.yml",
		"Copy-Item deploy/.env.example",
		"-dProductVersion=$env:RELEASE_TAG",
	} {
		if strings.Contains(workflow, forbidden) {
			t.Fatalf("Windows MSI workflow retains stale path/version seam %q", forbidden)
		}
	}

	repoRoot := filepath.Dir(mustGetwd(t))
	wixPath := filepath.Join(repoRoot, "deploy", "installers", "bitriver-live.wxs")
	wixBytes, err := os.ReadFile(wixPath)
	if err != nil {
		t.Fatalf("read WiX source: %v", err)
	}
	wix := strings.ReplaceAll(string(wixBytes), "\r\n", "\n")
	for _, required := range []string{
		`<Directory Id="RELEASEASSETSDIR" Name="bitriver-live" />`,
		`<ComponentGroupRef Id="ReleaseAssets" />`,
	} {
		if !strings.Contains(wix, required) {
			t.Fatalf("WiX source missing harvested release-asset invariant %q", required)
		}
	}
	if strings.Contains(wix, `Source="$(var.SourceDir)\share\docker-compose.yml"`) {
		t.Fatal("WiX source must not retain the old two-file share layout")
	}
}

func readReleaseWorkflow(t *testing.T) string {
	t.Helper()
	path := filepath.Join(filepath.Dir(mustGetwd(t)), ".github", "workflows", "release.yml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}
	return strings.ReplaceAll(string(content), "\r\n", "\n")
}
