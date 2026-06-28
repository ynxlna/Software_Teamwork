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

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/config"
	httpapi "github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/http"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/repository"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "service", "qa", "operation", "load_config", "error", err)
		os.Exit(1)
	}

	store := repository.NewMemoryStore()
	conversations := service.NewConversationService(store, service.ContextLimits{
		MaxMessages: cfg.MaxContextMessages,
		MaxChars:    cfg.MaxContextChars,
	})

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.NewRouter(conversations),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("qa service starting", "service", "qa", "addr", cfg.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("qa service failed", "service", "qa", "operation", "listen", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	slog.Info("qa service shutting down", "service", "qa")
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("qa service shutdown failed", "service", "qa", "operation", "shutdown", "error", err)
		os.Exit(1)
	}
}
