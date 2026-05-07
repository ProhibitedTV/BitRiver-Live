package main

import (
	"bitriver-live/internal/config"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
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
		"BITRIVER_LIVE_IMAGE_DIGEST":              "@sha256:1111111111111111111111111111111111111111111111111111111111111111",
		"BITRIVER_VIEWER_IMAGE_TAG":               "1.0.0",
		"BITRIVER_VIEWER_IMAGE_DIGEST":            "@sha256:2222222222222222222222222222222222222222222222222222222222222222",
		"BITRIVER_SRS_CONTROLLER_IMAGE_TAG":       "1.0.0",
		"BITRIVER_SRS_CONTROLLER_IMAGE_DIGEST":    "@sha256:3333333333333333333333333333333333333333333333333333333333333333",
		"BITRIVER_TRANSCODER_IMAGE_TAG":           "1.0.0",
		"BITRIVER_TRANSCODER_IMAGE_DIGEST":        "@sha256:4444444444444444444444444444444444444444444444444444444444444444",
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
		"BITRIVER_LIVE_STORAGE_DRIVER":            "postgres",
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
	values, err := loadEnvValues(envPath, false)
	if err != nil {
		t.Fatalf("read env file: %v", err)
	}
	if values["BITRIVER_POSTGRES_PASSWORD"] == "P0stgres-Example!" || values["BITRIVER_POSTGRES_PASSWORD"] == "" {
		t.Fatalf("expected postgres password to be generated, got %q", values["BITRIVER_POSTGRES_PASSWORD"])
	}
	if values["BITRIVER_LIVE_ADMIN_EMAIL"] == "" || values["BITRIVER_LIVE_ADMIN_EMAIL"] == "admin@stream.example.com" {
		t.Fatalf("expected admin email to be set, got %q", values["BITRIVER_LIVE_ADMIN_EMAIL"])
	}
	if values["BITRIVER_OME_API_TOKEN"] == "" {
		t.Fatalf("expected OME API token to be generated, got empty value")
	}
	if values["BITRIVER_LIVE_MODE"] != "production" {
		t.Fatalf("expected generated .env to persist BITRIVER_LIVE_MODE=production, got %q", values["BITRIVER_LIVE_MODE"])
	}
}
func TestGenerateEnvValuesGeneratesOMEAPIToken(t *testing.T) {
	generated, _, err := generateEnvValues(map[string]string{})
	if err != nil {
		t.Fatalf("generate env values: %v", err)
	}
	if generated["BITRIVER_LIVE_MODE"] != "production" {
		t.Fatalf("expected BITRIVER_LIVE_MODE default to production, got %q", generated["BITRIVER_LIVE_MODE"])
	}
	apiToken := generated["BITRIVER_OME_API_TOKEN"]
	if apiToken == "" {
		t.Fatal("expected OME API token to be generated")
	}
}
func TestGenerateEnvValuesEmptyModeDefaultsToProduction(t *testing.T) {
	generated, _, err := generateEnvValues(map[string]string{"BITRIVER_LIVE_MODE": "   "})
	if err != nil {
		t.Fatalf("generate env values: %v", err)
	}
	if generated["BITRIVER_LIVE_MODE"] != "production" {
		t.Fatalf("expected empty mode to default to production, got %q", generated["BITRIVER_LIVE_MODE"])
	}
}
func TestGenerateEnvValuesPromotesDevelopmentModeToProduction(t *testing.T) {
	generated, _, err := generateEnvValues(map[string]string{"BITRIVER_LIVE_MODE": "development"})
	if err != nil {
		t.Fatalf("generate env values: %v", err)
	}
	if generated["BITRIVER_LIVE_MODE"] != "production" {
		t.Fatalf("expected development mode to be rewritten to production, got %q", generated["BITRIVER_LIVE_MODE"])
	}
}

func TestGenerateEnvValuesNormalizesOMEHealthcheckAuthMode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "empty defaults to access token", input: "", expected: "accesstoken"},
		{name: "basic stays basic", input: "basic", expected: "basic"},
		{name: "accesstoken stays accesstoken", input: "accesstoken", expected: "accesstoken"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			generated, _, err := generateEnvValues(map[string]string{"BITRIVER_OME_HEALTHCHECK_AUTH_MODE": tc.input})
			if err != nil {
				t.Fatalf("generate env values: %v", err)
			}
			if generated["BITRIVER_OME_HEALTHCHECK_AUTH_MODE"] != tc.expected {
				t.Fatalf("expected auth mode %q, got %q", tc.expected, generated["BITRIVER_OME_HEALTHCHECK_AUTH_MODE"])
			}
		})
	}
}
func TestGenerateEnvValuesPlaceholderModeDefaultsToProduction(t *testing.T) {
	generated, _, err := generateEnvValues(map[string]string{"BITRIVER_LIVE_MODE": "placeholder"})
	if err != nil {
		t.Fatalf("generate env values: %v", err)
	}
	if generated["BITRIVER_LIVE_MODE"] != "production" {
		t.Fatalf("expected placeholder mode to default to production, got %q", generated["BITRIVER_LIVE_MODE"])
	}
}

