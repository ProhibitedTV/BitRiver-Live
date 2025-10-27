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
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dataPath := flag.String("data", "data/store.json", "path to JSON datastore")
	flag.Parse()

	store, err := storage.NewStorage(*dataPath)
	if err != nil {
		log.Fatalf("failed to open datastore: %v", err)
	}

	handler := api.NewHandler(store)
	srv := server.New(handler, *addr)

	errs := make(chan error, 1)
	go func() {
		log.Printf("BitRiver Live API listening on %s", *addr)
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
