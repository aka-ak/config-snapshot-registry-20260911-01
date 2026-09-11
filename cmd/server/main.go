package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"example.com/config-snapshot-registry/internal/config"
	"example.com/config-snapshot-registry/internal/httpapi"
	"example.com/config-snapshot-registry/internal/retention"
	"example.com/config-snapshot-registry/internal/service"
	"example.com/config-snapshot-registry/internal/store"
)

func main() {
	cfg := config.Load()
	svc := service.New(store.NewFile(cfg.StateFile))
	server := &http.Server{Addr: cfg.ListenAddr, Handler: httpapi.New(svc), ReadHeaderTimeout: 5 * time.Second}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go retention.Worker{Service: svc, Interval: cfg.RetentionInterval, MaxAge: cfg.RetentionAge}.Run(ctx)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("config snapshot registry listening on %s", cfg.ListenAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
