package scripts_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanHostQualificationUsesOnlySignedReleaseArtifacts(t *testing.T) {
	workflow := readCleanHostQualificationWorkflow(t)
	for _, required := range []string{
		"workflow_dispatch:",
		"permissions:\n  contents: read",
		"runs-on: ubuntu-24.04",
		"test ! -e \"$GITHUB_WORKSPACE/.git\"",
		"qualification_root=\"$RUNNER_TEMP/bitriver-clean-host\"",
		"echo \"QUALIFICATION_ROOT=$qualification_root\"",
		"echo \"DOWNLOAD_ROOT=$download_root\"",
		"echo \"EVIDENCE_ROOT=$evidence_root\"",
		"rm -rf \"$qualification_root\"",
		"mkdir -p \"$download_root\" \"$evidence_root\"",
		"gh release download \"$CANDIDATE_TAG\"",
		"actual_release_set_sha256",
		"test \"$actual_release_set_sha256\" = \"$EXPECTED_RELEASE_SET_SHA256\"",
		"release-set candidate identity mismatch",
		"release-set workflow identity mismatch",
		"package does not match signed release-set checksum",
		"image signature bundle does not match signed image entry",
		"image signature bundle does not match signed artifact entry",
		"image signature bundle CHECKSUMS entry is missing or incorrect",
		"gh release download \"$COSIGN_VERSION\"",
		"--repo sigstore/cosign",
		"cosign_checksums.txt",
		"verify-blob",
		"--certificate-identity \"$RELEASE_WORKFLOW_IDENTITY\"",
		"--certificate-oidc-issuer https://token.actions.githubusercontent.com",
		"while IFS=$'\\t' read -r immutable bundle",
		"docker logout ghcr.io",
		"sudo dpkg --install \"$DOWNLOAD_ROOT/$PACKAGE_NAME\"",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("clean-host workflow missing signed artifact boundary %q", required)
		}
	}

	provenance := strings.Index(workflow, "- name: Download and verify signed public release set")
	install := strings.Index(workflow, "- name: Install the published Ubuntu package")
	if provenance == -1 || install <= provenance {
		t.Fatal("package installation must occur only after signed provenance verification")
	}
	imageVerify := strings.Index(workflow, `"$cosign_root/cosign-linux-amd64" verify \`)
	if imageVerify == -1 {
		t.Fatal("clean-host workflow is missing registry-backed image verification")
	}
	imageVerifyWindow := workflow[imageVerify:]
	if end := strings.Index(imageVerifyWindow, "done <"); end >= 0 {
		imageVerifyWindow = imageVerifyWindow[:end]
	}
	if strings.Contains(imageVerifyWindow, "--bundle") {
		t.Fatal("Cosign 3 container verify does not support --bundle; downloaded bundle bytes must be checked separately")
	}

	for _, forbidden := range []string{
		"actions/checkout@",
		"contents: write",
		"packages: write",
		"id-token: write",
		"docker login",
		"git clone",
		"go run ./",
		"./scripts/",
		"${{ runner.",
	} {
		if strings.Contains(workflow, forbidden) {
			t.Fatalf("clean-host workflow crosses release-only boundary with %q", forbidden)
		}
	}
}

func TestCleanHostQualificationCoversLifecycleAndRecovery(t *testing.T) {
	workflow := readCleanHostQualificationWorkflow(t)
	for _, required := range []string{
		"timeout-minutes: 75",
		"sudo bitriver-host install --operator-user \"$USER\"",
		"systemctl is-enabled bitriver-live-compose.service",
		"env validate --env-file \"$ENV_FILE\"",
		"sudo bitriver-host doctor",
		"docker compose \\\n            --profile '*' \\",
		"config --images",
		"sudo bitriver-host activate",
		"$INSTALL_ROOT/bin/bitriver\" smoke",
		"/v1/vhosts/default/apps/live",
		"Authorization\": \"Basic \" + auth",
		"aggregate OME health is not ok",
		"sudo bitriver-host upgrade --operator-user \"$USER\"",
		"before_upgrade_sha256",
		"restart ome",
		"sudo systemctl restart docker.service",
		"sudo systemctl restart bitriver-live-compose.service",
		"sudo bitriver-host uninstall",
		"sudo dpkg --remove bitriver-live",
		"qualification-retained",
		"before_uninstall_sha256",
		"test ! -e \"$INSTALL_ROOT\"",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("clean-host workflow missing lifecycle invariant %q", required)
		}
	}

	for _, recovery := range []string{"OME restart", "Docker daemon restart", "full systemd restart"} {
		start := strings.Index(workflow, "- name: Prove "+recovery)
		if start == -1 {
			t.Fatalf("clean-host workflow missing bounded %s stage", recovery)
		}
		window := workflow[start:]
		if next := strings.Index(window[1:], "\n      - name:"); next >= 0 {
			window = window[:next+1]
		}
		if !strings.Contains(window, "timeout-minutes:") {
			t.Fatalf("%s stage has no explicit timeout", recovery)
		}
	}
}

func TestCleanHostQualificationEvidenceIsSanitizedAndBounded(t *testing.T) {
	workflow := readCleanHostQualificationWorkflow(t)
	for _, required := range []string{
		"if: always()",
		"bitriver.clean-host-qualification/v1",
		"releaseSetSha256",
		"sourceCheckout\": \"absent\"",
		"registryAuthentication\": \"anonymous\"",
		"retainedSecrets\": \"none\"",
		"real XCP-ng/XOA VM reboot",
		"Nginx Proxy Manager TLS/WebSocket browser path",
		"target-host real RTMP ingest and decoded playback after reboot",
		"sanitized evidence contained an ephemeral secret",
		"BITRIVER_OME_API_TOKEN=",
		"actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a # v7.0.1",
		"retention-days: 30",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("clean-host workflow missing evidence invariant %q", required)
		}
	}

	if count := strings.Count(workflow, "uses: actions/upload-artifact@"); count != 1 {
		t.Fatalf("clean-host workflow upload count=%d, want 1", count)
	}
	for _, forbidden := range []string{
		"journalctl",
		" compose logs",
		"cat \"$ENV_FILE\"",
		"Server.generated.xml",
		"srs.generated.conf",
	} {
		if strings.Contains(workflow, forbidden) {
			t.Fatalf("clean-host workflow retains unsafe diagnostic seam %q", forbidden)
		}
	}
}

func readCleanHostQualificationWorkflow(t *testing.T) string {
	t.Helper()
	path := filepath.Join(filepath.Dir(mustGetwd(t)), ".github", "workflows", "clean-host-qualification.yml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read clean-host qualification workflow: %v", err)
	}
	return strings.ReplaceAll(string(content), "\r\n", "\n")
}
