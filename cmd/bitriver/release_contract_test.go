package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunReleaseRequiresSubcommand(t *testing.T) {
	if err := runRelease(nil); err == nil {
		t.Fatal("expected missing release subcommand to fail")
	}
}

func TestRunReleaseRejectsUnknownSubcommand(t *testing.T) {
	if err := runRelease([]string{"does-not-exist"}); err == nil {
		t.Fatal("expected unknown release subcommand to fail")
	}
}

func TestBuildContractSnapshotCapturesOperatorContract(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env.example")
	composePath := filepath.Join(dir, "docker-compose.yml")
	migrationsDir := filepath.Join(dir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0o755); err != nil {
		t.Fatalf("create migrations dir: %v", err)
	}
	writeTestFile(t, envPath, `
# API port exposed to operators.
BITRIVER_LIVE_PORT=8080
# Token for protected metrics.
BITRIVER_LIVE_METRICS_TOKEN=metrics-token
BITRIVER_UNDOCUMENTED=value
`)
	writeTestFile(t, composePath, `
services:
  bitriver-live:
    image: ${BITRIVER_LIVE_IMAGE_TAG:-bitriver-live:dev}
    ports:
      - "${BITRIVER_LIVE_PORT:-8080}:8080"
    volumes:
      - ./data:/data
    depends_on:
      - postgres
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/healthz"]
  postgres:
    image: postgres:16
    profiles:
      - storage
`)
	writeTestFile(t, filepath.Join(migrationsDir, "0001_initial.sql"), "create table test (id text);\n")

	snapshot, err := buildContractSnapshot(envPath, composePath, migrationsDir)
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	if snapshot.SchemaVersion != contractSnapshotSchemaVersion {
		t.Fatalf("unexpected schema version %q", snapshot.SchemaVersion)
	}
	if len(snapshot.Migrations) != 1 || snapshot.Migrations[0].File != "0001_initial.sql" || snapshot.Migrations[0].SHA256 == "" {
		t.Fatalf("unexpected migrations: %#v", snapshot.Migrations)
	}

	envByKey := map[string]contractEnvVar{}
	for _, envVar := range snapshot.Env {
		envByKey[envVar.Key] = envVar
	}
	if !envByKey["BITRIVER_LIVE_METRICS_TOKEN"].SecuritySensitive {
		t.Fatal("expected metrics token to be security sensitive")
	}
	if !envByKey["BITRIVER_LIVE_PORT"].Documented {
		t.Fatal("expected port env var to keep adjacent documentation")
	}
	if envByKey["BITRIVER_UNDOCUMENTED"].Documented {
		t.Fatal("expected undocumented env var to be flagged")
	}

	services := map[string]contractComposeService{}
	for _, service := range snapshot.ComposeServices {
		services[service.Name] = service
	}
	api := services["bitriver-live"]
	if api.Image != "${BITRIVER_LIVE_IMAGE_TAG:-bitriver-live:dev}" {
		t.Fatalf("unexpected api image %q", api.Image)
	}
	if !api.Healthcheck {
		t.Fatal("expected healthcheck to be detected")
	}
	if !containsString(api.EnvRefs, "BITRIVER_LIVE_PORT") {
		t.Fatalf("expected env refs to include BITRIVER_LIVE_PORT, got %#v", api.EnvRefs)
	}
	if !containsString(api.DependsOn, "postgres") {
		t.Fatalf("expected depends_on to include postgres, got %#v", api.DependsOn)
	}
}

func TestDiffContractSnapshotsClassifiesDrift(t *testing.T) {
	base := contractSnapshot{
		SchemaVersion: contractSnapshotSchemaVersion,
		Env: []contractEnvVar{
			{Key: "BITRIVER_LIVE_PORT", Default: "8080", Documented: true},
			{Key: "BITRIVER_LIVE_METRICS_TOKEN", Default: "old", SecuritySensitive: true, Documented: true},
		},
		ComposeServices: []contractComposeService{{Name: "bitriver-live", Image: "api:old", Healthcheck: true}},
		Migrations:      []contractMigration{{File: "0001_initial.sql", SHA256: "aaa"}},
		HealthEndpoints: []contractHealthEndpoint{{Name: "API health", Method: "GET", Path: "/healthz"}},
	}
	head := contractSnapshot{
		SchemaVersion: contractSnapshotSchemaVersion,
		Env: []contractEnvVar{
			{Key: "BITRIVER_LIVE_METRICS_TOKEN", Default: "new", SecuritySensitive: true, Documented: true},
			{Key: "BITRIVER_NEW_FLAG", Default: "true", Documented: false},
		},
		ComposeServices: []contractComposeService{{Name: "bitriver-live", Image: "api:new", Healthcheck: false}},
		Migrations:      []contractMigration{{File: "0001_initial.sql", SHA256: "bbb"}, {File: "0002_next.sql", SHA256: "ccc"}},
		HealthEndpoints: []contractHealthEndpoint{{Name: "API health", Method: "GET", Path: "/healthz"}},
	}

	report := diffContractSnapshots(base, head)
	if report.Summary.Security != 1 {
		t.Fatalf("expected one security drift, got %#v", report.Summary)
	}
	if report.Summary.Undocumented != 1 {
		t.Fatalf("expected one undocumented drift, got %#v", report.Summary)
	}
	if report.Summary.Additive != 1 {
		t.Fatalf("expected one additive drift, got %#v", report.Summary)
	}
	if report.Summary.Breaking < 4 {
		t.Fatalf("expected multiple breaking changes, got %#v changes=%#v", report.Summary, report.Changes)
	}
}

func TestDiffContractSnapshotsIdenticalHasNoDrift(t *testing.T) {
	snapshot := contractSnapshot{
		SchemaVersion: contractSnapshotSchemaVersion,
		Env:           []contractEnvVar{{Key: "BITRIVER_LIVE_PORT", Default: "8080", Documented: true}},
	}
	report := diffContractSnapshots(snapshot, snapshot)
	if report.Summary != (contractDiffSummary{}) {
		t.Fatalf("expected empty summary, got %#v", report.Summary)
	}
	if len(report.Changes) != 0 {
		t.Fatalf("expected no changes, got %#v", report.Changes)
	}
}

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
