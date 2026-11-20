package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vexterAA/url-shortener/internal/core/links"
	"github.com/vexterAA/url-shortener/internal/httpapi"
	"github.com/vexterAA/url-shortener/internal/storage/inmemory"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(logger); err != nil {
		logger.Error("run HTTP server", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	service := links.NewService(inmemory.New())
	server := &http.Server{
		Addr:              ":8080",
		Handler:           httpapi.NewMux(service, logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("listen for HTTP: %w", err)
	}

	logger.Info("HTTP server listening", "addr", listener.Addr().String(), "storage", "in-memory")
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Serve(listener)
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("serve HTTP: %w", err)
	case <-signalCtx.Done():
		stop()
	}

	logger.Info("shutting down HTTP server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shut down HTTP server: %w", err)
	}
	if err := <-serverErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}

	logger.Info("HTTP server stopped")
	return nil
}
