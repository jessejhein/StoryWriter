package main

// main.go starts the production Storywork HTTP server and handles graceful shutdown.

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"storywork/internal/app"
)

const version = "0.0.0-dev"

// main starts the local API server and shuts it down on process signals.
func main() {
	address := os.Getenv("STORYWORK_ADDR")
	if address == "" {
		address = "127.0.0.1:9090"
	}
	server := &http.Server{
		Addr:              address,
		Handler:           app.NewHandler(version),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown: %v", err)
		}
	}()

	log.Printf("storywork API listening on http://%s", address)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
