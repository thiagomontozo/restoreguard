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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thiagomontozo/restoreguard/backend/internal/database"
	"github.com/thiagomontozo/restoreguard/backend/internal/drill"
	"github.com/thiagomontozo/restoreguard/backend/internal/httpapi"
	"github.com/thiagomontozo/restoreguard/backend/internal/platform"
	"github.com/thiagomontozo/restoreguard/backend/internal/scheduler"
	"github.com/thiagomontozo/restoreguard/backend/internal/security"
	"github.com/thiagomontozo/restoreguard/backend/internal/storage"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(logger); err != nil {
		logger.Error("restoreguard stopped", "error", err)
		os.Exit(1)
	}
}
func run(logger *slog.Logger) error {
	cfg, err := platform.LoadConfig()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()
	migrationCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err = database.Migrate(migrationCtx, pool, cfg.MigrationPath); err != nil {
		return err
	}
	store := database.NewStore(pool)
	if err = store.BootstrapDemo(ctx, cfg.BootstrapEmail, cfg.BootstrapPassword); err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}
	if cfg.MasterKey != "" {
		if _, err = security.NewSecretStore(cfg.MasterKey); err != nil {
			return err
		}
	}
	var objects storage.ObjectStorage
	if cfg.ObjectStorageType == "s3" {
		s3, err := storage.NewS3(storage.S3Config{Endpoint: cfg.S3Endpoint, AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey, Bucket: cfg.S3Bucket, UseTLS: cfg.S3UseTLS, MaxBytes: cfg.MaxArtifactBytes})
		if err != nil {
			return err
		}
		if err = s3.EnsureBucket(ctx); err != nil {
			return fmt.Errorf("ensure S3 bucket: %w", err)
		}
		objects = s3
	} else {
		objects, err = storage.NewLocal(cfg.LocalStoragePath, cfg.MaxArtifactBytes)
		if err != nil {
			return err
		}
	}
	healthCtx, healthCancel := context.WithTimeout(ctx, 5*time.Second)
	defer healthCancel()
	if err = objects.Health(healthCtx); err != nil {
		return fmt.Errorf("object storage: %w", err)
	}
	hub := drill.NewEventHub()
	defer hub.Close()
	executor := drill.NewDockerSandboxExecutor(cfg.AllowedPostgresImage)
	workers := drill.NewWorkerPool(pool, platform.RealClock{}, hub, cfg.MaxConcurrentDrills, drill.WorkerDependencies{
		Executor:         executor,
		Objects:          objects,
		AllowedImage:     cfg.AllowedPostgresImage,
		MaxArtifactBytes: cfg.MaxArtifactBytes,
		DrillTimeout:     cfg.DefaultDrillTimeout,
	})
	schedulerCtx, schedulerCancel := context.WithCancel(ctx)
	defer schedulerCancel()
	go scheduler.New(pool, 30*time.Second, logger).Run(schedulerCtx)
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: httpapi.New(store, workers, hub, logger, cfg.CookieSecure, cfg.CORSOrigin), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 0, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("RestoreGuard API listening", "address", cfg.HTTPAddr, "environment", cfg.Environment)
		serverErrors <- server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
	case err = <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	schedulerCancel()
	if err = server.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err = workers.Close(shutdownCtx); err != nil {
		return err
	}
	return nil
}
