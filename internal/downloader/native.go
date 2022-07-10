package downloader

import (
	"context"
	"daunrodo/internal/consts"
	"daunrodo/internal/entity"
	"log/slog"
	"time"
)

type native struct {
	log *slog.Logger
}

// NewNative creates a new native downloader without yt-dlp
func NewNative(log *slog.Logger) Downloader {
	return &native{log: log}
}

func (n *native) Process(ctx context.Context, job *entity.Job, updateStatusFunc StatusUpdater) error {
	return nil
}

func (n *native) Timeout() time.Duration {
	return consts.DefaultJobTimeout
}
