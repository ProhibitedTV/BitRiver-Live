package oauth

import (
	"fmt"
	"strings"

	"bitriver-live/internal/config"
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
	// Env snapshots environment variables for OAuth source/override resolution.
	Env config.Environment
}

// LoadFromFlagsAndEnv resolves provider configuration from the provided flag values
// and environment variables. It returns the resolved provider list and an OAuth
// manager constructed from that configuration.
func LoadFromFlagsAndEnv(input LoadInput) ([]ProviderConfig, Service, error) {
	envCfg := config.LoadOAuthFromEnv(input.Env)
	sources := make([]string, 0, len(envCfg.Sources)+1)
	if source := strings.TrimSpace(input.Source); source != "" {
		sources = append(sources, source)
	}
	sources = append(sources, envCfg.Sources...)

	providers, err := ResolveConfigSources(sources...)
	if err != nil {
		return nil, nil, fmt.Errorf("load oauth providers: %w", err)
	}

	if len(providers) == 0 {
		return nil, nil, nil
	}

	providers = OverrideCredentials(providers, input.ClientIDs, input.ClientSecrets, input.RedirectURLs)
	providers = applyEnvOverrides(providers, input.Env)
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

func applyEnvOverrides(configs []ProviderConfig, env config.Environment) []ProviderConfig {
	if len(configs) == 0 {
		return configs
	}

	names := make([]string, 0, len(configs))
	for _, cfg := range configs {
		names = append(names, cfg.Name)
	}
	overrides := config.LoadOAuthProviderOverridesFromEnv(env, names)
	return OverrideCredentials(configs, overrides.ClientIDs, overrides.ClientSecrets, overrides.RedirectURLs)
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
