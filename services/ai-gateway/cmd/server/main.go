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

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/ai-gateway/internal/config"
	httpapi "github.com/Sakayori-Iroha-168/Software_Teamwork/services/ai-gateway/internal/http"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/ai-gateway/internal/middleware"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/ai-gateway/internal/provider"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/ai-gateway/internal/repository"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/ai-gateway/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration failed", "service", "ai-gateway", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	repo, err := repository.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database initialization failed", "service", "ai-gateway", "dependency", "postgres", "error", err)
		os.Exit(1)
	}
	defer repo.Close()

	encryptor, err := service.NewCredentialEncryptor(cfg.CredentialEncryptionKey, cfg.CredentialEncryptionKeyRef)
	if err != nil {
		logger.Error("credential encryption initialization failed", "service", "ai-gateway", "error", err)
		os.Exit(1)
	}
	authenticator, err := middleware.NewServiceTokenAuthenticator(cfg.ServiceTokenHashes)
	if err != nil {
		logger.Error("service token authentication initialization failed", "service", "ai-gateway", "error", err)
		os.Exit(1)
	}

	profiles := service.NewWithChatProvider(repo, encryptor, cfg.DefaultTimeoutMS, provider.NewHTTPChatClient(nil), provider.NewHTTPClient(nil))
	handler := httpapi.NewServer(httpapi.Config{
		Logger:          logger,
		Profiles:        profiles,
		Authenticator:   authenticator,
		MaxRequestBytes: cfg.MaxRequestBytes,
	})
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	go func() {
		logger.Info("ai-gateway service starting", "service", "ai-gateway", "addr", cfg.HTTPAddr, "secret_mode", cfg.SecretMode)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("ai-gateway service stopped unexpectedly", "service", "ai-gateway", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	logger.Info("ai-gateway service shutdown started", "service", "ai-gateway")
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("ai-gateway service shutdown failed", "service", "ai-gateway", "error", err)
		os.Exit(1)
	}
	logger.Info("ai-gateway service shutdown complete", "service", "ai-gateway")
}
