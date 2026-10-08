package scripts_test

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// This checks documentation drift, not authenticity. Publication qualification
// must independently verify the downloaded root, signatures and public bytes.
func TestCurrentReleaseDocumentationConsistency(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	index := readRepoFile(t, repoRoot, filepath.Join("docs", "releases", "README.md"))
	tagMatch := regexp.MustCompile(`\]\((v[0-9]+\.[0-9]+\.[0-9]+-rc\.[0-9]+)\.md\)`).FindStringSubmatch(index)
	if len(tagMatch) != 2 {
		t.Fatal("release index must link a published current candidate snapshot")
	}
	tag := tagMatch[1]
	draftPath := "docs/releases/" + strings.SplitN(tag, "-", 2)[0] + "-draft.md"
	note := readRepoFile(t, repoRoot, filepath.Join("docs", "releases", tag+".md"))
	rootMatch := regexp.MustCompile("Signed release-set SHA-256: `([a-f0-9]{64})`").FindStringSubmatch(note)
	if len(rootMatch) != 2 {
		t.Fatal("current candidate snapshot must name one well-formed signed-root hash")
	}
	digestPattern := regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
	digests := map[string]string{}
	for _, name := range []string{"bitriver-live", "bitriver-viewer", "bitriver-srs-controller", "bitriver-transcoder", "bitriver-ome-config"} {
		for _, line := range strings.Split(note, "\n") {
			fields := strings.Split(line, "|")
			if len(fields) != 4 || strings.TrimSpace(fields[1]) != "`"+name+"`" {
				continue
			}
			if _, exists := digests[name]; exists {
				t.Fatalf("current snapshot repeats image %s", name)
			}
			digest := strings.Trim(strings.TrimSpace(fields[2]), "`")
			if !digestPattern.MatchString(digest) {
				t.Fatalf("current snapshot has malformed digest for %s", name)
			}
			digests[name] = digest
		}
		if _, exists := digests[name]; !exists {
			t.Fatalf("current snapshot must name exact digest for %s", name)
		}
	}
	for _, path := range []string{
		"README.md", "SUPPORT.md", "docs/quickstart.md",
		"docs/installing-on-ubuntu.md", "docs/viewer-deployment.md",
		"docs/production-status.md", "docs/production-release.md",
		draftPath,
	} {
		content := readRepoFile(t, repoRoot, filepath.FromSlash(path))
		if !strings.Contains(content, tag) {
			t.Errorf("%s must reference current published candidate %s", path, tag)
		}
		if path == "README.md" || path == "docs/installing-on-ubuntu.md" {
			if !strings.Contains(content, "release_tag="+tag) {
				t.Errorf("%s download command must select current candidate %s", path, tag)
			}
		}
		if path == "docs/installing-on-ubuntu.md" || path == "docs/production-status.md" || path == draftPath {
			if !strings.Contains(content, rootMatch[1]) {
				t.Errorf("%s must agree with current candidate signed-root hash", path)
			}
		}
		if path == "docs/viewer-deployment.md" {
			reference := "ghcr.io/prohibitedtv/bitriver-viewer:" + tag + "@" + digests["bitriver-viewer"]
			if !strings.Contains(content, reference) {
				t.Error("viewer deployment example must pair current candidate with its exact signed digest")
			}
		}
	}
}
