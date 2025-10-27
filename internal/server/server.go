package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"bitriver-live/internal/api"
	"bitriver-live/web"
)

type Server struct {
	httpServer *http.Server
}

func New(handler *api.Handler, addr string) (*Server, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handler.Health)
	mux.HandleFunc("/api/auth/signup", handler.Signup)
	mux.HandleFunc("/api/auth/login", handler.Login)
	mux.HandleFunc("/api/auth/session", handler.Session)
	mux.HandleFunc("/api/users", handler.Users)
	mux.HandleFunc("/api/users/", handler.UserByID)
	mux.HandleFunc("/api/channels", handler.Channels)
	mux.HandleFunc("/api/channels/", handler.ChannelByID)
	mux.HandleFunc("/api/profiles", handler.Profiles)
	mux.HandleFunc("/api/profiles/", handler.ProfileByID)

	staticFS, err := web.Static()
	if err != nil {
		return nil, fmt.Errorf("load web assets: %w", err)
	}
	index, err := fs.ReadFile(staticFS, "index.html")
	if err != nil {
		return nil, fmt.Errorf("read web index: %w", err)
	}
	fileServer := http.FileServer(http.FS(staticFS))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))
	mux.HandleFunc("/", spaHandler(staticFS, index, fileServer))

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           loggingMiddleware(authMiddleware(handler, mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return &Server{httpServer: httpServer}, nil
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

func authMiddleware(handler *api.Handler, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/healthz" || strings.HasPrefix(path, "/api/auth/") || !strings.HasPrefix(path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		user, err := handler.AuthenticateRequest(r)
		if err != nil {
			api.WriteError(w, http.StatusUnauthorized, err)
			return
		}
		ctx := api.ContextWithUser(r.Context(), user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func spaHandler(staticFS fs.FS, index []byte, fileServer http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, fmt.Sprintf("method %s not allowed", r.Method), http.StatusMethodNotAllowed)
			return
		}

		requested := strings.TrimPrefix(r.URL.Path, "/")
		if requested != "" {
			file, err := staticFS.Open(requested)
			if err == nil {
				defer file.Close()
				info, statErr := file.Stat()
				if statErr == nil && !info.IsDir() {
					fileServer.ServeHTTP(w, r)
					return
				}
				if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
					http.Error(w, statErr.Error(), http.StatusInternalServerError)
					return
				}
			} else if !errors.Is(err, fs.ErrNotExist) {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		_, _ = w.Write(index)
	}
}
