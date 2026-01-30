package downloader

import (
	"context"
	"daunrodo/internal/consts"
	"daunrodo/internal/entity"
	"daunrodo/internal/storage"
	"fmt"
	"log/slog"
	"time"
)

// mock is a mock implementation of the Downloader interface for testing purposes.
type mock struct {
	log *slog.Logger
}

// ProgressCallbackFunc defines the type for progress update callbacks.
type ProgressCallbackFunc func(update ProgressUpdateMock)

// ProgressUpdateMock represents a mock progress update.
type ProgressUpdateMock struct {
	Progress int
	ETA      string
}

// NewMock creates a new Mock downloader instance.
func NewMock(log *slog.Logger) Downloader {
	return &mock{log: log.With(slog.String("package", "downloader"), slog.String("downloader", consts.DownloaderMock))}
}

// Process processes the download job and updates the job status in the storage.
func (m *mock) Process(ctx context.Context, job *entity.Job, storer storage.Storer) error {
	if job == nil {
		return fmt.Errorf("job is nil")
	}

	log := m.log.With(slog.Any("job", job))

	storer.UpdateJobStatus(ctx, job.UUID, entity.JobStatusDownloading, 0, "")

	progressFn := func(prog ProgressUpdateMock) {
		log.InfoContext(ctx, "job progress", slog.Int("progress", prog.Progress), slog.String("eta", prog.ETA))
		storer.UpdateJobStatus(ctx, job.UUID, entity.JobStatusDownloading, prog.Progress, "")
	}

	err := simulateDownload(ctx, consts.DefaultSimulateTime, progressFn)
	if err != nil {
		log.Error("simulate download", slog.Any("error", err))
		storer.UpdateJobStatus(ctx, job.UUID, entity.JobStatusError, 0, err.Error())

		return err
	}

	storer.UpdateJobStatus(ctx, job.UUID, entity.JobStatusFinished, fullProgress, "")

	log.InfoContext(ctx, "processing job")

	return err
}

func simulateDownload(ctx context.Context, duration time.Duration, progressFn ProgressCallbackFunc) error {
	steps := 10
	step := 0

	ticker := time.NewTicker(duration / time.Duration(steps))
	defer ticker.Stop()

	start := time.Now()

	for step <= steps {
		select {
		case <-ctx.Done():
			return fmt.Errorf("simulate download: %w", ctx.Err())
		case <-ticker.C:
			progress := step * (fullProgress / steps)
			eta := fmt.Sprintf("%.0f seconds", duration.Seconds()-time.Since(start).Seconds())
			progressFn(ProgressUpdateMock{Progress: progress, ETA: eta})

			step++
		}
	}

	return nil
}
