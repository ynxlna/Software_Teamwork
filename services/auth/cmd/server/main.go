package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v4/pgxpool"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/auth/internal/config"
	authhttp "github.com/Sakayori-Iroha-168/Software_Teamwork/services/auth/internal/http"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/auth/internal/repository"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/auth/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration failed", "service", "auth", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var pool *pgxpool.Pool
	var readinessChecker authhttp.ReadinessChecker
	var authService authhttp.AuthService
	if cfg.DatabaseURL != "" {
		pool, err = pgxpool.Connect(ctx, cfg.DatabaseURL)
		if err != nil {
			logger.Error("postgres connection failed", "service", "auth", "dependency", "postgres", "error", err)
			os.Exit(1)
		}
		defer pool.Close()
		readinessChecker = repository.NewReadinessChecker(pool)
		authRepo := repository.NewPostgresRepositoryFromPool(pool)
		authService = service.New(authRepo,
			service.WithTokenHashSecret([]byte(cfg.TokenHashSecret)),
			service.WithTokenHashKeyVersion(cfg.TokenKeyVersion),
			service.WithSessionTTL(cfg.SessionTTL),
			service.WithDefaultRoleCode(cfg.DefaultRoleCode),
			service.WithLogger(logger),
		)
	}

	handler := authhttp.NewServer(authhttp.Config{
		ServiceVersion:   cfg.ServiceVersion,
		Environment:      cfg.Environment,
		ReadinessTimeout: cfg.ReadinessTimeout,
		ReadinessChecker: readinessChecker,
		Auth:             authService,
		ServiceToken:     cfg.ServiceToken,
		Logger:           logger,
	})

	server := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: handler,
	}

	go func() {
		logger.Info("auth service starting", "service", "auth", "addr", cfg.HTTPAddr, "environment", cfg.Environment)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("auth service stopped unexpectedly", "service", "auth", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	logger.Info("auth service shutdown started", "service", "auth")
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("auth service shutdown failed", "service", "auth", "error", err)
		os.Exit(1)
	}
	logger.Info("auth service shutdown complete", "service", "auth")
}
