package oauth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProviderConfig describes the configuration for a single OAuth 2.0 provider.
type ProviderConfig struct {
	Name         string            `json:"name"`
	DisplayName  string            `json:"displayName"`
	AuthorizeURL string            `json:"authorizeURL"`
	TokenURL     string            `json:"tokenURL"`
	UserInfoURL  string            `json:"userInfoURL"`
	ClientID     string            `json:"clientID"`
	ClientSecret string            `json:"clientSecret"`
	RedirectURL  string            `json:"redirectURL"`
	Scopes       []string          `json:"scopes"`
	AuthParams   map[string]string `json:"authParams"`
	Profile      ProfileMapping    `json:"profile"`
}

// ProfileMapping defines how to map fields from the provider's userinfo response.
type ProfileMapping struct {
	IDField    string `json:"idField"`
	EmailField string `json:"emailField"`
	NameField  string `json:"nameField"`
}

// ParseProviders decodes the JSON payload into provider configurations. The payload
// may either be a JSON array or an object containing a "providers" array.
func ParseProviders(data []byte) ([]ProviderConfig, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, nil
	}
	if strings.HasPrefix(trimmed, "{") {
		var wrapper struct {
			Providers []ProviderConfig `json:"providers"`
		}
		if err := json.Unmarshal([]byte(trimmed), &wrapper); err != nil {
			return nil, fmt.Errorf("decode oauth providers: %w", err)
		}
		return sanitizeProviders(wrapper.Providers), nil
	}
	var providers []ProviderConfig
	if err := json.Unmarshal([]byte(trimmed), &providers); err != nil {
		return nil, fmt.Errorf("decode oauth providers: %w", err)
	}
	return sanitizeProviders(providers), nil
}

// LoadProviders loads provider configuration from a JSON string or file path.
func LoadProviders(source string) ([]ProviderConfig, error) {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return nil, nil
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return ParseProviders([]byte(trimmed))
	}
	// Attempt to read from disk. If the file cannot be read, treat the value as
	// inline JSON to avoid surprising behaviour.
	content, err := os.ReadFile(trimmed)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Provide a clearer error that includes the attempted path.
			return nil, fmt.Errorf("read oauth provider file %s: %w", trimmed, err)
		}
		// Fall back to parsing the literal value when the path fails to read
		// but resembled JSON.
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			return ParseProviders([]byte(trimmed))
		}
		return nil, fmt.Errorf("read oauth provider config %s: %w", trimmed, err)
	}
	return ParseProviders(content)
}

func sanitizeProviders(items []ProviderConfig) []ProviderConfig {
	sanitized := make([]ProviderConfig, 0, len(items))
	for _, item := range items {
		item.Name = strings.TrimSpace(strings.ToLower(item.Name))
		item.DisplayName = strings.TrimSpace(item.DisplayName)
		item.AuthorizeURL = strings.TrimSpace(item.AuthorizeURL)
		item.TokenURL = strings.TrimSpace(item.TokenURL)
		item.UserInfoURL = strings.TrimSpace(item.UserInfoURL)
		item.ClientID = strings.TrimSpace(item.ClientID)
		item.ClientSecret = strings.TrimSpace(item.ClientSecret)
		item.RedirectURL = strings.TrimSpace(item.RedirectURL)
		if item.AuthParams == nil {
			item.AuthParams = map[string]string{}
		}
		item.Profile.IDField = strings.TrimSpace(item.Profile.IDField)
		item.Profile.EmailField = strings.TrimSpace(item.Profile.EmailField)
		item.Profile.NameField = strings.TrimSpace(item.Profile.NameField)
		scopes := make([]string, 0, len(item.Scopes))
		for _, scope := range item.Scopes {
			trimmed := strings.TrimSpace(scope)
			if trimmed == "" {
				continue
			}
			scopes = append(scopes, trimmed)
		}
		item.Scopes = scopes
		if item.Name != "" {
			sanitized = append(sanitized, item)
		}
	}
	return sanitized
}

// OverrideCredentials applies runtime overrides for client identifiers, secrets,
// and redirect URLs. Keys are matched case-insensitively.
func OverrideCredentials(configs []ProviderConfig, clientIDs, secrets, redirects map[string]string) []ProviderConfig {
	if len(configs) == 0 {
		return configs
	}
	for i := range configs {
		key := configs[i].Name
		if id, ok := lookupOverride(clientIDs, key); ok {
			configs[i].ClientID = id
		}
		if secret, ok := lookupOverride(secrets, key); ok {
			configs[i].ClientSecret = secret
		}
		if redirect, ok := lookupOverride(redirects, key); ok {
			configs[i].RedirectURL = redirect
		}
	}
	return configs
}

func lookupOverride(values map[string]string, key string) (string, bool) {
	if len(values) == 0 {
		return "", false
	}
	normalized := strings.ToLower(strings.TrimSpace(key))
	if normalized == "" {
		return "", false
	}
	if value, ok := values[normalized]; ok {
		return value, true
	}
	return "", false
}

// Validate ensures the provider configuration contains the required fields.
func (cfg ProviderConfig) Validate() error {
	if cfg.Name == "" {
		return errors.New("provider name is required")
	}
	if cfg.DisplayName == "" {
		return fmt.Errorf("displayName required for provider %s", cfg.Name)
	}
	if cfg.AuthorizeURL == "" {
		return fmt.Errorf("authorizeURL required for provider %s", cfg.Name)
	}
	if cfg.TokenURL == "" {
		return fmt.Errorf("tokenURL required for provider %s", cfg.Name)
	}
	if cfg.UserInfoURL == "" {
		return fmt.Errorf("userInfoURL required for provider %s", cfg.Name)
	}
	if cfg.ClientID == "" {
		return fmt.Errorf("clientID required for provider %s", cfg.Name)
	}
	if cfg.ClientSecret == "" {
		return fmt.Errorf("clientSecret required for provider %s", cfg.Name)
	}
	if cfg.RedirectURL == "" {
		return fmt.Errorf("redirectURL required for provider %s", cfg.Name)
	}
	if cfg.Profile.IDField == "" {
		return fmt.Errorf("profile.idField required for provider %s", cfg.Name)
	}
	if cfg.Profile.EmailField == "" {
		return fmt.Errorf("profile.emailField required for provider %s", cfg.Name)
	}
	if cfg.Profile.NameField == "" {
		return fmt.Errorf("profile.nameField required for provider %s", cfg.Name)
	}
	return nil
}

// MustLoadProviders is a helper used in tests to panic when configuration loading
// fails.
func MustLoadProviders(source string) []ProviderConfig {
	providers, err := LoadProviders(source)
	if err != nil {
		panic(err)
	}
	return providers
}

// ResolveConfigSources combines multiple configuration sources, preferring later
// entries when duplicates exist.
func ResolveConfigSources(sources ...string) ([]ProviderConfig, error) {
	var providers []ProviderConfig
	for _, source := range sources {
		trimmed := strings.TrimSpace(source)
		if trimmed == "" {
			continue
		}
		loaded, err := LoadProviders(trimmed)
		if err != nil {
			return nil, err
		}
		providers = append(providers, loaded...)
	}
	return providers, nil
}

// ResolveConfigFromDir reads a default providers.json file from the directory when
// present. This is primarily used by tests.
func ResolveConfigFromDir(dir string) ([]ProviderConfig, error) {
	path := filepath.Join(dir, "providers.json")
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat providers.json: %w", err)
	}
	return LoadProviders(path)
}
