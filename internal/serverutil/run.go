package serverutil

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// TLSConfig defines certificate and key paths for enabling TLS listeners.
type TLSConfig struct {
	CertFile string
	KeyFile  string
}

// Config controls the HTTP server runtime behaviour.
type Config struct {
	Server          *http.Server
	TLS             TLSConfig
	ShutdownTimeout time.Duration
}

// DefaultShutdownTimeout bounds graceful shutdown when the context is cancelled.
const DefaultShutdownTimeout = 10 * time.Second

// Run starts the provided HTTP server and blocks until it stops. If TLS
// certificate and key files are provided, the server will listen with TLS.
// When the context is cancelled, Run attempts a graceful shutdown bounded by
// ShutdownTimeout.
func Run(ctx context.Context, cfg Config) error {
	if cfg.Server == nil {
		return fmt.Errorf("server is required")
	}

	if (cfg.TLS.CertFile == "") != (cfg.TLS.KeyFile == "") {
		return fmt.Errorf("both TLS cert file and key file must be provided")
	}

	timeout := cfg.ShutdownTimeout
	if timeout <= 0 {
		timeout = DefaultShutdownTimeout
	}

	serveErr := make(chan error, 1)
	go func() {
		var err error
		if cfg.TLS.CertFile != "" {
			err = cfg.Server.ListenAndServeTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile)
		} else {
			err = cfg.Server.ListenAndServe()
		}
		serveErr <- err
	}()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	shutdownErr := cfg.Server.Shutdown(shutdownCtx)

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-shutdownCtx.Done():
		if shutdownErr != nil {
			return shutdownErr
		}
		return shutdownCtx.Err()
	}

	return shutdownErr
}
