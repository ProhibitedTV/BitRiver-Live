package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type UploadSourceStorageConfig struct {
	Endpoint       string
	Bucket         string
	Prefix         string
	PublicEndpoint string
	UseSSL         bool
	RequestTimeout time.Duration
}

type storedUploadSource struct {
	Key         string
	PublicURL   string
	ContentType string
	Body        []byte
	ModTime     time.Time
}

type uploadSourceStorage interface {
	Enabled() bool
	Store(ctx context.Context, uploadID, filename, contentType string, body []byte) (storedUploadSource, error)
	Get(ctx context.Context, key string) (storedUploadSource, error)
	Delete(ctx context.Context, key string) error
}

type noopUploadSourceStorage struct{}

func (noopUploadSourceStorage) Enabled() bool { return false }

func (noopUploadSourceStorage) Store(context.Context, string, string, string, []byte) (storedUploadSource, error) {
	return storedUploadSource{}, nil
}

func (noopUploadSourceStorage) Get(context.Context, string) (storedUploadSource, error) {
	return storedUploadSource{}, fmt.Errorf("upload source storage disabled")
}

func (noopUploadSourceStorage) Delete(context.Context, string) error { return nil }

type objectUploadSourceStorage struct {
	cfg        UploadSourceStorageConfig
	endpoint   *url.URL
	httpClient *http.Client
}

func newUploadSourceStorage(cfg UploadSourceStorageConfig) uploadSourceStorage {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	bucket := strings.TrimSpace(cfg.Bucket)
	if endpoint == "" || bucket == "" {
		return noopUploadSourceStorage{}
	}
	scheme := "http"
	if cfg.UseSSL {
		scheme = "https"
	}
	if strings.Contains(endpoint, "://") {
		if parsed, err := url.Parse(endpoint); err == nil {
			if parsed.Host != "" {
				endpoint = parsed.Host
			}
		}
	}
	baseURL := &url.URL{Scheme: scheme, Host: endpoint}
	if baseURL.Host == "" {
		return noopUploadSourceStorage{}
	}
	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	cfg.Bucket = bucket
	return &objectUploadSourceStorage{cfg: cfg, endpoint: baseURL, httpClient: &http.Client{Timeout: timeout}}
}

func (s *objectUploadSourceStorage) Enabled() bool { return true }

func (s *objectUploadSourceStorage) Store(ctx context.Context, uploadID, filename, contentType string, body []byte) (storedUploadSource, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		ext = ".bin"
	}
	key := strings.TrimLeft(strings.TrimSpace(uploadID+ext), "/")
	finalKey := s.applyPrefix(key)
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, s.objectURL(finalKey).String(), bytes.NewReader(body))
	if err != nil {
		return storedUploadSource{}, fmt.Errorf("create upload source put request: %w", err)
	}
	if strings.TrimSpace(contentType) != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := s.httpClient.Do(request)
	if err != nil {
		return storedUploadSource{}, fmt.Errorf("store upload source %s: %w", finalKey, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return storedUploadSource{}, fmt.Errorf("store upload source %s: unexpected status %d", finalKey, response.StatusCode)
	}
	return storedUploadSource{Key: finalKey, PublicURL: s.publicURL(finalKey), ContentType: contentType}, nil
}

func (s *objectUploadSourceStorage) Get(ctx context.Context, key string) (storedUploadSource, error) {
	finalKey := s.applyPrefix(key)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.objectURL(finalKey).String(), nil)
	if err != nil {
		return storedUploadSource{}, fmt.Errorf("create upload source get request: %w", err)
	}
	response, err := s.httpClient.Do(request)
	if err != nil {
		return storedUploadSource{}, fmt.Errorf("get upload source %s: %w", finalKey, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return storedUploadSource{}, fmt.Errorf("get upload source %s: unexpected status %d", finalKey, response.StatusCode)
	}
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return storedUploadSource{}, fmt.Errorf("read upload source %s: %w", finalKey, err)
	}
	return storedUploadSource{Key: finalKey, ContentType: response.Header.Get("Content-Type"), Body: payload, ModTime: time.Now().UTC()}, nil
}

func (s *objectUploadSourceStorage) Delete(ctx context.Context, key string) error {
	finalKey := s.applyPrefix(key)
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.objectURL(finalKey).String(), nil)
	if err != nil {
		return fmt.Errorf("create upload source delete request: %w", err)
	}
	response, err := s.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("delete upload source %s: %w", finalKey, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode == http.StatusNotFound {
		return nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("delete upload source %s: unexpected status %d", finalKey, response.StatusCode)
	}
	return nil
}

func (s *objectUploadSourceStorage) applyPrefix(key string) string {
	trimmed := strings.TrimLeft(strings.TrimSpace(key), "/")
	prefix := strings.Trim(strings.TrimSpace(s.cfg.Prefix), "/")
	if prefix == "" {
		return trimmed
	}
	if trimmed == "" {
		return prefix
	}
	if trimmed == prefix || strings.HasPrefix(trimmed, prefix+"/") {
		return trimmed
	}
	return prefix + "/" + trimmed
}

func (s *objectUploadSourceStorage) objectURL(key string) *url.URL {
	path := "/" + strings.TrimLeft(s.cfg.Bucket, "/")
	trimmedKey := strings.TrimLeft(key, "/")
	if trimmedKey != "" {
		path += "/" + trimmedKey
	}
	u := *s.endpoint
	u.Path = path
	return &u
}

func (s *objectUploadSourceStorage) publicURL(key string) string {
	base := strings.TrimSpace(s.cfg.PublicEndpoint)
	if base == "" {
		return ""
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(key, "/")
}

func readUploadMediaFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
