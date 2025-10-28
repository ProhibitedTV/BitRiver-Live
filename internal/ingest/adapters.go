package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type channelAdapter interface {
	CreateChannel(ctx context.Context, channelID, streamKey string) (primary string, backup string, err error)
	DeleteChannel(ctx context.Context, channelID string) error
}

type applicationAdapter interface {
	CreateApplication(ctx context.Context, channelID string, renditions []string) (originURL, playbackURL string, err error)
	DeleteApplication(ctx context.Context, channelID string) error
}

type transcoderAdapter interface {
	StartJobs(ctx context.Context, channelID, sessionID, originURL string, ladder []Rendition) ([]string, []Rendition, error)
	StopJob(ctx context.Context, jobID string) error
}

type httpChannelAdapter struct {
	baseURL string
	token   string
	client  *http.Client
}

type httpApplicationAdapter struct {
	baseURL  string
	username string
	password string
	client   *http.Client
}

type httpTranscoderAdapter struct {
	baseURL string
	token   string
	client  *http.Client
}

type srsChannelRequest struct {
	ChannelID string `json:"channelId"`
	StreamKey string `json:"streamKey"`
}

type srsChannelResponse struct {
	PrimaryIngest string `json:"primaryIngest"`
	BackupIngest  string `json:"backupIngest"`
}

type omeApplicationRequest struct {
	ChannelID  string   `json:"channelId"`
	Renditions []string `json:"renditions"`
}

type omeApplicationResponse struct {
	OriginURL   string `json:"originUrl"`
	PlaybackURL string `json:"playbackUrl"`
}

type ffmpegJobRequest struct {
	ChannelID  string      `json:"channelId"`
	SessionID  string      `json:"sessionId"`
	OriginURL  string      `json:"originUrl"`
	Renditions []Rendition `json:"renditions"`
}

type ffmpegJobResponse struct {
	JobID      string      `json:"jobId"`
	JobIDs     []string    `json:"jobIds"`
	Renditions []Rendition `json:"renditions"`
}

func newHTTPChannelAdapter(baseURL, token string, client *http.Client) *httpChannelAdapter {
	return &httpChannelAdapter{baseURL: strings.TrimRight(baseURL, "/"), token: token, client: client}
}

func newHTTPApplicationAdapter(baseURL, username, password string, client *http.Client) *httpApplicationAdapter {
	return &httpApplicationAdapter{baseURL: strings.TrimRight(baseURL, "/"), username: username, password: password, client: client}
}

func newHTTPTranscoderAdapter(baseURL, token string, client *http.Client) *httpTranscoderAdapter {
	return &httpTranscoderAdapter{baseURL: strings.TrimRight(baseURL, "/"), token: token, client: client}
}

func (a *httpChannelAdapter) CreateChannel(ctx context.Context, channelID, streamKey string) (string, string, error) {
	payload := srsChannelRequest{ChannelID: channelID, StreamKey: streamKey}
	var response srsChannelResponse
	if err := postJSON(ctx, a.client, fmt.Sprintf("%s/v1/channels", a.baseURL), payload, &response, func(req *http.Request) {
		setBearer(req, a.token)
	}); err != nil {
		return "", "", err
	}
	return response.PrimaryIngest, response.BackupIngest, nil
}

func (a *httpChannelAdapter) DeleteChannel(ctx context.Context, channelID string) error {
	return deleteRequest(ctx, a.client, fmt.Sprintf("%s/v1/channels/%s", a.baseURL, channelID), func(req *http.Request) {
		setBearer(req, a.token)
	})
}

func (a *httpApplicationAdapter) CreateApplication(ctx context.Context, channelID string, renditions []string) (string, string, error) {
	payload := omeApplicationRequest{ChannelID: channelID, Renditions: append([]string{}, renditions...)}
	var response omeApplicationResponse
	if err := postJSON(ctx, a.client, fmt.Sprintf("%s/v1/applications", a.baseURL), payload, &response, func(req *http.Request) {
		req.SetBasicAuth(a.username, a.password)
	}); err != nil {
		return "", "", err
	}
	return response.OriginURL, response.PlaybackURL, nil
}

func (a *httpApplicationAdapter) DeleteApplication(ctx context.Context, channelID string) error {
	return deleteRequest(ctx, a.client, fmt.Sprintf("%s/v1/applications/%s", a.baseURL, channelID), func(req *http.Request) {
		req.SetBasicAuth(a.username, a.password)
	})
}

func (a *httpTranscoderAdapter) StartJobs(ctx context.Context, channelID, sessionID, originURL string, ladder []Rendition) ([]string, []Rendition, error) {
	payload := ffmpegJobRequest{ChannelID: channelID, SessionID: sessionID, OriginURL: originURL, Renditions: cloneRenditions(ladder)}
	var response ffmpegJobResponse
	if err := postJSON(ctx, a.client, fmt.Sprintf("%s/v1/jobs", a.baseURL), payload, &response, func(req *http.Request) {
		setBearer(req, a.token)
	}); err != nil {
		return nil, nil, err
	}
	jobIDs := append([]string{}, response.JobIDs...)
	if response.JobID != "" {
		jobIDs = append(jobIDs, response.JobID)
	}
	renditions := append([]Rendition(nil), response.Renditions...)
	return jobIDs, renditions, nil
}

func (a *httpTranscoderAdapter) StopJob(ctx context.Context, jobID string) error {
	return deleteRequest(ctx, a.client, fmt.Sprintf("%s/v1/jobs/%s", a.baseURL, jobID), func(req *http.Request) {
		setBearer(req, a.token)
	})
}

func postJSON(ctx context.Context, client *http.Client, url string, payload interface{}, dest interface{}, mutate func(*http.Request)) error {
	if client == nil {
		client = &http.Client{}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if mutate != nil {
		mutate(req)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	if dest == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}

func deleteRequest(ctx context.Context, client *http.Client, url string, mutate func(*http.Request)) error {
	if client == nil {
		client = &http.Client{}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	if mutate != nil {
		mutate(req)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	data, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(data)))
}

func setBearer(req *http.Request, token string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
}

func cloneRenditions(input []Rendition) []Rendition {
	if len(input) == 0 {
		return nil
	}
	out := make([]Rendition, len(input))
	copy(out, input)
	return out
}
