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
		"prepare_release_candidate.py dependencies",
		"--unpublished-first-party-digests",
		"--third-party-evidence",
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
		`pattern: "!*.dockerbuild"`,
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

	scanStart := strings.Index(workflow, "- name: Scan and inventory release payload")
	uploadStart := strings.Index(workflow, "- name: Upload release publication evidence")
	if scanStart == -1 || uploadStart <= scanStart {
		t.Fatal("release payload scan step boundaries not found")
	}
	if !strings.Contains(workflow[scanStart:uploadStart], "timeout-minutes: 10") {
		t.Fatal("release payload scan must fail closed within an explicit timeout")
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
		"name: Download verified release dependency evidence",
		"name: release-contract-evidence",
		`dependency_evidence="$RUNNER_TEMP/release-contract-evidence/release-dependencies.json"`,
		"--third-party-evidence",
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

func TestReleaseWorkflowResolvesThirdPartyDependenciesOnce(t *testing.T) {
	workflow := readReleaseWorkflow(t)
	if count := strings.Count(workflow, "prepare_release_candidate.py dependencies"); count != 1 {
		t.Fatalf("release must resolve third-party dependencies exactly once, got %d", count)
	}
	if strings.Contains(workflow, "--resolve-digests") {
		t.Fatal("release env jobs must consume immutable dependency evidence instead of resolving tags again")
	}
	for _, required := range []string{
		`dependency_evidence="$evidence_dir/release-dependencies.json"`,
		`--output "$dependency_evidence"`,
		"name: Download verified release dependency evidence",
		"path: ${{ runner.temp }}/release-contract-evidence",
		`--third-party-evidence "$dependency_evidence"`,
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("release workflow missing reusable dependency evidence invariant %q", required)
		}
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

func TestReleaseWorkflowStampsExactTagIntoEveryInstallerPayload(t *testing.T) {
	workflow := readReleaseWorkflow(t)
	if count := strings.Count(workflow, `--release-tag "$RELEASE_TAG"`); count != 3 {
		t.Fatalf("every release-asset staging path must receive the immutable tag; got %d tagged calls, want 3", count)
	}
	for _, required := range []string{
		"GOOS: ${{ matrix.goos }}\n          GOARCH: ${{ matrix.goarch }}\n          RELEASE_TAG: ${{ github.ref_name }}",
		"name: Stage canonical Windows release assets\n        shell: bash\n        env:\n          RELEASE_TAG: ${{ github.ref_name }}",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("release workflow missing tag-stamping environment %q", required)
		}
	}
}

func TestReleaseWorkflowUsesFileBackedReleaseNotes(t *testing.T) {
	workflow := readReleaseWorkflow(t)
	for _, required := range []string{
		"const fs = require('fs');",
		"path.join(process.env.RUNNER_TEMP, 'bitriver-release-notes.md')",
		"fs.writeFileSync(notesPath, result.data.body, { encoding: 'utf8', mode: 0o600 });",
		"core.setOutput('path', notesPath);",
		"body_path: ${{ steps.notes.outputs.path }}",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("release workflow missing file-backed notes invariant %q", required)
		}
	}
	for _, forbidden := range []string{
		"core.setOutput('body', result.data.body);",
		"body: ${{ steps.notes.outputs.body }}",
	} {
		if strings.Contains(workflow, forbidden) {
			t.Fatalf("release workflow passes oversized notes inline through %q", forbidden)
		}
	}
}

