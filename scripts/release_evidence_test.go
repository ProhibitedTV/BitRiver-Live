package scripts_test

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseEvidenceScannerAllowsRedactedEvidence(t *testing.T) {
	root := releaseEvidenceTempDir(t)
	writeReleaseEvidenceFixture(t, filepath.Join(root, "contract-status.json"), `{"status":"passed","token":"[redacted]"}`)
	writeReleaseEvidenceFixture(t, filepath.Join(root, ".env.example"), "BITRIVER_SRS_TOKEN=srs-token-example\n")
	writeReleaseEvidenceFixture(t, filepath.Join(root, "Server.generated.xml"), "<AccessToken>OME-Example-Access-Token</AccessToken>\n")

	output, err := runReleaseEvidenceScanner(t, root, "", "artifact-inventory.tsv")
	if err != nil {
		t.Fatalf("expected redacted evidence to pass: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Release evidence secret scan passed.") {
		t.Fatalf("expected pass summary, got %q", output)
	}
	if _, err := os.Stat(filepath.Join(root, "artifact-inventory.tsv")); err != nil {
		t.Fatalf("expected inventory: %v", err)
	}
}

func TestReleaseEvidenceScannerRejectsSecretMaterialWithoutEchoingValues(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		content  string
		sentinel string
		rule     string
	}{
		{name: "environment file", path: ".env", content: "BITRIVER_SRS_TOKEN=not-printed-value\n", rule: "forbidden-file"},
		{name: "named environment file", path: "production.env", content: "BITRIVER_SRS_TOKEN=not-printed-value\n", rule: "forbidden-file"},
		{name: "private key", path: "operator.pem", content: "-----BEGIN PRIVATE KEY-----\nnot-printed-key\n", rule: "private-key"},
		{name: "credential url", path: "diagnostics.log", content: "postgres://release-user:not-printed-pass@db.internal/live\n", rule: "credential-url"},
		{name: "credential url bypass", path: "diagnostics.log", content: "postgres://release-user:not-printed-pass@db.internal/live example follows\n", rule: "credential-url"},
		{name: "secret assignment", path: "report.json", content: `{"admin_password":"not-printed-password"}`, rule: "secret-assignment"},
		{name: "secret assignment bypass", path: "report.json", content: `{"admin_password":"not-printed-password","note":"redacted output"}`, rule: "secret-assignment"},
		{name: "xml credential", path: "Server.generated.xml", content: `<AccessToken>not-printed-token</AccessToken>`, rule: "xml-credential"},
		{name: "known value", path: "runner.log", content: "prefix sentinel-not-printed-987 suffix\n", sentinel: "sentinel-not-printed-987", rule: "known-secret-value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := releaseEvidenceTempDir(t)
			writeReleaseEvidenceFixture(t, filepath.Join(root, tt.path), tt.content)
			output, err := runReleaseEvidenceScanner(t, root, tt.sentinel, "")
			if err == nil {
				t.Fatalf("expected scanner failure, got %q", output)
			}
			if !strings.Contains(output, "["+tt.rule+"]") {
				t.Fatalf("expected %s finding, got %q", tt.rule, output)
			}
			for _, secret := range []string{"not-printed-value", "not-printed-key", "not-printed-pass", "not-printed-password", tt.sentinel} {
				if secret != "" && strings.Contains(output, secret) {
					t.Fatalf("scanner output exposed matched value %q: %q", secret, output)
				}
			}
		})
	}
}

func TestReleaseEvidenceScannerInspectsArchives(t *testing.T) {
	root := releaseEvidenceTempDir(t)
	archivePath := filepath.Join(root, "release.tar.gz")
	writeReleaseEvidenceArchive(t, archivePath, "bundle/diagnostics.log", "sentinel-archive-not-printed-2468\n")

	output, err := runReleaseEvidenceScanner(t, root, "sentinel-archive-not-printed-2468", "")
	if err == nil {
		t.Fatalf("expected archive leak to fail, got %q", output)
	}
	if !strings.Contains(output, "release.tar.gz!bundle/diagnostics.log") {
		t.Fatalf("expected archive member path, got %q", output)
	}
	if strings.Contains(output, "sentinel-archive-not-printed-2468") {
		t.Fatalf("scanner output exposed archive sentinel: %q", output)
	}
}

func runReleaseEvidenceScanner(t *testing.T, root, sentinel, inventory string) (string, error) {
	t.Helper()
	repoRoot := filepath.Dir(mustGetwd(t))
	args := []string{shellPath(filepath.Join(repoRoot, "scripts", "scan-release-evidence.sh")), "--root", shellPath(root)}
	if sentinel != "" {
		sentinelPath := filepath.Join(releaseEvidenceTempDir(t), "sentinels.txt")
		writeReleaseEvidenceFixture(t, sentinelPath, sentinel+"\n")
		args = append(args, "--sentinel-file", shellPath(sentinelPath))
	}
	if inventory != "" {
		args = append(args, "--inventory", inventory)
	}
	cmd := exec.Command(testBash(t), args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func releaseEvidenceTempDir(t *testing.T) string {
	t.Helper()
	repoRoot := filepath.Dir(mustGetwd(t))
	tempRoot := filepath.Join(repoRoot, ".tmp", "release-evidence-tests")
	if err := os.MkdirAll(tempRoot, 0o755); err != nil {
		t.Fatalf("create test temp root: %v", err)
	}
	dir, err := os.MkdirTemp(tempRoot, "fixture-")
	if err != nil {
		t.Fatalf("create test temp directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	return wd
}

func writeReleaseEvidenceFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

func writeReleaseEvidenceArchive(t *testing.T, path, name, content string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	data := []byte(content)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(data))}); err != nil {
		t.Fatalf("write archive header: %v", err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatalf("write archive content: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar archive: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip archive: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close archive file: %v", err)
	}
}
