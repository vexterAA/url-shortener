package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/vexterAA/url-shortener/internal/core/links"
	"github.com/vexterAA/url-shortener/internal/httpapi"
	"github.com/vexterAA/url-shortener/internal/storage/inmemory"
	"github.com/vexterAA/url-shortener/internal/storage/postgres"
)

func main() {
	storage := flag.String("storage", "memory", "storage type (memory or postgres)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(logger, *storage); err != nil {
		logger.Error("run HTTP server", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger, storage string) error {
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var repo links.Repository
	switch storage {
	case "memory":
		repo = inmemory.New()
	case "postgres":
		databaseURL := os.Getenv("DATABASE_URL")
		if databaseURL == "" {
			return errors.New("DATABASE_URL is required when -storage=postgres")
		}
		config, err := pgx.ParseConfig(databaseURL)
		if err != nil {
			return errors.New("invalid DATABASE_URL")
		}
		db := stdlib.OpenDB(*config)
		defer db.Close()

		pingCtx, cancel := context.WithTimeout(signalCtx, 5*time.Second)
		err = db.PingContext(pingCtx)
		cancel()
		if err != nil {
			return fmt.Errorf("ping postgres: %w", err)
		}
		repo = postgres.New(db)
	default:
		return fmt.Errorf("unsupported storage %q, use memory or postgres", storage)
	}

	service := links.NewService(repo)
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

	logger.Info("HTTP server listening", "addr", listener.Addr().String(), "storage", storage)
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
