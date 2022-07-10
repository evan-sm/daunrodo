package downloader

import (
	"context"
	"daunrodo/internal/consts"
	"daunrodo/internal/entity"
	"fmt"
	"log/slog"
	"time"
)

type mock struct {
	log *slog.Logger
}

type ProgressCallbackFunc func(update ProgressUpdateMock)

type ProgressUpdateMock struct {
	Progress int
	ETA      string
}

func NewMock(log *slog.Logger) Downloader {
	return &mock{log: log.With(slog.String("package", "downloader"), slog.String("downloader", consts.DownloaderMock))}
}

func (m *mock) Process(ctx context.Context, job *entity.Job, updateStatusFn StatusUpdater) (err error) {
	if job == nil {
		return fmt.Errorf("job is nil")
	}

	log := m.log.With(slog.String("func", "Process"), "job", job)

	updateStatusFn(ctx, job, entity.JobStatusDownloading, 0, "")

	progressFn := func(prog ProgressUpdateMock) {
		log.InfoContext(ctx, "job progress", slog.Int("progress", prog.Progress), slog.String("eta", prog.ETA))
		updateStatusFn(ctx, job, entity.JobStatusDownloading, prog.Progress, "")
	}

	err = simulateDownload(ctx, consts.DefaultSimulateTime, progressFn)
	if err != nil {
		log.Error("simulate download", slog.Any("error", err))
		updateStatusFn(ctx, job, entity.JobStatusError, 0, err.Error())

		return err
	}

	updateStatusFn(ctx, job, entity.JobStatusFinished, 100, "")

	log.InfoContext(ctx, "processing job")

	return err
}

func simulateDownload(ctx context.Context, duration time.Duration, progressFn ProgressCallbackFunc) (err error) {
	steps := 10
	step := 0

	ticker := time.NewTicker(duration / time.Duration(steps))
	defer ticker.Stop()

	start := time.Now()

	for step <= steps {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			progress := step * (100 / steps)
			eta := fmt.Sprintf("%.0f seconds", duration.Seconds()-time.Since(start).Seconds())
			progressFn(ProgressUpdateMock{Progress: progress, ETA: eta})
			step++
		}
	}

	return err
}
