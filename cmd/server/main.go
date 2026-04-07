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

	res, err := bootstrap(cfg, logger)
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