func TestWorkflowOwnedPostgresServicesApplyMigrations(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	postgresPath := filepath.Join(".github", "workflows", "postgres-tests.yml")
	workflow := readRepoFile(t, repoRoot, postgresPath)
	stepStart := strings.Index(workflow, "- name: Run postgres-tagged storage tests")
	if stepStart == -1 {
		t.Fatal("postgres storage-test step not found")
	}
	step := workflow[stepStart:]
	if nextStep := strings.Index(step[1:], "\n      - name:"); nextStep != -1 {
		step = step[:nextStep+1]
	}
	for _, required := range []string{
		"BITRIVER_TEST_POSTGRES_DSN:",
		`BITRIVER_TEST_POSTGRES_RUN_MIGRATIONS: "1"`,
		"run: ./scripts/test-postgres.sh",
	} {
		if !strings.Contains(step, required) {
			t.Fatalf("fresh workflow-owned Postgres service missing %q", required)
		}
	}

	release := readReleaseWorkflow(t)
	if !strings.Contains(release, "postgres-tests:\n    uses: ./.github/workflows/postgres-tests.yml") {
		t.Fatal("release must call the migrated reusable Postgres workflow")
	}
	for _, duplicate := range []string{
		"image: postgres:15-alpine",
		"POSTGRES_DB: bitriver_test",
		"postgres service container not found",
	} {
		if strings.Contains(release, duplicate) {
			t.Errorf("release retains duplicated Postgres implementation %q", duplicate)
		}
	}
}

func TestReleaseVerificationRestoresDependencyNetwork(t *testing.T) {
	workflow := readReleaseWorkflow(t)
	goJobStart := strings.Index(workflow, "\n  go-tests:\n")
	postgresJobStart := strings.Index(workflow, "\n  postgres-tests:\n")
	if goJobStart == -1 || postgresJobStart == -1 || postgresJobStart <= goJobStart {
		t.Fatal("release Go/Postgres job boundaries not found")
	}
	goJob := workflow[goJobStart:postgresJobStart]
	for _, required := range []string{
		"GOTOOLCHAIN: local",
		"GOPROXY: off",
		"GOSUMDB: off",
		"- name: Run verification gate",
		"GOPROXY: https://proxy.golang.org,direct",
		"GOSUMDB: sum.golang.org",
		"run: ./scripts/verify.sh",
	} {
		if !strings.Contains(goJob, required) {
			t.Errorf("release Go verification missing %q", required)
		}
	}

	repoRoot := filepath.Dir(mustGetwd(t))
	verify := readRepoFile(t, repoRoot, filepath.Join("scripts", "verify.sh"))
	if !strings.Contains(verify, "env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off") {
		t.Fatal("verify.sh must continue to force host Go tests offline")
	}
}

func TestReleaseImagePublisherUsesVerifiedModuleProxy(t *testing.T) {
	workflow := readReleaseWorkflow(t)
	publishStart := strings.Index(workflow, "\n  publish-images:\n")
	viewerStart := strings.Index(workflow, "\n  publish-viewer-architectures:\n")
	if publishStart == -1 || viewerStart == -1 || viewerStart <= publishStart {
		t.Fatal("release image publisher boundaries not found")
	}
	publisher := workflow[publishStart:viewerStart]
	buildStart := strings.Index(publisher, "- name: Build and push ${{ matrix.component }} image")
	sbomStart := strings.Index(publisher, "- name: Generate SBOM for ${{ matrix.component }} image")
	if buildStart == -1 || sbomStart == -1 || sbomStart <= buildStart {
		t.Fatal("release image build step boundaries not found")
	}
	buildStep := publisher[buildStart:sbomStart]
	for _, required := range []string{
		"build-args: |",
		"GOPROXY=https://proxy.golang.org,direct",
		"GOSUMDB=sum.golang.org",
	} {
		if !strings.Contains(buildStep, required) {
			t.Errorf("release image publisher missing %q", required)
		}
	}

	repoRoot := filepath.Dir(mustGetwd(t))
	verify := readRepoFile(t, repoRoot, filepath.Join("scripts", "verify.sh"))
	if !strings.Contains(verify, "env GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off") {
		t.Fatal("release image network settings must not weaken offline host Go verification")
	}
}

