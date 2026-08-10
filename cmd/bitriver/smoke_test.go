package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSmokePasses(t *testing.T) {
	originalPrerequisites := smokePrerequisiteRunner
	originalPS := smokeComposePSRunner
	originalHTTP := smokeHTTPClient
	defer func() {
		smokePrerequisiteRunner = originalPrerequisites
		smokeComposePSRunner = originalPS
		smokeHTTPClient = originalHTTP
	}()

	smokeHTTPClient = &http.Client{Transport: smokeRoundTripper{}}

	t.Setenv("BITRIVER_LIVE_ADDR", ":19080")
	t.Setenv("BITRIVER_OME_HTTP_PORT", "19081")
	t.Setenv("BITRIVER_SRS_CONTROLLER_PORT", "19086")
	t.Setenv("BITRIVER_TRANSCODER_HOST_PORT", "19091")

	envPath := writeSmokeEnvFile(t, []string{
		"BITRIVER_LIVE_PORT=19080",
		"BITRIVER_OME_HTTP_PORT=19081",
		"BITRIVER_SRS_CONTROLLER_PORT=19086",
		"BITRIVER_TRANSCODER_HOST_PORT=19091",
	})

	smokePrerequisiteRunner = func() error { return nil }
	smokeComposePSRunner = func(string, string) ([]byte, error) {
		return []byte(`[
			{"Service":"bitriver-live","State":"running","Status":"Up","Health":"healthy"},
			{"Service":"ome","State":"running","Status":"Up","Health":"healthy"}
		]`), nil
	}

	if err := runSmoke([]string{"--env-file", envPath}); err != nil {
		t.Fatalf("runSmoke failed: %v", err)
	}
}

func TestRunSmokeFailsWhenPrerequisitesFail(t *testing.T) {
	originalPrerequisites := smokePrerequisiteRunner
	defer func() { smokePrerequisiteRunner = originalPrerequisites }()
	smokePrerequisiteRunner = func() error { return errors.New("Docker daemon unavailable") }

	err := runSmoke(nil)
	if err == nil {
		t.Fatal("expected smoke to fail when Docker prerequisites fail")
	}
	if !strings.Contains(err.Error(), "smoke checks failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSmokePrerequisitesDoNotCheckPrestartPortAvailability(t *testing.T) {
	originalLookPath := doctorLookPath
	originalCommandOutput := doctorCommandOutput
	originalHostPortChecker := doctorHostPortChecker
	defer func() {
		doctorLookPath = originalLookPath
		doctorCommandOutput = originalCommandOutput
		doctorHostPortChecker = originalHostPortChecker
	}()

	doctorLookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	doctorCommandOutput = func(name string, args ...string) (string, error) {
		if name != "docker" {
			return "", fmt.Errorf("unexpected command %s", name)
		}
		if len(args) > 0 && args[0] == "compose" {
			return "2.38.2", nil
		}
		return "28.0.4", nil
	}
	doctorHostPortChecker = func(string, int) error {
		t.Fatal("post-start smoke prerequisites must not check whether stack ports are free")
		return nil
	}

	if err := runSmokePrerequisites(); err != nil {
		t.Fatalf("runSmokePrerequisites failed: %v", err)
	}
}

func TestSmokeCheckComposeStateFailsWithoutBitriverLive(t *testing.T) {
	originalPS := smokeComposePSRunner
	defer func() { smokeComposePSRunner = originalPS }()

	smokeComposePSRunner = func(string, string) ([]byte, error) {
		return []byte(`[{"Service":"redis","State":"running","Status":"Up","Health":"healthy"}]`), nil
	}

	_, err := smokeCheckComposeState("deploy/docker-compose.yml", ".env")
	if err == nil {
		t.Fatal("expected compose check to fail")
	}
	if !strings.Contains(err.Error(), "missing service \"bitriver-live\"") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseComposeServiceStatesSupportsNDJSON(t *testing.T) {
	states, err := parseComposeServiceStates([]byte(strings.Join([]string{
		`{"Service":"bitriver-live","State":"running","Status":"Up","Health":"healthy"}`,
		`{"Service":"redis","State":"running","Status":"Up","Health":"healthy"}`,
	}, "\n")))
	if err != nil {
		t.Fatalf("parse compose NDJSON: %v", err)
	}
	if _, ok := states["bitriver-live"]; !ok {
		t.Fatalf("missing bitriver-live state: %#v", states)
	}
	if _, ok := states["redis"]; !ok {
		t.Fatalf("missing redis state: %#v", states)
	}
}

func TestPortOrDefaultFallsBackOnInvalid(t *testing.T) {
	values := map[string]string{"BITRIVER_OME_HTTP_PORT": "not-a-number"}
	if got := portOrDefault(values, "BITRIVER_OME_HTTP_PORT", "8081"); got != "8081" {
		t.Fatalf("portOrDefault = %s, want 8081", got)
	}
}

func writeSmokeEnvFile(t *testing.T, lines []string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}
	return path
}

type smokeRoundTripper struct{}

func (smokeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	path := req.URL.Path
	port := req.URL.Port()

	switch fmt.Sprintf("%s:%s", port, path) {
	case "19080:/readyz", "19080:/healthz", "19086:/healthz", "19091:/healthz", "19081:/":
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("ok")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	default:
		return nil, fmt.Errorf("unexpected smoke URL %s", req.URL.String())
	}
}
