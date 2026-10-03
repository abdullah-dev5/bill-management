// Command app runs the Bill Management API.
//
//	app serve            run migrations, then the HTTP API (default)
//	app migrate up       apply migrations
//	app migrate down     roll back one migration
//	app migrate status   show migration status
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // household timezones work in minimal containers

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/abdullah-dev5/bill-management/backend/internal/config"
	"github.com/abdullah-dev5/bill-management/backend/internal/httpapi"
	"github.com/abdullah-dev5/bill-management/backend/internal/mail"
	"github.com/abdullah-dev5/bill-management/backend/internal/secure"
	"github.com/abdullah-dev5/bill-management/backend/internal/storage"
	"github.com/abdullah-dev5/bill-management/backend/internal/store"
	"github.com/abdullah-dev5/bill-management/backend/migrations"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := newLogger(cfg)
	keys, err := secure.New(cfg.DataKey)
	if err != nil {
		return fmt.Errorf("DATA_ENCRYPTION_KEY: %w", err)
	}

	cmd := "serve"
	if len(args) > 0 {
		cmd = args[0]
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cmd {
	case "serve":
		if err := migrate(ctx, cfg.DatabaseURL, "up", keys); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
		return serve(ctx, cfg, log, keys)
	case "migrate":
		direction := "up"
		if len(args) > 1 {
			direction = args[1]
		}
		return migrate(ctx, cfg.DatabaseURL, direction, keys)
	default:
		return fmt.Errorf("unknown command %q (want serve or migrate)", cmd)
	}
}

func serve(ctx context.Context, cfg config.Config, log *slog.Logger, keys *secure.Keys) error {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	var mailer mail.Mailer = mail.LogMailer{Log: log}
	if cfg.ResendAPIKey != "" {
		mailer = mail.NewResend(cfg.ResendAPIKey, cfg.MailFrom)
	} else if cfg.IsProduction() {
		log.Warn("RESEND_API_KEY not set: emails will only be logged")
	}

	var files storage.Store
	switch cfg.StorageDriver {
	case "s3":
		files = storage.NewS3(storage.S3Config(cfg.S3))
	default:
		if files, err = storage.NewLocal(cfg.StorageDir); err != nil {
			return err
		}
		log.Info("storing documents on local disk", "dir", cfg.StorageDir)
	}

	api := httpapi.New(store.NewDB(pool), cfg, log, mailer, keys, files)
	api.StartWorkers(ctx, 2)
	if cfg.GeminiAPIKey == "" && cfg.OCRSpaceAPIKey == "" {
		log.Warn("no GEMINI_API_KEY or OCRSPACE_API_KEY: only text PDFs can be read automatically (see docs/setup-external-services.md)")
	}
	handler, err := api.Handler()
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "port", cfg.Port, "env", cfg.Env)
		errCh <- srv.ListenAndServe()
	}()
	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("shutting down")
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func migrate(ctx context.Context, databaseURL, direction string, keys *secure.Keys) error {
	poolCfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return err
	}
	db := stdlib.OpenDB(*poolCfg.ConnConfig)
	defer db.Close()
	return migrations.Run(ctx, db, direction, keys)
}

func newLogger(cfg config.Config) *slog.Logger {
	if cfg.IsProduction() {
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}
