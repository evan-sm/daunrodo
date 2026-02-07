// Package downloader defines the downloader interface and related models.
package downloader

import (
	"context"
	"daunrodo/internal/entity"
	"daunrodo/internal/storage"
	"errors"
	"time"
)

const (
	defaultProgressFreq = 200 * time.Millisecond
)

// Downloader defines the interface for downloading content based on a job.
type Downloader interface {
	Process(ctx context.Context, job *entity.Job, storer storage.Storer) error
}

func classifyProcessingError(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	default:
		return "process"
	}
}
