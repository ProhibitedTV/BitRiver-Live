package scripts_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestViewerCompatibilityMatrixContract(t *testing.T) {
	repoRoot := filepath.Dir(mustGetwd(t))
	config := readRepoFile(t, repoRoot, filepath.Join("web", "viewer", "playwright.config.ts"))

	for _, required := range []string{
		`retries: 0`,
		`name: "chromium-regression"`,
		`name: "chromium-compat"`,
		`name: "firefox-compat"`,
		`name: "webkit-compat"`,
		`name: "android-chrome-compat"`,
		`name: "iphone-webkit-compat"`,
		`devices["Desktop Chrome"]`,
		`devices["Desktop Firefox"]`,
		`devices["Desktop Safari"]`,
		`devices["Pixel 7"]`,
		`devices["iPhone 15"]`,
		`const compatibilitySpec = "**/compatibility.spec.ts"`,
		`testIgnore: compatibilitySpec`,
		`testMatch: compatibilitySpec`,
	} {
		if !strings.Contains(config, required) {
			t.Errorf("viewer Playwright config missing compatibility invariant %q", required)
		}
	}

	compatibilitySpec := readRepoFile(t, repoRoot, filepath.Join("web", "viewer", "tests", "compatibility.spec.ts"))
	for _, required := range []string{
		`handles HLS capability, reads chat, and sends a message`,
		`recovers when the playback API returns a transient failure`,
		`signed-out navigation, auth, and accessibility`,
		`loads the creator live setup`,
		`touch navigation and the auth overlay`,
		`chat authentication fallback`,
		`new AxeBuilder`,
		`.tap()`,
		`toBeLessThanOrEqual(1)`,
	} {
		if !strings.Contains(compatibilitySpec, required) {
			t.Errorf("viewer compatibility spec missing critical-flow invariant %q", required)
		}
	}

	viewerCI := readRepoFile(t, repoRoot, filepath.Join(".github", "workflows", "viewer-ci.yml"))
	if !strings.Contains(viewerCI, "npx playwright install --with-deps") {
		t.Fatal("viewer CI must install all Playwright browser engines required by the compatibility matrix")
	}
	if !strings.Contains(viewerCI, "npm run test:integration") {
		t.Fatal("viewer CI must execute the integration script that owns the Playwright compatibility gate")
	}

	doc := readRepoFile(t, repoRoot, filepath.Join("docs", "viewer-compatibility.md"))
	for _, required := range []string{
		"Playwright WebKit is a Safari-equivalent engine check",
		"not evidence of testing a particular shipping Safari build",
		"WebRTC is **not promoted to a browser-wide support claim",
		"does not promise audible autoplay",
		"retries to hide it",
		"the narrower proven boundary wins",
	} {
		if !strings.Contains(doc, required) {
			t.Errorf("viewer compatibility documentation missing support-boundary invariant %q", required)
		}
	}
}
