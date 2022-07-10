package service

import (
	"context"
	"daunrodo/internal/config"
	"daunrodo/internal/downloader"
	"daunrodo/internal/entity"
	"daunrodo/internal/errs"
	"errors"
	"log/slog"
	"os"
	"testing"
	"testing/synctest"
	"time"
)

const (
	testURL  = "https://example.com/test.mp4"
	testURL2 = "https://example.com/test2.mp4"

	testPresetMP4 = "mp4"
	testPresetAAC = "aac"

	testConfigPresetDefault  = "default"
	testConfigPresetNegative = "negative"
	testConfigPresetCustom   = "custom"
)

func NewTestService(cfg *config.Config) *job {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	downloader := downloader.NewMock(log)
	svc := New(cfg, log, downloader).(*job)

	return svc
}

func NewTestCfg(preset string) *config.Config {
	switch preset {
	case "empty", "default":
		cfg, _ := config.New()
		return cfg
	case "negative", "wrong":
		cfg, _ := config.New()
		cfg.App.Job.Workers = -1
		cfg.App.Job.StorageTTL = -1
		cfg.App.Job.Timeout = -1

		return cfg
	case "custom":
		cfg, _ := config.New()
		cfg.App.Job.Workers = 2
		cfg.App.Job.StorageTTL = 2 * time.Hour
		cfg.App.Job.Timeout = 5 * time.Minute
		cfg.App.Job.QueueSize = 100

		return cfg
	default:
		cfg, _ := config.New()
		return cfg
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
	}{
		{
			name: "cfg is empty/default",
			cfg:  NewTestCfg(testConfigPresetDefault),
		},
		{
			name: "cfg is negative",
			cfg:  NewTestCfg(testConfigPresetNegative),
		},
		{
			name: "cfg is custom",
			cfg:  NewTestCfg(testConfigPresetCustom),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewTestService(tt.cfg)

			if svc.cfg.App.Job.Workers != tt.cfg.Job.Workers {
				t.Errorf("expected %d workers, got %d", tt.cfg.Job.Workers, svc.cfg.App.Job.Workers)
			}
			if cap(svc.jobQueue) != tt.cfg.App.Job.QueueSize {
				t.Errorf("expected %d queue size, got %d", tt.cfg.App.Job.QueueSize, cap(svc.jobQueue))
			}
		})
	}
}

func TestStartAndEnqueue(t *testing.T) {
	tests := []struct {
		name        string
		url         []string
		preset      []string
		expectError bool
	}{
		{
			name:        "same url, same preset",
			url:         []string{testURL, testURL},
			preset:      []string{testPresetMP4, testPresetMP4},
			expectError: true,
		},
		{
			name:        "same url, different preset",
			url:         []string{testURL, testURL},
			preset:      []string{testPresetMP4, testPresetAAC},
			expectError: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				cfg := NewTestCfg(testConfigPresetCustom)
				svc := NewTestService(cfg)

				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				svc.Start(ctx)

				// first enqueue
				_, err := svc.Enqueue(ctx, tc.url[0], tc.preset[0])
				if err != nil {
					t.Errorf("first enqueue failed: %v", err)
				}

				// second enqueue
				_, err = svc.Enqueue(ctx, tc.url[1], tc.preset[1])
				if tc.expectError {
					if !errors.Is(err, errs.ErrJobAlreadyExists) {
						t.Errorf("expected ErrJobAlreadyExists, got: %v", err)
					}
				}
				if !tc.expectError {
					if err != nil {
						t.Errorf("failed to enqueue job: %v", err)
					}
				}

				if job := svc.GetByURLAndPreset(ctx, testURL, testPresetMP4); job != nil && job.Status != entity.JobStatusStarting {
					t.Errorf("expected job to be started")
				}

				cancel()
				synctest.Wait()

				_, err = svc.Enqueue(ctx, testURL, testPresetMP4)
				if err != errs.ErrServiceClosed {
					t.Errorf("expected: %v got: %v", errs.ErrServiceClosed, err)
				}
			})
		})

	}

}

func TestWorker(t *testing.T) {
	tests := []struct {
		name           string
		step           time.Duration
		timeout        time.Duration
		expectDeadline bool
		expectStatus   entity.JobStatus
		expectProgress int
	}{
		{
			name:           "should finish in time",
			timeout:        2 * time.Second,
			expectDeadline: false,
			expectStatus:   entity.JobStatusFinished,
		},
		{
			name:           "should timeout",
			timeout:        500 * time.Millisecond,
			expectDeadline: true,
			expectStatus:   entity.JobStatusError,
			expectProgress: 40,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				cfg := NewTestCfg(testConfigPresetCustom)
				svc := NewTestService(cfg)

				ctx, cancel := context.WithTimeout(t.Context(), tc.timeout)
				defer cancel()

				svc.Start(ctx)

				job, err := svc.Enqueue(ctx, testURL, testPresetMP4)
				if err != nil {
					t.Errorf("failed to enqueue job: %v", err)
				}

				switch job.Status {
				case entity.JobStatusStarting, entity.JobStatusDownloading:
					break
				default:
					t.Errorf("unexpected job status: %v", job.Status)
				}

				<-ctx.Done()
				synctest.Wait()

				ctxErr := ctx.Err()
				if tc.expectDeadline {
					job = svc.GetByURLAndPreset(ctx, testURL, testPresetMP4)
					if job == nil {
						t.Errorf("failed to get job: %v", err)
					}
					if job != nil {
						if job.Status != tc.expectStatus {
							t.Errorf("expected job to be failed")
						}
						if job.Progress != tc.expectProgress {
							t.Errorf("expected job progress to be %d, got: %d", tc.expectProgress, job.Progress)
						}
						if ctxErr != context.DeadlineExceeded {
							t.Errorf("expected context.DeadlineExceeded, got: %v", ctxErr)
						}
					}
				}
				if !tc.expectDeadline {
					job := svc.GetByURLAndPreset(ctx, testURL, testPresetMP4)
					if job == nil {
						t.Errorf("failed to get job: %v", err)
					}
					if job != nil {
						if job.Status != tc.expectStatus {
							t.Errorf("expected job to be finished")
						}
					}
				}
			})
		})
	}
}

func TestGetJob(t *testing.T) {
	svc := NewTestService(NewTestCfg(testConfigPresetDefault))

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	svc.Start(ctx)

	job, err := svc.Enqueue(ctx, testURL, testPresetMP4)
	if err != nil {
		t.Errorf("failed to enqueue job: %v", err)
	}

	got := svc.GetByURLAndPreset(ctx, testURL, testPresetMP4)
	if got == nil && job.Status != entity.JobStatusStarting {
		t.Errorf("expected job to be started")
	}

	if got != job {
		t.Errorf("expected job pointer to be the same")
	}

	got = svc.GetByID(ctx, job.ID)
	if got == nil && job.Status != entity.JobStatusStarting {
		t.Errorf("expected job to be started")
	}

	job = svc.GetByURLAndPreset(ctx, testURL2, testPresetMP4)
	if job != nil {
		t.Errorf("expected error for non-existent job, got: %v", err)
	}
}
