// Package storage provides functionalities for managing storage, including cleanup of expired jobs.
package storage

import (
	"context"
	"daunrodo/internal/entity"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

func (stg *storage) CleanupExpiredJobs(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log := stg.log.With(slog.String("action", "cleanup_expired_jobs"), slog.Duration("interval", interval))

	for {
		select {
		case <-ticker.C:
			stg.performCleanup(ctx)
		case <-ctx.Done():
			log.Info("cleanup expired jobs stopped")

			return
		}
	}
}

func (stg *storage) performCleanup(ctx context.Context) {
	log := stg.log
	now := time.Now()

	stg.mu.Lock()
	expiredJobs := stg.getExpiredJobs(now)
	stg.mu.Unlock()

	if len(expiredJobs) == 0 {
		log.DebugContext(ctx, "no expired jobs found to clean up")

		return
	}

	log.InfoContext(ctx, "about to remove expired jobs", slog.Int("count", len(expiredJobs)))

	for _, job := range expiredJobs {
		stg.cleanupJob(ctx, job)
	}
}

func (stg *storage) getExpiredJobs(now time.Time) []*entity.Job {
	var expiredJobs []*entity.Job

	for _, job := range stg.jobs {
		if job.ExpiresAt.Before(now) {
			expiredJobs = append(expiredJobs, job)
		}
	}

	return expiredJobs
}

func (stg *storage) cleanupJob(ctx context.Context, job *entity.Job) {
	if job == nil {
		return
	}

	log := stg.log
	deletedFiles := 0

	for _, pub := range job.Publications {
		if !filepath.IsAbs(pub.Filename) {
			log.ErrorContext(ctx, "non-absolute path found", slog.String("filename", pub.Filename))

			continue
		}

		err := os.Remove(pub.Filename)
		if os.IsNotExist(err) {
			log.ErrorContext(ctx, "failed to delete file", slog.String("filename", pub.Filename), slog.Any("error", err))

			continue
		}

		deletedFiles++

		log.DebugContext(ctx, "successfully deleted file", slog.String("filename", pub.Filename))
	}

	stg.mu.Lock()
	defer stg.mu.Unlock()

	for _, pub := range job.Publications {
		delete(stg.publications, pub.UUID)
	}

	delete(stg.jobs, job.UUID)

	log.DebugContext(ctx, "job cleaned up",
		slog.String("job_id", job.UUID),
		slog.Int("deleted_files", deletedFiles),
		slog.Int("publications_count", len(job.Publications)))
}
