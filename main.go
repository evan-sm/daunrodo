package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"daunrodo/internal/config"
	"daunrodo/internal/downloader"
	httprouter "daunrodo/internal/infrastructure/delivery/http"
	"daunrodo/internal/service"
	httpserver "daunrodo/pkg/http/server"
	"daunrodo/pkg/logger"

	"github.com/lrstanley/go-ytdlp"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		slog.Error("config new", slog.Any("error", err))
		os.Exit(1)
	}

	log, err := logger.New(&logger.Options{
		AddSource: true,
		Level:     cfg.LogLevel,
	})
	if err != nil {
		slog.Error("logger new", slog.Any("error", err))
		os.Exit(1)
	}

	downloader := downloader.NewYTdlp(log, cfg)

	// Service
	svc := service.New(cfg, log, downloader)

	// HTTP Server
	router := httprouter.New(log, svc)

	httpSrv := httpserver.New(router, httpserver.Options{
		Addr:            cfg.App.HttpPort,
		ShutdownTimeout: time.Second * 10,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// If yt-dlp, ffmpeg, ffprobe isn't installed yet, download and cache it for further use.
	ytdlp.MustInstallAll(ctx)

	svc.Start(ctx)

	// Waiting for shutdown signal
	<-ctx.Done()

	err = httpSrv.Shutdown()
	if err != nil {
		log.Error(err.Error())
	}

	log.InfoContext(ctx, "daunrodo shut down gracefully")
}
