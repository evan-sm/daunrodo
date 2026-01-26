// Package service provides job management and processing services.
package service

import (
	"context"
	"daunrodo/internal/config"
	"daunrodo/internal/downloader"
	"daunrodo/internal/entity"
	"daunrodo/internal/errs"
	"daunrodo/internal/storage"
	"daunrodo/pkg/gen"
	"daunrodo/pkg/urls"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Job represents a download job.
type Job struct {
	log        *slog.Logger
	Cfg        *config.Config
	JobQueue   chan *entity.Job
	Storer     storage.Storer
	Downloader downloader.Downloader

	wg        sync.WaitGroup
	closed    atomic.Bool
	startOnce sync.Once
}

// JobManager defines the interface for job management.
type JobManager interface {
	Start(ctx context.Context)

	Enqueue(ctx context.Context, url string, preset string) (*entity.Job, error)
}

var _ JobManager = (*Job)(nil)

// New creates a new Job service instance.
func New(cfg *config.Config, log *slog.Logger, downloader downloader.Downloader, storer storage.Storer) JobManager {
	return &Job{
		JobQueue:   make(chan *entity.Job, cfg.Job.QueueSize),
		Downloader: downloader,
		Storer:     storer,
		Cfg:        cfg,
		log:        log,
	}
}

// Start initializes the worker pool to process jobs.
func (svc *Job) Start(ctx context.Context) {
	svc.startOnce.Do(func() {
		for i := range svc.Cfg.Job.Workers {
			svc.wg.Add(1)

			go svc.worker(ctx, i)
		}
	})
}

// Enqueue adds a new job to the processing queue.
func (svc *Job) Enqueue(ctx context.Context, url, preset string) (*entity.Job, error) {
	if svc.closed.Load() {
		return nil, errs.ErrServiceClosed
	}

	url = urls.Normalize(url)

	job := svc.Storer.GetJobByURLAndPreset(ctx, url, preset)
	if job != nil && job.Status != entity.JobStatusError {
		return job, errs.ErrJobAlreadyExists
	}

	job = &entity.Job{
		UUID:      gen.UUIDv5(url, preset),
		URL:       url,
		Preset:    preset,
		Status:    entity.JobStatusStarting,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ExpiresAt: time.Now().Add(svc.Cfg.Storage.TTL),
	}

	svc.Storer.SetJob(ctx, job)

	select {
	case svc.JobQueue <- job:
		return job, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("enqueue job canceled: %w", ctx.Err())
	default:
		svc.Storer.UpdateJobStatus(ctx, job, entity.JobStatusError, 0, "job queue is full")

		return nil, fmt.Errorf("%w: %d/%d", errs.ErrJobQueueFull, len(svc.JobQueue), cap(svc.JobQueue))
	}
}

func (svc *Job) worker(ctx context.Context, workerID int) {
	defer svc.wg.Done()

	log := svc.log.With(slog.Int("worker_id", workerID))

	for {
		select {
		case job, ok := <-svc.JobQueue:
			if !ok {
				log.WarnContext(ctx, "job queue closed")

				return
			}

			if job == nil {
				log.WarnContext(ctx, "received nil job")

				continue
			}

			svc.processJob(ctx, job)
		case <-ctx.Done():
			svc.closed.Store(true)
			log.InfoContext(ctx, "got ctx done signal", slog.Any("error", ctx.Err()))

			return
		}
	}
}

func (svc *Job) processJob(ctx context.Context, job *entity.Job) {
	log := svc.log
	if job == nil {
		log.WarnContext(ctx, "process job: nil job received")

		return
	}

	jobCtx, cancel := context.WithTimeout(ctx, svc.Cfg.Job.Timeout)
	defer cancel()

	// Register cancel function for job cancellation support
	svc.Storer.RegisterCancelFunc(job.UUID, cancel)
	defer svc.Storer.UnregisterCancelFunc(job.UUID)

	err := svc.Downloader.Process(jobCtx, job, svc.Storer)
	if err != nil {
		// Check if it was cancelled
		if jobCtx.Err() == context.Canceled {
			log.InfoContext(ctx, "job cancelled", slog.Any("job_id", job.UUID))
			svc.Storer.UpdateJobStatus(ctx, job, entity.JobStatusCancelled, 0, "job cancelled by user")

			return
		}

		log.ErrorContext(ctx, "downloader process", slog.Any("error", err))
		svc.Storer.UpdateJobStatus(ctx, job, entity.JobStatusError, 0, err.Error())

		return
	}

	log.DebugContext(ctx, "job processed", "job", job)
}
