package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func containsString(values []string, target string) bool {
	for _, v := range values {
		if strings.Contains(v, target) {
			return true
		}
	}

	return false
}

func buildValidProductionEnv(t *testing.T) map[string]string {
	t.Helper()

	return map[string]string{
		"BITRIVER_LIVE_IMAGE_TAG":                 "1.0.0",
		"BITRIVER_VIEWER_IMAGE_TAG":               "1.0.0",
		"BITRIVER_SRS_CONTROLLER_IMAGE_TAG":       "1.0.0",
		"BITRIVER_TRANSCODER_IMAGE_TAG":           "1.0.0",
		"BITRIVER_SRS_IMAGE_TAG":                  "v5.0.185",
		"BITRIVER_OME_IMAGE_TAG":                  "0.16.0",
		"BITRIVER_POSTGRES_USER":                  "brlive_app",
		"BITRIVER_POSTGRES_PASSWORD":              "secret",
		"BITRIVER_REDIS_PASSWORD":                 "secret",
		"BITRIVER_OME_API":                        "http://ome.internal:8081",
		"BITRIVER_OME_BIND":                       "10.0.0.5",
		"BITRIVER_OME_IP":                         "10.0.0.6",
		"BITRIVER_OME_SERVER_PORT":                "9000",
		"BITRIVER_OME_SERVER_TLS_PORT":            "9443",
		"BITRIVER_LIVE_ADMIN_EMAIL":               "admin@bitriver.test",
		"BITRIVER_LIVE_ADMIN_PASSWORD":            "secure",
		"BITRIVER_LIVE_SESSION_TTL":               "168h",
		"BITRIVER_LIVE_ALLOW_SELF_SIGNUP":         "false",
		"BITRIVER_SRS_TOKEN":                      "token",
		"BITRIVER_OME_USERNAME":                   "omeuser",
		"BITRIVER_OME_PASSWORD":                   "omepass",
		"BITRIVER_OME_API_TOKEN":                  "apitoken",
		"BITRIVER_OME_ACCESS_TOKEN":               "accesstoken",
		"BITRIVER_TRANSCODER_TOKEN":               "transcodertoken",
		"BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD": "secret",
		"BITRIVER_TRANSCODER_PUBLIC_BASE_URL":     "http://cdn.edge/hls",
		"NEXT_PUBLIC_VIEWER_URL":                  "http://viewer.internal/viewer",
		"NEXT_PUBLIC_API_BASE_URL":                "http://api.internal",
		"BITRIVER_LIVE_MODE":                      "production",
		"BITRIVER_LIVE_METRICS_TOKEN":             "metrics-token",
		"BITRIVER_LIVE_RATE_LOGIN_LIMIT":          "10",
	}
}

func TestVersionOutputIncludesVersionLabel(t *testing.T) {
	var buf bytes.Buffer
	Version = "test-version"
	Commit = "test-commit"
	Date = "2024-01-01"

	printVersionInfo(&buf)

	output := buf.String()
	if !strings.Contains(output, "Version:") {
		t.Fatalf("expected output to contain Version:, got %q", output)
	}
	if !strings.Contains(output, "Commit:") {
		t.Fatalf("expected output to contain Commit:, got %q", output)
	}
	if !strings.Contains(output, "Date:") {
		t.Fatalf("expected output to contain Date:, got %q", output)
	}
}

func TestEnvInitWritesGeneratedValues(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	examplePath := defaultExampleEnv()

	if err := runEnvInit([]string{"--env-file", envPath, "--example", examplePath}); err != nil {
		t.Fatalf("env init failed: %v", err)
	}

	values, err := readEnvFile(envPath)
	if err != nil {
		t.Fatalf("read env file: %v", err)
	}

	if values["BITRIVER_POSTGRES_PASSWORD"] == "P0stgres-Example!" || values["BITRIVER_POSTGRES_PASSWORD"] == "" {
		t.Fatalf("expected postgres password to be generated, got %q", values["BITRIVER_POSTGRES_PASSWORD"])
	}

	if values["BITRIVER_LIVE_ADMIN_EMAIL"] == "" || values["BITRIVER_LIVE_ADMIN_EMAIL"] == "admin@stream.example.com" {
		t.Fatalf("expected admin email to be set, got %q", values["BITRIVER_LIVE_ADMIN_EMAIL"])
	}
}

func TestEnvValidateFailsForMissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.env")
	if err := runEnvValidate([]string{"--env-file", missing}); err == nil {
		t.Fatal("expected validation to fail for missing env file")
	}
}

func TestEnvValidateBlocksPlaceholders(t *testing.T) {
	root := t.TempDir()
	envPath := filepath.Join(root, ".env")
	content := strings.Join([]string{
		"BITRIVER_LIVE_IMAGE_TAG=v1.2.3",
		"BITRIVER_VIEWER_IMAGE_TAG=v1.2.3",
		"BITRIVER_SRS_CONTROLLER_IMAGE_TAG=v1.2.3",
		"BITRIVER_TRANSCODER_IMAGE_TAG=v1.2.3",
		"BITRIVER_SRS_IMAGE_TAG=v5.0.185",
		"BITRIVER_OME_IMAGE_TAG=0.16.0",
		"BITRIVER_POSTGRES_USER=brlive_app",
		"BITRIVER_POSTGRES_PASSWORD=P0stgres-Example!",
		"BITRIVER_REDIS_PASSWORD=secret",
		"BITRIVER_OME_API=http://ome:8081",
		"BITRIVER_OME_BIND=0.0.0.0",
		"BITRIVER_OME_IP=0.0.0.0",
		"BITRIVER_OME_SERVER_PORT=9000",
		"BITRIVER_OME_SERVER_TLS_PORT=9443",
		"BITRIVER_LIVE_ADMIN_EMAIL=admin@bitriver.local",
		"BITRIVER_LIVE_ADMIN_PASSWORD=password",
		"BITRIVER_LIVE_SESSION_TTL=168h",
		"BITRIVER_LIVE_ALLOW_SELF_SIGNUP=false",
		"BITRIVER_SRS_TOKEN=token",
		"BITRIVER_OME_USERNAME=ome-operator",
		"BITRIVER_OME_PASSWORD=ome-password",
		"BITRIVER_OME_API_TOKEN=api-token",
		"BITRIVER_OME_ACCESS_TOKEN=access-token",
		"BITRIVER_TRANSCODER_TOKEN=transcoder-token",
		"BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD=secret",
		"BITRIVER_TRANSCODER_PUBLIC_BASE_URL=http://localhost:9001/hls",
		"NEXT_PUBLIC_VIEWER_URL=http://localhost:8080/viewer",
		"BITRIVER_LIVE_MODE=development",
	}, "\n") + "\n"

	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	err := runEnvValidate([]string{"--env-file", envPath})
	if err == nil {
		t.Fatal("expected validation to fail due to placeholder password")
	}
}

func TestValidateEnvRequiresMetricsProtectionInProduction(t *testing.T) {
	values := buildValidProductionEnv(t)
	values["BITRIVER_LIVE_METRICS_TOKEN"] = ""
	values["BITRIVER_LIVE_METRICS_ALLOW_NETWORKS"] = ""

	res := validateEnv(values)

	if !containsString(res.Errors, "requires protecting /metrics") {
		t.Fatalf("expected metrics protection requirement in production, got errors=%v", res.Errors)
	}
}

func TestValidateEnvRequiresLoginRateLimitInProduction(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "empty", value: ""},
		{name: "zero", value: "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := buildValidProductionEnv(t)
			values["BITRIVER_LIVE_RATE_LOGIN_LIMIT"] = tt.value

			res := validateEnv(values)

			if !containsString(res.Errors, "login throttling") {
				t.Fatalf("expected login throttling requirement for %s value, got errors=%v", tt.name, res.Errors)
			}
		})
	}
}

func TestComposeRejectsUnknownSubcommand(t *testing.T) {
	if err := runCompose([]string{"noop"}); err == nil {
		t.Fatal("expected compose to error on unknown subcommand")
	}
}

