package config

import "strings"

// DeployImageSourceConfig stores image source mode defaults for CLI compose flows.
type DeployImageSourceConfig struct {
	Mode string
}

// LoadDeployImageSourceFromEnv resolves BITRIVER_DEPLOY_IMAGE_SOURCE.
func LoadDeployImageSourceFromEnv(env Environment) DeployImageSourceConfig {
	return DeployImageSourceConfig{Mode: strings.ToLower(strings.TrimSpace(env.Get("BITRIVER_DEPLOY_IMAGE_SOURCE")))}
}

// OMEHealthcheckContractConfig stores OME contract values used to validate the generated XML.
type OMEHealthcheckContractConfig struct {
	HTTPPort         string
	HealthcheckToken string
}

// LoadOMEHealthcheckContractFromEnv resolves healthcheck contract defaults.
func LoadOMEHealthcheckContractFromEnv(env Environment) OMEHealthcheckContractConfig {
	httpPort := strings.TrimSpace(env.Get("BITRIVER_OME_HTTP_PORT"))
	if httpPort == "" {
		httpPort = "8081"
	}
	apiToken := strings.TrimSpace(env.Get("BITRIVER_OME_API_TOKEN"))
	healthcheckToken := firstNonEmpty(strings.TrimSpace(env.Get("BITRIVER_OME_HEALTHCHECK_TOKEN")), apiToken)
	return OMEHealthcheckContractConfig{HTTPPort: httpPort, HealthcheckToken: healthcheckToken}
}

// JSONToPostgresMigrationConfig stores environment defaults for migrate-json-to-postgres.
type JSONToPostgresMigrationConfig struct {
	PostgresDSN string
}

// LoadJSONToPostgresMigrationFromEnv resolves DSN fallback order.
func LoadJSONToPostgresMigrationFromEnv(env Environment) JSONToPostgresMigrationConfig {
	dsn := firstNonEmpty(strings.TrimSpace(env.Get("BITRIVER_LIVE_POSTGRES_DSN")), strings.TrimSpace(env.Get("DATABASE_URL")))
	return JSONToPostgresMigrationConfig{PostgresDSN: dsn}
}

// OAuthEnvConfig stores OAuth provider source and per-provider env overrides.
type OAuthEnvConfig struct {
	Sources       []string
	ClientIDs     map[string]string
	ClientSecrets map[string]string
	RedirectURLs  map[string]string
}

// LoadOAuthFromEnv resolves OAuth provider source env values.
func LoadOAuthFromEnv(env Environment) OAuthEnvConfig {
	sources := make([]string, 0, 2)
	if source := strings.TrimSpace(env.Get("BITRIVER_LIVE_OAUTH_CONFIG")); source != "" {
		sources = append(sources, source)
	}
	if source := strings.TrimSpace(env.Get("BITRIVER_LIVE_OAUTH_PROVIDERS")); source != "" {
		sources = append(sources, source)
	}
	return OAuthEnvConfig{Sources: sources}
}

// LoadOAuthProviderOverridesFromEnv resolves per-provider OAuth credential overrides.
func LoadOAuthProviderOverridesFromEnv(env Environment, providerNames []string) OAuthEnvConfig {
	overrides := OAuthEnvConfig{
		ClientIDs:     map[string]string{},
		ClientSecrets: map[string]string{},
		RedirectURLs:  map[string]string{},
	}
	for _, name := range providerNames {
		normalized := sanitizeEnvName(name)
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			continue
		}
		if v := strings.TrimSpace(env.Get("BITRIVER_LIVE_OAUTH_" + normalized + "_CLIENT_ID")); v != "" {
			overrides.ClientIDs[key] = v
		}
		if v := strings.TrimSpace(env.Get("BITRIVER_LIVE_OAUTH_" + normalized + "_CLIENT_SECRET")); v != "" {
			overrides.ClientSecrets[key] = v
		}
		if v := strings.TrimSpace(env.Get("BITRIVER_LIVE_OAUTH_" + normalized + "_REDIRECT_URL")); v != "" {
			overrides.RedirectURLs[key] = v
		}
	}
	return overrides
}

func sanitizeEnvName(name string) string {
	upper := strings.ToUpper(name)
	var builder strings.Builder
	for _, r := range upper {
		switch {
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		default:
			builder.WriteRune('_')
		}
	}
	return builder.String()
}
