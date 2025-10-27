package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bitriver-live/internal/api"
	"bitriver-live/internal/server"
	"bitriver-live/internal/storage"
)

func main() {
	addr := flag.String("addr", "", "HTTP listen address")
	dataPath := flag.String("data", "", "path to JSON datastore")
	flag.Parse()

	listenAddr := *addr
	if listenAddr == "" {
		listenAddr = os.Getenv("BITRIVER_LIVE_ADDR")
		if listenAddr == "" {
			listenAddr = ":8080"
		}
	}

	path := *dataPath
	if path == "" {
		path = os.Getenv("BITRIVER_LIVE_DATA")
		if path == "" {
			path = "data/store.json"
		}
	}

	store, err := storage.NewStorage(path)
	if err != nil {
		log.Fatalf("failed to open datastore: %v", err)
	}

	handler := api.NewHandler(store)
	srv, err := server.New(handler, listenAddr)
	if err != nil {
		log.Fatalf("failed to initialise server: %v", err)
	}

	errs := make(chan error, 1)
	go func() {
		log.Printf("BitRiver Live API listening on %s", listenAddr)
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Printf("received signal %s, shutting down", sig)
	case err := <-errs:
		log.Printf("server error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}

	log.Println("server stopped")
}
