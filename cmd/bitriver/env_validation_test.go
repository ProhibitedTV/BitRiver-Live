package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEnvInitThenValidateRequiresProductionRoutableOverridesOnFreshRepo(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")

	if err := runEnvInit([]string{"--env-file", envPath, "--example", defaultExampleEnv()}); err != nil {
		t.Fatalf("env init failed: %v", err)
	}

	err := runEnvValidate([]string{"--env-file", envPath})
	if err == nil {
		t.Fatal("expected env validate to fail until production-safe routable values are set")
	}
}

func TestValidateEnvRequiresImageTags(t *testing.T) {
	cert, key := tempTLSFiles(t)
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
		"BITRIVER_LIVE_TLS_CERT":                  cert,
		"BITRIVER_LIVE_TLS_KEY":                   key,
	}

	res := validateEnv(values)
	if len(res.Missing) == 0 {
		t.Fatalf("expected missing image tags to be reported")
	}

	if len(res.Blocked) > 0 {
		t.Fatalf("did not expect placeholders to be blocked, got %v", res.Blocked)
	}
}

func TestValidateEnvBlocksSampleCredentials(t *testing.T) {
	placeholders, err := loadSampleCredentialValues(defaultExampleEnv(), sampleCredentialKeys)
	if err != nil {
		t.Fatalf("load sample credentials: %v", err)
	}

	cert, key := tempTLSFiles(t)
	values := map[string]string{
		"BITRIVER_POSTGRES_USER":              "brlive_app",
		"BITRIVER_OME_API":                    "http://ome:8081",
		"BITRIVER_OME_BIND":                   "10.0.0.5",
		"BITRIVER_OME_IP":                     "10.0.0.6",
		"BITRIVER_OME_SERVER_PORT":            "9000",
		"BITRIVER_OME_SERVER_TLS_PORT":        "9443",
		"BITRIVER_LIVE_SESSION_TTL":           "168h",
		"BITRIVER_LIVE_ALLOW_SELF_SIGNUP":     "false",
		"BITRIVER_TRANSCODER_PUBLIC_BASE_URL": "https://cdn.example.net/hls",
		"NEXT_PUBLIC_API_BASE_URL":            "https://api.example.net",
		"NEXT_PUBLIC_VIEWER_URL":              "https://viewer.example.net",
		"BITRIVER_LIVE_IMAGE_TAG":             "1.2.3",
		"BITRIVER_VIEWER_IMAGE_TAG":           "1.2.3",
		"BITRIVER_SRS_CONTROLLER_IMAGE_TAG":   "1.2.3",
		"BITRIVER_TRANSCODER_IMAGE_TAG":       "1.2.3",
		"BITRIVER_SRS_IMAGE_TAG":              "v5.0.185",
		"BITRIVER_OME_IMAGE_TAG":              "0.16.1",
		"BITRIVER_LIVE_MODE":                  "production",
		"BITRIVER_LIVE_TLS_CERT":              cert,
		"BITRIVER_LIVE_TLS_KEY":               key,
	}

	for key, value := range placeholders {
		values[key] = value
	}

	res := validateEnv(values)

	for _, key := range sampleCredentialKeys {
		if !containsValue(res.Blocked, key) {
			t.Fatalf("expected %s to be blocked when using the sample credential", key)
		}
	}
}

