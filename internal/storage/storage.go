package storage

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"daunrodo/internal/config"
	"daunrodo/internal/entity"
	"daunrodo/internal/errs"
	"daunrodo/pkg/gen"
	"daunrodo/pkg/urls"
)

// Storer defines the interface for storage operations.
type Storer interface {
	SetJob(ctx context.Context, job *entity.Job)
	GetJobByURLAndPreset(ctx context.Context, url, preset string) *entity.Job
	GetJobByID(ctx context.Context, id string) *entity.Job
	GetJobs(ctx context.Context) ([]*entity.Job, error)
	UpdateJobStatus(ctx context.Context, job *entity.Job, status entity.JobStatus, progress int, errorMsg string)

	SetPublication(ctx context.Context, jobID string, publication *entity.Publication) error
	GetPublicationByID(ctx context.Context, id string) (*entity.Publication, error)

	CleanupExpiredJobs(ctx context.Context, interval time.Duration)
}

type storage struct {
	log *slog.Logger
	cfg *config.Config

	mu           sync.RWMutex
	jobs         map[string]*entity.Job         // job UUID : job
	publications map[string]*entity.Publication // publication UUID : publication
}

// New creates a new in-memory storage instance.
func New(ctx context.Context, log *slog.Logger, cfg *config.Config) Storer {
	storage := &storage{
		log:          log,
		cfg:          cfg,
		jobs:         make(map[string]*entity.Job),
		publications: make(map[string]*entity.Publication),
		mu:           sync.RWMutex{},
	}

	go storage.CleanupExpiredJobs(ctx, cfg.Storage.CleanupInterval)

	return storage
}

func (stg *storage) SetJob(ctx context.Context, job *entity.Job) {
	if job == nil || job.UUID == "" {
		stg.log.ErrorContext(ctx, "set job: nil job")

		return
	}

	stg.mu.Lock()
	defer stg.mu.Unlock()

	stg.jobs[job.UUID] = job
}

func (stg *storage) GetJobByURLAndPreset(_ context.Context, url, preset string) *entity.Job {
	stg.mu.RLock()
	defer stg.mu.RUnlock()

	url = urls.Normalize(url)
	id := gen.UUIDv5(url, preset)

	job := stg.jobs[id]

	return job
}

func (stg *storage) GetJobByID(_ context.Context, id string) *entity.Job {
	stg.mu.RLock()
	defer stg.mu.RUnlock()

	job := stg.jobs[id]

	return job
}

func (stg *storage) GetJobs(_ context.Context) ([]*entity.Job, error) {
	stg.mu.RLock()
	defer stg.mu.RUnlock()

	if len(stg.jobs) == 0 {
		return nil, errs.ErrNoJobs
	}

	jobs := make([]*entity.Job, 0, len(stg.jobs))
	for _, job := range stg.jobs {
		jobs = append(jobs, job)
	}

	return jobs, nil
}

func (stg *storage) UpdateJobStatus(ctx context.Context,
	job *entity.Job,
	status entity.JobStatus,
	progress int,
	errorMsg string) {
	log := stg.log

	if job == nil {
		log.ErrorContext(ctx, "update job status: nil job")

		return
	}

	stg.mu.Lock()
	defer stg.mu.Unlock()

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

func (stg *storage) SetPublication(ctx context.Context, jobID string, publication *entity.Publication) error {
	if publication == nil {
		return errs.ErrPublicationNil
	}

	if publication.UUID == "" {
		return errs.ErrPublicationUUID
	}

	if jobID == "" {
		return errs.ErrJobIDEmpty
	}

	stg.mu.Lock()
	defer stg.mu.Unlock()

	_, exists := stg.jobs[jobID]
	if !exists {
		return errs.ErrJobNotFound
	}

	stg.publications[publication.UUID] = publication

	stg.log.DebugContext(ctx, "publication stored", slog.Any("publication", publication))

	return nil
}

func (stg *storage) GetPublicationByID(_ context.Context, id string) (*entity.Publication, error) {
	stg.mu.RLock()
	defer stg.mu.RUnlock()

	pub := stg.publications[id]
	if pub == nil {
		return nil, errs.ErrPublicationNotFound
	}

	return pub, nil
}
