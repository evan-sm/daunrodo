package service_test

import (
	"context"
	"daunrodo/internal/config"
	"daunrodo/internal/downloader"
	"daunrodo/internal/entity"
	"daunrodo/internal/errs"
	"daunrodo/internal/service"
	"daunrodo/internal/storage"
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

func NewTestService(t *testing.T, cfg *config.Config) *service.Job {
	t.Helper()

	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	downloader := downloader.NewMock(log)
	storage := storage.New(t.Context(), log, NewTestCfg(testConfigPresetDefault))
	svc := service.New(cfg, log, downloader, storage).(*service.Job)

	return svc
}

func NewTestCfg(preset string) *config.Config {
	switch preset {
	case "empty", "default":
		cfg, _ := config.New()

		return cfg
	case "negative", "wrong":
		cfg, _ := config.New()
		cfg.Job.Workers = -1
		cfg.Storage.TTL = -1
		cfg.Job.Timeout = -1

		return cfg
	case "custom":
		cfg, _ := config.New()
		cfg.Job.Workers = 2
		cfg.Storage.TTL = 2 * time.Hour
		cfg.Job.Timeout = 5 * time.Minute
		cfg.Job.QueueSize = 100

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
			svc := NewTestService(t, tt.cfg)

			if svc.Cfg.Job.Workers != tt.cfg.Job.Workers {
				t.Errorf("expected %d workers, got %d", tt.cfg.Job.Workers, svc.Cfg.Job.Workers)
			}

			if cap(svc.JobQueue) != tt.cfg.Job.QueueSize {
				t.Errorf("expected %d queue size, got %d", tt.cfg.Job.QueueSize, cap(svc.JobQueue))
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
				svc := NewTestService(t, cfg)

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

				job, exists := svc.Storer.GetJobByURLAndPreset(ctx, testURL, testPresetMP4)
				if exists && job.Status != entity.JobStatusStarting {
					t.Errorf("expected job to be started")
				}

				cancel()
				synctest.Wait()

				_, err = svc.Enqueue(ctx, testURL, testPresetMP4)
				if !errors.Is(err, errs.ErrServiceClosed) {
					t.Errorf("expected: %v got: %v", errs.ErrServiceClosed, err)
				}
			})
		})
	}
}

func TestWorker(t *testing.T) {
	tests := []struct {
		name           string
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
				svc := NewTestService(t, cfg)

				ctx, cancel := context.WithTimeout(t.Context(), tc.timeout)
				defer cancel()

				svc.Start(ctx)

				job, err := svc.Enqueue(ctx, testURL, testPresetMP4)
				if err != nil {
					t.Fatalf("failed to enqueue job: %v", err)
				}

				validateInitialJobStatus(t, job)

				<-ctx.Done()
				synctest.Wait()

				storedJob, exists := svc.Storer.GetJobByURLAndPreset(ctx, testURL, testPresetMP4)
				if !exists {
					t.Fatalf("failed to get job")
				}

				if tc.expectDeadline {
					assertJobDeadline(t, &storedJob, tc.expectStatus, tc.expectProgress, ctx.Err())
				} else {
					assertJobFinished(t, &storedJob, tc.expectStatus)
				}
			})
		})
	}
}

func validateInitialJobStatus(t *testing.T, job *entity.Job) {
	t.Helper()

	switch job.Status {
	case entity.JobStatusStarting, entity.JobStatusDownloading:
		return
	default:
		t.Errorf("unexpected job status immediately after enqueue: %v", job.Status)
	}
}

func assertJobDeadline(t *testing.T, job *entity.Job,
	expectedStatus entity.JobStatus,
	expectedProgress int,
	ctxErr error) {
	t.Helper()

	if job.Status != expectedStatus {
		t.Errorf("expected job to be failed, got %v", job.Status)
	}

	if job.Progress != expectedProgress {
		t.Errorf("expected job progress %d, got %d", expectedProgress, job.Progress)
	}

	if !errors.Is(ctxErr, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got: %v", ctxErr)
	}
}

