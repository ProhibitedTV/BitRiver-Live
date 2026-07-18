package scripts_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	goToolchainVersion = "1.26.5"
	goMinimumVersion   = "1.26.0"
	nodeMajorVersion   = "24"
)

func TestGoRuntimeBaselineIsAligned(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	if got := strings.TrimSpace(readRepoFile(t, repoRoot, ".go-version")); got != goToolchainVersion {
		t.Fatalf(".go-version=%q, want %q", got, goToolchainVersion)
	}

	goMod := readRepoFile(t, repoRoot, "go.mod")
	if !hasGoDirective(goMod, goMinimumVersion) {
		t.Fatalf("go.mod must declare go %s", goMinimumVersion)
	}

	dockerfiles := []string{
		"Dockerfile",
		filepath.Join("cmd", "transcoder", "Dockerfile"),
		filepath.Join("cmd", "srs-controller", "Dockerfile"),
		filepath.Join("deploy", "ome-config", "Dockerfile"),
	}
	for _, path := range dockerfiles {
		contents := readRepoFile(t, repoRoot, path)
		if !strings.Contains(contents, "golang:"+goToolchainVersion) {
			t.Errorf("%s must use golang:%s", path, goToolchainVersion)
		}
		for _, required := range []string{"cmd/tools/production-module", "go.production.mod", "cmd/tools/verify-production-binary"} {
			if !strings.Contains(contents, required) {
				t.Errorf("%s missing production dependency invariant %q", path, required)
			}
		}
	}

	setupAction := readRepoFile(t, repoRoot, filepath.Join(".github", "actions", "setup-go", "action.yml"))
	if !strings.Contains(setupAction, "default: .go-version") {
		t.Fatal("setup-go action must default to the exact .go-version toolchain")
	}
	releaseWorkflow := readRepoFile(t, repoRoot, filepath.Join(".github", "workflows", "release.yml"))
	if strings.Contains(releaseWorkflow, "go-version-file: 'go.mod'") {
		t.Fatal("release workflow must not infer an unpatched Go toolchain from go.mod")
	}
	if count := strings.Count(releaseWorkflow, "go-version-file: '.go-version'"); count != 7 {
		t.Fatalf("release workflow exact Go setup count=%d, want 7", count)
	}
}

func TestProductionBuildsUseCompleteUpstreamModuleGraph(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	releaseWorkflow := readRepoFile(t, repoRoot, filepath.Join(".github", "workflows", "release.yml"))
	for _, required := range []string{
		"Prepare upstream production module graph",
		"go run ./cmd/tools/production-module",
		"go mod download -modfile=\"$PRODUCTION_MODFILE\" all",
		"go run ./cmd/tools/verify-production-binary",
		"--require-module github.com/jackc/pgx/v5",
	} {
		if !strings.Contains(releaseWorkflow, required) {
			t.Fatalf("release workflow missing production dependency invariant %q", required)
		}
	}

	installer := readRepoFile(t, repoRoot, filepath.Join("deploy", "install", "ubuntu.sh"))
	for _, required := range []string{
		"./scripts/check-go-toolchain.sh",
		"go run ./cmd/tools/production-module",
		"go mod download -modfile=\"$production_mod\" all",
		"verify-production-binary --require-module github.com/jackc/pgx/v5",
	} {
		if !strings.Contains(installer, required) {
			t.Fatalf("source installer missing production dependency invariant %q", required)
		}
	}

	refreshScript := readRepoFile(t, repoRoot, filepath.Join("scripts", "refresh-go-sum.sh"))
	if strings.Contains(refreshScript, "module bitriver-live-go-sum-sync") || strings.Contains(refreshScript, "github.com/jackc/pgx/v5 v") {
		t.Fatal("checksum refresh must derive dependencies from the production module helper, not duplicate versions")
	}
	if !strings.Contains(refreshScript, "go run ./cmd/tools/production-module") {
		t.Fatal("checksum refresh must use the production module helper")
	}
}

func TestArchitectureCheckDisablesVCSStamping(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	architectureCheck := readRepoFile(t, repoRoot, filepath.Join("scripts", "check-architecture-deps.sh"))
	if !strings.Contains(architectureCheck, "-buildvcs=false") {
		t.Fatal("architecture dependency check must disable VCS stamping for bounded mounted-workspace verification")
	}
}

