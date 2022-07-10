package downloader

import (
	"context"
	"daunrodo/internal/entity"
	"time"
)

const (
	defaultPreset         = "mp4"
	defaultOutputTemplate = "%(extractor)s - %(title)s [%(id)s].%(ext)s"
	defaultProgressFreq   = 200 * time.Millisecond
)

type Downloader interface {
	Process(ctx context.Context, job *entity.Job, updateStatusFunc StatusUpdater) error
	// JobTimeout() time.Duration
}

type StatusUpdater func(ctx context.Context, job *entity.Job, status entity.JobStatus, progress int, errorMsg string)
