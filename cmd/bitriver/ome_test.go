package main

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPrepareOMERenderConfigUsesEnvDefaults(t *testing.T) {
	workDir := t.TempDir()
	envValues := map[string]string{
		"BITRIVER_OME_BIND":            "0.0.0.0",
		"BITRIVER_OME_IP":              "1.2.3.4",
		"BITRIVER_OME_SERVER_PORT":     "9000",
		"BITRIVER_OME_SERVER_TLS_PORT": "9443",
		"BITRIVER_OME_TCP_RELAY":       "*:3478",
		"BITRIVER_OME_ICE_CANDIDATE":   "*:10000-10009/udp",
		"BITRIVER_OME_USERNAME":        "user",
		"BITRIVER_OME_PASSWORD":        "pass",
		"BITRIVER_OME_API_TOKEN":       "api-token",
		"BITRIVER_OME_ACCESS_TOKEN":    "",
		"BITRIVER_OME_IMAGE_TAG":       "v1",
	}

	cfg, err := prepareOMERenderConfig(envValues, omeRenderInputs{}, workDir)
	if err != nil {
		t.Fatalf("prepareOMERenderConfig returned error: %v", err)
	}

	if cfg.ScriptPath != filepath.Join(workDir, "scripts", "render_ome_config.py") {
		t.Fatalf("unexpected script path: %s", cfg.ScriptPath)
	}
	if cfg.Template != filepath.Join(workDir, "deploy", "ome", "Server.xml") {
		t.Fatalf("unexpected template: %s", cfg.Template)
	}
	if cfg.Output != filepath.Join(workDir, "deploy", "ome", "Server.generated.xml") {
		t.Fatalf("unexpected output: %s", cfg.Output)
	}

	if cfg.ServerIP != "1.2.3.4" {
		t.Fatalf("expected server IP from env, got %s", cfg.ServerIP)
	}
	if cfg.AccessToken != "api-token" {
		t.Fatalf("expected access token to default to API token, got %s", cfg.AccessToken)
	}
	if cfg.ImageTag != "v1" {
		t.Fatalf("expected image tag from env, got %s", cfg.ImageTag)
	}
}

func TestPrepareOMERenderConfigMissingRequired(t *testing.T) {
	_, err := prepareOMERenderConfig(map[string]string{}, omeRenderInputs{}, t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing required values")
	}
	if !strings.Contains(err.Error(), "bind") {
		t.Fatalf("expected missing bind in error, got %v", err)
	}
}

func TestRenderOMEConfigPassesArguments(t *testing.T) {
	inputs := omeRenderInputs{
		ScriptPath:   filepath.Join("scripts", "render_ome_config.py"),
		Template:     "template.xml",
		Output:       "output.xml",
		Bind:         "0.0.0.0",
		ServerIP:     "1.2.3.4",
		Port:         "9000",
		TLSPort:      "9443",
		TCPRelay:     "*:3478",
		ICECandidate: "*:10000-10009/udp",
		Username:     "user",
		Password:     "pass",
		APIToken:     "api-token",
		AccessToken:  "access-token",
		ImageTag:     "v1",
	}

	var capturedArgs []string
	runner := func(pythonPath string, args []string, _ io.Writer) error {
		capturedArgs = append([]string{pythonPath}, args...)
		return nil
	}

	if err := renderOMEConfig("python", inputs, runner); err != nil {
		t.Fatalf("renderOMEConfig returned error: %v", err)
	}

	expected := []string{
		"python",
		filepath.Join("scripts", "render_ome_config.py"),
		"--template", "template.xml",
		"--output", "output.xml",
		"--bind", "0.0.0.0",
		"--server-ip", "1.2.3.4",
		"--tcp-relay", "*:3478",
		"--ice-candidate", "*:10000-10009/udp",
		"--port", "9000",
		"--tls-port", "9443",
		"--username", "user",
		"--password", "pass",
		"--api-token", "api-token",
		"--access-token", "access-token",
		"--image-tag", "v1",
	}

	if !reflect.DeepEqual(expected, capturedArgs) {
		t.Fatalf("unexpected args: got %v, want %v", capturedArgs, expected)
	}
}

func TestLoadEnvValuesPrefersFile(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	if err := os.WriteFile(envPath, []byte("BITRIVER_OME_BIND=from-file\n"), 0o644); err != nil {
		t.Fatalf("failed to write env file: %v", err)
	}

	t.Setenv("BITRIVER_OME_BIND", "from-env")

	values, err := loadEnvValues(envPath)
	if err != nil {
		t.Fatalf("loadEnvValues returned error: %v", err)
	}

	if values["BITRIVER_OME_BIND"] != "from-file" {
		t.Fatalf("expected file value to win, got %s", values["BITRIVER_OME_BIND"])
	}
}