func assertJobFinished(t *testing.T, job *entity.Job, expectedStatus entity.JobStatus) {
	t.Helper()

	if job.Status != expectedStatus {
		t.Errorf("expected job to be finished, got %v", job.Status)
	}
}

// func TestWorker(t *testing.T) {
// 	tests := []struct {
// 		name           string
// 		step           time.Duration
// 		timeout        time.Duration
// 		expectDeadline bool
// 		expectStatus   entity.JobStatus
// 		expectProgress int
// 	}{
// 		{
// 			name:           "should finish in time",
// 			timeout:        2 * time.Second,
// 			expectDeadline: false,
// 			expectStatus:   entity.JobStatusFinished,
// 		},
// 		{
// 			name:           "should timeout",
// 			timeout:        500 * time.Millisecond,
// 			expectDeadline: true,
// 			expectStatus:   entity.JobStatusError,
// 			expectProgress: 40,
// 		},
// 	}

// 	for _, tc := range tests {
// 		t.Run(tc.name, func(t *testing.T) {
// 			synctest.Test(t, func(t *testing.T) {
// 				cfg := NewTestCfg(testConfigPresetCustom)
// 				svc := NewTestService(t, cfg)

// 				ctx, cancel := context.WithTimeout(t.Context(), tc.timeout)
// 				defer cancel()

// 				svc.Start(ctx)

// 				job, err := svc.Enqueue(ctx, testURL, testPresetMP4)
// 				if err != nil {
// 					t.Errorf("failed to enqueue job: %v", err)
// 				}

// 				switch job.Status {
// 				case entity.JobStatusStarting, entity.JobStatusDownloading:
// 					break
// 				default:
// 					t.Errorf("unexpected job status: %v", job.Status)
// 				}

// 				<-ctx.Done()
// 				synctest.Wait()

// 				ctxErr := ctx.Err()
// 				if tc.expectDeadline {
// 					job = svc.Storer.GetJobByURLAndPreset(ctx, testURL, testPresetMP4)
// 					if job == nil {
// 						t.Errorf("failed to get job: %v", err)
// 					}

// 					if job != nil {
// 						if job.Status != tc.expectStatus {
// 							t.Errorf("expected job to be failed")
// 						}

// 						if job.Progress != tc.expectProgress {
// 							t.Errorf("expected job progress to be %d, got: %d", tc.expectProgress, job.Progress)
// 						}

// 						if !errors.Is(ctxErr, context.DeadlineExceeded) {
// 							t.Errorf("expected context.DeadlineExceeded, got: %v", ctxErr)
// 						}
// 					}
// 				}

// 				if !tc.expectDeadline {
// 					job := svc.Storer.GetJobByURLAndPreset(ctx, testURL, testPresetMP4)
// 					if job == nil {
// 						t.Errorf("failed to get job: %v", err)
// 					}

// 					if job != nil {
// 						if job.Status != tc.expectStatus {
// 							t.Errorf("expected job to be finished")
// 						}
// 					}
// 				}
// 			})
// 		})
// 	}
// }

func TestEnqueue(t *testing.T) {
	svc := NewTestService(t, NewTestCfg(testConfigPresetDefault))

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	svc.Start(ctx)

	job, err := svc.Enqueue(ctx, testURL, testPresetMP4)
	if err != nil {
		t.Errorf("failed to enqueue job: %v", err)
	}

	_, exists := svc.Storer.GetJobByURLAndPreset(ctx, testURL, testPresetMP4)
	if !exists {
		t.Errorf("expected job to be found")
	}

	if exists && job.Status != entity.JobStatusStarting {
		t.Errorf("expected job to be started")
	}

	_, exists = svc.Storer.GetJobByID(ctx, job.UUID)
	if !exists {
		t.Errorf("expected job to be found")
	}

	if exists && job.Status != entity.JobStatusStarting {
		t.Errorf("expected job to be started")
	}

	_, exists = svc.Storer.GetJobByURLAndPreset(ctx, testURL2, testPresetMP4)
	if exists {
		t.Errorf("expected error for non-existent job, got: %v", err)
	}
}
