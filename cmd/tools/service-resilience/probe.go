package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"
)

type fixtureIdentity struct {
	UserID       string
	ChannelID    string
	ChannelTitle string
	Email        string
	Password     string
}

type apiClient struct {
	baseURL       string
	client        *http.Client
	sessionCookie *http.Cookie
}

type httpObservation struct {
	StatusCode int
	Body       []byte
	Err        error
}

type statusPayload struct {
	Status string `json:"status"`
	Checks []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	} `json:"checks"`
}

func newAPIClient(baseURL string) (*apiClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}
	return &apiClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client: &http.Client{
			Jar:     jar,
			Timeout: 15 * time.Second,
		},
	}, nil
}

func randomHex(byteCount int) (string, error) {
	value := make([]byte, byteCount)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate random value: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func (c *apiClient) createFixture(ctx context.Context) (fixtureIdentity, error) {
	nonce, err := randomHex(12)
	if err != nil {
		return fixtureIdentity{}, err
	}
	fixture := fixtureIdentity{
		ChannelTitle: "Resilience rehearsal " + nonce[:8],
		Email:        "resilience-" + nonce + "@example.invalid",
		Password:     "R3silience!" + nonce,
	}

	signup := map[string]string{
		"displayName": "Resilience Rehearsal",
		"email":       fixture.Email,
		"password":    fixture.Password,
	}
	var auth struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := c.jsonRequest(ctx, http.MethodPost, "/api/auth/signup", signup, http.StatusCreated, &auth); err != nil {
		return fixtureIdentity{}, fmt.Errorf("create rehearsal account: %w", err)
	}
	if auth.User.ID == "" {
		return fixtureIdentity{}, fmt.Errorf("signup response omitted user id")
	}
	fixture.UserID = auth.User.ID

	channelRequest := map[string]any{
		"title":    fixture.ChannelTitle,
		"category": "testing",
		"tags":     []string{"release-gate", "resilience"},
	}
	var channel struct {
		ID      string `json:"id"`
		OwnerID string `json:"ownerId"`
		Title   string `json:"title"`
	}
	if err := c.jsonRequest(ctx, http.MethodPost, "/api/channels", channelRequest, http.StatusCreated, &channel); err != nil {
		return fixtureIdentity{}, fmt.Errorf("create rehearsal channel: %w", err)
	}
	if channel.ID == "" || channel.OwnerID != fixture.UserID || channel.Title != fixture.ChannelTitle {
		return fixtureIdentity{}, fmt.Errorf("created channel identity mismatch")
	}
	fixture.ChannelID = channel.ID
	return fixture, nil
}

func (c *apiClient) verifyDurableState(ctx context.Context, fixture fixtureIdentity) (durableEvidence, error) {
	var me struct {
		User *struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := c.jsonRequest(ctx, http.MethodGet, "/api/viewer/me", nil, http.StatusOK, &me); err != nil {
		return durableEvidence{}, fmt.Errorf("verify persisted session: %w", err)
	}
	sessionPreserved := me.User != nil && me.User.ID == fixture.UserID

	var channel struct {
		ID      string `json:"id"`
		OwnerID string `json:"ownerId"`
		Title   string `json:"title"`
	}
	path := "/api/channels/" + fixture.ChannelID
	if err := c.jsonRequest(ctx, http.MethodGet, path, nil, http.StatusOK, &channel); err != nil {
		return durableEvidence{}, fmt.Errorf("verify persisted channel: %w", err)
	}
	channelPreserved := channel.ID == fixture.ChannelID && channel.OwnerID == fixture.UserID && channel.Title == fixture.ChannelTitle
	evidence := durableEvidence{SessionPreserved: sessionPreserved, ChannelPreserved: channelPreserved}
	if !sessionPreserved || !channelPreserved {
		return evidence, fmt.Errorf("durable fixture identity changed")
	}
	return evidence, nil
}

func (c *apiClient) observe(ctx context.Context, path string) httpObservation {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return httpObservation{Err: err}
	}
	c.attachSession(req)
	resp, err := c.client.Do(req)
	if err != nil {
		return httpObservation{Err: err}
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return httpObservation{StatusCode: resp.StatusCode, Body: body, Err: readErr}
}

func (c *apiClient) jsonRequest(ctx context.Context, method, path string, requestBody any, expectedStatus int, responseBody any) error {
	var body io.Reader
	if requestBody != nil {
		payload, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.attachSession(req)
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", path, err)
	}
	defer resp.Body.Close()
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "bitriver_session" {
			copy := *cookie
			c.sessionCookie = &copy
		}
	}
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read response %s: %w", path, err)
	}
	if resp.StatusCode != expectedStatus {
		return fmt.Errorf("request %s returned HTTP %d", path, resp.StatusCode)
	}
	if responseBody != nil {
		if err := json.Unmarshal(payload, responseBody); err != nil {
			return fmt.Errorf("decode response %s: %w", path, err)
		}
	}
	return nil
}

func (c *apiClient) attachSession(req *http.Request) {
	if c.sessionCookie == nil {
		return
	}
	req.AddCookie(c.sessionCookie)
}

func readyRecovered(observation httpObservation) bool {
	if observation.Err != nil || observation.StatusCode != http.StatusOK {
		return false
	}
	var payload struct {
		Status string `json:"status"`
	}
	return json.Unmarshal(observation.Body, &payload) == nil && payload.Status == "ok"
}

func readyDegraded(observation httpObservation) bool {
	return observation.Err == nil && observation.StatusCode == http.StatusServiceUnavailable
}

func unavailable(observation httpObservation) bool {
	return observation.Err != nil || observation.StatusCode >= http.StatusInternalServerError
}

func pageRecovered(observation httpObservation) bool {
	return observation.Err == nil && observation.StatusCode >= 200 && observation.StatusCode < 400
}

func statusComponent(observation httpObservation, name string, wantReady bool) bool {
	if observation.Err != nil || observation.StatusCode != http.StatusOK {
		return false
	}
	var payload statusPayload
	if err := json.Unmarshal(observation.Body, &payload); err != nil {
		return false
	}
	for _, check := range payload.Checks {
		if check.Name != name {
			continue
		}
		if wantReady {
			return check.Status == "ready"
		}
		return check.Status == "down" || check.Status == "degraded"
	}
	return false
}

func waitFor(ctx context.Context, interval time.Duration, probe func(context.Context) bool) (time.Duration, error) {
	started := time.Now()
	for {
		if probe(ctx) {
			return time.Since(started), nil
		}
		select {
		case <-ctx.Done():
			return time.Since(started), fmt.Errorf("bounded wait expired: %w", ctx.Err())
		case <-time.After(interval):
		}
	}
}
