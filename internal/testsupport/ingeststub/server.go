package ingeststub

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

// Options describes how the fake control plane should behave.
type Options struct {
	// PrimaryIngest and BackupIngest are returned from the channel create endpoint.
	PrimaryIngest string
	BackupIngest  string

	// OriginURL and PlaybackURL are returned from the application create endpoint.
	OriginURL   string
	PlaybackURL string

	// LiveJobIDs are returned from the job start endpoint.
	LiveJobIDs []string

	// Renditions are echoed back from the job start endpoint.
	Renditions []map[string]interface{}

	// FailChannelCreates causes the first N channel create requests to return
	// HTTP 503. Subsequent attempts succeed.
	FailChannelCreates int

	// FailJobStarts causes the first N job start requests to return HTTP 502.
	// Subsequent attempts succeed.
	FailJobStarts int

	// Expected tokens/credentials enforced by the stub. If empty, the check is
	// skipped.
	SRSToken        string
	TranscoderToken string
	OMEUser         string
	OMEPassword     string
}

// Operation represents a recorded control-plane interaction.
type Operation struct {
	Kind       string
	ChannelID  string
	StreamKey  string
	SessionID  string
	JobID      string
	Renditions []string
	Ladder     []map[string]interface{}
	Attempt    int
	Status     int
	Timestamp  time.Time
}

// ControlPlane hosts a single httptest.Server that serves all ingest endpoints.
type ControlPlane struct {
	server *httptest.Server
	opts   Options

	mu         sync.Mutex
	operations []Operation
	channelErr int
	jobErr     int
}

// Start spins up a new control-plane stub using the provided options.
func Start(opts Options) *ControlPlane {
	cp := &ControlPlane{opts: opts}
	cp.server = httptest.NewServer(http.HandlerFunc(cp.handle))
	return cp
}

// Close shuts down the underlying HTTP server.
func (c *ControlPlane) Close() {
	if c.server != nil {
		c.server.Close()
	}
}

// BaseURL returns the HTTP base URL for all control-plane endpoints.
func (c *ControlPlane) BaseURL() string {
	return c.server.URL
}

// Operations returns a copy of all recorded operations in the order they occurred.
func (c *ControlPlane) Operations() []Operation {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Operation, len(c.operations))
	copy(out, c.operations)
	return out
}

