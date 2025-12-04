package serverutil

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
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
	Ready           chan<- struct{}
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

	listenConfig := cfg.Server
	ln, err := net.Listen("tcp", listenConfig.Addr)
	if err != nil {
		return err
	}

	var serve func(net.Listener) error
	if cfg.TLS.CertFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLS.CertFile, cfg.TLS.KeyFile)
		if err != nil {
			ln.Close()
			return err
		}

		tlsCfg := cfg.Server.TLSConfig
		if tlsCfg == nil {
			tlsCfg = &tls.Config{}
		} else {
			tlsCfg = tlsCfg.Clone()
		}
		tlsCfg.Certificates = append([]tls.Certificate{cert}, tlsCfg.Certificates...)
		cfg.Server.TLSConfig = tlsCfg
		serve = cfg.Server.Serve
		ln = tls.NewListener(ln, tlsCfg)
	} else {
		serve = cfg.Server.Serve
	}

	if cfg.Ready != nil {
		close(cfg.Ready)
	}

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- serve(ln)
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
