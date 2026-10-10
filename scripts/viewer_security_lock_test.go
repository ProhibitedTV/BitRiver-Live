package scripts_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestViewerSecurityPatchesAreLocked(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	var lock struct {
		Packages map[string]struct {
			Version string `json:"version"`
		} `json:"packages"`
	}
	if err := json.Unmarshal([]byte(readRepoFile(t, repoRoot, filepath.Join("web", "viewer", "package-lock.json"))), &lock); err != nil {
		t.Fatalf("decode viewer security lock: %v", err)
	}
	// Check nested copies too: an override string alone cannot prove that every
	// consumer resolves the reviewed patch. npm ci/audit remain separate gates.
	for name, want := range map[string]string{"handlebars": "4.7.10", "sharp": "0.35.5", "source-map-js": "1.2.2"} {
		found := false
		for path, pkg := range lock.Packages {
			if path != "node_modules/"+name && !strings.HasSuffix(path, "/node_modules/"+name) {
				continue
			}
			found = true
			if pkg.Version != want {
				t.Errorf("%s version=%q, want reviewed security patch %q", path, pkg.Version, want)
			}
		}
		if !found {
			t.Errorf("viewer lock missing security dependency %s", name)
		}
	}
}