func TestWindowsMSIUsesCanonicalReleaseAssets(t *testing.T) {
	workflow := readReleaseWorkflow(t)
	for _, required := range []string{
		"name: Stage canonical Windows release assets",
		"stage-release-assets.sh",
		`--output "$launcher_root/share/bitriver-live"`,
		`--release-tag "$RELEASE_TAG"`,
		"name: Install pinned WiX Toolset",
		"https://github.com/wixtoolset/wix3/releases/download/wix3141rtm/wix314-binaries.zip",
		"WIX_ARCHIVE_SHA256: 6ac824e1642d6f7277d0ed7ea09411a508f6116ba6fae0aa5f2c7daa2ff43d31",
		"Get-FileHash -Algorithm SHA256",
		`$wixRoot = $env:BITRIVER_WIX_ROOT`,
		"heat.exe",
		"-cg ReleaseAssets",
		"-dr RELEASEASSETSDIR",
		`$productVersionArg = "-dProductVersion=$($env:MSI_VERSION)"`,
		`$sourceDirArg = "-dSourceDir=$launcherRoot"`,
		`$releaseAssetsDirArg = "-dReleaseAssetsDir=$releaseAssetsRoot"`,
		`& $candle -nologo $productVersionArg $sourceDirArg $releaseAssetsDirArg`,
		`$wixUIExtension = Join-Path $wixRoot "WixUIExtension.dll"`,
		"bitriver-release-assets.wixobj",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("Windows MSI workflow missing canonical-asset invariant %q", required)
		}
	}
	for _, forbidden := range []string{
		"Copy-Item deploy/docker-compose.yml",
		"Copy-Item deploy/.env.example",
		"-dProductVersion=$env:MSI_VERSION",
		"-dProductVersion=$env:RELEASE_TAG",
		`C:\\Program Files (x86)\\WiX Toolset`,
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
		`Target="[SystemFolder]WindowsPowerShell\v1.0\powershell.exe"`,
	} {
		if !strings.Contains(wix, required) {
			t.Fatalf("WiX source missing harvested release-asset invariant %q", required)
		}
	}
	if strings.Contains(wix, `Source="$(var.SourceDir)\share\docker-compose.yml"`) {
		t.Fatal("WiX source must not retain the old two-file share layout")
	}
	if count := strings.Count(wix, `Key="Software\BitRiverLive"`); count != 2 {
		t.Fatalf("WiX shortcut registry key count=%d, want 2 canonical paths", count)
	}
	for _, forbidden := range []string{
		`Key="Software\\BitRiverLive"`,
		`WindowsPowerShell\\v1.0\\powershell.exe`,
	} {
		if strings.Contains(wix, forbidden) {
			t.Fatalf("WiX source retains doubled-backslash Windows path %q", forbidden)
		}
	}
}

func TestViewerImagePublicationUsesNativeArchitectures(t *testing.T) {
	workflow := readReleaseWorkflow(t)
	genericStart := strings.Index(workflow, "\n  publish-images:\n")
	nativeStart := strings.Index(workflow, "\n  publish-viewer-architectures:\n")
	manifestStart := strings.Index(workflow, "\n  publish-viewer-image:\n")
	productGateStart := strings.Index(workflow, "\n  pull-only-product-gate:\n")
	if genericStart == -1 || nativeStart <= genericStart || manifestStart <= nativeStart || productGateStart <= manifestStart {
		t.Fatal("release workflow must order generic images, native viewer images, viewer manifest, and product gate")
	}

	genericImages := workflow[genericStart:nativeStart]
	if strings.Contains(genericImages, "component: viewer") {
		t.Fatal("viewer must not execute npm under the generic QEMU multi-architecture publisher")
	}
	if !strings.Contains(genericImages, "timeout-minutes: 45") {
		t.Fatal("generic image publication must have a bounded timeout")
	}

	nativeViewer := workflow[nativeStart:manifestStart]
	for _, required := range []string{
		"runs-on: ${{ matrix.runner }}",
		"timeout-minutes: 30",
		"platform: linux/amd64",
		"runner: ubuntu-latest",
		"platform: linux/arm64",
		"runner: ubuntu-24.04-arm",
		"platforms: ${{ matrix.platform }}",
		"${{ env.RELEASE_TAG }}-${{ matrix.arch }}",
		"Verify native runner architecture",
		`actual="$(uname -m)"`,
		"Verify pushed viewer architecture",
		`docker run --rm --entrypoint uname "$image" -m`,
		`docker run --rm --entrypoint node "$image" --version`,
	} {
		if !strings.Contains(nativeViewer, required) {
			t.Errorf("native viewer image workflow missing %q", required)
		}
	}
	if strings.Contains(nativeViewer, "setup-qemu") || strings.Contains(nativeViewer, "Set up QEMU") {
		t.Fatal("native viewer image publication must not install QEMU")
	}

	viewerManifest := workflow[manifestStart:productGateStart]
	for _, required := range []string{
		"timeout-minutes: 15",
		"- publish-viewer-architectures",
		"docker buildx imagetools create",
		`"$release_ref-amd64"`,
		`"$release_ref-arm64"`,
		"needs.verify-env.outputs.publish_latest",
		"container-sbom-viewer",
	} {
		if !strings.Contains(viewerManifest, required) {
			t.Errorf("viewer manifest workflow missing %q", required)
		}
	}
	if count := strings.Count(workflow, "- publish-viewer-image"); count != 2 {
		t.Fatalf("viewer manifest downstream dependency count=%d, want product gate and release", count)
	}
}

