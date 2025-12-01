package oauth

import (
	"fmt"
	"os"
	"strings"
)

// LoadInput describes how to load OAuth provider configuration from flag values and
// environment variables.
type LoadInput struct {
	// Source is the flag-provided provider configuration (JSON array or path).
	Source string
	// ClientIDs holds flag-provided per-provider client IDs.
	ClientIDs map[string]string
	// ClientSecrets holds flag-provided per-provider client secrets.
	ClientSecrets map[string]string
	// RedirectURLs holds flag-provided per-provider redirect URLs.
	RedirectURLs map[string]string
	// LookupEnv overrides environment lookup for testing.
	LookupEnv func(string) string
}

// LoadFromFlagsAndEnv resolves provider configuration from the provided flag values
// and environment variables. It returns the resolved provider list and an OAuth
// manager constructed from that configuration.
func LoadFromFlagsAndEnv(input LoadInput) ([]ProviderConfig, Service, error) {
	lookupEnv := input.LookupEnv
	if lookupEnv == nil {
		lookupEnv = os.Getenv
	}

	var sources []string
	if source := strings.TrimSpace(input.Source); source != "" {
		sources = append(sources, source)
	}
	if envSource := strings.TrimSpace(lookupEnv("BITRIVER_LIVE_OAUTH_CONFIG")); envSource != "" {
		sources = append(sources, envSource)
	}
	if envSource := strings.TrimSpace(lookupEnv("BITRIVER_LIVE_OAUTH_PROVIDERS")); envSource != "" {
		sources = append(sources, envSource)
	}

	providers, err := ResolveConfigSources(sources...)
	if err != nil {
		return nil, nil, fmt.Errorf("load oauth providers: %w", err)
	}

	if len(providers) == 0 {
		return nil, nil, nil
	}

	providers = OverrideCredentials(providers, input.ClientIDs, input.ClientSecrets, input.RedirectURLs)
	providers = applyEnvOverrides(providers, lookupEnv)
	providers = resolveProviderSet(providers)
	if len(providers) == 0 {
		return nil, nil, nil
	}

	manager, err := NewManager(providers)
	if err != nil {
		return nil, nil, fmt.Errorf("configure oauth: %w", err)
	}

	return providers, manager, nil
}

func applyEnvOverrides(configs []ProviderConfig, lookupEnv func(string) string) []ProviderConfig {
	if len(configs) == 0 {
		return configs
	}

	ids := make(map[string]string)
	secrets := make(map[string]string)
	redirects := make(map[string]string)
	for _, cfg := range configs {
		normalized := sanitizeEnvName(cfg.Name)
		if v := strings.TrimSpace(lookupEnv(fmt.Sprintf("BITRIVER_LIVE_OAUTH_%s_CLIENT_ID", normalized))); v != "" {
			ids[strings.ToLower(cfg.Name)] = v
		}
		if v := strings.TrimSpace(lookupEnv(fmt.Sprintf("BITRIVER_LIVE_OAUTH_%s_CLIENT_SECRET", normalized))); v != "" {
			secrets[strings.ToLower(cfg.Name)] = v
		}
		if v := strings.TrimSpace(lookupEnv(fmt.Sprintf("BITRIVER_LIVE_OAUTH_%s_REDIRECT_URL", normalized))); v != "" {
			redirects[strings.ToLower(cfg.Name)] = v
		}
	}
	return OverrideCredentials(configs, ids, secrets, redirects)
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

func resolveProviderSet(configs []ProviderConfig) []ProviderConfig {
	if len(configs) == 0 {
		return configs
	}

	merged := make(map[string]ProviderConfig)
	order := make([]string, 0, len(configs))
	for _, cfg := range configs {
		key := strings.ToLower(strings.TrimSpace(cfg.Name))
		if key == "" {
			continue
		}
		if _, seen := merged[key]; !seen {
			order = append(order, key)
		}
		merged[key] = cfg
	}
	resolved := make([]ProviderConfig, 0, len(merged))
	for _, key := range order {
		resolved = append(resolved, merged[key])
	}
	return resolved
}