func TestModelsImportCheckStaysInsideFirstPartyGoRoots(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	modelsCheck := readRepoFile(t, repoRoot, filepath.Join("scripts", "check-no-models-imports.sh"))
	if !strings.Contains(modelsCheck, "GO_ROOTS=(cmd internal scripts web)") {
		t.Fatal("models import check must scope recursive traversal to first-party Go roots")
	}
	if strings.Contains(modelsCheck, "find . -type f") {
		t.Fatal("models import check must not recursively traverse the entire workspace")
	}
}

func TestVerifyUsesBoundedFirstPartyGoRoots(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	verifyScript := readRepoFile(t, repoRoot, filepath.Join("scripts", "verify.sh"))
	for _, required := range []string{"./cmd/...", "./internal/...", "./scripts/...", "./web", "-buildvcs=false"} {
		if !strings.Contains(verifyScript, required) {
			t.Fatalf("verify script missing bounded Go test invariant %q", required)
		}
	}
	if strings.Contains(verifyScript, `go_test_packages="./..."`) {
		t.Fatal("verify script must not recursively discover packages across non-Go workspace trees")
	}
}

func TestViewerRuntimeBaselineIsAligned(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	viewerRoot := filepath.Join(repoRoot, "web", "viewer")

	if got := strings.TrimSpace(readRepoFile(t, viewerRoot, ".nvmrc")); got != nodeMajorVersion {
		t.Fatalf("viewer .nvmrc=%q, want %q", got, nodeMajorVersion)
	}

	packageJSON := readRepoFile(t, viewerRoot, "package.json")
	for _, required := range []string{
		`"node": ">=24 <25"`,
		`"npm": ">=11 <12"`,
		`"next": "16.2.10"`,
		`"react": "19.2.7"`,
		`"react-dom": "19.2.7"`,
	} {
		if !strings.Contains(packageJSON, required) {
			t.Errorf("viewer package.json missing runtime invariant %q", required)
		}
	}

	viewerDockerfile := readRepoFile(t, viewerRoot, "Dockerfile")
	if !strings.Contains(viewerDockerfile, "node:"+nodeMajorVersion+"-alpine") {
		t.Fatalf("viewer Dockerfile must use node:%s-alpine", nodeMajorVersion)
	}

	setupAction := readRepoFile(t, repoRoot, filepath.Join(".github", "actions", "setup-node-viewer", "action.yml"))
	if !strings.Contains(setupAction, "default: '"+nodeMajorVersion+"'") {
		t.Fatalf("viewer setup action must default to Node %s", nodeMajorVersion)
	}

	releaseWorkflow := readRepoFile(t, repoRoot, filepath.Join(".github", "workflows", "release.yml"))
	if !strings.Contains(releaseWorkflow, "node-version: "+nodeMajorVersion) &&
		!strings.Contains(releaseWorkflow, "node-version: '"+nodeMajorVersion+"'") {
		t.Fatalf("release workflow must use Node %s", nodeMajorVersion)
	}

	dockerignore := readRepoFile(t, viewerRoot, ".dockerignore")
	for _, excluded := range []string{"node_modules", ".next", "playwright-report", "test-results"} {
		if !strings.Contains(dockerignore, excluded) {
			t.Errorf("viewer .dockerignore must exclude %q", excluded)
		}
	}
}

func TestOfflineMirrorsDeclareCurrentMinimumGoVersion(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	thirdPartyRoot := filepath.Join(repoRoot, "third_party")
	err := filepath.WalkDir(thirdPartyRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Name() != "go.mod" {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !hasGoDirective(string(contents), goMinimumVersion) {
			t.Errorf("%s must declare go %s", path, goMinimumVersion)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk third_party modules: %v", err)
	}
}

func hasGoDirective(contents, version string) bool {
	normalized := strings.ReplaceAll(contents, "\r\n", "\n")
	return strings.Contains(normalized, "\ngo "+version+"\n")
}

func TestHasGoDirectiveAcceptsCommonLineEndings(t *testing.T) {
	for name, contents := range map[string]string{
		"LF":   "module example\n\ngo 1.26.0\n",
		"CRLF": "module example\r\n\r\ngo 1.26.0\r\n",
	} {
		t.Run(name, func(t *testing.T) {
			if !hasGoDirective(contents, goMinimumVersion) {
				t.Fatalf("expected %s Go directive to match", name)
			}
		})
	}
}

func readRepoFile(t *testing.T, repoRoot, relativePath string) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(repoRoot, relativePath))
	if err != nil {
		t.Fatalf("read %s: %v", relativePath, err)
	}
	return string(contents)
}
