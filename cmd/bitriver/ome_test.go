package main

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"bitriver-live/internal/executil"
	"bitriver-live/internal/platformutil"
)

type fakeRunner struct {
	calls []fakeCall
	errOn func(executable string, args []string) error
}

type fakeCall struct {
	executable string
	args       []string
}

func (r *fakeRunner) Run(name string, args []string, _ ...executil.RunOption) error {
	r.calls = append(r.calls, fakeCall{executable: name, args: args})
	if r.errOn != nil {
		return r.errOn(name, args)
	}
	return nil
}

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

	commands := []platformutil.Command{{Executable: "python"}}
	runner := &fakeRunner{}
	if err := renderOMEConfig(commands, inputs, runner); err != nil {
		t.Fatalf("renderOMEConfig returned error: %v", err)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("expected one call, got %d", len(runner.calls))
	}

	capturedArgs := append([]string{runner.calls[0].executable}, runner.calls[0].args...)
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

func TestRenderOMEConfigFallsBackBetweenPythonCommands(t *testing.T) {
	inputs := omeRenderInputs{ScriptPath: "script.py", Template: "tmpl", Output: "out", Bind: "b", ServerIP: "ip", Port: "p", TLSPort: "tp", TCPRelay: "relay", ICECandidate: "ice", Username: "u", Password: "pw", APIToken: "api"}

	commands := []platformutil.Command{{Executable: "py", Args: []string{"-3"}}, {Executable: "py"}}

	runner := &fakeRunner{errOn: func(executable string, args []string) error {
		if executable == "py" && len(args) > 0 && args[0] == "-3" {
			return errors.New("-3 not available")
		}
		return nil
	}}

	if err := renderOMEConfig(commands, inputs, runner); err != nil {
		t.Fatalf("expected fallback to succeed, got %v", err)
	}
}

func TestRunOMERenderInjectsPythonFinder(t *testing.T) {
	findPython := func() ([]platformutil.Command, error) {
		return []platformutil.Command{{Executable: "python"}}, nil
	}

	runner := &fakeRunner{}
	if err := runOMERenderWithRunner([]string{"--template", "tmpl"}, findPython, runner); err == nil {
		// missing required flags should fail before executing
		t.Fatalf("expected validation failure for missing values")
	}

	if len(runner.calls) != 0 {
		t.Fatalf("runner should not be called when validation fails")
	}
}