func TestRunQuickstartBootstrapsAfterReady(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	composePath := filepath.Join(t.TempDir(), "compose.yml")

	envContent := strings.Join([]string{
		"BITRIVER_LIVE_ADMIN_EMAIL=admin@example.com",
		"BITRIVER_LIVE_ADMIN_PASSWORD=supersecret",
		"BITRIVER_LIVE_PORT=18080",
		"BITRIVER_POSTGRES_USER=brlive_app",
		"BITRIVER_POSTGRES_PASSWORD=testpass",
	}, "\n") + "\n"
	if err := os.WriteFile(envPath, []byte(envContent), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	originalDoctor := doctorRunner
	originalEnvInit := envInitRunner
	originalEnvValidate := envValidateRunner
	originalOMERunner := omeRunner
	originalMigrations := migrationsRunner
	originalCompose := composeUpRunner
	originalWaiter := quickstartWaiter
	originalBootstrap := bootstrapAdminRunner
	t.Cleanup(func() {
		doctorRunner = originalDoctor
		envInitRunner = originalEnvInit
		envValidateRunner = originalEnvValidate
		omeRunner = originalOMERunner
		migrationsRunner = originalMigrations
		composeUpRunner = originalCompose
		quickstartWaiter = originalWaiter
		bootstrapAdminRunner = originalBootstrap
	})

	var calls []string
	doctorRunner = func([]string) bool {
		calls = append(calls, "doctor")
		return true
	}
	envInitRunner = func(args []string) error {
		calls = append(calls, "env-init")
		expected := []string{"--env-file", envPath}
		if !reflect.DeepEqual(args, expected) {
			t.Fatalf("env init args = %v, want %v", args, expected)
		}
		return nil
	}
	envValidateRunner = func(args []string) error {
		calls = append(calls, "env-validate")
		expected := []string{"--env-file", envPath}
		if !reflect.DeepEqual(args, expected) {
			t.Fatalf("env validate args = %v, want %v", args, expected)
		}
		return nil
	}
	omeRunner = func(args []string) error {
		calls = append(calls, "ome")
		expected := []string{"render", "--env-file", envPath, "--force"}
		if !reflect.DeepEqual(args, expected) {
			t.Fatalf("ome args = %v, want %v", args, expected)
		}
		return nil
	}
	migrationsRunner = func(composeFile, envFile string) error {
		calls = append(calls, "migrations")
		if composeFile != composePath {
			t.Fatalf("migrations compose file = %s, want %s", composeFile, composePath)
		}
		if envFile != envPath {
			t.Fatalf("migrations env file = %s, want %s", envFile, envPath)
		}
		return nil
	}
	composeUpRunner = func(args []string) error {
		calls = append(calls, "compose-up")
		expected := []string{"--file", composePath, "--env-file", envPath}
		if !reflect.DeepEqual(args, expected) {
			t.Fatalf("compose args = %v, want %v", args, expected)
		}
		return nil
	}
	quickstartWaiter = func(values map[string]string) error {
		calls = append(calls, "wait")
		if values["BITRIVER_LIVE_PORT"] != "18080" {
			t.Fatalf("expected env values to be passed to waiter, got %v", values["BITRIVER_LIVE_PORT"])
		}
		return nil
	}
	bootstrapAdminRunner = func(composeFile string, values map[string]string) error {
		calls = append(calls, "bootstrap")
		if composeFile != composePath {
			t.Fatalf("bootstrap compose file = %s, want %s", composeFile, composePath)
		}
		if values["BITRIVER_LIVE_ADMIN_EMAIL"] != "admin@example.com" {
			t.Fatalf("expected admin email to propagate, got %s", values["BITRIVER_LIVE_ADMIN_EMAIL"])
		}
		return nil
	}

	if err := runQuickstart([]string{"--env-file", envPath, "--compose-file", composePath}); err != nil {
		t.Fatalf("quickstart failed: %v", err)
	}

	expectedCalls := []string{"doctor", "env-init", "env-validate", "ome", "migrations", "compose-up", "wait", "bootstrap"}
	if !reflect.DeepEqual(calls, expectedCalls) {
		t.Fatalf("call order = %v, want %v", calls, expectedCalls)
	}
}
