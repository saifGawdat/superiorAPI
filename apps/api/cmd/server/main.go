// Command server runs the superiorAPI profiler backend.
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

	"superiorapi/internal/api"
	"superiorapi/internal/bench"
	"superiorapi/internal/config"
	"superiorapi/internal/httpclient"
	"superiorapi/internal/security"
	"superiorapi/internal/testrun"
)

const userAgent = "superiorAPI/0.1 (API performance profiler)"

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "err", err)
		os.Exit(1)
	}
	if cfg.AllowPrivateTargets {
		slog.Warn("ALLOW_PRIVATE_TARGETS is on: SSRF protection is disabled. Never use this in a public deployment.")
	}

	policy := security.Policy{AllowPrivate: cfg.AllowPrivateTargets, AllowedPorts: cfg.AllowedPorts}
	newRunner := func(c bench.Config) (*bench.Runner, func()) {
		client := httpclient.New(httpclient.Options{
			Policy: policy, Concurrency: c.Concurrency, Timeout: c.Timeout, MaxRedirects: cfg.MaxRedirects,
		})
		return &bench.Runner{Client: client, MaxResponseBytes: cfg.MaxResponseBytes, UserAgent: userAgent}, client.CloseIdleConnections
	}
	manager := testrun.NewManager(newRunner, cfg.Limits, cfg.ProbeRegion, cfg.MaxResponseBytes)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go manager.RunJanitor(ctx, time.Minute)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api.New(cfg, policy, manager),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		IdleTimeout:       120 * time.Second,
		// No WriteTimeout: SSE streams stay open for the length of a test.
	}
	go func() {
		slog.Info("listening", "addr", srv.Addr, "origins", cfg.AllowedOrigins, "region", cfg.ProbeRegion)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	manager.Shutdown() // finishes running tests so open streams receive "done"
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}
