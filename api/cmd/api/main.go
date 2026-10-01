package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rifqif16/posq/api/internal/app"
	"github.com/rifqif16/posq/api/internal/platform/config"
	"github.com/rifqif16/posq/api/internal/platform/database"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("api berhenti dengan error", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	envFile, err := config.LoadDotEnv(".")
	if err != nil {
		return err
	}
	if envFile != "" {
		log.Info("memuat .env", "path", envFile)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	handler, err := app.NewRouter(app.Deps{Pool: pool, Config: cfg, Logger: log})
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr: cfg.HTTPAddr, Handler: handler,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 35 * time.Second, IdleTimeout: 60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	log.Info("api berjalan", "addr", cfg.HTTPAddr)

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
