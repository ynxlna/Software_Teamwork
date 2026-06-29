package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/config"
	knowledgehttp "github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/http"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/repository"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration failed", "service", "knowledge", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := connectPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("postgres connection failed", "service", "knowledge", "dependency", "postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	repo := repository.NewPostgresRepository(pool)
	knowledgeService := service.New(repo)
	handler := knowledgehttp.NewServer(knowledgeService, knowledgehttp.Config{
		ServiceVersion: cfg.ServiceVersion,
		Environment:    cfg.Environment,
		Logger:         logger,
	})

	server := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: handler,
	}

	go func() {
		logger.Info("knowledge service starting", "service", "knowledge", "addr", cfg.HTTPAddr, "environment", cfg.Environment)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("knowledge service stopped unexpectedly", "service", "knowledge", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	logger.Info("knowledge service shutdown started", "service", "knowledge")
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("knowledge service shutdown failed", "service", "knowledge", "error", err)
		os.Exit(1)
	}
	logger.Info("knowledge service shutdown complete", "service", "knowledge")
}

func connectPostgres(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
