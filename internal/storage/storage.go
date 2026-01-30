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
type Storer interface { //nolint:interfacebloat
	SetJob(ctx context.Context, job entity.Job)
	GetJobByURLAndPreset(ctx context.Context, url, preset string) (entity.Job, bool)
	GetJobByID(ctx context.Context, id string) (entity.Job, bool)
	GetJobs(ctx context.Context) ([]entity.Job, error)
	UpdateJobStatus(ctx context.Context, jobID string, status entity.JobStatus, progress int, errorMsg string)
	UpdateJobEstimatedSize(ctx context.Context, jobID string, estimatedSize int64)
	UpdateJobPublications(ctx context.Context, jobID string, publications []entity.Publication)

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

	mu           sync.Mutex
	jobs         map[string]entity.Job          // job UUID : job
	publications map[string]*entity.Publication // publication UUID : publication

	cancelMu    sync.Mutex
	cancelFuncs map[string]context.CancelFunc // job UUID : cancel func
}

// New creates a new in-memory storage instance.
func New(ctx context.Context, log *slog.Logger, cfg *config.Config) Storer {
	storage := &storage{
		log:          log,
		cfg:          cfg,
		jobs:         make(map[string]entity.Job),
		publications: make(map[string]*entity.Publication),
		cancelFuncs:  make(map[string]context.CancelFunc),
	}

	go storage.CleanupExpiredJobs(ctx, cfg.Storage.CleanupInterval)

	return storage
}

// SetJob stores a new job in storage.
func (stg *storage) SetJob(ctx context.Context, job entity.Job) {
	if job.UUID == "" {
		stg.log.ErrorContext(ctx, "set job: empty job id")

		return
	}

	stg.mu.Lock()
	defer stg.mu.Unlock()

	stg.jobs[job.UUID] = copyJob(job)
}

// GetJobByURLAndPreset retrieves a job by its URL and preset.
func (stg *storage) GetJobByURLAndPreset(_ context.Context, url, preset string) (entity.Job, bool) {
	stg.mu.Lock()
	defer stg.mu.Unlock()

	url = urls.Normalize(url)
	id := gen.UUIDv5(url, preset)

	job, ok := stg.jobs[id]
	if !ok {
		return entity.Job{}, false
	}

	return copyJob(job), true
}

// GetJobByID retrieves a job by its ID.
func (stg *storage) GetJobByID(_ context.Context, id string) (entity.Job, bool) {
	stg.mu.Lock()
	defer stg.mu.Unlock()

	job, ok := stg.jobs[id]
	if !ok {
		return entity.Job{}, false
	}

	return copyJob(job), true
}

// GetJobs retrieves all jobs from storage.
func (stg *storage) GetJobs(_ context.Context) ([]entity.Job, error) {
	stg.mu.Lock()
	defer stg.mu.Unlock()

	if len(stg.jobs) == 0 {
		return nil, errs.ErrNoJobs
	}

	jobs := make([]entity.Job, 0, len(stg.jobs))
	for _, job := range stg.jobs {
		jobs = append(jobs, copyJob(job))
	}

	return jobs, nil
}

// UpdateJobStatus updates the status of a job.
func (stg *storage) UpdateJobStatus(ctx context.Context,
	jobID string,
	status entity.JobStatus,
	progress int,
	errorMsg string) {
	log := stg.log

	if jobID == "" {
		log.ErrorContext(ctx, "update job status: empty job id")

		return
	}

	stg.mu.Lock()
	defer stg.mu.Unlock()

	job, ok := stg.jobs[jobID]
	if !ok {
		stg.log.WarnContext(ctx, "job not found in storage", slog.String("job_id", jobID))

		return
	}

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
		job.EstimatedETA = time.Duration(float64(elapsed) * (100.0/float64(job.Progress) - 1.0))
	}

	stg.jobs[jobID] = job

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
	stg.mu.Lock()
	defer stg.mu.Unlock()

	pub := stg.publications[id]
	if pub == nil {
		return nil, errs.ErrPublicationNotFound
	}

	return pub, nil
}

func (stg *storage) UpdateJobEstimatedSize(ctx context.Context, jobID string, estimatedSize int64) {
	log := stg.log

	if jobID == "" {
		log.ErrorContext(ctx, "update job estimated size: empty job id")

		return
	}

	stg.mu.Lock()
	defer stg.mu.Unlock()

	job, ok := stg.jobs[jobID]
	if !ok {
		stg.log.WarnContext(ctx, "job not found in storage", slog.String("job_id", jobID))

		return
	}

	job.EstimatedSize = estimatedSize
	job.UpdatedAt = time.Now()
	stg.jobs[jobID] = job

	log.DebugContext(ctx, "job estimated size updated", "job", job)
}

func (stg *storage) UpdateJobPublications(ctx context.Context, jobID string, publications []entity.Publication) {
	log := stg.log

	if jobID == "" {
		log.ErrorContext(ctx, "update job publications: empty job id")

		return
	}

	stg.mu.Lock()
	defer stg.mu.Unlock()

	job, ok := stg.jobs[jobID]
	if !ok {
		stg.log.WarnContext(ctx, "job not found in storage", slog.String("job_id", jobID))

		return
	}

	if len(publications) == 0 {
		job.Publications = nil
		job.TotalSize = 0
		job.UpdatedAt = time.Now()

		stg.jobs[jobID] = job

		log.DebugContext(ctx, "job publications updated", "job", job)

		return
	}

	job.Publications = append([]entity.Publication(nil), publications...)

	var totalSize int64
	for _, pub := range job.Publications {
		totalSize += pub.FileSize
	}

	job.TotalSize = totalSize
	job.UpdatedAt = time.Now()

	stg.jobs[jobID] = job

	log.DebugContext(ctx, "job publications updated", "job", job)
}

// CancelJob cancels a job by its ID by calling its cancel function.
func (stg *storage) CancelJob(ctx context.Context, jobID string) error {
	stg.mu.Lock()
	job, ok := stg.jobs[jobID]
	stg.mu.Unlock()

	if !ok {
		return errs.ErrJobNotFound
	}

	// Check if job is in a cancellable state
	if job.Status == entity.JobStatusFinished ||
		job.Status == entity.JobStatusError ||
		job.Status == entity.JobStatusCancelled {
		return errs.ErrJobCancelled
	}

	// Get and call the cancel function
	stg.cancelMu.Lock()
	cancelFunc := stg.cancelFuncs[jobID]
	stg.cancelMu.Unlock()

	if cancelFunc == nil {
		stg.log.WarnContext(ctx, "no cancel func registered for job", slog.String("job_id", jobID))

		return errs.ErrJobCancelled
	}

	cancelFunc()

	stg.UpdateJobStatus(ctx, jobID, entity.JobStatusCancelled, 0, "job cancelled by user")

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

func copyJob(job entity.Job) entity.Job {
	if len(job.Publications) > 0 {
		publications := make([]entity.Publication, len(job.Publications))
		copy(publications, job.Publications)
		job.Publications = publications
	}

	return job
}
