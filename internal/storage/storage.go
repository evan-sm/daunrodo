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

	// CancelJob cancels a job by its ID.
	CancelJob(ctx context.Context, jobID string) error

	// RegisterCancelFunc stores a cancel function for a job.
	RegisterCancelFunc(jobID string, cancelFunc context.CancelFunc)

	// UnregisterCancelFunc removes the cancel function for a job.
	UnregisterCancelFunc(jobID string)

	CleanupExpiredJobs(ctx context.Context, interval time.Duration)
}

type storage struct {
	log *slog.Logger
	cfg *config.Config

	mu           sync.RWMutex
	jobs         map[string]*entity.Job         // job UUID : job
	publications map[string]*entity.Publication // publication UUID : publication

	cancelMu    sync.RWMutex
	cancelFuncs map[string]context.CancelFunc // job UUID : cancel func
}

// New creates a new in-memory storage instance.
func New(ctx context.Context, log *slog.Logger, cfg *config.Config) Storer {
	storage := &storage{
		log:          log,
		cfg:          cfg,
		jobs:         make(map[string]*entity.Job),
		publications: make(map[string]*entity.Publication),
		cancelFuncs:  make(map[string]context.CancelFunc),
		mu:           sync.RWMutex{},
		cancelMu:     sync.RWMutex{},
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

// CancelJob cancels a job by its ID by calling its cancel function.
func (stg *storage) CancelJob(ctx context.Context, jobID string) error {
	stg.mu.RLock()
	job := stg.jobs[jobID]
	stg.mu.RUnlock()

	if job == nil {
		return errs.ErrJobNotFound
	}

	// Check if job is in a cancellable state
	if job.Status == entity.JobStatusFinished ||
		job.Status == entity.JobStatusError ||
		job.Status == entity.JobStatusCancelled {
		return errs.ErrJobCancelled
	}

	// Get and call the cancel function
	stg.cancelMu.RLock()
	cancelFunc := stg.cancelFuncs[jobID]
	stg.cancelMu.RUnlock()

	if cancelFunc == nil {
		stg.log.WarnContext(ctx, "no cancel func registered for job", slog.String("job_id", jobID))

		return errs.ErrJobCancelled
	}

	cancelFunc()

	// Update job status
	stg.mu.Lock()

	job.Status = entity.JobStatusCancelled
	job.UpdatedAt = time.Now()

	stg.mu.Unlock()

	stg.log.InfoContext(ctx, "job cancelled", slog.String("job_id", jobID))

	return nil
}

// RegisterCancelFunc stores a cancel function for a job.
func (stg *storage) RegisterCancelFunc(jobID string, cancelFunc context.CancelFunc) {
	stg.cancelMu.Lock()
	defer stg.cancelMu.Unlock()

	stg.cancelFuncs[jobID] = cancelFunc
}

// UnregisterCancelFunc removes the cancel function for a job.
func (stg *storage) UnregisterCancelFunc(jobID string) {
	stg.cancelMu.Lock()
	defer stg.cancelMu.Unlock()

	delete(stg.cancelFuncs, jobID)
}
