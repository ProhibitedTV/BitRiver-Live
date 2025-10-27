package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"bitriver-live/internal/api"
)

type Server struct {
	httpServer *http.Server
}

func New(handler *api.Handler, addr string) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handler.Health)
	mux.HandleFunc("/api/users", handler.Users)
	mux.HandleFunc("/api/channels", handler.Channels)
	mux.HandleFunc("/api/channels/", handler.ChannelByID)
	mux.HandleFunc("/api/profiles", handler.Profiles)
	mux.HandleFunc("/api/profiles/", handler.ProfileByID)

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           loggingMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return &Server{httpServer: httpServer}
}

func (s *Server) Start() error {
	if s.httpServer == nil {
		return fmt.Errorf("http server is not configured")
	}
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(status int) {
	sr.status = status
	sr.ResponseWriter.WriteHeader(status)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(recorder, r)
		duration := time.Since(start)
		fmt.Printf("%s %s %d %s\n", r.Method, r.URL.Path, recorder.status, duration)
	})
}
