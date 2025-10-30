package chat_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bitriver-live/internal/chat"
)

func TestDialWS(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := chat.Accept(w, r)
		if err != nil {
			t.Fatalf("Accept: %v", err)
		}
		defer conn.Close()

		if err := conn.WriteText([]byte("hello")); err != nil {
			t.Fatalf("WriteText: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, err := chat.Dial(context.Background(), wsURL, http.Header{}, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})

	message, err := conn.ReadMessage(context.Background())
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if string(message) != "hello" {
		t.Fatalf("unexpected message %q", message)
	}
}

func TestDialWSS(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := chat.Accept(w, r)
		if err != nil {
			t.Fatalf("Accept: %v", err)
		}
		defer conn.Close()

		if err := conn.WriteText([]byte("secure")); err != nil {
			t.Fatalf("WriteText: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())

	tlsConfig := &tls.Config{RootCAs: pool}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	wssURL := "wss" + strings.TrimPrefix(server.URL, "https")
	conn, err := chat.Dial(ctx, wssURL, http.Header{}, tlsConfig)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})

	message, err := conn.ReadMessage(context.Background())
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if string(message) != "secure" {
		t.Fatalf("unexpected message %q", message)
	}
}
