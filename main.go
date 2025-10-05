// entry point of the application
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"daunrodo/internal/config"
	"daunrodo/internal/downloader"
	httprouter "daunrodo/internal/infrastructure/delivery/http"
	"daunrodo/internal/service"
	"daunrodo/internal/storage"
	httpserver "daunrodo/pkg/http/server"
	"daunrodo/pkg/logger"

	"github.com/lrstanley/go-ytdlp"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
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

	log.InfoContext(ctx, "checking if yt-dlp, ffmpeg, ffprobe isn't installed yet. it may take some time...")
	// If yt-dlp, ffmpeg, ffprobe isn't installed yet, download and cache it for further use.
	ytdlp.MustInstallAll(ctx)

	downloader := downloader.NewYTdlp(log, cfg)
	storer := storage.New(ctx, log, cfg)

	// Service
	svc := service.New(cfg, log, downloader, storer)

	// HTTP Server
	router := httprouter.New(log, cfg, svc, storer)

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
