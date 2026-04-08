package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/ovander/parashift/internal/config"
	"github.com/ovander/parashift/internal/handler"
	"github.com/sirupsen/logrus"
)

// version, buildTime, and commit are injected at link time via -ldflags:
//
//	-X main.version=1.0.3 -X main.buildTime=2026-04-08T05:00:00Z -X main.commit=a3f9c12
//
// All default to "dev" / "unknown" when built without those flags (local dev).
var (
	version   = "dev"
	buildTime = "unknown"
	commit    = "unknown"
)

func main() {
	// Load .env if present (dev convenience; no-op in production where env vars are injected directly)
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		log.Printf("warning: could not load .env file: %v", err)
	}

	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config invalid: %v", err)
	}

	logger := newLogger(cfg)
	logger.WithFields(logrus.Fields{
		"version":    version,
		"build_time": buildTime,
		"commit":     commit,
	}).Info("ParaShift binary starting")

	// migrate-only mode: apply SQL migrations and exit.
	// Usage: ./server migrate
	// Intended for use as a one-shot init container or pre-deploy job.
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		db, err := initDB(cfg, logger)
		if err != nil {
			logger.Fatalf("migrate: db connect: %v", err)
		}
		if err := runMigrations(cfg, db); err != nil {
			logger.Fatalf("migrate: %v", err)
		}
		logger.Info("migrations complete — exiting")
		os.Exit(0)
	}

	res, err := bootstrap(cfg, logger, handler.BuildInfo{
		Version:   version,
		Commit:    commit,
		BuildTime: buildTime,
	})
	if err != nil {
		logger.Fatalf("bootstrap: %v", err)
	}

	logger.Infof("ParaShift starting on :%d", cfg.Port)
	go func() {
		if err := res.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := res.Server.Shutdown(ctx); err != nil {
		logger.WithError(err).Error("HTTP server shutdown failed")
	}
	res.Services.Emitter.Close()
	res.Limiter.Stop()
	sqlDB, _ := res.DB.DB()
	sqlDB.Close()
	logger.Info("shutdown complete")
}
