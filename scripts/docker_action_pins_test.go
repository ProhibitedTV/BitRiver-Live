package scripts_test

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Shared actions must stay immutable and consistent across scanning, candidate
// builds, and promotion. This is a source contract, not release execution proof.
func TestDockerActionPinsAreImmutableAndConsistent(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	workflowDir := filepath.Join(repoRoot, ".github", "workflows")
	paths, err := filepath.Glob(filepath.Join(workflowDir, "*.yml"))
	if err != nil {
		t.Fatal(err)
	}
	uses := regexp.MustCompile(`(?m)^\s*(?:-\s*)?uses:\s*(docker/(?:setup-buildx|setup-qemu|build-push)-action)@([^\s#]+)`)
	immutable := regexp.MustCompile(`^[0-9a-f]{40}$`)
	pins := make(map[string]string)
	seen := make(map[string]map[string]bool)
	for _, path := range paths {
		workflow := filepath.Base(path)
		content := readRepoFile(t, repoRoot, filepath.Join(".github", "workflows", workflow))
		for _, match := range uses.FindAllStringSubmatch(content, -1) {
			action, pin := match[1], match[2]
			if !immutable.MatchString(pin) {
				t.Errorf("%s uses mutable %s pin %q", workflow, action, pin)
			}
			if prior, ok := pins[action]; ok && prior != pin {
				t.Errorf("%s has inconsistent %s pin: %s versus %s", workflow, action, prior, pin)
			}
			pins[action] = pin
			if seen[action] == nil {
				seen[action] = make(map[string]bool)
			}
			seen[action][workflow] = true
		}
	}
	for action, workflows := range map[string]string{
		"docker/setup-buildx-action": "image-scan.yml release.yml stable-promotion.yml",
		"docker/setup-qemu-action":   "release.yml",
		"docker/build-push-action":   "release.yml",
	} {
		for _, workflow := range strings.Fields(workflows) {
			if !seen[action][workflow] {
				t.Errorf("missing immutable %s reference in %s", action, workflow)
			}
		}
	}
}
