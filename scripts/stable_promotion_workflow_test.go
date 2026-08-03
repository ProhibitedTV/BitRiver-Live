package scripts_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStablePromotionWorkflowValidatesBeforeEnvironmentWrites(t *testing.T) {
	workflow := readStablePromotionWorkflow(t)
	for _, required := range []string{
		"name: Stable promotion gate",
		"contents: read\n      packages: read",
		"name: Download candidate release without mutation",
		"name: Verify public asset digests and revocation state",
		"name: Verify candidate manifest and signatures",
		"./scripts/release_set.py verify-candidate",
		"--promotion-record \"$RUNNER_TEMP/promotion-input/promotion-record.json\"",
		"cosign verify-blob",
		"jq -r '.images[].immutableReference'",
		"name: Verify tracked promotion gates are closed",
		"for issue in 1297 1299 1298 1303 1304 1305 1306 1307",
		"name: Verify existing stable state before approval",
		"./scripts/release_set.py classify-state",
		"name: Upload validated promotion input",
		"environment: stable-promotion",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("stable promotion workflow missing validation boundary %q", required)
		}
	}

	gateStart := strings.Index(workflow, "\n  stable-promotion-gate:\n")
	promoteStart := strings.Index(workflow, "\n  promote-stable:\n")
	if gateStart == -1 || promoteStart <= gateStart {
		t.Fatal("stable promotion gate and write-job boundaries not found")
	}
	gate := workflow[gateStart:promoteStart]
	if strings.Contains(gate, "environment: stable-promotion") {
		t.Fatal("read-only gate must fail before environment approval")
	}
	for _, forbidden := range []string{
		"contents: write",
		"packages: write",
		"id-token: write",
		"gh release create",
		"gh release upload",
		"docker buildx imagetools create",
	} {
		if strings.Contains(gate, forbidden) {
			t.Fatalf("read-only gate contains mutation seam %q", forbidden)
		}
	}
}

func TestStablePromotionWorkflowPromotesExactBytesWithoutBuilding(t *testing.T) {
	workflow := readStablePromotionWorkflow(t)
	for _, required := range []string{
		"contents: write\n      packages: write\n      id-token: write",
		"name: Revalidate immutable inputs and live stable state",
		"stable image alias $alias points at $current instead of $digest",
		"name: Prepare signed stable and rollback metadata",
		"./scripts/release_set.py stable",
		"--previous-stable-manifest",
		"--output \"$payload/stable-release-set.json\"",
		"--rollback-output \"$payload/rollback-release-set.json\"",
		"bundle=\"$payload/stable-release-set.sigstore.json\"",
		"--bundle \"$bundle\"",
		"--output \"$payload/PROMOTION-CHECKSUMS.txt\"",
		"name: Create exact stable tag and draft release",
		"gh release create \"$STABLE_TAG\"",
		"--draft",
		"name: Retag exact image digests without rebuilding",
		"docker buildx imagetools create --tag \"$stable_ref\" \"$immutable\"",
		"name: Upload missing stable assets and publish draft",
		"stable asset $name already exists with different bytes",
		"--draft=false",
		"name: Verify published stable state",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("stable promotion workflow missing immutable promotion invariant %q", required)
		}
	}
	for _, forbidden := range []string{
		"docker/build-push-action@",
		"go build ",
		"npm run build",
		"--clobber",
	} {
		if strings.Contains(workflow, forbidden) {
			t.Fatalf("stable promotion workflow contains rebuild/overwrite seam %q", forbidden)
		}
	}
}

func TestStablePromotionWorkflowAppendsSignedRevocations(t *testing.T) {
	workflow := readStablePromotionWorkflow(t)
	for _, required := range []string{
		"name: Append candidate revocation",
		"if: inputs.operation == 'revoke'",
		"environment: stable-promotion",
		"./scripts/release_set.py revoke",
		"candidate-revocation-$GITHUB_RUN_ID.json",
		"candidate-revocation-$GITHUB_RUN_ID.sigstore.json",
		"refusing to overwrite existing revocation asset",
		"gh release upload \"$CANDIDATE_TAG\" \"$file\"",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("stable promotion workflow missing revocation invariant %q", required)
		}
	}
	revokeStart := strings.Index(workflow, "\n  revoke-candidate:\n")
	if revokeStart == -1 {
		t.Fatal("revocation write job not found")
	}
	revoke := workflow[revokeStart:]
	if strings.Contains(revoke, "packages: write") || strings.Contains(revoke, "docker buildx") {
		t.Fatal("revocation job must not mutate image or package state")
	}
}

func readStablePromotionWorkflow(t *testing.T) string {
	t.Helper()
	path := filepath.Join(filepath.Dir(mustGetwd(t)), ".github", "workflows", "stable-promotion.yml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read stable promotion workflow: %v", err)
	}
	return strings.ReplaceAll(string(content), "\r\n", "\n")
}