func TestReleaseArtifactFanoutUsesHostAndCurrentToolContracts(t *testing.T) {
	workflow := readReleaseWorkflow(t)

	if count := strings.Count(workflow, `host_goos="$(go env GOHOSTOS)"`); count != 3 {
		t.Fatalf("cross-build host GOOS discovery count=%d, want 3 release matrix steps", count)
	}
	if count := strings.Count(workflow, `host_goarch="$(go env GOHOSTARCH)"`); count != 3 {
		t.Fatalf("cross-build host GOARCH discovery count=%d, want 3 release matrix steps", count)
	}
	if count := strings.Count(workflow, `GOOS="$host_goos" GOARCH="$host_goarch"`); count != 5 {
		t.Fatalf("host-scoped verifier/tool build count=%d, want 5 call sites", count)
	}

	for _, required := range []string{
		`$modFileArg = "-modfile=$($env:PRODUCTION_MODFILE)"`,
		"go mod download $modFileArg all",
		"go build $modFileArg -trimpath",
		`cosign sign-blob --yes \`,
		`--bundle "$launcher_root/bin/bitriver${bin_ext}.sigstore.json"`,
		`GOBIN="$host_tools" \`,
		"go install github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.47.0",
		`nfpm="$host_tools/nfpm"`,
		`GOMAXPROCS=2 "$nfpm" pkg --packager deb`,
		`GOMAXPROCS=2 "$nfpm" pkg --packager rpm`,
		"if [ -d public ]; then",
		`cp -R public "$bundle_root/public"`,
		`artifact_root="$GITHUB_WORKSPACE/dist"`,
		`tar -C "$artifact_root" -czf "$artifact_root/bitriver-viewer-${RELEASE_TAG}.tar.gz" bitriver-viewer`,
		"path: ${{ github.workspace }}/dist/bitriver-viewer-${{ github.ref_name }}.tar.gz",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("release workflow missing repaired fan-out contract %q", required)
		}
	}
	for _, forbidden := range []string{
		"mod download -modfile=$env:PRODUCTION_MODFILE",
		"go build -modfile=$env:PRODUCTION_MODFILE",
		"--output-signature",
		`.sig" "$launcher_root/bin/bitriver`,
		`export PATH="$HOME/go/bin:$PATH"`,
		`path: dist/bitriver-viewer-${{ github.ref_name }}.tar.gz`,
	} {
		if strings.Contains(workflow, forbidden) {
			t.Fatalf("release workflow retains failed rc.3 contract %q", forbidden)
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
	return strings.ReplaceAll(string(content), "\r\n", "\n")
}
