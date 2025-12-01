// Command srs-controller proxies SRS raw API calls and enforces bearer auth.
package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	defaultBind     = ":1985"
	defaultUpstream = "http://localhost:1985/api/"
)

type controller struct {
	token   string
	client  *http.Client
	baseURL *url.URL

	mu               sync.Mutex
	lastUpstreamErr  error
	lastUpstreamTime time.Time
	lastSuccessTime  time.Time
}

func main() {
	bind := envOrDefault("SRS_CONTROLLER_BIND", defaultBind)
	upstreamRaw := strings.TrimSpace(os.Getenv("SRS_CONTROLLER_UPSTREAM"))
	if upstreamRaw == "" {
		upstreamRaw = defaultUpstream
	}
	upstream, err := url.Parse(upstreamRaw)
	if err != nil {
		log.Fatalf("parse upstream URL: %v", err)
	}
	if upstream.Scheme == "" || upstream.Host == "" {
		log.Fatalf("SRS_CONTROLLER_UPSTREAM must include scheme and host")
	}
	if !strings.HasSuffix(upstream.Path, "/") {
		upstream.Path += "/"
	}

	token := strings.TrimSpace(os.Getenv("BITRIVER_SRS_TOKEN"))
	if token == "" {
		log.Fatal("BITRIVER_SRS_TOKEN must be set")
	}

	ctrl := &controller{
		token: token,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		baseURL: upstream,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", ctrl.healthz)
	proxyHandler := http.HandlerFunc(ctrl.proxyRequest)
	mux.Handle("/v1/channels", proxyHandler)
	mux.Handle("/v1/channels/", proxyHandler)

	server := &http.Server{
		Addr:              bind,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("srs controller listening on %s", bind)
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	log.Println("srs controller stopped")
}

func (c *controller) proxyRequest(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/v1/channels") {
		http.NotFound(w, r)
		return
	}
	if !c.authorized(r.Header.Get("Authorization")) {
		log.Printf("unauthorized request rejected: path=%s remote=%s", r.URL.Path, r.RemoteAddr)
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
		log.Printf("upstream request failed: target=%s err=%v", target.String(), err)
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
		log.Printf("stream upstream body: %v", err)
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
