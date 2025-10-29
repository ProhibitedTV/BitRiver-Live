package oauth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// ErrProviderNotConfigured is returned when an OAuth flow is requested for an
// unknown provider.
var ErrProviderNotConfigured = errors.New("oauth provider not configured")

// ErrStateInvalid is returned when the state parameter is missing or expired.
var ErrStateInvalid = errors.New("oauth state invalid or expired")

// Service exposes the operations required by the HTTP handlers to drive an
// OAuth 2.0 authorisation code flow.
type Service interface {
	Providers() []ProviderInfo
	Begin(provider, returnTo string) (BeginResult, error)
	Complete(provider, state, code string) (Completion, error)
	Cancel(state string) (string, error)
}

// ProviderInfo is a lightweight description of a configured provider.
type ProviderInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

// BeginResult is returned when an authorisation request is constructed.
type BeginResult struct {
	URL   string
	State string
}

// Completion contains the outcome of a successful OAuth flow.
type Completion struct {
	Profile  UserProfile
	ReturnTo string
}

// UserProfile captures the identity data returned by the provider.
type UserProfile struct {
	Provider    string
	Subject     string
	Email       string
	DisplayName string
	Raw         map[string]any
}

// Manager coordinates OAuth flows for a set of providers.
type Manager struct {
	providers map[string]provider
	state     StateStore
	client    *http.Client
	stateTTL  time.Duration
}

type provider struct {
	config ProviderConfig
}

// Option customises the OAuth manager.
type Option func(*Manager)

// WithStateStore injects a custom state store.
func WithStateStore(store StateStore) Option {
	return func(m *Manager) {
		if store != nil {
			m.state = store
		}
	}
}

// WithHTTPClient overrides the HTTP client used for token exchanges.
func WithHTTPClient(client *http.Client) Option {
	return func(m *Manager) {
		if client != nil {
			m.client = client
		}
	}
}

// WithStateTTL adjusts how long state parameters remain valid.
func WithStateTTL(ttl time.Duration) Option {
	return func(m *Manager) {
		if ttl > 0 {
			m.stateTTL = ttl
		}
	}
}

// NewManager constructs an OAuth manager for the provided configuration.
func NewManager(configs []ProviderConfig, opts ...Option) (*Manager, error) {
	mgr := &Manager{
		providers: make(map[string]provider),
		state:     NewMemoryStateStore(),
		client:    &http.Client{Timeout: 10 * time.Second},
		stateTTL:  10 * time.Minute,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(mgr)
		}
	}
	for _, cfg := range configs {
		if err := cfg.Validate(); err != nil {
			return nil, err
		}
		key := strings.ToLower(cfg.Name)
		mgr.providers[key] = provider{config: cfg}
	}
	return mgr, nil
}

// Providers lists the configured providers.
func (m *Manager) Providers() []ProviderInfo {
	infos := make([]ProviderInfo, 0, len(m.providers))
	for _, item := range m.providers {
		infos = append(infos, ProviderInfo{Name: item.config.Name, DisplayName: item.config.DisplayName})
	}
	sortProviders(infos)
	return infos
}

func sortProviders(items []ProviderInfo) {
	if len(items) < 2 {
		return
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].DisplayName == items[j].DisplayName {
			return items[i].Name < items[j].Name
		}
		return items[i].DisplayName < items[j].DisplayName
	})
}

// Begin initialises an OAuth flow for the selected provider.
func (m *Manager) Begin(name, returnTo string) (BeginResult, error) {
	provider, ok := m.providers[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return BeginResult{}, ErrProviderNotConfigured
	}
	state, err := GenerateState()
	if err != nil {
		return BeginResult{}, err
	}
	if err := m.state.Put(state, StateData{Provider: provider.config.Name, ReturnTo: returnTo}, m.stateTTL); err != nil {
		return BeginResult{}, err
	}
	authURL, err := buildAuthorizeURL(provider.config, state)
	if err != nil {
		return BeginResult{}, err
	}
	return BeginResult{URL: authURL, State: state}, nil
}

// Complete exchanges the authorisation code and returns the provider profile.
func (m *Manager) Complete(name, state, code string) (Completion, error) {
	provider, ok := m.providers[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return Completion{}, ErrProviderNotConfigured
	}
	state = strings.TrimSpace(state)
	if state == "" {
		return Completion{}, ErrStateInvalid
	}
	data, ok := m.state.Take(state)
	if !ok {
		return Completion{}, ErrStateInvalid
	}
	if !strings.EqualFold(data.Provider, provider.config.Name) {
		return Completion{ReturnTo: data.ReturnTo}, ErrStateInvalid
	}
	completion := Completion{ReturnTo: data.ReturnTo}
	token, err := m.exchangeCode(provider.config, code)
	if err != nil {
		return completion, err
	}
	profile, err := m.fetchUserInfo(provider.config, token)
	if err != nil {
		return completion, err
	}
	completion.Profile = profile
	return completion, nil
}

