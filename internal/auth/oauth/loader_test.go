package oauth

import (
	"testing"
)

func TestLoadFromFlagsAndEnv(t *testing.T) {
	lookup := func(values map[string]string) func(string) string {
		return func(key string) string { return values[key] }
	}

	env := map[string]string{
		"BITRIVER_LIVE_OAUTH_CONFIG": `[{
                        "name": "github",
                        "displayName": "GitHub",
                        "authorizeURL": "https://flag/auth",
                        "tokenURL": "https://flag/token",
                        "userInfoURL": "https://flag/user",
                        "clientID": "flag-id",
                        "clientSecret": "flag-secret",
                        "redirectURL": "https://flag/redirect",
                        "profile": {"idField": "id", "emailField": "email", "nameField": "name"}
                }]`,
		"BITRIVER_LIVE_OAUTH_PROVIDERS": `[{
                        "name": "github",
                        "displayName": "GitHub",
                        "authorizeURL": "https://env/auth",
                        "tokenURL": "https://env/token",
                        "userInfoURL": "https://env/user",
                        "clientID": "env-id",
                        "clientSecret": "env-secret",
                        "redirectURL": "https://env/redirect",
                        "profile": {"idField": "id", "emailField": "email", "nameField": "name"}
                }]`,
		"BITRIVER_LIVE_OAUTH_GITHUB_CLIENT_SECRET": "secret-from-env-var",
	}

	providers, manager, err := LoadFromFlagsAndEnv(LoadInput{
		Source: `[{
                        "name": "github",
                        "displayName": "GitHub",
                        "authorizeURL": "https://cli/auth",
                        "tokenURL": "https://cli/token",
                        "userInfoURL": "https://cli/user",
                        "clientID": "cli-id",
                        "clientSecret": "cli-secret",
                        "redirectURL": "https://cli/redirect",
                        "profile": {"idField": "id", "emailField": "email", "nameField": "name"}
                }]`,
		ClientIDs:     map[string]string{"github": "cli-override-id"},
		ClientSecrets: map[string]string{"github": "cli-override-secret"},
		RedirectURLs:  map[string]string{"github": "https://cli/override"},
		LookupEnv:     lookup(env),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("expected one provider, got %d", len(providers))
	}
	provider := providers[0]
	if provider.AuthorizeURL != "https://env/auth" {
		t.Errorf("expected env provider to take precedence, got %s", provider.AuthorizeURL)
	}
	if provider.ClientID != "cli-override-id" {
		t.Errorf("expected cli override id, got %s", provider.ClientID)
	}
	if provider.ClientSecret != "secret-from-env-var" {
		t.Errorf("expected env var secret override, got %s", provider.ClientSecret)
	}
	if provider.RedirectURL != "https://cli/override" {
		t.Errorf("expected cli redirect override, got %s", provider.RedirectURL)
	}
	if manager == nil {
		t.Fatalf("expected manager to be constructed")
	}
	infos := manager.Providers()
	if len(infos) != 1 || infos[0].Name != "github" {
		t.Fatalf("unexpected provider infos: %+v", infos)
	}
}

func TestLoadFromFlagsAndEnvSanitizesProviderNames(t *testing.T) {
	env := map[string]string{
		"BITRIVER_LIVE_OAUTH_GIT_HUB_CLIENT_SECRET": "dash-override",
	}
	providers, _, err := LoadFromFlagsAndEnv(LoadInput{
		Source: `[{
                        "name": "git-hub",
                        "displayName": "GitHub",
                        "authorizeURL": "https://cli/auth",
                        "tokenURL": "https://cli/token",
                        "userInfoURL": "https://cli/user",
                        "clientID": "cli-id",
                        "clientSecret": "cli-secret",
                        "redirectURL": "https://cli/redirect",
                        "profile": {"idField": "id", "emailField": "email", "nameField": "name"}
                }]`,
		LookupEnv: func(key string) string { return env[key] },
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("expected one provider, got %d", len(providers))
	}
	if providers[0].ClientSecret != "dash-override" {
		t.Fatalf("expected sanitized env override, got %s", providers[0].ClientSecret)
	}
}
