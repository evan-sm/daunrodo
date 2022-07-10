package service

import (
	"context"
	"daunrodo/internal/config"
	"daunrodo/internal/consts"
	"daunrodo/internal/downloader"
	"daunrodo/internal/entity"
	"daunrodo/internal/errs"
	"daunrodo/pkg/jobid"
	"daunrodo/pkg/urls"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

type job struct {
	log        *slog.Logger
	cfg        *config.Config
	jobQueue   chan *entity.Job
	mu         sync.RWMutex
	jobStorage map[string]*entity.Job
	downloader downloader.Downloader

	wg        sync.WaitGroup
	closed    atomic.Bool
	startOnce sync.Once
}

type Job interface {
	Start(ctx context.Context)

	Enqueue(ctx context.Context, url string, preset string) (*entity.Job, error)

	GetByURLAndPreset(ctx context.Context, url string, preset string) *entity.Job
	GetByID(ctx context.Context, id string) *entity.Job
	GetAll(ctx context.Context) ([]*entity.Job, error)

	UpdateStatus(ctx context.Context, job *entity.Job, status entity.JobStatus, progress int, errorMsg string)
}

var _ Job = (*job)(nil)

func New(cfg *config.Config, log *slog.Logger, downloader downloader.Downloader) Job {
	return &job{
		jobQueue:   make(chan *entity.Job, cfg.App.Job.QueueSize),
		jobStorage: make(map[string]*entity.Job),
		downloader: downloader,
		cfg:        cfg,
		log:        log.With(slog.String("package", "service")),
	}
}

func (svc *job) Start(ctx context.Context) {
	svc.startOnce.Do(func() {
		for i := range svc.cfg.App.Job.Workers {
			svc.wg.Add(1)
			go svc.worker(ctx, i)
		}
	})
}

func (svc *job) Enqueue(ctx context.Context, url, preset string) (job *entity.Job, err error) {
	if svc.closed.Load() {
		return nil, errs.ErrServiceClosed
	}

	url = urls.Normalize(url)

	job = svc.GetByURLAndPreset(ctx, url, preset)
	if job != nil && job.Status != entity.JobStatusError {
		return job, errs.ErrJobAlreadyExists
	}

	job = &entity.Job{
		ID:        jobid.GenUUIDv5(url, preset),
		URL:       url,
		Preset:    preset,
		Status:    entity.JobStatusStarting,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ExpiresAt: time.Now().Add(consts.DefaultJobTTL),
	}

	svc.mu.Lock()
	svc.jobStorage[job.ID] = job
	svc.mu.Unlock()

	select {
	case svc.jobQueue <- job:

		return job, err
	case <-ctx.Done():

		return nil, fmt.Errorf("enqueue job canceled: %w", ctx.Err())
	default:
		svc.UpdateStatus(ctx, job, entity.JobStatusError, 0, "job queue is full")

		return nil, fmt.Errorf("job queue is full: %d/%d", len(svc.jobQueue), cap(svc.jobQueue))
	}
}

func (svc *job) worker(ctx context.Context, workerID int) {
	defer svc.wg.Done()

	log := svc.log.With(slog.Int("worker_id", workerID))

	for {
		select {
		case job, ok := <-svc.jobQueue:
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

func (svc *job) processJob(ctx context.Context, job *entity.Job) {
	log := svc.log.With(slog.String("func", "processJob"))
	if job == nil {
		log.WarnContext(ctx, "process job: nil job received")

		return
	}

	jobCtx, cancel := context.WithTimeout(ctx, svc.cfg.App.Job.Timeout)
	defer cancel()

	progressFn := func(_ctx context.Context, job *entity.Job, status entity.JobStatus, progress int, errorMsg string) {
		svc.UpdateStatus(jobCtx, job, status, progress, errorMsg)
	}

	err := svc.downloader.Process(jobCtx, job, progressFn)
	if err != nil {
		log.ErrorContext(ctx, "downloader process", slog.Any("error", err))
		svc.UpdateStatus(ctx, job, entity.JobStatusError, 0, err.Error())

		return
	}

	log.DebugContext(ctx, "job processed", "job", job)
}

func (svc *job) UpdateStatus(ctx context.Context, job *entity.Job, status entity.JobStatus, progress int, errorMsg string) {
	log := svc.log.With(slog.String("func", "UpdateStatus"))

	if job == nil {
		log.ErrorContext(ctx, "update job status: nil job")

		return
	}

	svc.mu.Lock()
	defer svc.mu.Unlock()

	job.Status = status
	job.UpdatedAt = time.Now()
	if progress != 0 {
		job.Progress = progress
	}
	if errorMsg != "" {
		job.Error = errorMsg
	}

	if job.Progress > 0 {
		elapsed := time.Since(job.CreatedAt)
		job.EstimatedETA = time.Duration(float64(elapsed) * (100.0/float64(progress) - 1.0))
	}

	log.DebugContext(ctx, "job status updated", "job", job)
}

func (svc *job) GetByURLAndPreset(ctx context.Context, url, preset string) *entity.Job {
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	id := jobid.GenUUIDv5(url, preset)

	job := svc.jobStorage[id]

	return job
}

func (svc *job) GetByID(ctx context.Context, id string) *entity.Job {
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	job := svc.jobStorage[id]

	return job
}

func (svc *job) GetAll(ctx context.Context) ([]*entity.Job, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	if len(svc.jobStorage) == 0 {
		return nil, errs.ErrNoJobs
	}

	jobs := make([]*entity.Job, 0, len(svc.jobStorage))
	for _, job := range svc.jobStorage {
		jobs = append(jobs, job)
	}

	return jobs, nil
}
