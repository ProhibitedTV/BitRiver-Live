package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type job struct {
	ID         string
	ChannelID  string
	SessionID  string
	OriginURL  string
	Renditions []map[string]any
	CreatedAt  time.Time
}

type uploadJob struct {
	ID         string
	ChannelID  string
	UploadID   string
	SourceURL  string
	Filename   string
	Renditions []map[string]any
	Playback   string
	CreatedAt  time.Time
}

type server struct {
	httpServer *http.Server
	token      string
	mu         sync.RWMutex
	jobs       map[string]job
	uploads    map[string]uploadJob
}

type jobRequest struct {
	ChannelID  string          `json:"channelId"`
	SessionID  string          `json:"sessionId"`
	OriginURL  string          `json:"originUrl"`
	Renditions json.RawMessage `json:"renditions"`
}

type jobResponse struct {
	JobID      string          `json:"jobId"`
	JobIDs     []string        `json:"jobIds"`
	Renditions json.RawMessage `json:"renditions"`
}

type uploadRequest struct {
	ChannelID  string          `json:"channelId"`
	UploadID   string          `json:"uploadId"`
	SourceURL  string          `json:"sourceUrl"`
	Filename   string          `json:"filename"`
	Renditions json.RawMessage `json:"renditions"`
}

type uploadResponse struct {
	JobID       string          `json:"jobId"`
	PlaybackURL string          `json:"playbackUrl"`
	Renditions  json.RawMessage `json:"renditions"`
}

func main() {
	bind := envOrDefault("JOB_CONTROLLER_BIND", ":9000")
	token := strings.TrimSpace(os.Getenv("JOB_CONTROLLER_TOKEN"))

	mux := http.NewServeMux()
	srv := &server{
		token:   token,
		jobs:    make(map[string]job),
		uploads: make(map[string]uploadJob),
	}

	mux.HandleFunc("/healthz", srv.handleHealthz)
	mux.HandleFunc("/v1/jobs", srv.handleJobs)
	mux.HandleFunc("/v1/jobs/", srv.handleJobByID)
	mux.HandleFunc("/v1/uploads", srv.handleUploads)

	httpServer := &http.Server{
		Addr:              bind,
		Handler:           logRequests(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	srv.httpServer = httpServer

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("ffmpeg job controller listening on %s", bind)
		if err := httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	log.Println("ffmpeg job controller stopped")
}

func (s *server) authorize(r *http.Request) bool {
	if s.token == "" {
		return true
	}
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return false
	}
	token := strings.TrimSpace(header[7:])
	return token == s.token
}

func (s *server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authorize(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req jobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.ChannelID) == "" || strings.TrimSpace(req.SessionID) == "" || strings.TrimSpace(req.OriginURL) == "" {
		http.Error(w, "channelId, sessionId, and originUrl are required", http.StatusBadRequest)
		return
	}
	jobID := newID("live")

	s.mu.Lock()
	s.jobs[jobID] = job{
		ID:         jobID,
		ChannelID:  req.ChannelID,
		SessionID:  req.SessionID,
		OriginURL:  req.OriginURL,
		Renditions: decodeRenditions(req.Renditions),
		CreatedAt:  time.Now().UTC(),
	}
	s.mu.Unlock()

	resp := jobResponse{
		JobID:      jobID,
		JobIDs:     []string{jobID},
		Renditions: normalizeRenditions(req.Renditions),
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *server) handleJobByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authorize(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/v1/jobs/")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.jobs[id]; !ok {
		http.NotFound(w, r)
		return
	}
	delete(s.jobs, id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleUploads(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authorize(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req uploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.ChannelID) == "" || strings.TrimSpace(req.UploadID) == "" || strings.TrimSpace(req.SourceURL) == "" {
		http.Error(w, "channelId, uploadId, and sourceUrl are required", http.StatusBadRequest)
		return
	}

	jobID := newID("upload")
	playback := fmt.Sprintf("%s/%s/index.m3u8", strings.TrimRight(req.SourceURL, "/"), jobID)

	s.mu.Lock()
	s.uploads[jobID] = uploadJob{
		ID:         jobID,
		ChannelID:  req.ChannelID,
		UploadID:   req.UploadID,
		SourceURL:  req.SourceURL,
		Filename:   req.Filename,
		Renditions: decodeRenditions(req.Renditions),
		Playback:   playback,
		CreatedAt:  time.Now().UTC(),
	}
	s.mu.Unlock()

	resp := uploadResponse{
		JobID:       jobID,
		PlaybackURL: playback,
		Renditions:  normalizeRenditions(req.Renditions),
	}
	writeJSON(w, http.StatusAccepted, resp)
}

func decodeRenditions(raw json.RawMessage) []map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var payload []map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	return payload
}

func normalizeRenditions(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("[]")
	}
	return raw
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(lrw, r)
		log.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, lrw.status, time.Since(start))
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.status = code
	lrw.ResponseWriter.WriteHeader(code)
}

func newID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, rand.Int63())
}

func envOrDefault(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