func (c *ControlPlane) handle(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/channels":
		c.handleCreateChannel(w, r)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/channels/"):
		c.handleDeleteChannel(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/applications":
		c.handleCreateApplication(w, r)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/applications/"):
		c.handleDeleteApplication(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/jobs":
		c.handleStartJobs(w, r)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/jobs/"):
		c.handleStopJob(w, r)
	default:
		http.Error(w, "unexpected request", http.StatusNotFound)
	}
}

func (c *ControlPlane) handleCreateChannel(w http.ResponseWriter, r *http.Request) {
	if !c.expectBearer(w, r, c.opts.SRSToken) {
		return
	}

	type channelRequest struct {
		ChannelID string `json:"channelId"`
		StreamKey string `json:"streamKey"`
	}
	type channelResponse struct {
		PrimaryIngest string `json:"primaryIngest"`
		BackupIngest  string `json:"backupIngest"`
	}

	var req channelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	c.mu.Lock()
	c.channelErr++
	attempt := c.channelErr
	c.mu.Unlock()

	op := Operation{
		Kind:      "channel-create",
		ChannelID: req.ChannelID,
		StreamKey: req.StreamKey,
		Attempt:   attempt,
		Status:    http.StatusOK,
		Timestamp: time.Now(),
	}

	if attempt <= c.opts.FailChannelCreates {
		op.Status = http.StatusServiceUnavailable
		c.record(op)
		http.Error(w, "srs unavailable", http.StatusServiceUnavailable)
		return
	}

	c.record(op)

	resp := channelResponse{PrimaryIngest: c.opts.PrimaryIngest, BackupIngest: c.opts.BackupIngest}
	if resp.PrimaryIngest == "" {
		resp.PrimaryIngest = "rtmp://primary"
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (c *ControlPlane) handleDeleteChannel(w http.ResponseWriter, r *http.Request) {
	if !c.expectBearer(w, r, c.opts.SRSToken) {
		return
	}
	channelID := strings.TrimPrefix(r.URL.Path, "/v1/channels/")
	c.record(Operation{Kind: "channel-delete", ChannelID: channelID, Status: http.StatusNoContent})
	w.WriteHeader(http.StatusNoContent)
}

func (c *ControlPlane) handleCreateApplication(w http.ResponseWriter, r *http.Request) {
	if !c.expectBasic(w, r, c.opts.OMEUser, c.opts.OMEPassword) {
		return
	}
	type appRequest struct {
		ChannelID  string   `json:"channelId"`
		Renditions []string `json:"renditions"`
	}
	type appResponse struct {
		OriginURL   string `json:"originUrl"`
		PlaybackURL string `json:"playbackUrl"`
	}

	var req appRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	c.record(Operation{
		Kind:       "application-create",
		ChannelID:  req.ChannelID,
		Renditions: append([]string{}, req.Renditions...),
		Status:     http.StatusOK,
	})

	resp := appResponse{OriginURL: c.opts.OriginURL, PlaybackURL: c.opts.PlaybackURL}
	if resp.OriginURL == "" {
		resp.OriginURL = "http://origin"
	}
	if resp.PlaybackURL == "" {
		resp.PlaybackURL = "https://playback"
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (c *ControlPlane) handleDeleteApplication(w http.ResponseWriter, r *http.Request) {
	if !c.expectBasic(w, r, c.opts.OMEUser, c.opts.OMEPassword) {
		return
	}
	channelID := strings.TrimPrefix(r.URL.Path, "/v1/applications/")
	c.record(Operation{Kind: "application-delete", ChannelID: channelID, Status: http.StatusNoContent})
	w.WriteHeader(http.StatusNoContent)
}

func (c *ControlPlane) handleStartJobs(w http.ResponseWriter, r *http.Request) {
	if !c.expectBearer(w, r, c.opts.TranscoderToken) {
		return
	}
	type jobRequest struct {
		ChannelID  string                   `json:"channelId"`
		SessionID  string                   `json:"sessionId"`
		OriginURL  string                   `json:"originUrl"`
		Renditions []map[string]interface{} `json:"renditions"`
	}
	type jobResponse struct {
		JobIDs     []string                 `json:"jobIds"`
		Renditions []map[string]interface{} `json:"renditions"`
	}

	var req jobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	c.mu.Lock()
	c.jobErr++
	attempt := c.jobErr
	c.mu.Unlock()

	op := Operation{
		Kind:      "job-start",
		ChannelID: req.ChannelID,
		SessionID: req.SessionID,
		Ladder:    cloneAnyRenditions(req.Renditions),
		Attempt:   attempt,
		Status:    http.StatusOK,
		Timestamp: time.Now(),
	}

	if attempt <= c.opts.FailJobStarts {
		op.Status = http.StatusBadGateway
		c.record(op)
		http.Error(w, "transcoder offline", http.StatusBadGateway)
		return
	}

	c.record(op)

	renditions := req.Renditions
	if len(c.opts.Renditions) > 0 {
		renditions = c.opts.Renditions
	}

	resp := jobResponse{JobIDs: c.opts.LiveJobIDs, Renditions: renditions}
	if len(resp.JobIDs) == 0 {
		resp.JobIDs = []string{"job-live-1"}
	}

	_ = json.NewEncoder(w).Encode(resp)
}

func (c *ControlPlane) handleStopJob(w http.ResponseWriter, r *http.Request) {
	if !c.expectBearer(w, r, c.opts.TranscoderToken) {
		return
	}
	jobID := strings.TrimPrefix(r.URL.Path, "/v1/jobs/")
	c.record(Operation{Kind: "job-stop", JobID: jobID, Status: http.StatusNoContent})
	w.WriteHeader(http.StatusNoContent)
}

func (c *ControlPlane) record(op Operation) {
	if op.Timestamp.IsZero() {
		op.Timestamp = time.Now()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.operations = append(c.operations, op)
}

func (c *ControlPlane) expectBearer(w http.ResponseWriter, r *http.Request, expected string) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return true
	}
	if got := r.Header.Get("Authorization"); got != fmt.Sprintf("Bearer %s", expected) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func (c *ControlPlane) expectBasic(w http.ResponseWriter, r *http.Request, user, pass string) bool {
	user = strings.TrimSpace(user)
	pass = strings.TrimSpace(pass)
	if user == "" && pass == "" {
		return true
	}
	gotUser, gotPass, ok := r.BasicAuth()
	if !ok || gotUser != user || gotPass != pass {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func cloneAnyRenditions(input []map[string]interface{}) []map[string]interface{} {
	if len(input) == 0 {
		return nil
	}
	out := make([]map[string]interface{}, len(input))
	for i, r := range input {
		next := make(map[string]interface{}, len(r))
		for k, v := range r {
			next[k] = v
		}
		out[i] = next
	}
	return out
}