// Cancel invalidates the provided state token and returns the saved return URL.
func (m *Manager) Cancel(state string) (string, error) {
	state = strings.TrimSpace(state)
	if state == "" {
		return "", ErrStateInvalid
	}
	data, ok := m.state.Take(state)
	if !ok {
		return "", ErrStateInvalid
	}
	return data.ReturnTo, nil
}

func buildAuthorizeURL(cfg ProviderConfig, state string) (string, error) {
	parsed, err := url.Parse(cfg.AuthorizeURL)
	if err != nil {
		return "", fmt.Errorf("parse authorize url: %w", err)
	}
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", cfg.ClientID)
	query.Set("redirect_uri", cfg.RedirectURL)
	if len(cfg.Scopes) > 0 {
		query.Set("scope", strings.Join(cfg.Scopes, " "))
	}
	query.Set("state", state)
	for key, value := range cfg.AuthParams {
		query.Set(key, value)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

type tokenResponse struct {
	AccessToken string
	TokenType   string
	IDToken     string
	Raw         map[string]any
}

func (m *Manager) exchangeCode(cfg ProviderConfig, code string) (tokenResponse, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return tokenResponse{}, fmt.Errorf("authorization code is required")
	}
	payload := url.Values{}
	payload.Set("grant_type", "authorization_code")
	payload.Set("code", code)
	payload.Set("redirect_uri", cfg.RedirectURL)
	payload.Set("client_id", cfg.ClientID)
	payload.Set("client_secret", cfg.ClientSecret)

	request, err := http.NewRequest(http.MethodPost, cfg.TokenURL, strings.NewReader(payload.Encode()))
	if err != nil {
		return tokenResponse{}, fmt.Errorf("create token request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")

	response, err := m.client.Do(request)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("exchange token: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("read token response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		snippet := string(bytes.TrimSpace(body))
		if len(snippet) > 512 {
			snippet = snippet[:512]
		}
		return tokenResponse{}, fmt.Errorf("token exchange failed: %s", snippet)
	}
	token, err := parseTokenResponse(body)
	if err != nil {
		return tokenResponse{}, err
	}
	if token.AccessToken == "" {
		return tokenResponse{}, fmt.Errorf("token response missing access_token")
	}
	return token, nil
}

func parseTokenResponse(body []byte) (tokenResponse, error) {
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err == nil {
		token := tokenResponse{Raw: parsed}
		token.AccessToken = stringFromAny(parsed["access_token"])
		token.TokenType = stringFromAny(parsed["token_type"])
		token.IDToken = stringFromAny(parsed["id_token"])
		return token, nil
	}
	// Some providers return x-www-form-urlencoded payloads.
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return tokenResponse{}, fmt.Errorf("parse token response: %w", err)
	}
	token := tokenResponse{Raw: map[string]any{}}
	for key, vals := range values {
		if len(vals) == 0 {
			continue
		}
		token.Raw[key] = vals[0]
	}
	token.AccessToken = values.Get("access_token")
	token.TokenType = values.Get("token_type")
	token.IDToken = values.Get("id_token")
	return token, nil
}

func (m *Manager) fetchUserInfo(cfg ProviderConfig, token tokenResponse) (UserProfile, error) {
	request, err := http.NewRequest(http.MethodGet, cfg.UserInfoURL, nil)
	if err != nil {
		return UserProfile{}, fmt.Errorf("create userinfo request: %w", err)
	}
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	request.Header.Set("Accept", "application/json")

	response, err := m.client.Do(request)
	if err != nil {
		return UserProfile{}, fmt.Errorf("fetch userinfo: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return UserProfile{}, fmt.Errorf("read userinfo response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		snippet := string(bytes.TrimSpace(body))
		if len(snippet) > 512 {
			snippet = snippet[:512]
		}
		return UserProfile{}, fmt.Errorf("userinfo request failed: %s", snippet)
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return UserProfile{}, fmt.Errorf("decode userinfo response: %w", err)
	}
	profile := UserProfile{Provider: cfg.Name, Raw: parsed}
	subject, err := lookupProfileValue(parsed, cfg.Profile.IDField)
	if err != nil {
		return UserProfile{}, err
	}
	profile.Subject = subject
	if email, err := lookupProfileValue(parsed, cfg.Profile.EmailField); err == nil {
		profile.Email = email
	}
	if name, err := lookupProfileValue(parsed, cfg.Profile.NameField); err == nil {
		profile.DisplayName = name
	}
	return profile, nil
}

func lookupProfileValue(data map[string]any, path string) (string, error) {
	parts := strings.Split(path, ".")
	var current any = data
	for _, part := range parts {
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[part]
			if !ok {
				return "", fmt.Errorf("profile field %s missing", path)
			}
			current = next
		default:
			return "", fmt.Errorf("profile field %s missing", path)
		}
	}
	return stringFromAny(current), nil
}

func stringFromAny(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case json.Number:
		return v.String()
	case float64:
		return strings.TrimSuffix(strings.TrimSuffix(fmt.Sprintf("%f", v), "0"), ".")
	case int, int32, int64:
		return fmt.Sprintf("%d", v)
	case uint, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}
