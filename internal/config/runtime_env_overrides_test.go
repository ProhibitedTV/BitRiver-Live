package config

import (
	"slices"
	"testing"
)

func TestLoadDeployImageSourceFromEnv(t *testing.T) {
	cfg := LoadDeployImageSourceFromEnv(Environment{values: map[string]string{"BITRIVER_DEPLOY_IMAGE_SOURCE": " Build "}})
	if cfg.Mode != "build" {
		t.Fatalf("mode=%q want build", cfg.Mode)
	}
}

func TestLoadOMEHealthcheckContractFromEnvDefaults(t *testing.T) {
	cfg := LoadOMEHealthcheckContractFromEnv(Environment{values: map[string]string{}})
	if cfg.HTTPPort != "8081" {
		t.Fatalf("HTTPPort=%q want 8081", cfg.HTTPPort)
	}
	if cfg.HealthcheckToken != "" {
		t.Fatalf("HealthcheckToken=%q want empty", cfg.HealthcheckToken)
	}
}

func TestLoadOMEHealthcheckContractFromEnvUsesFallbackToken(t *testing.T) {
	cfg := LoadOMEHealthcheckContractFromEnv(Environment{values: map[string]string{"BITRIVER_OME_API_TOKEN": "api-token"}})
	if cfg.HealthcheckToken != "api-token" {
		t.Fatalf("HealthcheckToken=%q want api-token", cfg.HealthcheckToken)
	}
}

func TestLoadJSONToPostgresMigrationFromEnv(t *testing.T) {
	cfg := LoadJSONToPostgresMigrationFromEnv(Environment{values: map[string]string{"DATABASE_URL": "postgres://fallback"}})
	if cfg.PostgresDSN != "postgres://fallback" {
		t.Fatalf("PostgresDSN=%q want postgres://fallback", cfg.PostgresDSN)
	}
}

func TestLoadOAuthFromEnv(t *testing.T) {
	cfg := LoadOAuthFromEnv(Environment{values: map[string]string{
		"BITRIVER_LIVE_OAUTH_CONFIG":    "config-json",
		"BITRIVER_LIVE_OAUTH_PROVIDERS": "providers-json",
	}})
	if !slices.Equal(cfg.Sources, []string{"config-json", "providers-json"}) {
		t.Fatalf("sources=%v", cfg.Sources)
	}
}

func TestLoadOAuthProviderOverridesFromEnv(t *testing.T) {
	cfg := LoadOAuthProviderOverridesFromEnv(Environment{values: map[string]string{
		"BITRIVER_LIVE_OAUTH_GIT_HUB_CLIENT_SECRET": "secret",
		"BITRIVER_LIVE_OAUTH_GIT_HUB_CLIENT_ID":     "id",
	}}, []string{"git-hub"})
	if cfg.ClientSecrets["git-hub"] != "secret" {
		t.Fatalf("secret override missing: %#v", cfg.ClientSecrets)
	}
	if cfg.ClientIDs["git-hub"] != "id" {
		t.Fatalf("id override missing: %#v", cfg.ClientIDs)
	}
}