func tempTLSFiles(t *testing.T) (string, string) {
	t.Helper()

	dir := t.TempDir()
	cert := filepath.Join(dir, "cert.pem")
	key := filepath.Join(dir, "key.pem")

	if err := os.WriteFile(cert, []byte("dummy-cert"), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(key, []byte("dummy-key"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	return cert, key
}

func TestValidateEnvAllowsWildcardOMEListenersInProduction(t *testing.T) {
	values := map[string]string{
		"BITRIVER_POSTGRES_USER":                  "brlive_app",
		"BITRIVER_POSTGRES_PASSWORD":              "secret",
		"BITRIVER_REDIS_PASSWORD":                 "secret",
		"BITRIVER_OME_API":                        "https://ome.stream.local",
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
		"BITRIVER_TRANSCODER_PUBLIC_BASE_URL":     "https://cdn.stream.local/hls",
		"BITRIVER_OME_PUBLIC_LLHLS_BASE_URL":      "https://stream.stream.local/live",
		"BITRIVER_SRS_PUBLIC_RTMP_BASE_URL":       "rtmp://ingest.stream.local:1935/live",
		"NEXT_PUBLIC_VIEWER_URL":                  "https://viewer.stream.local",
		"BITRIVER_LIVE_IMAGE_TAG":                 "1.0.0",
		"BITRIVER_VIEWER_IMAGE_TAG":               "1.0.0",
		"BITRIVER_SRS_CONTROLLER_IMAGE_TAG":       "1.0.0",
		"BITRIVER_TRANSCODER_IMAGE_TAG":           "1.0.0",
		"BITRIVER_SRS_IMAGE_TAG":                  "v5.0.185",
		"BITRIVER_OME_IMAGE_TAG":                  "0.16.0",
		"BITRIVER_LIVE_MODE":                      "production",
		"BITRIVER_LIVE_METRICS_TOKEN":             "metrics-token",
		"BITRIVER_LIVE_RATE_LOGIN_LIMIT":          "10",
	}

	res := validateEnv(values)
	joinedErrors := strings.Join(res.Errors, " ")
	if strings.Contains(joinedErrors, "BITRIVER_OME_BIND") || strings.Contains(joinedErrors, "BITRIVER_OME_IP") {
		t.Fatalf("OME wildcard listener values must remain valid inside a production container, got %v", res.Errors)
	}
}

func TestValidateEnvRejectsExternalInsecurePostgresDSN(t *testing.T) {
	cert, key := tempTLSFiles(t)
	values := baseEnvValues(cert, key)
	values["BITRIVER_LIVE_POSTGRES_DSN"] = "postgres://user:secret@db.example:5432/bitriver?sslmode=disable"

	res := validateEnv(values)
	if len(res.Errors) == 0 {
		t.Fatal("expected sslmode=disable to be rejected for external Postgres")
	}
	if !strings.Contains(strings.Join(res.Errors, " "), "sslmode") {
		t.Fatalf("expected sslmode guidance in errors, got %v", res.Errors)
	}
}

func TestValidateEnvRequiresProductionMode(t *testing.T) {
	cert, key := tempTLSFiles(t)
	values := baseEnvValues(cert, key)
	delete(values, "BITRIVER_LIVE_MODE")

	res := validateEnv(values)
	if len(res.Errors) == 0 {
		t.Fatal("expected missing BITRIVER_LIVE_MODE to be rejected")
	}
	if !strings.Contains(strings.Join(res.Errors, " "), "BITRIVER_LIVE_MODE must be set to production") {
		t.Fatalf("expected production mode guidance, got %v", res.Errors)
	}
}

func TestValidateEnvRejectsDevelopmentMode(t *testing.T) {
	cert, key := tempTLSFiles(t)
	values := baseEnvValues(cert, key)
	values["BITRIVER_LIVE_MODE"] = "development"

	res := validateEnv(values)
	if len(res.Errors) == 0 {
		t.Fatal("expected development mode to be rejected")
	}
	if !strings.Contains(strings.Join(res.Errors, " "), "development") {
		t.Fatalf("expected development mode guidance, got %v", res.Errors)
	}
}

func TestValidateEnvRequiresPairedNumericHostIdentity(t *testing.T) {
	cert, key := tempTLSFiles(t)

	t.Run("paired numeric values", func(t *testing.T) {
		values := baseEnvValues(cert, key)
		values["BITRIVER_HOST_UID"] = "1001"
		values["BITRIVER_HOST_GID"] = "1001"
		res := validateEnv(values)
		for _, message := range res.Errors {
			if strings.Contains(message, "BITRIVER_HOST_UID") || strings.Contains(message, "BITRIVER_HOST_GID") {
				t.Fatalf("numeric host identity should be accepted, got %q", message)
			}
		}
	})

	t.Run("missing pair", func(t *testing.T) {
		values := baseEnvValues(cert, key)
		values["BITRIVER_HOST_UID"] = "1001"
		res := validateEnv(values)
		if !strings.Contains(strings.Join(res.Errors, " "), "must both be set or both be empty") {
			t.Fatalf("expected paired host identity error, got %v", res.Errors)
		}
	})

	t.Run("non numeric", func(t *testing.T) {
		values := baseEnvValues(cert, key)
		values["BITRIVER_HOST_UID"] = "operator"
		values["BITRIVER_HOST_GID"] = "1001"
		res := validateEnv(values)
		if !strings.Contains(strings.Join(res.Errors, " "), "unsigned 32-bit numeric ID") {
			t.Fatalf("expected numeric host identity error, got %v", res.Errors)
		}
	})
}

func TestValidateEnvRuntimeDevelopmentOverrideKeepsSavedProductionMode(t *testing.T) {
	values := buildValidProductionEnv(t)
	values["BITRIVER_LIVE_MODE"] = "production"
	values["BITRIVER_OME_BIND"] = "0.0.0.0"
	values["BITRIVER_OME_IP"] = "0.0.0.0"
	values["NEXT_PUBLIC_VIEWER_URL"] = "http://localhost:8080/viewer"
	values["BITRIVER_TRANSCODER_PUBLIC_BASE_URL"] = "http://localhost:9080/hls"

	result := validateEnvWithRuntimeMode(values, "development")
	for _, err := range result.Errors {
		if strings.Contains(err, "BITRIVER_OME_BIND") || strings.Contains(err, "BITRIVER_OME_IP") || strings.Contains(err, "BITRIVER_LIVE_MODE") {
			t.Fatalf("inline development override should keep saved production mode and wildcard OME listeners valid, got %q", err)
		}
	}
	joinedWarnings := strings.Join(result.Warnings, " ")
	if strings.Contains(joinedWarnings, "BITRIVER_OME_BIND") || strings.Contains(joinedWarnings, "BITRIVER_OME_IP") {
		t.Fatalf("did not expect warnings for valid OME wildcard listeners, got %v", result.Warnings)
	}
}

func TestValidateEnvRejectsUnreadableTLSCert(t *testing.T) {
	cert, key := tempTLSFiles(t)
	values := baseEnvValues(cert, key)
	values["BITRIVER_LIVE_TLS_CERT"] = filepath.Join(t.TempDir(), "missing.pem")

	res := validateEnv(values)
	if len(res.Errors) == 0 {
		t.Fatal("expected unreadable TLS cert to be rejected")
	}
	if !strings.Contains(strings.Join(res.Errors, " "), "BITRIVER_LIVE_TLS_CERT") {
		t.Fatalf("expected TLS cert guidance, got %v", res.Errors)
	}
}

func TestValidateEnvRejectsUnsupportedOMEHealthcheckAuthMode(t *testing.T) {
	cert, key := tempTLSFiles(t)
	values := baseEnvValues(cert, key)
	values["BITRIVER_OME_HEALTHCHECK_AUTH_MODE"] = "token+digest"

	res := validateEnv(values)
	if !containsString(res.Errors, "BITRIVER_OME_HEALTHCHECK_AUTH_MODE must be accesstoken or basic") {
		t.Fatalf("expected unsupported auth mode error, got %v", res.Errors)
	}
}

func TestValidateEnvRejectsDeprecatedOMEHealthcheckAuthMode(t *testing.T) {
	cert, key := tempTLSFiles(t)
	values := baseEnvValues(cert, key)
	values["BITRIVER_OME_HEALTHCHECK_AUTH_MODE"] = "token+basic"

	res := validateEnv(values)
	if !containsString(res.Errors, "BITRIVER_OME_HEALTHCHECK_AUTH_MODE must be accesstoken or basic") {
		t.Fatalf("expected deprecated alias to be rejected, got errors=%v warnings=%v", res.Errors, res.Warnings)
	}
}

func TestValidateEnvAllowsWildcardOMEWithComposeAPIInProduction(t *testing.T) {
	values := map[string]string{
		"BITRIVER_POSTGRES_USER":                  "brlive_app",
		"BITRIVER_POSTGRES_PASSWORD":              "secret",
		"BITRIVER_REDIS_PASSWORD":                 "secret",
		"BITRIVER_OME_API":                        "http://ome:8081",
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
		"BITRIVER_TRANSCODER_PUBLIC_BASE_URL":     "https://cdn.stream.local/hls",
		"NEXT_PUBLIC_VIEWER_URL":                  "https://viewer.stream.local",
		"BITRIVER_LIVE_IMAGE_TAG":                 "1.0.0",
		"BITRIVER_VIEWER_IMAGE_TAG":               "1.0.0",
		"BITRIVER_SRS_CONTROLLER_IMAGE_TAG":       "1.0.0",
		"BITRIVER_TRANSCODER_IMAGE_TAG":           "1.0.0",
		"BITRIVER_SRS_IMAGE_TAG":                  "v5.0.185",
		"BITRIVER_OME_IMAGE_TAG":                  "0.16.0",
		"BITRIVER_LIVE_MODE":                      "production",
		"BITRIVER_LIVE_METRICS_TOKEN":             "metrics-token",
		"BITRIVER_LIVE_RATE_LOGIN_LIMIT":          "10",
	}

	res := validateEnv(values)
	joinedErrors := strings.Join(res.Errors, " ")
	if strings.Contains(joinedErrors, "BITRIVER_OME_BIND") || strings.Contains(joinedErrors, "BITRIVER_OME_IP") {
		t.Fatalf("OME wildcard listener values must be allowed with the Compose API hostname, got %v", res.Errors)
	}
}

func TestLoadEnvValuesPreservesQuotedValues(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	content := "" +
		"PLAIN=value\n" +
		"QUOTED=\"quoted value with spaces\"\n" +
		"TRIM=  spaced  \n"

	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	values, err := loadEnvValues(envPath, false)
	if err != nil {
		t.Fatalf("load env values: %v", err)
	}

	if values["PLAIN"] != "value" {
		t.Fatalf("expected PLAIN parsed, got %q", values["PLAIN"])
	}

	if values["QUOTED"] != "\"quoted value with spaces\"" {
		t.Fatalf("expected QUOTED to preserve quotes, got %q", values["QUOTED"])
	}

	if values["TRIM"] != "spaced" {
		t.Fatalf("expected TRIM whitespace trimmed, got %q", values["TRIM"])
	}
}

func TestValidateEnvRejectsLoopbackQuickstartDefaultsInProduction(t *testing.T) {
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
		"BITRIVER_LIVE_METRICS_TOKEN":             "metrics-token",
		"BITRIVER_LIVE_RATE_LOGIN_LIMIT":          "10",
	}

	res := validateEnv(values)
	joinedErrors := strings.Join(res.Errors, " ")
	if !strings.Contains(joinedErrors, "BITRIVER_TRANSCODER_PUBLIC_BASE_URL") ||
		!strings.Contains(joinedErrors, "NEXT_PUBLIC_VIEWER_URL") {
		t.Fatalf("expected production errors for loopback quickstart defaults, got errors=%v warnings=%v", res.Errors, res.Warnings)
	}
	if strings.Contains(strings.Join(res.Warnings, " "), "first-run Docker Desktop quickstart demos") {
		t.Fatalf("did not expect demo-only warning text in production, got %v", res.Warnings)
	}
}

func TestValidateEnvKeepsProductionLoopbackStrictWhenNotQuickstart(t *testing.T) {
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
		"BITRIVER_TRANSCODER_PUBLIC_BASE_URL":     "https://cdn.stream.local/hls",
		"NEXT_PUBLIC_VIEWER_URL":                  "https://viewer.stream.local",
		"NEXT_PUBLIC_API_BASE_URL":                "https://api.stream.local",
		"BITRIVER_LIVE_IMAGE_TAG":                 "1.0.0",
		"BITRIVER_VIEWER_IMAGE_TAG":               "1.0.0",
		"BITRIVER_SRS_CONTROLLER_IMAGE_TAG":       "1.0.0",
		"BITRIVER_TRANSCODER_IMAGE_TAG":           "1.0.0",
		"BITRIVER_SRS_IMAGE_TAG":                  "v5.0.185",
		"BITRIVER_OME_IMAGE_TAG":                  "0.16.0",
		"BITRIVER_LIVE_MODE":                      "production",
		"BITRIVER_LIVE_METRICS_TOKEN":             "metrics-token",
		"BITRIVER_LIVE_RATE_LOGIN_LIMIT":          "10",
	}

	res := validateEnv(values)
	joinedErrors := strings.Join(res.Errors, " ")
	if !strings.Contains(joinedErrors, "BITRIVER_OME_API points at loopback") {
		t.Fatalf("expected strict production loopback error outside quickstart defaults, got %v", res.Errors)
	}
	if strings.Contains(strings.Join(res.Warnings, " "), "expected for first-run Docker Desktop quickstart demos") {
		t.Fatalf("did not expect quickstart-only wording in non-quickstart production warnings, got %v", res.Warnings)
	}
}
func TestValidateEnvAllowsComposeInsecurePostgresDSN(t *testing.T) {
	cert, key := tempTLSFiles(t)
	values := baseEnvValues(cert, key)
	values["BITRIVER_LIVE_POSTGRES_DSN"] = "postgres://user:secret@postgres:5432/bitriver?sslmode=disable"

	res := validateEnv(values)
	for _, err := range res.Errors {
		if strings.Contains(err, "sslmode") {
			t.Fatalf("did not expect sslmode error for local compose DSN, got %v", res.Errors)
		}
	}
}

func TestValidateEnvAcceptsFreshInitDefaults(t *testing.T) {
	generated, _, err := generateEnvValues(map[string]string{})
	if err != nil {
		t.Fatalf("generate env values: %v", err)
	}
	values := baseEnvValues("", "")

	for key, value := range generated {
		values[key] = value
	}

	res := validateEnv(values)
	for _, err := range res.Errors {
		if strings.Contains(err, "BITRIVER_LIVE_MODE") {
			t.Fatalf("did not expect fresh init defaults to fail mode validation, got %v", res.Errors)
		}
	}
	if values["BITRIVER_LIVE_MODE"] != "production" {
		t.Fatalf("expected fresh init defaults to set BITRIVER_LIVE_MODE=production, got %q", values["BITRIVER_LIVE_MODE"])
	}
}

func TestValidateEnvAcceptsFreshInitDefaultsWithPlaceholderMode(t *testing.T) {
	generated, _, err := generateEnvValues(map[string]string{"BITRIVER_LIVE_MODE": "development"})
	if err != nil {
		t.Fatalf("generate env values: %v", err)
	}
	values := baseEnvValues("", "")

	for key, value := range generated {
		values[key] = value
	}

	res := validateEnv(values)
	for _, err := range res.Errors {
		if strings.Contains(err, "BITRIVER_LIVE_MODE") {
			t.Fatalf("did not expect placeholder mode defaults to fail mode validation, got %v", res.Errors)
		}
	}
	if values["BITRIVER_LIVE_MODE"] != "production" {
		t.Fatalf("expected placeholder mode defaults to set BITRIVER_LIVE_MODE=production, got %q", values["BITRIVER_LIVE_MODE"])
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
	if strings.Contains(data, "<Bind>\n        <IP>") || strings.Contains(data, "<Bind>\n        <Address>") {
		t.Fatalf("expected root <Bind> to omit host binding fields, got %q", data)
	}
	if !strings.Contains(data, "<IP>10.1.2.4</IP>") {
		t.Fatalf("expected top-level server IP in output, got %q", data)
	}
	if !strings.Contains(data, "apitoken") {
		t.Fatalf("expected API token in output")
	}
	if !strings.Contains(data, "<!-- Rendered for BITRIVER_OME_IMAGE_TAG=0.16.0 -->") {
		t.Fatalf("expected image tag marker in output")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(out)
		if err != nil {
			t.Fatalf("stat rendered config: %v", err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("expected new secret-bearing OME config mode 0600, got %04o", got)
		}
	}
}

func baseEnvValues(cert, key string) map[string]string {
	return map[string]string{
		"BITRIVER_POSTGRES_USER":                  "brlive_app",
		"BITRIVER_POSTGRES_PASSWORD":              "secret",
		"BITRIVER_REDIS_PASSWORD":                 "secret",
		"BITRIVER_OME_API":                        "http://ome:8081",
		"BITRIVER_OME_BIND":                       "10.0.0.5",
		"BITRIVER_OME_IP":                         "10.0.0.6",
		"BITRIVER_OME_SERVER_PORT":                "9000",
		"BITRIVER_OME_SERVER_TLS_PORT":            "9443",
		"BITRIVER_LIVE_ADMIN_EMAIL":               "admin@stream.local",
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
		"BITRIVER_TRANSCODER_PUBLIC_BASE_URL":     "https://cdn.stream.local/hls",
		"NEXT_PUBLIC_VIEWER_URL":                  "https://viewer.stream.local",
		"BITRIVER_LIVE_IMAGE_TAG":                 "1.0.0",
		"BITRIVER_VIEWER_IMAGE_TAG":               "1.0.0",
		"BITRIVER_SRS_CONTROLLER_IMAGE_TAG":       "1.0.0",
		"BITRIVER_TRANSCODER_IMAGE_TAG":           "1.0.0",
		"BITRIVER_SRS_IMAGE_TAG":                  "v5.0.185",
		"BITRIVER_OME_IMAGE_TAG":                  "0.16.0",
		"BITRIVER_LIVE_MODE":                      "production",
		"BITRIVER_LIVE_METRICS_TOKEN":             "metrics-token",
		"BITRIVER_LIVE_RATE_LOGIN_LIMIT":          "10",
		"BITRIVER_LIVE_TLS_CERT":                  cert,
		"BITRIVER_LIVE_TLS_KEY":                   key,
	}
}

func TestRenderOMEConfigRejectsTestDefaults(t *testing.T) {
	workspace, envPath := setupOMERenderWorkspace(t)

	env := strings.Join([]string{
		"BITRIVER_OME_BIND=10.1.2.3",
		"BITRIVER_OME_SERVER_PORT=9999",
		"BITRIVER_OME_SERVER_TLS_PORT=9443",
		"BITRIVER_OME_USERNAME=ome-test-user",
		"BITRIVER_OME_PASSWORD=ome-test-pass",
		"BITRIVER_OME_API_TOKEN=ome-test-access-token",
		"BITRIVER_OME_ACCESS_TOKEN=ome-test-access-token",
	}, "\n") + "\n"

	if err := os.WriteFile(envPath, []byte(env), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	err := renderOMEFromEnvOutput(envPath, filepath.Join(workspace, "deploy", "ome", "Server.generated.xml"), true, false, true)
	if err == nil {
		t.Fatalf("expected ome-test defaults to be rejected")
	}
	if !strings.Contains(err.Error(), "ome-test") {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(workspace, "deploy", "ome", "Server.generated.xml")); err == nil {
		t.Fatalf("generated config should not be written when ome-test defaults are present")
	}
}

func TestRenderOMEConfigFromEnvWritesSecrets(t *testing.T) {
	workspace, envPath := setupOMERenderWorkspace(t)

	env := strings.Join([]string{
		"BITRIVER_OME_BIND=10.9.0.1",
		"BITRIVER_OME_SERVER_PORT=9999",
		"BITRIVER_OME_SERVER_TLS_PORT=9443",
		"BITRIVER_OME_USERNAME=operator-user",
		"BITRIVER_OME_PASSWORD=operator-pass",
		"BITRIVER_OME_API_TOKEN=operator-api-token",
		"BITRIVER_OME_ACCESS_TOKEN=operator-access-token",
		"BITRIVER_OME_IP=10.9.0.2",
		"BITRIVER_OME_ICE_PORT_RANGE=20000-20009",
		"BITRIVER_OME_TCP_RELAY=25000",
		"BITRIVER_OME_IMAGE_TAG=0.17.1",
	}, "\n") + "\n"

	if err := os.WriteFile(envPath, []byte(env), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	if err := renderOMEFromEnvOutput(envPath, filepath.Join(workspace, "deploy", "ome", "Server.generated.xml"), true, false, true); err != nil {
		t.Fatalf("render config: %v", err)
	}

	data := readFile(t, filepath.Join(workspace, "deploy", "ome", "Server.generated.xml"))
	for _, expected := range []string{"operator-api-token", "<!-- Rendered for BITRIVER_OME_IMAGE_TAG=0.17.1 -->"} {
		if !strings.Contains(data, expected) {
			t.Fatalf("expected %q in generated config, got %q", expected, data)
		}
	}
}

func setupOMERenderWorkspace(t *testing.T) (string, string) {
	t.Helper()

	templateRoot := repoRoot()
	previousRoot := cachedRepoRoot

	workspace := t.TempDir()
	omeDir := filepath.Join(workspace, "deploy", "ome")
	if err := os.MkdirAll(omeDir, 0o755); err != nil {
		t.Fatalf("mkdir ome dir: %v", err)
	}

	template := readFile(t, filepath.Join(templateRoot, "deploy", "ome", "Server.xml"))
	if err := os.WriteFile(filepath.Join(omeDir, "Server.xml"), []byte(template), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	cachedRepoRoot = workspace
	t.Cleanup(func() {
		cachedRepoRoot = previousRoot
	})

	return workspace, filepath.Join(workspace, ".env")
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	return string(data)
}

func containsValue(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func TestValidateEnvRejectsInvalidResourceCPUValue(t *testing.T) {
	cert, key := tempTLSFiles(t)
	values := baseEnvValues(cert, key)
	values["BITRIVER_API_CPUS"] = "zero"

	res := validateEnv(values)
	if !containsString(res.Errors, "BITRIVER_API_CPUS must be a positive decimal CPU value") {
		t.Fatalf("expected invalid cpu error, got %v", res.Errors)
	}
}

func TestValidateEnvRejectsInvalidResourceMemoryValue(t *testing.T) {
	cert, key := tempTLSFiles(t)
	values := baseEnvValues(cert, key)
	values["BITRIVER_API_MEM"] = "abc"

	res := validateEnv(values)
	if !containsString(res.Errors, "BITRIVER_API_MEM must be a Docker Compose memory size") {
		t.Fatalf("expected invalid memory error, got %v", res.Errors)
	}
}

func TestValidateEnvSecretResolutionDirectValueOnly(t *testing.T) {
	cert, key := tempTLSFiles(t)
	values := baseEnvValues(cert, key)
	values["BITRIVER_SRS_TOKEN"] = "direct-token"
	values["BITRIVER_SRS_TOKEN_FILE"] = ""

	res := validateEnv(values)
	if containsValue(res.Missing, "BITRIVER_SRS_TOKEN") {
		t.Fatalf("expected BITRIVER_SRS_TOKEN to be satisfied by direct value, got missing=%v", res.Missing)
	}
	if containsString(res.Errors, "BITRIVER_SRS_TOKEN_FILE") {
		t.Fatalf("did not expect _FILE errors, got %v", res.Errors)
	}
}

func TestValidateEnvSecretResolutionFileOnly(t *testing.T) {
	cert, key := tempTLSFiles(t)
	values := baseEnvValues(cert, key)
	values["BITRIVER_SRS_TOKEN"] = ""

	secretFile := filepath.Join(t.TempDir(), "srs-token")
	if err := os.WriteFile(secretFile, []byte("file-token\n"), 0o600); err != nil {
		t.Fatalf("write secret file: %v", err)
	}
	values["BITRIVER_SRS_TOKEN_FILE"] = secretFile

	res := validateEnv(values)
	if containsValue(res.Missing, "BITRIVER_SRS_TOKEN") {
		t.Fatalf("expected BITRIVER_SRS_TOKEN to resolve from _FILE, got missing=%v", res.Missing)
	}
	if containsString(res.Errors, "BITRIVER_SRS_TOKEN_FILE") {
		t.Fatalf("did not expect _FILE errors, got %v", res.Errors)
	}
}

func TestValidateEnvSecretResolutionDirectValueWinsOverFile(t *testing.T) {
	cert, key := tempTLSFiles(t)
	values := baseEnvValues(cert, key)
	values["BITRIVER_SRS_TOKEN"] = "direct-token"

	secretFile := filepath.Join(t.TempDir(), "srs-token")
	if err := os.WriteFile(secretFile, []byte("file-token\n"), 0o600); err != nil {
		t.Fatalf("write secret file: %v", err)
	}
	values["BITRIVER_SRS_TOKEN_FILE"] = secretFile

	res := validateEnv(values)
	if containsValue(res.Missing, "BITRIVER_SRS_TOKEN") {
		t.Fatalf("expected BITRIVER_SRS_TOKEN to remain satisfied, got missing=%v", res.Missing)
	}
	if !containsString(res.Warnings, "BITRIVER_SRS_TOKEN and BITRIVER_SRS_TOKEN_FILE are both set") {
		t.Fatalf("expected precedence warning when both direct and _FILE are set, got warnings=%v", res.Warnings)
	}
}

func TestValidateEnvSecretResolutionUnreadableFile(t *testing.T) {
	cert, key := tempTLSFiles(t)
	values := baseEnvValues(cert, key)
	values["BITRIVER_SRS_TOKEN"] = ""
	values["BITRIVER_SRS_TOKEN_FILE"] = t.TempDir()

	res := validateEnv(values)
	if !containsValue(res.Missing, "BITRIVER_SRS_TOKEN") {
		t.Fatalf("expected BITRIVER_SRS_TOKEN to be missing when _FILE is unreadable, got missing=%v", res.Missing)
	}
	if !containsString(res.Errors, "BITRIVER_SRS_TOKEN_FILE points to") {
		t.Fatalf("expected _FILE unreadable error, got %v", res.Errors)
	}
}

func TestValidateEnvSecretResolutionEmptyFileContent(t *testing.T) {
	cert, key := tempTLSFiles(t)
	values := baseEnvValues(cert, key)
	values["BITRIVER_SRS_TOKEN"] = ""

	secretFile := filepath.Join(t.TempDir(), "srs-token-empty")
	if err := os.WriteFile(secretFile, []byte("\n\t \n"), 0o600); err != nil {
		t.Fatalf("write empty secret file: %v", err)
	}
	values["BITRIVER_SRS_TOKEN_FILE"] = secretFile

	res := validateEnv(values)
	if !containsValue(res.Missing, "BITRIVER_SRS_TOKEN") {
		t.Fatalf("expected BITRIVER_SRS_TOKEN to be missing when _FILE content is empty, got missing=%v", res.Missing)
	}
	if containsString(res.Errors, "BITRIVER_SRS_TOKEN_FILE") {
		t.Fatalf("did not expect read error for empty _FILE content, got %v", res.Errors)
	}
}

func TestValidateEnvSecretResolutionFileValueStillAppliesBlockedChecks(t *testing.T) {
	cert, key := tempTLSFiles(t)
	values := baseEnvValues(cert, key)
	values["BITRIVER_POSTGRES_PASSWORD"] = ""

	secretFile := filepath.Join(t.TempDir(), "postgres-password")
	if err := os.WriteFile(secretFile, []byte("P0stgres-Example!\n"), 0o600); err != nil {
		t.Fatalf("write secret file: %v", err)
	}
	values["BITRIVER_POSTGRES_PASSWORD_FILE"] = secretFile

	res := validateEnv(values)
	if !containsValue(res.Blocked, "BITRIVER_POSTGRES_PASSWORD") {
		t.Fatalf("expected BITRIVER_POSTGRES_PASSWORD to be blocked when _FILE resolves to sample placeholder, got blocked=%v", res.Blocked)
	}
}

func TestValidateEnvSecretResolutionOptionalKeyFileUnreadable(t *testing.T) {
	cert, key := tempTLSFiles(t)
	values := baseEnvValues(cert, key)
	values["BITRIVER_OME_HEALTHCHECK_TOKEN"] = ""
	values["BITRIVER_OME_HEALTHCHECK_TOKEN_FILE"] = t.TempDir()

	res := validateEnv(values)
	if !containsString(res.Errors, "BITRIVER_OME_HEALTHCHECK_TOKEN_FILE points to") {
		t.Fatalf("expected _FILE unreadable error for optional healthcheck token, got %v", res.Errors)
	}
}