func TestGenerateEnvValuesReturnsErrorWhenSecretEntropyReadFails(t *testing.T) {
	orig := entropyRead
	entropyRead = func([]byte) (int, error) {
		return 0, errors.New("entropy unavailable")
	}
	t.Cleanup(func() { entropyRead = orig })

	_, _, err := generateEnvValues(map[string]string{})
	if err == nil {
		t.Fatal("expected generateEnvValues to fail when entropy read fails")
	}
	if !strings.Contains(err.Error(), "generate BITRIVER_") {
		t.Fatalf("expected key context in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "read entropy for secret") {
		t.Fatalf("expected entropy read context in error, got %v", err)
	}
}

func TestGenerateEnvValuesReturnsErrorWhenSuffixEntropyReadFails(t *testing.T) {
	orig := entropyRead
	entropyRead = func(b []byte) (int, error) {
		if len(b) == 6 {
			return 0, errors.New("entropy unavailable")
		}
		for i := range b {
			b[i] = byte(i + 1)
		}
		return len(b), nil
	}
	t.Cleanup(func() { entropyRead = orig })

	existing := map[string]string{}
	for key := range defaultEnvSecrets.secrets {
		existing[key] = "already-set"
	}
	existing["BITRIVER_LIVE_METRICS_TOKEN"] = "already-set"
	existing["BITRIVER_OME_USERNAME"] = ""

	_, _, err := generateEnvValues(existing)
	if err == nil {
		t.Fatal("expected generateEnvValues to fail when suffix entropy read fails")
	}
	if !strings.Contains(err.Error(), "generate BITRIVER_OME_USERNAME suffix") {
		t.Fatalf("expected username suffix context in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "read entropy for suffix") {
		t.Fatalf("expected entropy read context in error, got %v", err)
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
func TestValidateEnvRejectsInvalidLLHLSPorts(t *testing.T) {
	values := buildValidProductionEnv(t)
	values["BITRIVER_OME_LLHLS_PORT"] = "70000"
	values["BITRIVER_OME_LLHLS_TLS_PORT"] = "invalid"
	res := validateEnv(values)
	if !containsString(res.Errors, "BITRIVER_OME_LLHLS_PORT") {
		t.Fatalf("expected BITRIVER_OME_LLHLS_PORT error, got %v", res.Errors)
	}
	if !containsString(res.Errors, "BITRIVER_OME_LLHLS_TLS_PORT") {
		t.Fatalf("expected BITRIVER_OME_LLHLS_TLS_PORT error, got %v", res.Errors)
	}
}
func TestValidateOMEGeneratedConfigRejectsDeprecatedBindAddress(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Server.generated.xml")
	content := strings.Join([]string{
		"<Server>",
		"  <Server.bind.Address>127.0.0.1</Server.bind.Address>",
		"</Server>",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	err := validateOMEGeneratedConfig(path, config.NewEnvironmentFromMap(map[string]string{}))
	if err == nil {
		t.Fatal("expected validation to fail for deprecated Server.bind.Address")
	}
	if !strings.Contains(err.Error(), "deploy/ome/Server.generated.xml") {
		t.Fatalf("expected error to reference deploy/ome/Server.generated.xml, got %v", err)
	}
	if !strings.Contains(err.Error(), "go run ./cmd/bitriver ome render --force --env-file ./.env") {
		t.Fatalf("expected error to suggest go run command, got %v", err)
	}
	if !strings.Contains(err.Error(), "./scripts/render-ome-config.sh --force") {
		t.Fatalf("expected error to mention render-ome-config.sh, got %v", err)
	}
}
func TestValidateOMEGeneratedConfigRejectsApplicationOutputsWrapper(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Server.generated.xml")
	content := strings.Join([]string{
		"<Server>",
		"  <Managers><API><AccessToken>token</AccessToken></API></Managers>",
		"  <Bind><Managers><API><Port>8081</Port><TLSPort>9443</TLSPort><WorkerCount>1</WorkerCount></API></Managers></Bind>",
		"  <VirtualHosts><VirtualHost><Applications><Application><Outputs><OutputProfiles /></Outputs></Application></Applications></VirtualHost></VirtualHosts>",
		"</Server>",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	err := validateOMEGeneratedConfig(path, config.NewEnvironmentFromMap(map[string]string{}))
	if err == nil {
		t.Fatal("expected validation to fail for deprecated <Application><Outputs>")
	}
	if !strings.Contains(err.Error(), "<Application><Outputs>") {
		t.Fatalf("expected outputs-wrapper validation error, got %v", err)
	}
}

func TestValidateOMEGeneratedConfigAcceptsMatchingHealthcheckAccessToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Server.generated.xml")
	content := strings.Join([]string{
		"<Server>",
		"  <Managers><API><AccessToken>token</AccessToken></API></Managers>",
		"  <Bind><Managers><API><Port>8081</Port><TLSPort>9443</TLSPort><WorkerCount>1</WorkerCount></API></Managers></Bind>",
		"</Server>",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	originalAPIToken := os.Getenv("BITRIVER_OME_API_TOKEN")
	if err := os.Setenv("BITRIVER_OME_API_TOKEN", "token"); err != nil {
		t.Fatalf("set env: %v", err)
	}
	t.Cleanup(func() {
		if originalAPIToken == "" {
			_ = os.Unsetenv("BITRIVER_OME_API_TOKEN")
		} else {
			_ = os.Setenv("BITRIVER_OME_API_TOKEN", originalAPIToken)
		}
	})

	if err := validateOMEGeneratedConfig(path, config.NewEnvironmentFromMap(map[string]string{"BITRIVER_OME_API_TOKEN": "token"})); err != nil {
		t.Fatalf("expected matching BITRIVER_OME_API_TOKEN to pass validation, got %v", err)
	}
}

func TestValidateOMEGeneratedConfigRejectsMismatchedHealthcheckAccessToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Server.generated.xml")
	content := strings.Join([]string{
		"<Server>",
		"  <Managers><API><AccessToken>rendered-token</AccessToken></API></Managers>",
		"  <Bind><Managers><API><Port>8081</Port><TLSPort>9443</TLSPort><WorkerCount>1</WorkerCount></API></Managers></Bind>",
		"</Server>",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	originalAPIToken := os.Getenv("BITRIVER_OME_API_TOKEN")
	originalHealthcheckToken := os.Getenv("BITRIVER_OME_HEALTHCHECK_TOKEN")
	originalUsername := os.Getenv("BITRIVER_OME_USERNAME")
	originalPassword := os.Getenv("BITRIVER_OME_PASSWORD")
	if err := os.Setenv("BITRIVER_OME_HEALTHCHECK_TOKEN", "different-healthcheck-token"); err != nil {
		t.Fatalf("set env: %v", err)
	}
	if err := os.Unsetenv("BITRIVER_OME_API_TOKEN"); err != nil {
		t.Fatalf("unset env: %v", err)
	}
	if err := os.Unsetenv("BITRIVER_OME_USERNAME"); err != nil {
		t.Fatalf("unset env: %v", err)
	}
	if err := os.Unsetenv("BITRIVER_OME_PASSWORD"); err != nil {
		t.Fatalf("unset env: %v", err)
	}
	t.Cleanup(func() {
		if originalAPIToken == "" {
			_ = os.Unsetenv("BITRIVER_OME_API_TOKEN")
		} else {
			_ = os.Setenv("BITRIVER_OME_API_TOKEN", originalAPIToken)
		}
		if originalHealthcheckToken == "" {
			_ = os.Unsetenv("BITRIVER_OME_HEALTHCHECK_TOKEN")
		} else {
			_ = os.Setenv("BITRIVER_OME_HEALTHCHECK_TOKEN", originalHealthcheckToken)
		}
		if originalUsername == "" {
			_ = os.Unsetenv("BITRIVER_OME_USERNAME")
		} else {
			_ = os.Setenv("BITRIVER_OME_USERNAME", originalUsername)
		}
		if originalPassword == "" {
			_ = os.Unsetenv("BITRIVER_OME_PASSWORD")
		} else {
			_ = os.Setenv("BITRIVER_OME_PASSWORD", originalPassword)
		}
	})

	err := validateOMEGeneratedConfig(path, config.NewEnvironmentFromMap(map[string]string{"BITRIVER_OME_HEALTHCHECK_TOKEN": "different-healthcheck-token"}))
	if err == nil {
		t.Fatal("expected validation to fail for mismatched healthcheck access token")
	}
	if !strings.Contains(err.Error(), "BITRIVER_OME_HEALTHCHECK_TOKEN") || !strings.Contains(err.Error(), "BITRIVER_OME_API_TOKEN") {
		t.Fatalf("expected mismatch error to mention canonical token precedence variables, got %v", err)
	}
}

func TestRunOMERenderFailsWhenBindManagersAPIPortMismatchesComposeHealthcheckContract(t *testing.T) {
	_, envPath := setupOMERenderWorkspace(t)
	env := strings.Join([]string{
		"BITRIVER_OME_BIND=0.0.0.0",
		"BITRIVER_OME_IP=10.9.0.2",
		"BITRIVER_OME_SERVER_PORT=9000",
		"BITRIVER_OME_SERVER_TLS_PORT=9443",
		"BITRIVER_OME_HTTP_PORT=19001",
		"BITRIVER_OME_HTTP_TLS_PORT=19444",
		"BITRIVER_OME_LLHLS_PORT=8080",
		"BITRIVER_OME_LLHLS_TLS_PORT=8443",
		"BITRIVER_OME_API_TOKEN=operator-api-token",
	}, "\n") + "\n"
	if err := os.WriteFile(envPath, []byte(env), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	originalHealthcheckPort := os.Getenv("BITRIVER_OME_HTTP_PORT")
	if err := os.Setenv("BITRIVER_OME_HTTP_PORT", "8081"); err != nil {
		t.Fatalf("set env: %v", err)
	}
	t.Cleanup(func() {
		if originalHealthcheckPort == "" {
			_ = os.Unsetenv("BITRIVER_OME_HTTP_PORT")
			return
		}
		_ = os.Setenv("BITRIVER_OME_HTTP_PORT", originalHealthcheckPort)
	})

	err := runOMERender([]string{"--env-file", envPath, "--force", "--quiet"})
	if err == nil {
		t.Fatal("expected runOMERender to fail for compose healthcheck contract mismatch")
	}
	if !strings.Contains(err.Error(), "rendered <Server><Bind><Managers><API><Port> is \"19001\"") {
		t.Fatalf("expected mismatch error to include rendered API port, got %v", err)
	}
	if !strings.Contains(err.Error(), "BITRIVER_OME_HTTP_PORT=\"8081\"") {
		t.Fatalf("expected mismatch error to include expected healthcheck port, got %v", err)
	}
	if !strings.Contains(err.Error(), "BITRIVER_OME_API") {
		t.Fatalf("expected mismatch error to include remediation env vars, got %v", err)
	}
}

func TestValidateOMEGeneratedConfigRejectsApplicationLLHLS(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Server.generated.xml")
	content := strings.Join([]string{
		"<Server>",
		"  <Managers><API><AccessToken>token</AccessToken></API></Managers>",
		"  <Bind><Managers><API><Port>8081</Port><TLSPort>9443</TLSPort><WorkerCount>1</WorkerCount></API></Managers></Bind>",
		"  <VirtualHosts><VirtualHost><Applications><Application><LLHLS><Enable>true</Enable></LLHLS></Application></Applications></VirtualHost></VirtualHosts>",
		"</Server>",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	err := validateOMEGeneratedConfig(path, config.NewEnvironmentFromMap(map[string]string{}))
	if err == nil {
		t.Fatal("expected validation to fail for deprecated <Application><LLHLS>")
	}
	if !strings.Contains(err.Error(), "<Application><LLHLS>") {
		t.Fatalf("expected application-level LLHLS validation error, got %v", err)
	}
}

func TestRenderOMEConfigRewritesLegacyBindAddress(t *testing.T) {
	templatePath := filepath.Join(t.TempDir(), "Server.xml")
	outputPath := filepath.Join(t.TempDir(), "Server.generated.xml")
	templateBytes, err := os.ReadFile(filepath.Join(repoRoot(), "deploy", "ome", "Server.xml"))
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	template := strings.ReplaceAll(string(templateBytes), "<Address>", "<Server.bind.Address>")
	template = strings.ReplaceAll(template, "</Address>", "</Server.bind.Address>")
	if err := os.WriteFile(templatePath, []byte(template), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	cfg := omeRenderConfig{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Bind:         "10.0.0.10",
		ServerIP:     "10.0.0.11",
		Port:         "9000",
		TLSPort:      "9443",
		Username:     "ome-user",
		Password:     "ome-pass",
		APIToken:     "api-token",
		AccessToken:  "access-token",
		ImageTag:     "0.16.0",
		TCPRelay:     "127.0.0.1:3478",
		ICECandidate: "127.0.0.1:10000-10009/udp",
	}
	if err := renderOMEConfig(cfg); err != nil {
		t.Fatalf("render: %v", err)
	}
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	stripped, _ := stripXMLComments(string(output))
	if strings.Contains(stripped, "Server.bind.Address") {
		t.Fatalf("expected legacy bind address tag to be rewritten, got output:\n%s", string(output))
	}
	if !strings.Contains(string(output), "<Bind>") {
		t.Fatalf("expected output to include <Bind> block")
	}
	if strings.Contains(stripped, "<Bind>\n        <IP>") || strings.Contains(stripped, "<Bind>\n        <Address>") {
		t.Fatalf("expected root bind block to omit host binding fields, got output:\n%s", string(output))
	}
	if !strings.Contains(string(output), "<IP>10.0.0.11</IP>") {
		t.Fatalf("expected top-level server IP to be updated, got output:\n%s", string(output))
	}
}
func TestRenderOMEConfigPreservesXmlComments(t *testing.T) {
	templatePath := filepath.Join(t.TempDir(), "Server.xml")
	outputPath := filepath.Join(t.TempDir(), "Server.generated.xml")
	templateBytes, err := os.ReadFile(filepath.Join(repoRoot(), "deploy", "ome", "Server.xml"))
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	comment := "        <!-- legacy <Server.bind.Address> <IP>keep-me</IP> -->"
	template := string(templateBytes)
	if strings.Contains(template, "<Bind>\r\n") {
		template = strings.Replace(template, "<Bind>\r\n", "<Bind>\r\n"+comment+"\r\n", 1)
	} else {
		template = strings.Replace(template, "<Bind>\n", "<Bind>\n"+comment+"\n", 1)
	}
	if err := os.WriteFile(templatePath, []byte(template), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	cfg := omeRenderConfig{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Bind:         "10.0.0.10",
		ServerIP:     "10.0.0.11",
		Port:         "9000",
		TLSPort:      "9443",
		Username:     "ome-user",
		Password:     "ome-pass",
		APIToken:     "api-token",
		AccessToken:  "access-token",
		ImageTag:     "0.16.0",
		TCPRelay:     "127.0.0.1:3478",
		ICECandidate: "127.0.0.1:10000-10009/udp",
	}
	if err := renderOMEConfig(cfg); err != nil {
		t.Fatalf("render: %v", err)
	}
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(output), comment) {
		t.Fatalf("expected comment to be preserved, got output:\n%s", string(output))
	}
	var parsed struct{}
	if err := xml.Unmarshal(output, &parsed); err != nil {
		t.Fatalf("expected well-formed XML output, got error: %v", err)
	}
}
func TestRenderOMEConfigDiffersFromLegacyWhenUsingSplitAPIContexts(t *testing.T) {
	templatePath := filepath.Join(repoRoot(), "deploy", "ome", "Server.xml")
	outputPath := filepath.Join(t.TempDir(), "Server.generated.xml")
	cfg := omeRenderConfig{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Bind:         "10.0.0.10",
		ServerIP:     "10.0.0.11",
		Port:         "9000",
		TLSPort:      "9443",
		LLHLSPort:    "8080",
		LLHLSTLSPort: "8443",
		Username:     "ome-user",
		Password:     "ome-pass",
		APIToken:     "api-token",
		AccessToken:  "access-token",
		ImageTag:     "0.16.0",
		TCPRelay:     "127.0.0.1:3478",
		ICECandidate: "127.0.0.1:10000-10009/udp",
	}
	if err := renderOMEConfig(cfg); err != nil {
		t.Fatalf("render: %v", err)
	}
	if err := validateOMEGeneratedConfig(outputPath, config.NewEnvironmentFromMap(map[string]string{})); err != nil {
		t.Fatalf("expected generated output to pass split-context validation: %v", err)
	}
}

func TestRenderOMEConfigSkipsUnsupportedRootBindHostTagsAndFillsAccessTokenAndIceCandidates(t *testing.T) {
	templatePath := filepath.Join(t.TempDir(), "Server.xml")
	outputPath := filepath.Join(t.TempDir(), "Server.generated.xml")
	template := strings.Join([]string{
		"<Server version=\"10\">",
		"  <IP>0.0.0.0</IP>",
		"  <Managers>",
		"    <API>",
		"      <AccessToken>old-token</AccessToken>",
		"    </API>",
		"  </Managers>",
		"  <Bind>",
		"    <Address>0.0.0.0</Address>",
		"    <Managers>",
		"      <API>",
		"        <Port>9000</Port>",
		"        <TLSPort>9443</TLSPort>",
		"        <WorkerCount>1</WorkerCount>",
		"      </API>",
		"    </Managers>",
		"    <Publishers>",
		"      <LLHLS>",
		"        <Port>8080</Port>",
		"      </LLHLS>",
		"    </Publishers>",
		"  </Bind>",
		"  <IceCandidates>",
		"  </IceCandidates>",
		"</Server>",
	}, "\n")
	if err := os.WriteFile(templatePath, []byte(template), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	cfg := omeRenderConfig{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Bind:         "10.10.0.1",
		ServerIP:     "10.10.0.2",
		Port:         "9001",
		TLSPort:      "9444",
		HTTPPort:     "19001",
		HTTPTLSPort:  "19444",
		LLHLSPort:    "8081",
		LLHLSTLSPort: "8444",
		Username:     "ome-user",
		Password:     "ome-pass",
		APIToken:     "api-token",
		AccessToken:  "access-token",
		ImageTag:     "0.16.0",
		TCPRelay:     "127.0.0.1:3478",
		ICECandidate: "127.0.0.1:10000-10009/udp",
	}
	if err := renderOMEConfig(cfg); err != nil {
		t.Fatalf("render: %v", err)
	}
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	got := string(output)
	if strings.Contains(got, "<Bind>\n    <IP>") || strings.Contains(got, "<Bind>\n    <Address>") {
		t.Fatalf("expected root <Bind> to omit unsupported host tags, got output:\n%s", got)
	}
	if !strings.Contains(got, "<IP>10.10.0.2</IP>") {
		t.Fatalf("expected top-level <Server><IP> to be updated from BITRIVER_OME_IP, got output:\n%s", got)
	}
	if !strings.Contains(got, "<Managers>") || !strings.Contains(got, "<API>") {
		t.Fatalf("expected both top-level and bind API contexts, got output:\n%s", got)
	}
	var parsed struct {
		Managers struct {
			API struct {
				AccessToken string `xml:"AccessToken"`
				Port        string `xml:"Port"`
				TLSPort     string `xml:"TLSPort"`
				WorkerCount string `xml:"WorkerCount"`
			} `xml:"API"`
		} `xml:"Managers"`
		Bind struct {
			Managers struct {
				API struct {
					AccessToken string `xml:"AccessToken"`
					Port        string `xml:"Port"`
					TLSPort     string `xml:"TLSPort"`
					WorkerCount string `xml:"WorkerCount"`
				} `xml:"API"`
			} `xml:"Managers"`
		} `xml:"Bind"`
	}
	if err := xml.Unmarshal(output, &parsed); err != nil {
		t.Fatalf("parse output: %v", err)
	}
	if parsed.Managers.API.AccessToken != "access-token" {
		t.Fatalf("expected top-level auth access token replacement, got %q", parsed.Managers.API.AccessToken)
	}
	if parsed.Managers.API.Port != "" || parsed.Managers.API.TLSPort != "" || parsed.Managers.API.WorkerCount != "" {
		t.Fatalf("expected top-level auth block to omit listener fields, got port=%q tls=%q workers=%q", parsed.Managers.API.Port, parsed.Managers.API.TLSPort, parsed.Managers.API.WorkerCount)
	}
	if parsed.Bind.Managers.API.AccessToken != "" {
		t.Fatalf("expected bind listener block to omit <AccessToken>, got %q", parsed.Bind.Managers.API.AccessToken)
	}
	if parsed.Bind.Managers.API.Port != "19001" || parsed.Bind.Managers.API.TLSPort != "19444" || parsed.Bind.Managers.API.WorkerCount != "1" {
		t.Fatalf("expected <Bind><Managers><API> listener fields to be rewritten, got port=%q tls=%q workers=%q", parsed.Bind.Managers.API.Port, parsed.Bind.Managers.API.TLSPort, parsed.Bind.Managers.API.WorkerCount)
	}
	if strings.Contains(got, "<AccessTokens>") {
		t.Fatalf("expected singular <AccessToken> without deprecated <AccessTokens> wrapper, got output:\n%s", got)
	}
	if strings.Contains(got, "<Application><Outputs>") || strings.Contains(got, "<Outputs>") {
		t.Fatalf("expected rendered config to define output profiles directly under <Application><OutputProfiles>, got output:\n%s", got)
	}
	if !strings.Contains(got, "<TcpRelay>127.0.0.1:3478</TcpRelay>") {
		t.Fatalf("expected TcpRelay insertion, got output:\n%s", got)
	}
	if !strings.Contains(got, "<IceCandidate>127.0.0.1:10000-10009/udp</IceCandidate>") {
		t.Fatalf("expected IceCandidate insertion, got output:\n%s", got)
	}
}
func BenchmarkRenderOMEConfig(b *testing.B) {
	templatePath := filepath.Join(repoRoot(), "deploy", "ome", "Server.xml")
	outputPath := filepath.Join(b.TempDir(), "Server.generated.xml")
	cfg := omeRenderConfig{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Bind:         "10.0.0.10",
		ServerIP:     "10.0.0.11",
		Port:         "9000",
		TLSPort:      "9443",
		LLHLSPort:    "8080",
		LLHLSTLSPort: "8443",
		Username:     "ome-user",
		Password:     "ome-pass",
		APIToken:     "api-token",
		AccessToken:  "access-token",
		ImageTag:     "0.16.0",
		TCPRelay:     "127.0.0.1:3478",
		ICECandidate: "127.0.0.1:10000-10009/udp",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := renderOMEConfig(cfg); err != nil {
			b.Fatalf("render: %v", err)
		}
	}
}
func BenchmarkRenderOMEConfigLegacy(b *testing.B) {
	templatePath := filepath.Join(repoRoot(), "deploy", "ome", "Server.xml")
	cfg := omeRenderConfig{
		TemplatePath: templatePath,
		OutputPath:   filepath.Join(b.TempDir(), "Server.generated.xml"),
		Bind:         "10.0.0.10",
		ServerIP:     "10.0.0.11",
		Port:         "9000",
		TLSPort:      "9443",
		LLHLSPort:    "8080",
		LLHLSTLSPort: "8443",
		Username:     "ome-user",
		Password:     "ome-pass",
		APIToken:     "api-token",
		AccessToken:  "access-token",
		ImageTag:     "0.16.0",
		TCPRelay:     "127.0.0.1:3478",
		ICECandidate: "127.0.0.1:10000-10009/udp",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := renderOMEConfigLegacy(cfg); err != nil {
			b.Fatalf("render legacy: %v", err)
		}
	}
}
func TestRunMigrationsAddsNoTTYForNonTerminalStdin(t *testing.T) {
	tempDir := t.TempDir()
	dockerName := "docker"
	if runtime.GOOS == "windows" {
		dockerName = "docker.exe"
	}
	dockerPath := filepath.Join(tempDir, dockerName)
	if err := os.WriteFile(dockerPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write docker stub: %v", err)
	}
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	originalRunner := commandRunner
	originalStdin := os.Stdin
	t.Cleanup(func() {
		commandRunner = originalRunner
		os.Stdin = originalStdin
	})
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})
	os.Stdin = reader
	var gotArgs []string
	commandRunner = func(name string, args ...string) error {
		gotArgs = append([]string{name}, args...)
		return nil
	}
	if err := runMigrations("compose.yml", "env"); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	noTTYIndex := -1
	serviceIndex := -1
	for idx, arg := range gotArgs {
		if arg == "-T" {
			noTTYIndex = idx
		}
		if arg == "postgres-migrations" {
			serviceIndex = idx
		}
	}
	if noTTYIndex == -1 {
		t.Fatalf("expected -T in args, got %v", gotArgs)
	}
	if serviceIndex == -1 || noTTYIndex > serviceIndex {
		t.Fatalf("expected -T before service name, got %v", gotArgs)
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

func TestValidateComposeEffectiveEnvironmentRejectsInvalidOverride(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	values := buildValidProductionEnv(t)
	values["BITRIVER_OME_HEALTHCHECK_AUTH_MODE"] = "basic"
	var lines []string
	for key, value := range values {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}
	if err := os.WriteFile(envPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}
	t.Setenv("BITRIVER_OME_HEALTHCHECK_AUTH_MODE", "token+digest")

	err := validateComposeEffectiveEnvironment(envPath)
	if err == nil {
		t.Fatal("expected validation to fail when shell env override makes values invalid")
	}
	if !strings.Contains(err.Error(), "BITRIVER_OME_HEALTHCHECK_AUTH_MODE") {
		t.Fatalf("expected error to mention override key, got %v", err)
	}
	if !strings.Contains(err.Error(), "unset BITRIVER_OME_HEALTHCHECK_AUTH_MODE") {
		t.Fatalf("expected error to suggest unsetting override, got %v", err)
	}
}

func TestValidateComposeEffectiveEnvironmentRejectsCriticalOverrideEvenWhenValid(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	values := buildValidProductionEnv(t)
	values["BITRIVER_REDIS_PASSWORD"] = "from-env-file"
	var lines []string
	for key, value := range values {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}
	if err := os.WriteFile(envPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}
	t.Setenv("BITRIVER_REDIS_PASSWORD", "from-process")

	err := validateComposeEffectiveEnvironment(envPath)
	if err == nil {
		t.Fatal("expected validation to fail when critical key is overridden by process env")
	}
	if !strings.Contains(err.Error(), "BITRIVER_REDIS_PASSWORD") {
		t.Fatalf("expected error to mention override key, got %v", err)
	}
	if !strings.Contains(err.Error(), "Remove-Item Env:BITRIVER_REDIS_PASSWORD") {
		t.Fatalf("expected PowerShell unset guidance, got %v", err)
	}
}

func TestValidateComposeEffectiveEnvironmentAllowsMatchingHostOverride(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	values := buildValidProductionEnv(t)
	values["BITRIVER_OME_HEALTHCHECK_AUTH_MODE"] = "basic"
	var lines []string
	for key, value := range values {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}
	if err := os.WriteFile(envPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}
	t.Setenv("BITRIVER_OME_HEALTHCHECK_AUTH_MODE", "basic")

	if err := validateComposeEffectiveEnvironment(envPath); err != nil {
		t.Fatalf("expected matching host override to pass, got %v", err)
	}
}

func TestRunQuickstartBootstrapsAfterReady(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	composePath := filepath.Join(t.TempDir(), "compose.yml")
	values := buildValidProductionEnv(t)
	values["BITRIVER_LIVE_ADMIN_EMAIL"] = "admin@example.com"
	values["BITRIVER_LIVE_ADMIN_PASSWORD"] = "supersecret"
	values["BITRIVER_LIVE_PORT"] = "18080"
	var lines []string
	for key, value := range values {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}
	if err := os.WriteFile(envPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}
	originalDoctor := doctorRunner
	originalEnvInit := envInitRunner
	originalEnvValidate := envValidateRunner
	originalOMERunner := omeRunner
	originalMigrations := migrationsRunner
	originalCompose := composeUpRunner
	originalWaiter := quickstartWaiter
	originalComposeHealthWaiter := quickstartComposeHealthWaiter
	originalOMEPreflight := quickstartOMEAuthPreflightRunner
	originalImagePreflight := deployImageSourcePreflightRunner
	originalDockerVersion := dockerVersionRunner
	originalComposeVersion := dockerComposeVersionRunner
	originalPortPreflight := quickstartHostPortPreflightRunner
	originalBootstrap := bootstrapAdminRunner
	t.Cleanup(func() {
		doctorRunner = originalDoctor
		envInitRunner = originalEnvInit
		envValidateRunner = originalEnvValidate
		omeRunner = originalOMERunner
		migrationsRunner = originalMigrations
		composeUpRunner = originalCompose
		quickstartWaiter = originalWaiter
		quickstartComposeHealthWaiter = originalComposeHealthWaiter
		quickstartOMEAuthPreflightRunner = originalOMEPreflight
		deployImageSourcePreflightRunner = originalImagePreflight
		dockerVersionRunner = originalDockerVersion
		dockerComposeVersionRunner = originalComposeVersion
		quickstartHostPortPreflightRunner = originalPortPreflight
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
		expected := []string{"--file", composePath, "--env-file", envPath, "--image-source", deployImageSourcePull}
		if !reflect.DeepEqual(args, expected) {
			t.Fatalf("compose args = %v, want %v", args, expected)
		}
		return nil
	}
	quickstartWaiter = func(values map[string]string, composeFile, envFile string) error {
		calls = append(calls, "wait")
		if values["BITRIVER_LIVE_PORT"] != "18080" {
			t.Fatalf("expected env values to be passed to waiter, got %v", values["BITRIVER_LIVE_PORT"])
		}
		return nil
	}
	quickstartComposeHealthWaiter = func(composeFile, envFile string) error {
		calls = append(calls, "health")
		if composeFile != composePath {
			t.Fatalf("health compose file = %s, want %s", composeFile, composePath)
		}
		if envFile != envPath {
			t.Fatalf("health env file = %s, want %s", envFile, envPath)
		}
		return nil
	}
	quickstartOMEAuthPreflightRunner = func(string, map[string]string) error { return nil }
	dockerVersionRunner = func() error {
		calls = append(calls, "docker-version")
		return nil
	}
	dockerComposeVersionRunner = func() error {
		calls = append(calls, "docker-compose-version")
		return nil
	}
	quickstartHostPortPreflightRunner = func(map[string]string) error {
		calls = append(calls, "host-port-preflight")
		return nil
	}
	deployImageSourcePreflightRunner = func(mode string, values map[string]string, envFile string) error {
		calls = append(calls, "image-preflight")
		if mode != deployImageSourcePull {
			t.Fatalf("expected default image mode pull, got %s", mode)
		}
		return nil
	}
	bootstrapAdminRunner = func(composeFile, envFile string, values map[string]string) error {
		calls = append(calls, "bootstrap")
		if composeFile != composePath {
			t.Fatalf("bootstrap compose file = %s, want %s", composeFile, composePath)
		}
		if envFile != envPath {
			t.Fatalf("bootstrap env file = %s, want %s", envFile, envPath)
		}
		if values["BITRIVER_LIVE_ADMIN_EMAIL"] != "admin@example.com" {
			t.Fatalf("expected admin email to propagate, got %s", values["BITRIVER_LIVE_ADMIN_EMAIL"])
		}
		return nil
	}
	if err := runQuickstart([]string{"--env-file", envPath, "--compose-file", composePath}); err != nil {
		t.Fatalf("quickstart failed: %v", err)
	}
	expectedCalls := []string{"doctor", "env-init", "env-validate", "docker-version", "docker-compose-version", "host-port-preflight", "image-preflight", "migrations", "compose-up", "wait", "health", "bootstrap"}
	if !reflect.DeepEqual(calls, expectedCalls) {
		t.Fatalf("call order = %v, want %v", calls, expectedCalls)
	}
}

func TestRunQuickstartFailsWhenDeploymentPreflightFails(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	composePath := filepath.Join(t.TempDir(), "compose.yml")
	values := buildValidProductionEnv(t)
	values["BITRIVER_LIVE_PORT"] = "18080"
	var lines []string
	for key, value := range values {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}
	if err := os.WriteFile(envPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	originalDoctor := doctorRunner
	originalEnvInit := envInitRunner
	originalEnvValidate := envValidateRunner
	originalOMEPreflight := quickstartOMEAuthPreflightRunner
	originalDockerVersion := dockerVersionRunner
	originalComposeVersion := dockerComposeVersionRunner
	originalPortPreflight := quickstartHostPortPreflightRunner
	t.Cleanup(func() {
		doctorRunner = originalDoctor
		envInitRunner = originalEnvInit
		envValidateRunner = originalEnvValidate
		quickstartOMEAuthPreflightRunner = originalOMEPreflight
		dockerVersionRunner = originalDockerVersion
		dockerComposeVersionRunner = originalComposeVersion
		quickstartHostPortPreflightRunner = originalPortPreflight
	})

	doctorRunner = func([]string) bool { return true }
	envInitRunner = func([]string) error { return nil }
	envValidateRunner = func([]string) error { return nil }
	quickstartOMEAuthPreflightRunner = func(string, map[string]string) error { return nil }
	dockerVersionRunner = func() error { return nil }
	dockerComposeVersionRunner = func() error {
		return errors.New("docker compose version failed")
	}
	quickstartHostPortPreflightRunner = func(map[string]string) error { return nil }

	err := runQuickstart([]string{"--env-file", envPath, "--compose-file", composePath})
	if err == nil {
		t.Fatal("expected quickstart to fail on deployment preflight")
	}
	msg := err.Error()
	if !strings.Contains(msg, "quickstart stopped during Deployment preflight") {
		t.Fatalf("expected deployment preflight stage failure, got %v", err)
	}
	if !strings.Contains(msg, "Install/enable Docker Compose v2") {
		t.Fatalf("expected one-line next action for compose v2, got %v", err)
	}
}

func TestRunQuickstartHostPortPreflightReportsConflicts(t *testing.T) {
	values := buildValidProductionEnv(t)
	values["BITRIVER_LIVE_PORT"] = "18080"
	values["BITRIVER_SRS_CONTROLLER_PORT"] = "1986"
	originalChecker := quickstartHostPortAvailabilityChecker
	t.Cleanup(func() {
		quickstartHostPortAvailabilityChecker = originalChecker
	})
	quickstartHostPortAvailabilityChecker = func(protocol string, port int) error {
		if (protocol == "tcp" && (port == 18080 || port == 1986)) || (protocol == "udp" && port == 3478) {
			return errors.New("bind: address already in use")
		}
		return nil
	}

	err := runQuickstartHostPortPreflight(values)
	if err == nil {
		t.Fatal("expected host port preflight to report conflicts")
	}
	msg := err.Error()
	if !strings.Contains(msg, "host port conflicts detected") {
		t.Fatalf("expected conflict summary header, got %v", err)
	}
	if !strings.Contains(msg, "TCP 18080") || !strings.Contains(msg, "TCP 1986") || !strings.Contains(msg, "UDP 3478") {
		t.Fatalf("expected conflict list with ports/protocols, got %v", err)
	}
	if !strings.Contains(msg, "change the matching .env port value") {
		t.Fatalf("expected actionable next step, got %v", err)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	originalStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()

	fn()
	_ = w.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return string(data)
}

func TestPrintQuickstartSuccessSummaryIncludesHelpfulDetails(t *testing.T) {
	out := captureStdout(t, func() {
		printQuickstartSuccessSummary("deploy/docker-compose.yml", ".env", map[string]string{
			"BITRIVER_LIVE_PORT":        "18080",
			"BITRIVER_LIVE_ADMIN_EMAIL": "admin@example.com",
		})
	})

	checks := []string{
		"BitRiver Live is running",
		"Control/API URL: http://localhost:18080",
		"Viewer URL: http://localhost:18080/viewer",
		"Admin email: admin@example.com",
		"Env file: .env",
		"docker compose --file deploy/docker-compose.yml --env-file .env ps",
		"docker compose --file deploy/docker-compose.yml --env-file .env logs -f",
		"docker compose --file deploy/docker-compose.yml --env-file .env down",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("expected output to contain %q, got %q", check, out)
		}
	}
}

func TestQuickstartStageFailureUsesWhatHappenedAndNextSteps(t *testing.T) {
	err := quickstartStageFailure("Compose up", errors.New("docker compose up: missing network"), "Check logs and retry.")
	if err == nil {
		t.Fatal("expected error")
	}
	message := err.Error()
	if !strings.Contains(message, "What happened:") {
		t.Fatalf("expected What happened section, got %q", message)
	}
	if !strings.Contains(message, "What to do next:") {
		t.Fatalf("expected What to do next section, got %q", message)
	}
}

func TestWaitForComposeServiceHealthHealthy(t *testing.T) {
	originalRunner := composePSRunner
	originalTimeout := composeHealthWaitTimeout
	originalPollInterval := composeHealthPollInterval
	t.Cleanup(func() {
		composePSRunner = originalRunner
		composeHealthWaitTimeout = originalTimeout
		composeHealthPollInterval = originalPollInterval
	})

	composeHealthWaitTimeout = 100 * time.Millisecond
	composeHealthPollInterval = time.Millisecond
	composePSRunner = func(composeFile, envFile string) ([]byte, error) {
		return []byte(`[
			{"Service":"bitriver-live","State":"running","Health":"healthy","Status":"Up (healthy)"},
			{"Service":"ome","State":"running","Health":"healthy","Status":"Up (healthy)"},
			{"Service":"srs","State":"running","Health":"healthy","Status":"Up (healthy)"},
			{"Service":"transcoder","State":"running","Health":"healthy","Status":"Up (healthy)"},
			{"Service":"postgres","State":"running","Health":"healthy","Status":"Up (healthy)"},
			{"Service":"redis","State":"running","Health":"healthy","Status":"Up (healthy)"}
		]`), nil
	}

	if err := waitForComposeServiceHealth("compose.yml", ".env"); err != nil {
		t.Fatalf("waitForComposeServiceHealth failed: %v", err)
	}
}

func TestWaitForComposeServiceHealthFailsOnUnhealthyService(t *testing.T) {
	originalRunner := composePSRunner
	originalTimeout := composeHealthWaitTimeout
	originalPollInterval := composeHealthPollInterval
	t.Cleanup(func() {
		composePSRunner = originalRunner
		composeHealthWaitTimeout = originalTimeout
		composeHealthPollInterval = originalPollInterval
	})

	composeHealthWaitTimeout = 100 * time.Millisecond
	composeHealthPollInterval = time.Millisecond
	composePSRunner = func(composeFile, envFile string) ([]byte, error) {
		return []byte(`[
			{"Service":"bitriver-live","State":"running","Health":"unhealthy","Status":"Up (unhealthy)"},
			{"Service":"ome","State":"running","Health":"healthy","Status":"Up (healthy)"},
			{"Service":"srs","State":"running","Health":"healthy","Status":"Up (healthy)"},
			{"Service":"transcoder","State":"running","Health":"healthy","Status":"Up (healthy)"},
			{"Service":"postgres","State":"running","Health":"healthy","Status":"Up (healthy)"},
			{"Service":"redis","State":"running","Health":"healthy","Status":"Up (healthy)"}
		]`), nil
	}

	err := waitForComposeServiceHealth("compose.yml", ".env")
	if err == nil {
		t.Fatal("expected waitForComposeServiceHealth to fail for unhealthy service")
	}
	if !strings.Contains(err.Error(), "bitriver-live") {
		t.Fatalf("expected error to mention api service, got %v", err)
	}
	if !strings.Contains(err.Error(), "next commands:") {
		t.Fatalf("expected error to include next commands, got %v", err)
	}
}

func TestWaitForComposeServiceHealthTimesOutWithSummary(t *testing.T) {
	originalRunner := composePSRunner
	originalTimeout := composeHealthWaitTimeout
	originalPollInterval := composeHealthPollInterval
	t.Cleanup(func() {
		composePSRunner = originalRunner
		composeHealthWaitTimeout = originalTimeout
		composeHealthPollInterval = originalPollInterval
	})

	composeHealthWaitTimeout = 15 * time.Millisecond
	composeHealthPollInterval = time.Millisecond
	composePSRunner = func(composeFile, envFile string) ([]byte, error) {
		return []byte(`[
			{"Service":"bitriver-live","State":"running","Health":"starting","Status":"Up (health: starting)"},
			{"Service":"ome","State":"running","Health":"healthy","Status":"Up (healthy)"},
			{"Service":"srs","State":"running","Health":"healthy","Status":"Up (healthy)"},
			{"Service":"transcoder","State":"running","Health":"healthy","Status":"Up (healthy)"},
			{"Service":"postgres","State":"running","Health":"healthy","Status":"Up (healthy)"},
			{"Service":"redis","State":"running","Health":"healthy","Status":"Up (healthy)"}
		]`), nil
	}

	err := waitForComposeServiceHealth("compose.yml", ".env")
	if err == nil {
		t.Fatal("expected waitForComposeServiceHealth to timeout")
	}
	if !strings.Contains(err.Error(), "did not become healthy before timeout") {
		t.Fatalf("expected timeout message, got %v", err)
	}
	if !strings.Contains(err.Error(), "api=Up (health: starting)") {
		t.Fatalf("expected last-known state summary in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "failing services:") {
		t.Fatalf("expected timeout error to include failing services, got %v", err)
	}
}
func TestRunQuickstartFirstRunFailsUntilProductionOverridesAreSet(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	composePath := filepath.Join(t.TempDir(), "compose.yml")
	if err := os.WriteFile(composePath, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}
	originalDoctor := doctorRunner
	originalOMERunner := omeRunner
	originalMigrations := migrationsRunner
	originalCompose := composeUpRunner
	originalWaiter := quickstartWaiter
	originalComposeHealthWaiter := quickstartComposeHealthWaiter
	originalOMEPreflight := quickstartOMEAuthPreflightRunner
	originalBootstrap := bootstrapAdminRunner
	t.Cleanup(func() {
		doctorRunner = originalDoctor
		omeRunner = originalOMERunner
		migrationsRunner = originalMigrations
		composeUpRunner = originalCompose
		quickstartWaiter = originalWaiter
		quickstartComposeHealthWaiter = originalComposeHealthWaiter
		quickstartOMEAuthPreflightRunner = originalOMEPreflight
		bootstrapAdminRunner = originalBootstrap
	})
	doctorRunner = func([]string) bool { return true }
	omeRunner = func([]string) error { return nil }
	migrationsRunner = func(string, string) error { return nil }
	composeUpRunner = func([]string) error { return nil }
	quickstartWaiter = func(map[string]string, string, string) error { return nil }
	quickstartComposeHealthWaiter = func(string, string) error { return nil }
	quickstartOMEAuthPreflightRunner = func(string, map[string]string) error { return nil }
	bootstrapAdminRunner = func(string, string, map[string]string) error { return nil }
	err := runQuickstart([]string{"--env-file", envPath, "--compose-file", composePath})
	if err == nil {
		t.Fatal("expected quickstart to fail until production-safe overrides are configured")
	}
	if !strings.Contains(err.Error(), "quickstart-prod validation failed") {
		t.Fatalf("expected quickstart-prod actionable block, got %v", err)
	}
	values, err := loadEnvValues(envPath, false)
	if err != nil {
		t.Fatalf("read env file: %v", err)
	}
	if values["BITRIVER_LIVE_MODE"] != "production" {
		t.Fatalf("expected first-run quickstart env to persist production mode, got %q", values["BITRIVER_LIVE_MODE"])
	}
	envContents, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read env file contents: %v", err)
	}
	if !strings.Contains(string(envContents), "BITRIVER_LIVE_MODE=production") {
		t.Fatalf("expected generated .env to contain BITRIVER_LIVE_MODE=production, got:\n%s", string(envContents))
	}
	if strings.Contains(string(envContents), "BITRIVER_LIVE_MODE=development") {
		t.Fatalf("expected generated .env to avoid persisting development mode, got:\n%s", string(envContents))
	}
}

func TestRunQuickstartRejectsDeprecatedOMEHealthcheckAuthModes(t *testing.T) {
	restored := map[string]string{}
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		if !strings.HasPrefix(key, "BITRIVER_") {
			continue
		}
		restored[key] = parts[1]
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
	}
	t.Cleanup(func() {
		for key, value := range restored {
			_ = os.Setenv(key, value)
		}
	})

	for _, mode := range []string{"token", "token+basic"} {
		t.Run(mode, func(t *testing.T) {
			envPath := filepath.Join(t.TempDir(), ".env")
			composePath := filepath.Join(t.TempDir(), "compose.yml")
			envContent := strings.Join([]string{
				"BITRIVER_OME_HEALTHCHECK_AUTH_MODE=" + mode,
				"BITRIVER_LIVE_ADMIN_EMAIL=admin@example.com",
				"BITRIVER_LIVE_ADMIN_PASSWORD=supersecret",
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
			originalComposeHealthWaiter := quickstartComposeHealthWaiter
			originalOMEPreflight := quickstartOMEAuthPreflightRunner
			originalBootstrap := bootstrapAdminRunner
			t.Cleanup(func() {
				doctorRunner = originalDoctor
				envInitRunner = originalEnvInit
				envValidateRunner = originalEnvValidate
				omeRunner = originalOMERunner
				migrationsRunner = originalMigrations
				composeUpRunner = originalCompose
				quickstartWaiter = originalWaiter
				quickstartComposeHealthWaiter = originalComposeHealthWaiter
				quickstartOMEAuthPreflightRunner = originalOMEPreflight
				bootstrapAdminRunner = originalBootstrap
			})

			doctorRunner = func([]string) bool { return true }
			envInitRunner = runEnvInit
			envValidateRunner = runEnvValidate
			omeRunner = func([]string) error { return nil }
			migrationsRunner = func(string, string) error { return nil }
			composeUpRunner = func([]string) error { return nil }
			quickstartWaiter = func(map[string]string, string, string) error { return nil }
			quickstartComposeHealthWaiter = func(string, string) error { return nil }
			quickstartOMEAuthPreflightRunner = func(string, map[string]string) error { return nil }
			bootstrapAdminRunner = func(string, string, map[string]string) error { return nil }

			if err := runQuickstart([]string{"--env-file", envPath, "--compose-file", composePath}); err == nil {
				t.Fatalf("expected quickstart to reject deprecated mode %q", mode)
			}
		})
	}
}

func TestRunQuickstartRejectsInvalidOMEHealthcheckAuthModeBeforeInit(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	composePath := filepath.Join(t.TempDir(), "compose.yml")
	envContent := strings.Join([]string{
		"BITRIVER_OME_HEALTHCHECK_AUTH_MODE=token+digest",
		"BITRIVER_LIVE_ADMIN_EMAIL=admin@example.com",
		"BITRIVER_LIVE_ADMIN_PASSWORD=supersecret",
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
	originalComposeHealthWaiter := quickstartComposeHealthWaiter
	originalOMEPreflight := quickstartOMEAuthPreflightRunner
	originalBootstrap := bootstrapAdminRunner
	t.Cleanup(func() {
		doctorRunner = originalDoctor
		envInitRunner = originalEnvInit
		envValidateRunner = originalEnvValidate
		omeRunner = originalOMERunner
		migrationsRunner = originalMigrations
		composeUpRunner = originalCompose
		quickstartWaiter = originalWaiter
		quickstartComposeHealthWaiter = originalComposeHealthWaiter
		quickstartOMEAuthPreflightRunner = originalOMEPreflight
		bootstrapAdminRunner = originalBootstrap
	})

	doctorRunner = func([]string) bool { return true }
	envInitRunner = func([]string) error {
		t.Fatal("env init should not run when OME auth mode is invalid")
		return nil
	}
	envValidateRunner = func([]string) error {
		t.Fatal("env validate should not run when OME auth mode is invalid")
		return nil
	}
	omeRunner = func([]string) error {
		t.Fatal("OME render should not run when OME auth mode is invalid")
		return nil
	}
	migrationsRunner = func(string, string) error {
		t.Fatal("migrations should not run when OME auth mode is invalid")
		return nil
	}
	composeUpRunner = func([]string) error {
		t.Fatal("compose up should not run when OME auth mode is invalid")
		return nil
	}
	quickstartWaiter = func(map[string]string, string, string) error {
		t.Fatal("waiter should not run when OME auth mode is invalid")
		return nil
	}
	quickstartComposeHealthWaiter = func(string, string) error {
		t.Fatal("health waiter should not run when OME auth mode is invalid")
		return nil
	}
	quickstartOMEAuthPreflightRunner = func(string, map[string]string) error {
		t.Fatal("OME auth preflight should not run when OME auth mode is invalid")
		return nil
	}
	bootstrapAdminRunner = func(string, string, map[string]string) error {
		t.Fatal("bootstrap should not run when OME auth mode is invalid")
		return nil
	}

	err := runQuickstart([]string{"--env-file", envPath, "--compose-file", composePath})
	if err == nil {
		t.Fatal("expected quickstart to fail for invalid OME auth mode")
	}
	if !strings.Contains(err.Error(), "OME auth preflight failed: BITRIVER_OME_HEALTHCHECK_AUTH_MODE must be accesstoken, basic, or none/off/disabled") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "token+digest") {
		t.Fatalf("expected current value in error, got: %v", err)
	}
}
func TestRunQuickstartFirstRunInitPassesEnvValidationWindowsPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows path handling is only validated on windows")
	}
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, "config", ".env")
	if err := os.MkdirAll(filepath.Dir(envPath), 0o755); err != nil {
		t.Fatalf("mkdir env dir: %v", err)
	}
	composePath := filepath.Join(tempDir, "compose.yml")
	if err := os.WriteFile(composePath, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}
	originalDoctor := doctorRunner
	originalOMERunner := omeRunner
	originalMigrations := migrationsRunner
	originalCompose := composeUpRunner
	originalWaiter := quickstartWaiter
	originalComposeHealthWaiter := quickstartComposeHealthWaiter
	originalOMEPreflight := quickstartOMEAuthPreflightRunner
	originalBootstrap := bootstrapAdminRunner
	t.Cleanup(func() {
		doctorRunner = originalDoctor
		omeRunner = originalOMERunner
		migrationsRunner = originalMigrations
		composeUpRunner = originalCompose
		quickstartWaiter = originalWaiter
		quickstartComposeHealthWaiter = originalComposeHealthWaiter
		quickstartOMEAuthPreflightRunner = originalOMEPreflight
		bootstrapAdminRunner = originalBootstrap
	})
	doctorRunner = func([]string) bool { return true }
	omeRunner = func([]string) error { return nil }
	migrationsRunner = func(string, string) error { return nil }
	composeUpRunner = func([]string) error { return nil }
	quickstartWaiter = func(map[string]string, string, string) error { return nil }
	quickstartComposeHealthWaiter = func(string, string) error { return nil }
	quickstartOMEAuthPreflightRunner = func(string, map[string]string) error { return nil }
	bootstrapAdminRunner = func(string, string, map[string]string) error { return nil }
	err := runQuickstart([]string{"--env-file", envPath, "--compose-file", composePath})
	if err == nil {
		t.Fatal("expected quickstart to fail with production defaults on first run with windows path")
	}
	if !strings.Contains(err.Error(), "quickstart-prod validation failed") {
		t.Fatalf("expected quickstart-prod actionable block on windows path flow, got %v", err)
	}
	envContents, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read env file contents: %v", err)
	}
	if !strings.Contains(string(envContents), "BITRIVER_LIVE_MODE=production") {
		t.Fatalf("expected generated .env to contain BITRIVER_LIVE_MODE=production, got:\n%s", string(envContents))
	}
}
func TestRunQuickstartFirstRunInitValidateFailsFastWithActionableProductionBlock(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	composePath := filepath.Join(t.TempDir(), "compose.yml")
	if err := os.WriteFile(composePath, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}
	originalDoctor := doctorRunner
	originalEnvInit := envInitRunner
	originalEnvValidate := envValidateRunner
	originalOMERunner := omeRunner
	originalMigrations := migrationsRunner
	originalCompose := composeUpRunner
	originalWaiter := quickstartWaiter
	originalComposeHealthWaiter := quickstartComposeHealthWaiter
	originalOMEPreflight := quickstartOMEAuthPreflightRunner
	originalBootstrap := bootstrapAdminRunner
	t.Cleanup(func() {
		doctorRunner = originalDoctor
		envInitRunner = originalEnvInit
		envValidateRunner = originalEnvValidate
		omeRunner = originalOMERunner
		migrationsRunner = originalMigrations
		composeUpRunner = originalCompose
		quickstartWaiter = originalWaiter
		quickstartComposeHealthWaiter = originalComposeHealthWaiter
		quickstartOMEAuthPreflightRunner = originalOMEPreflight
		bootstrapAdminRunner = originalBootstrap
	})
	doctorRunner = func([]string) bool { return true }
	envInitRunner = runEnvInit
	envValidateRunner = runEnvValidate
	omeRunner = func([]string) error { return nil }
	migrationsRunner = func(string, string) error { return nil }
	composeUpRunner = func([]string) error { return nil }
	quickstartWaiter = func(map[string]string, string, string) error { return nil }
	quickstartComposeHealthWaiter = func(string, string) error { return nil }
	quickstartOMEAuthPreflightRunner = func(string, map[string]string) error { return nil }
	bootstrapAdminRunner = func(string, string, map[string]string) error { return nil }
	if _, err := os.Stat(envPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected first run to start without env file, got stat err=%v", err)
	}
	err := runQuickstart([]string{"--env-file", envPath, "--compose-file", composePath})
	if err == nil {
		t.Fatal("expected quickstart to fail fast with production defaults on first run")
	}
	if !strings.Contains(err.Error(), "quickstart-prod validation failed") {
		t.Fatalf("expected quickstart-prod actionable error block, got %v", err)
	}
	values, err := loadEnvValues(envPath, false)
	if err != nil {
		t.Fatalf("read env file: %v", err)
	}
	if values["BITRIVER_LIVE_MODE"] != "production" {
		t.Fatalf("expected generated .env to persist BITRIVER_LIVE_MODE=production, got %q", values["BITRIVER_LIVE_MODE"])
	}
	res := validateEnv(values)
	joinedErrors := strings.Join(res.Errors, " ")
	if !strings.Contains(joinedErrors, "BITRIVER_TRANSCODER_PUBLIC_BASE_URL") ||
		!strings.Contains(joinedErrors, "NEXT_PUBLIC_VIEWER_URL") ||
		!strings.Contains(joinedErrors, "BITRIVER_OME_BIND") ||
		!strings.Contains(joinedErrors, "BITRIVER_OME_IP") {
		t.Fatalf("expected strict production errors after first-run env init, got errors=%v warnings=%v", res.Errors, res.Warnings)
	}
}
func renderOMEConfigLegacy(cfg omeRenderConfig) (string, error) {
	data, err := os.ReadFile(cfg.TemplatePath)
	if err != nil {
		return "", fmt.Errorf("read template: %w", err)
	}
	text := string(data)
	text = replaceLegacyBindAddressLegacy(text)
	text = regexp.MustCompile(`<\s*Server\.bind\s*>`).ReplaceAllString(text, "<Bind>")
	text = regexp.MustCompile(`</\s*Server\.bind\s*>`).ReplaceAllString(text, "</Bind>")
	text, err = replaceRootBindingsLegacy(text, xmlEscape(cfg.Bind), xmlEscape(cfg.Port), xmlEscape(cfg.TLSPort))
	if err != nil {
		return "", err
	}
	text, err = replaceLLHLSPublisherPortsLegacy(text, xmlEscape(cfg.LLHLSPort), xmlEscape(cfg.LLHLSTLSPort))
	if err != nil {
		return "", err
	}
	text, err = replaceRootIPLegacy(text, xmlEscape(cfg.ServerIP))
	if err != nil {
		return "", err
	}
	text, err = scopedReplaceControlBindingsLegacy(text, xmlEscape(cfg.Bind))
	if err != nil {
		return "", err
	}
	text, err = ensureIceCandidatesTagLegacy(text, "TcpRelay", xmlEscape(cfg.TCPRelay), cfg.TemplatePath)
	if err != nil {
		return "", err
	}
	text, err = ensureIceCandidatesTagLegacy(text, "IceCandidate", xmlEscape(cfg.ICECandidate), cfg.TemplatePath)
	if err != nil {
		return "", err
	}
	text, err = replaceAccessTokenLegacy(text, cfg.AccessToken)
	if err != nil {
		return "", err
	}
	text = stampImageTag(text, cfg.ImageTag)
	text = collapseBlankLines(text)
	return text, nil
}
func replaceLegacyBindAddressLegacy(text string) string {
	text, comments := stripXMLComments(text)
	openLegacy := regexp.MustCompile(`<\s*Server\.bind\.Address\s*>`)
	closeLegacy := regexp.MustCompile(`</\s*Server\.bind\.Address\s*>`)
	if !openLegacy.MatchString(text) && !closeLegacy.MatchString(text) {
		return restoreXMLComments(text, comments)
	}
	if regexp.MustCompile(`<\s*Bind\s*>`).MatchString(text) || regexp.MustCompile(`<\s*Server\.bind\s*>`).MatchString(text) {
		text = openLegacy.ReplaceAllString(text, "<IP>")
		text = closeLegacy.ReplaceAllString(text, "</IP>")
		return restoreXMLComments(text, comments)
	}
	text = openLegacy.ReplaceAllString(text, "<Bind><IP>")
	text = closeLegacy.ReplaceAllString(text, "</IP></Bind>")
	return restoreXMLComments(text, comments)
}
func replaceTagContentLegacy(data, tag, value string) (string, error) {
	openTag := fmt.Sprintf("<%s>", tag)
	closeTag := fmt.Sprintf("</%s>", tag)
	start := strings.Index(data, openTag)
	if start == -1 {
		return "", fmt.Errorf("missing %s in template", openTag)
	}
	end := strings.Index(data[start:], closeTag)
	if end == -1 {
		return "", fmt.Errorf("missing %s in template", closeTag)
	}
	end += start
	return data[:start+len(openTag)] + value + data[end:], nil
}
func replaceAllTagContentLegacy(data, tag, value string, required bool) (string, error) {
	pattern := regexp.MustCompile(fmt.Sprintf(`(<%s>)([^<]*)(</%s>)`, tag, tag))
	replaced := pattern.ReplaceAllString(data, fmt.Sprintf(`${1}%s${3}`, value))
	if required && replaced == data {
		return "", fmt.Errorf("missing <%s> in template", tag)
	}
	return replaced, nil
}
func ensureIceCandidatesTagLegacy(text, tag, value, templatePath string) (string, error) {
	iceRe := regexp.MustCompile(`(?s)<IceCandidates>(.*?)</IceCandidates>`)
	matches := 0
	rewriteErr := error(nil)
	updated := iceRe.ReplaceAllStringFunc(text, func(section string) string {
		matches++
		if strings.Contains(section, "<"+tag+">") {
			replaced, err := replaceAllTagContentLegacy(section, tag, value, false)
			if err != nil {
				rewriteErr = err
				return section
			}
			return replaced
		}
		closing := "</IceCandidates>"
		insertPos := strings.LastIndex(section, closing)
		if insertPos == -1 {
			return section
		}
		indent := "    "
		if indentMatch := regexp.MustCompile(`\n([ \t]*)</IceCandidates>`).FindStringSubmatch(section); indentMatch != nil {
			indent = indentMatch[1]
		}
		childIndent := indent + "    "
		insertion := fmt.Sprintf("\n%s<%s>%s</%s>", childIndent, tag, value, tag)
		return section[:insertPos] + insertion + section[insertPos:]
	})
	if rewriteErr != nil {
		return "", rewriteErr
	}
	if matches == 0 {
		return "", fmt.Errorf("OME template %s missing <%s> (expected under <IceCandidates>)", templatePath, tag)
	}
	return updated, nil
}
func replaceRootBindingsLegacy(text, address, port, tlsPort string) (string, error) {
	text, comments := stripXMLComments(text)
	serverRe := regexp.MustCompile(`(?s)<Server[^>]*>(.*)</Server>`)
	serverLoc := serverRe.FindStringSubmatchIndex(text)
	if serverLoc == nil {
		return "", errors.New("missing <Server> root element in template")
	}
	serverBody := text[serverLoc[2]:serverLoc[3]]
	bindRe := regexp.MustCompile(`(?s)<Bind>(.*?)</Bind>`)
	bindLoc := bindRe.FindStringSubmatchIndex(serverBody)
	if bindLoc == nil {
		return "", errors.New("missing <Bind> section under <Server> in template")
	}
	bindBody := serverBody[bindLoc[2]:bindLoc[3]]
	var err error
	if strings.Contains(bindBody, "<Address>") {
		bindBody, err = replaceTagContentLegacy(bindBody, "Address", address)
		if err == nil {
			bindBody = strings.Replace(bindBody, "<Address>", "<IP>", 1)
			bindBody = strings.Replace(bindBody, "</Address>", "</IP>", 1)
		}
	} else if strings.Contains(bindBody, "<IP>") {
		bindBody, err = replaceTagContentLegacy(bindBody, "IP", address)
	}
	if err != nil {
		return "", err
	}
	signallingRe := regexp.MustCompile(`(?s)<Signalling>(.*?)</Signalling>`)
	rewriteErr := error(nil)
	signallingCount := 0
	bindBody = signallingRe.ReplaceAllStringFunc(bindBody, func(section string) string {
		signallingCount++
		match := signallingRe.FindStringSubmatch(section)
		inner := match[1]
		updated, errPort := replaceTagContentLegacy(inner, "Port", port)
		if errPort != nil {
			rewriteErr = errPort
			return section
		}
		updated, errPort = replaceTagContentLegacy(updated, "TLSPort", tlsPort)
		if errPort != nil {
			rewriteErr = errPort
			return section
		}
		return "<Signalling>" + updated + "</Signalling>"
	})
	if rewriteErr != nil {
		return "", rewriteErr
	}
	if signallingCount == 0 {
		bindBody, err = replaceTagContentLegacy(bindBody, "Port", port)
		if err != nil {
			return "", err
		}
		bindBody, err = replaceTagContentLegacy(bindBody, "TLSPort", tlsPort)
		if err != nil {
			return "", err
		}
	}
	serverBody = serverBody[:bindLoc[2]] + bindBody + serverBody[bindLoc[3]:]
	output := text[:serverLoc[2]] + serverBody + text[serverLoc[3]:]
	return restoreXMLComments(output, comments), nil
}
func replaceLLHLSPublisherPortsLegacy(text, port, tlsPort string) (string, error) {
	serverRe := regexp.MustCompile(`(?s)<Server[^>]*>(.*)</Server>`)
	serverLoc := serverRe.FindStringSubmatchIndex(text)
	if serverLoc == nil {
		return "", errors.New("missing <Server> root element in template")
	}
	serverBody := text[serverLoc[2]:serverLoc[3]]
	bindRe := regexp.MustCompile(`(?s)<Bind>(.*?)</Bind>`)
	bindLoc := bindRe.FindStringSubmatchIndex(serverBody)
	if bindLoc == nil {
		return "", errors.New("missing <Bind> section under <Server> in template")
	}
	bindBody := serverBody[bindLoc[2]:bindLoc[3]]
	publishersRe := regexp.MustCompile(`(?s)<Publishers>(.*?)</Publishers>`)
	publishersLoc := publishersRe.FindStringSubmatchIndex(bindBody)
	if publishersLoc == nil {
		return "", errors.New("missing <Publishers> section under <Bind> in template")
	}
	publishersBody := bindBody[publishersLoc[2]:publishersLoc[3]]
	llhlsRe := regexp.MustCompile(`(?s)<LLHLS>(.*?)</LLHLS>`)
	llhlsLoc := llhlsRe.FindStringSubmatchIndex(publishersBody)
	if llhlsLoc == nil {
		return "", errors.New("missing <LLHLS> section under <Publishers> in template")
	}
	llhlsBody := publishersBody[llhlsLoc[2]:llhlsLoc[3]]
	updated, err := replaceTagContentLegacy(llhlsBody, "Port", port)
	if err != nil {
		return "", err
	}
	if strings.Contains(updated, "<TLSPort>") {
		updated, err = replaceTagContentLegacy(updated, "TLSPort", tlsPort)
		if err != nil {
			return "", err
		}
	}
	publishersBody = publishersBody[:llhlsLoc[2]] + updated + publishersBody[llhlsLoc[3]:]
	bindBody = bindBody[:publishersLoc[2]] + publishersBody + bindBody[publishersLoc[3]:]
	serverBody = serverBody[:bindLoc[2]] + bindBody + serverBody[bindLoc[3]:]
	return text[:serverLoc[2]] + serverBody + text[serverLoc[3]:], nil
}
func replaceRootIPLegacy(text, ip string) (string, error) {
	serverRe := regexp.MustCompile(`(?s)<Server[^>]*>(.*)</Server>`)
	serverLoc := serverRe.FindStringSubmatchIndex(text)
	if serverLoc == nil {
		return "", errors.New("missing <Server> root element in template")
	}
	serverBody := text[serverLoc[2]:serverLoc[3]]
	ipRe := regexp.MustCompile(`(?s)<IP>(.*?)</IP>`)
	matches := ipRe.FindAllStringSubmatchIndex(serverBody, -1)
	for _, loc := range matches {
		start, end := loc[2], loc[3]
		bindOpen := strings.LastIndex(serverBody[:start], "<Bind>")
		bindClose := strings.LastIndex(serverBody[:start], "</Bind>")
		if bindOpen != -1 && (bindClose == -1 || bindClose < bindOpen) {
			continue
		}
		vhostOpen := strings.LastIndex(serverBody[:start], "<VirtualHosts>")
		vhostClose := strings.LastIndex(serverBody[:start], "</VirtualHosts>")
		if vhostOpen != -1 && (vhostClose == -1 || vhostClose < vhostOpen) {
			continue
		}
		serverBody = serverBody[:start] + ip + serverBody[end:]
		return text[:serverLoc[2]] + serverBody + text[serverLoc[3]:], nil
	}
	return text, nil
}
func scopedReplaceControlBindingsLegacy(text, bind string) (string, error) {
	text, comments := stripXMLComments(text)
	controlRe := regexp.MustCompile(`(?s)<Control>(.*?)</Control>`)
	controlLoc := controlRe.FindStringSubmatchIndex(text)
	if controlLoc == nil {
		return restoreXMLComments(text, comments), nil
	}
	controlBody := text[controlLoc[0]:controlLoc[1]]
	serverRe := regexp.MustCompile(`(?s)<Server>(.*?)</Server>`)
	serverLoc := serverRe.FindStringSubmatchIndex(controlBody)
	if serverLoc == nil {
		return restoreXMLComments(text, comments), nil
	}
	serverBody := controlBody[serverLoc[0]:serverLoc[1]]
	inner := serverLoc[2] - serverLoc[0]
	outer := serverLoc[3] - serverLoc[0]
	content := serverBody[inner:outer]
	var err error
	if strings.Contains(content, "<Bind>") {
		content, err = replaceAllTagContentLegacy(content, "Bind", bind, false)
		if err != nil {
			return "", err
		}
	}
	if strings.Contains(content, "<IP>") {
		content, err = replaceAllTagContentLegacy(content, "IP", bind, false)
		if err != nil {
			return "", err
		}
	}
	if strings.Contains(content, "<Address>") {
		content, err = replaceAllTagContentLegacy(content, "Address", bind, false)
		if err != nil {
			return "", err
		}
	}
	serverBody = serverBody[:inner] + content + serverBody[outer:]
	controlBody = controlBody[:serverLoc[0]] + serverBody + controlBody[serverLoc[1]:]
	output := text[:controlLoc[0]] + controlBody + text[controlLoc[1]:]
	return restoreXMLComments(output, comments), nil
}
func replaceAccessTokenLegacy(text, token string) (string, error) {
	token = xmlEscape(token)
	if strings.Contains(text, "<AccessToken>") {
		replaced, err := replaceTagContentLegacy(text, "AccessToken", token)
		if err != nil {
			return "", err
		}
		return replaced, nil
	}
	return "", errors.New("missing <AccessToken> in template")
}

func TestBuildOMERenderConfigSeparatesManagersAPIFromSignallingPorts(t *testing.T) {
	templatePath := filepath.Join(repoRoot(), "deploy", "ome", "Server.xml")
	outputPath := filepath.Join(t.TempDir(), "Server.generated.xml")
	values := map[string]string{
		"BITRIVER_OME_BIND":            "0.0.0.0",
		"BITRIVER_OME_IP":              "10.0.0.20",
		"BITRIVER_OME_SERVER_PORT":     "9000",
		"BITRIVER_OME_SERVER_TLS_PORT": "9443",
		"BITRIVER_OME_HTTP_PORT":       "18081",
		"BITRIVER_OME_HTTP_TLS_PORT":   "18082",
		"BITRIVER_OME_API_TOKEN":       "api-token",
		"BITRIVER_OME_ACCESS_TOKEN":    "health-token",
	}

	cfg, err := buildOMERenderConfig(values, templatePath, outputPath)
	if err != nil {
		t.Fatalf("build config: %v", err)
	}
	if cfg.Port != "9000" || cfg.TLSPort != "9443" {
		t.Fatalf("expected signalling ports from BITRIVER_OME_SERVER_* vars, got %q/%q", cfg.Port, cfg.TLSPort)
	}
	if cfg.HTTPPort != "18081" || cfg.HTTPTLSPort != "18082" {
		t.Fatalf("expected Managers API ports from BITRIVER_OME_HTTP_* vars, got %q/%q", cfg.HTTPPort, cfg.HTTPTLSPort)
	}
}

func TestPollUntilSuccess(t *testing.T) {
	calls := 0
	ready, err := pollUntil(context.Background(), 500*time.Millisecond, time.Millisecond, func(context.Context) (bool, error) {
		calls++
		return calls >= 3, nil
	})
	if err != nil {
		t.Fatalf("pollUntil returned error: %v", err)
	}
	if !ready {
		t.Fatal("expected pollUntil to report success")
	}
	if calls != 3 {
		t.Fatalf("expected 3 poll attempts, got %d", calls)
	}
}

func TestPollUntilTimeout(t *testing.T) {
	calls := 0
	ready, err := pollUntil(context.Background(), 5*time.Millisecond, time.Millisecond, func(context.Context) (bool, error) {
		calls++
		return false, nil
	})
	if err != nil {
		t.Fatalf("expected timeout without error, got %v", err)
	}
	if ready {
		t.Fatal("expected pollUntil timeout to report not ready")
	}
	if calls == 0 {
		t.Fatal("expected pollUntil to invoke poll function at least once")
	}
}

func TestPollUntilCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ready, err := pollUntil(ctx, 50*time.Millisecond, time.Millisecond, func(context.Context) (bool, error) {
		return false, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got %v", err)
	}
	if ready {
		t.Fatal("expected canceled pollUntil to report not ready")
	}
}

func TestWaitForAPIReadinessTimeoutIncludesDiagnostics(t *testing.T) {
	originalTimeout := readinessWaitTimeout
	originalPoll := readinessPollInterval
	originalDiagnostics := readinessDiagnosticsRunner
	t.Cleanup(func() {
		readinessWaitTimeout = originalTimeout
		readinessPollInterval = originalPoll
		readinessDiagnosticsRunner = originalDiagnostics
	})

	readinessWaitTimeout = 15 * time.Millisecond
	readinessPollInterval = time.Millisecond
	readinessDiagnosticsRunner = func(composeFile, envFile string) string {
		if composeFile != "deploy/docker-compose.yml" {
			t.Fatalf("compose file = %q, want %q", composeFile, "deploy/docker-compose.yml")
		}
		if envFile != ".env" {
			t.Fatalf("env file = %q, want %q", envFile, ".env")
		}
		return strings.Join([]string{
			"docker compose ps:",
			"bitriver-live  Exit 1",
			"bitriver-live key log lines:",
			"bitriver-live | fatal: postgres repository unavailable",
		}, "\n")
	}

	err := waitForAPIReadiness(map[string]string{"BITRIVER_LIVE_PORT": "1"}, "deploy/docker-compose.yml", ".env")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "API did not become ready before timeout") {
		t.Fatalf("expected timeout in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "docker compose ps") {
		t.Fatalf("expected compose diagnostics in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "postgres repository unavailable") {
		t.Fatalf("expected key error line in diagnostics, got %v", err)
	}
}

func TestGatherReadinessDiagnosticsIncludesStubbedPostgresHint(t *testing.T) {
	originalRunner := dockerComposeCommandRunner
	originalComposePSRunner := composePSRunner
	t.Cleanup(func() {
		dockerComposeCommandRunner = originalRunner
		composePSRunner = originalComposePSRunner
	})

	dockerComposeCommandRunner = func(composeFile, envFile string, extraArgs ...string) (string, error) {
		if composeFile != "deploy/docker-compose.yml" || envFile != ".env" {
			t.Fatalf("unexpected compose/env files: %q %q", composeFile, envFile)
		}
		joined := strings.Join(extraArgs, " ")
		switch joined {
		case "ps":
			return "NAME                STATUS\nbitriver-live       exited", nil
		case "ps --format json":
			return `[
				{"Service":"bitriver-live","State":"exited","Health":"","Status":"Exited (1)"},
				{"Service":"postgres","State":"running","Health":"healthy","Status":"Up (healthy)"},
				{"Service":"redis","State":"running","Health":"healthy","Status":"Up (healthy)"},
				{"Service":"srs","State":"running","Health":"healthy","Status":"Up (healthy)"},
				{"Service":"ome","State":"running","Health":"healthy","Status":"Up (healthy)"},
				{"Service":"transcoder","State":"running","Health":"healthy","Status":"Up (healthy)"}
			]`, nil
		case "logs --tail=80 bitriver-live":
			return strings.Join([]string{
				"bitriver-live | info: booting",
				"bitriver-live | fatal: pgx driver stubbed in this build",
				"bitriver-live | error: postgres repository unavailable",
			}, "\n"), nil
		default:
			t.Fatalf("unexpected compose args: %v", extraArgs)
			return "", nil
		}
	}
	composePSRunner = func(composeFile, envFile string) ([]byte, error) {
		return []byte(`[
			{"Service":"bitriver-live","State":"exited","Health":"","Status":"Exited (1)"},
			{"Service":"postgres","State":"running","Health":"healthy","Status":"Up (healthy)"},
			{"Service":"redis","State":"running","Health":"healthy","Status":"Up (healthy)"},
			{"Service":"srs","State":"running","Health":"healthy","Status":"Up (healthy)"},
			{"Service":"ome","State":"running","Health":"healthy","Status":"Up (healthy)"},
			{"Service":"transcoder","State":"running","Health":"healthy","Status":"Up (healthy)"}
		]`), nil
	}

	diagnostics := gatherReadinessDiagnostics("deploy/docker-compose.yml", ".env")
	if !strings.Contains(diagnostics, "docker compose ps:") {
		t.Fatalf("expected ps output in diagnostics, got %q", diagnostics)
	}
	if !strings.Contains(diagnostics, "critical service status:") {
		t.Fatalf("expected critical service status section, got %q", diagnostics)
	}
	if !strings.Contains(diagnostics, "- api: Exited (1)") {
		t.Fatalf("expected api status line, got %q", diagnostics)
	}
	if !strings.Contains(diagnostics, "next commands:") {
		t.Fatalf("expected per-service next command section, got %q", diagnostics)
	}
	if !strings.Contains(diagnostics, "pgx driver stubbed in this build") {
		t.Fatalf("expected stubbed pgx line in diagnostics, got %q", diagnostics)
	}
	if !strings.Contains(diagnostics, "Hint: bitriver-live appears to be running without the Postgres module wired in") {
		t.Fatalf("expected postgres wiring remediation hint, got %q", diagnostics)
	}
}

func TestResolveDeployImageSourceDefaultsToPull(t *testing.T) {
	cfg, err := resolveDeployImageSource("", map[string]string{}, config.NewEnvironmentFromMap(map[string]string{}))
	if err != nil {
		t.Fatalf("resolveDeployImageSource returned error: %v", err)
	}
	if cfg.mode != deployImageSourcePull {
		t.Fatalf("mode = %q, want %q", cfg.mode, deployImageSourcePull)
	}
}

func TestResolveDeployImageSourceRejectsInvalidValue(t *testing.T) {
	if _, err := resolveDeployImageSource("invalid", map[string]string{}, config.NewEnvironmentFromMap(map[string]string{})); err == nil {
		t.Fatal("expected invalid image source to fail")
	}
}

func TestRunPullImagePreflightReturnsAccessDeniedMessage(t *testing.T) {
	originalManifest := manifestInspectRunner
	t.Cleanup(func() {
		manifestInspectRunner = originalManifest
	})

	manifestInspectRunner = func(string) error {
		return errors.New("denied: requested access to the resource is denied")
	}

	err := runPullImagePreflight(map[string]string{
		"BITRIVER_LIVE_IMAGE_TAG":           "v1.2.3",
		"BITRIVER_VIEWER_IMAGE_TAG":         "v1.2.3",
		"BITRIVER_SRS_CONTROLLER_IMAGE_TAG": "v1.2.3",
		"BITRIVER_TRANSCODER_IMAGE_TAG":     "v1.2.3",
	})
	if err == nil {
		t.Fatal("expected pull preflight to fail")
	}
	if !strings.Contains(err.Error(), "docker login ghcr.io") {
		t.Fatalf("expected docker login guidance, got %v", err)
	}
}

func TestRunComposeUpRejectsBuildModeInProduction(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	values := buildValidProductionEnv(t)
	values["BITRIVER_OME_HEALTHCHECK_AUTH_MODE"] = "accesstoken"
	var lines []string
	for key, value := range values {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}
	if err := os.WriteFile(envPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	err := runComposeUp([]string{"--file", "deploy/docker-compose.yml", "--env-file", envPath, "--image-source", "build"})
	if err == nil {
		t.Fatal("expected production contract failure for build mode")
	}
	if !strings.Contains(err.Error(), "image source mode must be \"pull\"") {
		t.Fatalf("expected pull-mode contract error, got %v", err)
	}
}

func TestRunQuickstartPassesLimitsOverlayToComposeUp(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	composePath := filepath.Join(t.TempDir(), "compose.yml")
	values := buildValidProductionEnv(t)
	values["BITRIVER_LIVE_PORT"] = "18080"
	var lines []string
	for key, value := range values {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}
	if err := os.WriteFile(envPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}
	if err := os.WriteFile(composePath, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	originalDoctor := doctorRunner
	originalEnvInit := envInitRunner
	originalEnvValidate := envValidateRunner
	originalOME := omeRunner
	originalMigrations := migrationsRunner
	originalComposeUp := composeUpRunner
	originalWaiter := quickstartWaiter
	originalHealthWaiter := quickstartComposeHealthWaiter
	originalOMEPreflight := quickstartOMEAuthPreflightRunner
	originalDockerVersion := dockerVersionRunner
	originalComposeVersion := dockerComposeVersionRunner
	originalPortPreflight := quickstartHostPortPreflightRunner
	originalImagePreflight := deployImageSourcePreflightRunner
	originalBootstrap := bootstrapAdminRunner
	t.Cleanup(func() {
		doctorRunner = originalDoctor
		envInitRunner = originalEnvInit
		envValidateRunner = originalEnvValidate
		omeRunner = originalOME
		migrationsRunner = originalMigrations
		composeUpRunner = originalComposeUp
		quickstartWaiter = originalWaiter
		quickstartComposeHealthWaiter = originalHealthWaiter
		quickstartOMEAuthPreflightRunner = originalOMEPreflight
		dockerVersionRunner = originalDockerVersion
		dockerComposeVersionRunner = originalComposeVersion
		quickstartHostPortPreflightRunner = originalPortPreflight
		deployImageSourcePreflightRunner = originalImagePreflight
		bootstrapAdminRunner = originalBootstrap
	})

	doctorRunner = func([]string) bool { return true }
	envInitRunner = func([]string) error { return nil }
	envValidateRunner = func([]string) error { return nil }
	omeRunner = func([]string) error { return nil }
	migrationsRunner = func(string, string) error { return nil }
	quickstartWaiter = func(map[string]string, string, string) error { return nil }
	quickstartComposeHealthWaiter = func(string, string) error { return nil }
	quickstartOMEAuthPreflightRunner = func(string, map[string]string) error { return nil }
	dockerVersionRunner = func() error { return nil }
	dockerComposeVersionRunner = func() error { return nil }
	quickstartHostPortPreflightRunner = func(map[string]string) error { return nil }
	deployImageSourcePreflightRunner = func(string, map[string]string, string) error { return nil }
	bootstrapAdminRunner = func(string, string, map[string]string) error { return nil }
	composeUpRunner = func(args []string) error {
		if !reflect.DeepEqual(args, []string{"--file", composePath, "--env-file", envPath, "--limits", "--image-source", deployImageSourcePull}) {
			t.Fatalf("compose args = %v", args)
		}
		return nil
	}

	if err := runQuickstart([]string{"--env-file", envPath, "--compose-file", composePath, "--limits"}); err != nil {
		t.Fatalf("quickstart failed: %v", err)
	}
}

func TestRunComposeUpIncludesLimitsOverlayFile(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	values := buildValidProductionEnv(t)
	values["BITRIVER_OME_HEALTHCHECK_AUTH_MODE"] = "accesstoken"
	var lines []string
	for key, value := range values {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}
	if err := os.WriteFile(envPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	originalCommandRunner := commandRunner
	originalLookPath := lookPathRunner
	originalDeployPreflight := deployImageSourcePreflightRunner
	t.Cleanup(func() {
		commandRunner = originalCommandRunner
		lookPathRunner = originalLookPath
		deployImageSourcePreflightRunner = originalDeployPreflight
	})

	called := false
	commandRunner = func(name string, args ...string) error {
		called = true
		if name != "docker" {
			t.Fatalf("expected docker, got %s", name)
		}
		expectedPrefix := []string{"compose", "--env-file", envPath, "--file", "deploy/docker-compose.yml", "--file", defaultComposeLimitsFile(), "up"}
		if len(args) < len(expectedPrefix) || !reflect.DeepEqual(args[:len(expectedPrefix)], expectedPrefix) {
			t.Fatalf("unexpected compose args: %v", args)
		}
		return nil
	}
	lookPathRunner = func(file string) (string, error) { return "/usr/bin/docker", nil }
	deployImageSourcePreflightRunner = func(string, map[string]string, string) error { return nil }

	if err := runComposeUp([]string{"--file", "deploy/docker-compose.yml", "--env-file", envPath, "--limits"}); err != nil {
		t.Fatalf("compose up failed: %v", err)
	}
	if !called {
		t.Fatal("expected docker compose command to run")
	}
}
