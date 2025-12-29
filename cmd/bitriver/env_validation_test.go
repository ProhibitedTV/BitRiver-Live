package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateEnvRequiresImageTags(t *testing.T) {
	values := map[string]string{
		"BITRIVER_POSTGRES_USER":                  "brlive_app",
		"BITRIVER_POSTGRES_PASSWORD":              "secret",
		"BITRIVER_REDIS_PASSWORD":                 "secret",
		"BITRIVER_OME_API":                        "http://ome:8081",
		"BITRIVER_OME_BIND":                       "1.2.3.4",
		"BITRIVER_OME_IP":                         "1.2.3.4",
		"BITRIVER_OME_SERVER_PORT":                "9000",
		"BITRIVER_OME_SERVER_TLS_PORT":            "9443",
		"BITRIVER_LIVE_ADMIN_EMAIL":               "admin@example.com",
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
		"BITRIVER_TRANSCODER_PUBLIC_BASE_URL":     "https://cdn.example.com/hls",
		"NEXT_PUBLIC_VIEWER_URL":                  "https://viewer.example.com",
	}

	res := validateEnv(values)
	if len(res.Missing) == 0 {
		t.Fatalf("expected missing image tags to be reported")
	}

	if len(res.Blocked) > 0 {
		t.Fatalf("did not expect placeholders to be blocked, got %v", res.Blocked)
	}
}

func TestValidateEnvFlagsLoopbackInProduction(t *testing.T) {
	values := map[string]string{
		"BITRIVER_POSTGRES_USER":                  "brlive_app",
		"BITRIVER_POSTGRES_PASSWORD":              "secret",
		"BITRIVER_REDIS_PASSWORD":                 "secret",
		"BITRIVER_OME_API":                        "http://localhost:8081",
		"BITRIVER_OME_BIND":                       "0.0.0.0",
		"BITRIVER_OME_IP":                         "0.0.0.0",
		"BITRIVER_OME_SERVER_PORT":                "9000",
		"BITRIVER_OME_SERVER_TLS_PORT":            "9443",
		"BITRIVER_LIVE_ADMIN_EMAIL":               "admin@example.com",
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
		"BITRIVER_TRANSCODER_PUBLIC_BASE_URL":     "http://localhost:9001/hls",
		"NEXT_PUBLIC_VIEWER_URL":                  "http://localhost:8080/viewer",
		"BITRIVER_LIVE_IMAGE_TAG":                 "1.0.0",
		"BITRIVER_VIEWER_IMAGE_TAG":               "1.0.0",
		"BITRIVER_SRS_CONTROLLER_IMAGE_TAG":       "1.0.0",
		"BITRIVER_TRANSCODER_IMAGE_TAG":           "1.0.0",
		"BITRIVER_SRS_IMAGE_TAG":                  "v5.0.185",
		"BITRIVER_OME_IMAGE_TAG":                  "0.16.0",
		"BITRIVER_LIVE_MODE":                      "production",
	}

	res := validateEnv(values)
	if len(res.Errors) == 0 {
		t.Fatalf("expected loopback values to produce errors in production")
	}
}

func TestRenderOMEConfigFromEnv(t *testing.T) {
	env := map[string]string{
		"BITRIVER_OME_BIND":            "10.1.2.3",
		"BITRIVER_OME_SERVER_PORT":     "9999",
		"BITRIVER_OME_SERVER_TLS_PORT": "9443",
		"BITRIVER_OME_USERNAME":        "omeuser",
		"BITRIVER_OME_PASSWORD":        "omepass",
		"BITRIVER_OME_API_TOKEN":       "apitoken",
		"BITRIVER_OME_ACCESS_TOKEN":    "accesstoken",
		"BITRIVER_OME_IP":              "10.1.2.4",
		"BITRIVER_OME_ICE_PORT_RANGE":  "20000-20009",
		"BITRIVER_OME_TCP_RELAY":       "25000",
		"BITRIVER_OME_IMAGE_TAG":       "0.16.0",
	}

	out := filepath.Join(t.TempDir(), "Server.generated.xml")
	cfg, err := buildOMERenderConfig(env, filepath.Join(repoRoot(), "deploy", "ome", "Server.xml"), out)
	if err != nil {
		t.Fatalf("build config: %v", err)
	}

	if err := renderOMEConfig(cfg); err != nil {
		t.Fatalf("render config: %v", err)
	}

	data := readFile(t, out)
	if !strings.Contains(data, "10.1.2.3") {
		t.Fatalf("expected bind address in output, got %q", data)
	}
	if !strings.Contains(data, "accesstoken") {
		t.Fatalf("expected access token in output")
	}
	if !strings.Contains(data, "<!-- Rendered for BITRIVER_OME_IMAGE_TAG=0.16.0 -->") {
		t.Fatalf("expected image tag marker in output")
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	return string(data)
}
