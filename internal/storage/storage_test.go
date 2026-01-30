package storage_test

import (
	"context"
	"daunrodo/internal/config"
	"daunrodo/internal/entity"
	"daunrodo/internal/errs"
	"daunrodo/internal/storage"
	"daunrodo/pkg/gen"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestGetJob(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	cfg := &config.Config{Storage: config.Storage{CleanupInterval: time.Minute}}

	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	storer := storage.New(ctx, log, cfg)

	uuid := gen.UUIDv5("a", "b")

	storer.SetJob(ctx, entity.Job{UUID: uuid})

	job, ok := storer.GetJobByID(ctx, uuid)
	if !ok {
		t.Errorf("failed to get job")
	}

	got, ok := storer.GetJobByURLAndPreset(ctx, "a", "b")
	if !ok {
		t.Errorf("expected job to be found")
	}

	if got.UUID != job.UUID {
		t.Errorf("expected job UUID to match")
	}

	// test GetJobs
	jobs, err := storer.GetJobs(ctx)
	if len(jobs) == 0 || err != nil {
		t.Errorf("expected jobs to be found")
	}
}

func TestUpdateJobStatus(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	cfg := &config.Config{Storage: config.Storage{CleanupInterval: time.Minute}}

	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	storer := storage.New(ctx, log, cfg)

	tests := []struct {
		name     string
		job      entity.Job
		status   entity.JobStatus
		progress int
		errorMsg string
	}{
		{
			name:     "update job status",
			job:      entity.Job{UUID: gen.UUIDv5("a", "b"), Status: entity.JobStatusStarting},
			status:   entity.JobStatusFinished,
			progress: 100,
			errorMsg: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			storer.SetJob(ctx, tt.job)

			storer.UpdateJobStatus(ctx, tt.job.UUID, tt.status, tt.progress, tt.errorMsg)

			job, ok := storer.GetJobByID(ctx, tt.job.UUID)
			if !ok {
				t.Errorf("expected job to be found")
			}

			if job.Status != tt.status {
				t.Errorf("expected job status to be %v, got %v", tt.status, job.Status)
			}

			if job.Progress != tt.progress {
				t.Errorf("expected job progress to be %d, got %d", tt.progress, job.Progress)
			}

			if job.Error != tt.errorMsg {
				t.Errorf("expected job error message to be %q, got %q", tt.errorMsg, job.Error)
			}
		})
	}
}

func TestSetGetPublication(t *testing.T) {
	cfg := &config.Config{Storage: config.Storage{CleanupInterval: time.Minute}}

	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	storer := storage.New(t.Context(), log, cfg)

	tests := []struct {
		name        string
		job         entity.Job
		publication *entity.Publication
		wantErr     bool
	}{
		{
			name:        "job does not exist",
			job:         entity.Job{UUID: ""},
			publication: &entity.Publication{UUID: gen.UUIDv5("c", "d")},
			wantErr:     true,
		},
		{
			name:        "job exists",
			job:         entity.Job{UUID: gen.UUIDv5("a", "b")},
			publication: &entity.Publication{UUID: gen.UUIDv5("c", "d")},
			wantErr:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			storer.SetJob(ctx, tt.job)

			err := storer.SetPublication(ctx, tt.job.UUID, tt.publication)
			if tt.wantErr && err != nil {
				if !errors.Is(err, errs.ErrJobIDEmpty) {
					t.Fatal("expected ErrJobIDEmpty, got")
				}
			}

			publication, err := storer.GetPublicationByID(ctx, tt.publication.UUID)
			if tt.wantErr && err != nil {
				if !errors.Is(err, errs.ErrPublicationNotFound) {
					t.Fatal("expected ErrPublicationNotFound, got")
				}
			}

			if !tt.wantErr {
				if publication != tt.publication {
					t.Errorf("expected publication to have same pointer, got %v", publication)
				}
			}
		})
	}
}
