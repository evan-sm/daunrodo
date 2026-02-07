// entry point of the application
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"daunrodo/internal/config"
	"daunrodo/internal/depmanager"
	"daunrodo/internal/downloader"
	httprouter "daunrodo/internal/infrastructure/delivery/http"
	"daunrodo/internal/observability"
	"daunrodo/internal/proxymgr"
	"daunrodo/internal/service"
	"daunrodo/internal/storage"
	httpserver "daunrodo/pkg/http/server"
	"daunrodo/pkg/logger"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cfg, err := config.New()
	if err != nil {
		slog.Error("config new", slog.Any("error", err))
		stop()
		os.Exit(1)
	}

	log, err := logger.New(&logger.Options{
		AddSource: true,
		Level:     cfg.App.LogLevel,
	})
	if err != nil {
		slog.WarnContext(ctx, "logger level invalid; defaulting to info", slog.Any("error", err))
	}

	depMgr := depmanager.New(log, cfg)
	metrics := observability.New()

	log.InfoContext(ctx, "checking if yt-dlp, gallery-dl, deno are installed. it may take some time...")

	depMgr.Start(ctx)

	// Initialize proxy manager (can be nil if no proxies configured)
	var proxyMgr *proxymgr.Manager
	if len(cfg.Proxy.Proxies) > 0 {
		proxyMgr = proxymgr.New(log, cfg, metrics)
		go proxyMgr.StartHealthChecker(ctx)

		log.InfoContext(ctx, "proxy manager initialized", slog.Int("proxy_count", len(cfg.Proxy.Proxies)))
	}

	dl := downloader.NewYTdlp(log, cfg, depMgr, proxyMgr, metrics)
	storer := storage.New(ctx, log, cfg, metrics)

	// Service
	svc := service.New(cfg, log, dl, storer, metrics)

	// HTTP Server
	router := httprouter.New(log, cfg, svc, storer, metrics)

	httpSrv := httpserver.New(router, httpserver.Options{
		Addr:            cfg.HTTP.Port,
		ShutdownTimeout: cfg.HTTP.ShutdownTimeout,
	})

	svc.Start(ctx)

	log.InfoContext(ctx, "daunrodo started", slog.String("port", cfg.HTTP.Port))

	// Waiting for shutdown signal
	<-ctx.Done()

	err = httpSrv.Shutdown()
	if err != nil {
		log.Error(err.Error())
	}

	log.InfoContext(ctx, "daunrodo shut down gracefully")
}
