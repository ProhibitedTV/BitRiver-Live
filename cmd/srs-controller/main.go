// Command srs-controller proxies SRS raw API calls and enforces bearer auth.
package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"bitriver-live/internal/observability/logging"
	"bitriver-live/internal/observability/metrics"
	"bitriver-live/internal/serverutil"
)

const (
	defaultBind     = ":1985"
	defaultUpstream = "http://localhost:1985/api/"
)

type controller struct {
	token   string
	client  *http.Client
	baseURL *url.URL
	logger  *slog.Logger

	mu               sync.Mutex
	lastUpstreamErr  error
	lastUpstreamTime time.Time
	lastSuccessTime  time.Time
}

func main() {
	bind := envOrDefault("SRS_CONTROLLER_BIND", defaultBind)
	logger := logging.WithComponent(logging.Init(logging.Config{Format: string(logging.FormatJSON)}), "srs-controller")
	registry := metrics.NewRegistry()
	upstreamRaw := strings.TrimSpace(os.Getenv("SRS_CONTROLLER_UPSTREAM"))
	if upstreamRaw == "" {
		upstreamRaw = defaultUpstream
	}
	upstream, err := url.Parse(upstreamRaw)
	if err != nil {
		logger.Error("parse upstream URL", "error", err)
		os.Exit(1)
	}
	if upstream.Scheme == "" || upstream.Host == "" {
		logger.Error("SRS_CONTROLLER_UPSTREAM must include scheme and host")
		os.Exit(1)
	}
	if !strings.HasSuffix(upstream.Path, "/") {
		upstream.Path += "/"
	}

	token := strings.TrimSpace(os.Getenv("BITRIVER_SRS_TOKEN"))
	if token == "" {
		logger.Error("BITRIVER_SRS_TOKEN must be set")
		os.Exit(1)
	}

	ctrl := &controller{
		token: token,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		baseURL: upstream,
		logger:  logger,
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", registry.Handler())
	mux.HandleFunc("/healthz", ctrl.healthz)
	proxyHandler := http.HandlerFunc(ctrl.proxyRequest)
	mux.Handle("/v1/channels", proxyHandler)
	mux.Handle("/v1/channels/", proxyHandler)

	handler := http.Handler(mux)
	handler = registry.Middleware(handler)
	handler = logging.RequestLogger(logging.RequestLoggerConfig{Logger: logger})(handler)

	server := &http.Server{
		Addr:              bind,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("srs controller listening", "bind", bind)
	if err := serverutil.Run(ctx, serverutil.Config{Server: server, ShutdownTimeout: 10 * time.Second}); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
	logger.Info("srs controller stopped")
}

func (c *controller) proxyRequest(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/v1/channels") {
		http.NotFound(w, r)
		return
	}
	if !c.authorized(r.Header.Get("Authorization")) {
		if c.logger != nil {
			c.logger.Warn("unauthorized request rejected", "path", r.URL.Path, "remote_addr", r.RemoteAddr)
		}
		w.Header().Set("WWW-Authenticate", "Bearer")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	target := c.resolveTarget(r)
	body, err := readBody(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), bytes.NewReader(body))
	if err != nil {
		http.Error(w, "failed to build upstream request", http.StatusInternalServerError)
		return
	}

	copyHeaders(req.Header, r.Header)
	req.Header.Del("Authorization")

	resp, err := c.client.Do(req)
	if err != nil {
		c.recordUpstreamError(err)
		if c.logger != nil {
			c.logger.Error("upstream request failed", "target", target.String(), "error", err)
		}
		http.Error(w, "upstream request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		c.recordUpstreamError(fmt.Errorf("upstream status %d", resp.StatusCode))
	} else {
		c.recordSuccess()
	}

	copyResponseHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		if c.logger != nil {
			c.logger.Warn("stream upstream body", "error", err)
		}
	}
}

func (c *controller) authorized(header string) bool {
	if header == "" {
		return false
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	token := strings.TrimSpace(header[len(prefix):])
	if token == "" {
		return false
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(c.token)) != 1 {
		return false
	}
	return true
}

func (c *controller) resolveTarget(r *http.Request) *url.URL {
	relPath := strings.TrimPrefix(r.URL.Path, "/")
	relative := &url.URL{Path: relPath, RawQuery: r.URL.RawQuery}
	return c.baseURL.ResolveReference(relative)
}

func (c *controller) recordUpstreamError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastUpstreamErr = err
	c.lastUpstreamTime = time.Now()
}

func (c *controller) recordSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastUpstreamErr = nil
	c.lastSuccessTime = time.Now()
}

func (c *controller) healthz(w http.ResponseWriter, _ *http.Request) {
	c.mu.Lock()
	lastErr := c.lastUpstreamErr
	errTime := c.lastUpstreamTime
	lastSuccess := c.lastSuccessTime
	c.mu.Unlock()

	status := http.StatusOK
	payload := map[string]any{
		"status":      "ok",
		"lastSuccess": lastSuccess,
	}
	if lastErr != nil {
		status = http.StatusServiceUnavailable
		payload["status"] = "degraded"
		payload["upstreamError"] = lastErr.Error()
		payload["upstreamErrorAt"] = errTime
	}

	buf, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buf)
}

func readBody(rc io.ReadCloser) ([]byte, error) {
	if rc == nil {
		return nil, nil
	}
	defer rc.Close()
	body, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		if strings.EqualFold(key, "Host") {
			continue
		}
		dst[key] = append([]string(nil), values...)
	}
}

func copyResponseHeaders(dst, src http.Header) {
	for key, values := range src {
		if shouldSkipHeader(key) {
			continue
		}
		dst[key] = append([]string(nil), values...)
	}
}

func shouldSkipHeader(key string) bool {
	switch strings.ToLower(key) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailer", "transfer-encoding", "upgrade":
		return true
	default:
		return false
	}
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
