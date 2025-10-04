package storage_test

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"daunrodo/internal/config"
	"daunrodo/internal/entity"
	"daunrodo/internal/storage"
	"daunrodo/pkg/gen"
)

// testFileGenerator returns a function that generates testN.txt files
// inside dir, with counter reset for each generator instance.
func testFileGenerator(dir string) func() string {
	counter := 0

	return func() string {
		counter++
		filename := filepath.Join(dir, fmt.Sprintf("test%d.txt", counter))

		if err := os.WriteFile(filename, []byte("test"), 0600); err != nil {
			panic(fmt.Sprintf("failed to create %s: %v", filename, err))
		}

		return filename
	}
}

// uuidGenerator generates seeds "1","2", then "3","4", then "5","6", ... on each call.
func uuidGenerator() func() string {
	counter := 1

	return func() string {
		a := strconv.Itoa(counter)
		b := strconv.Itoa(counter + 1)
		counter += 2

		return gen.UUIDv5(a, b)
	}
}

func TestCleanupExpiredJobs(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tmpDir := t.TempDir()
	cleanupInterval := time.Minute
	testFile := testFileGenerator(tmpDir)
	uuid := uuidGenerator()

	tests := []struct {
		name    string
		cfg     *config.Config
		wantErr bool
	}{
		{
			name:    "cleanup expired jobs",
			cfg:     &config.Config{Storage: config.Storage{CleanupInterval: cleanupInterval}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx := t.Context()
				now := time.Now()

				expiredJob := &entity.Job{
					UUID:      uuid(),
					CreatedAt: now,
					UpdatedAt: now,
					ExpiresAt: now.Add(-1 * cleanupInterval),
					Publications: []entity.Publication{
						{
							UUID:     uuid(),
							Filename: testFile(),
						},
						{
							UUID:     uuid(),
							Filename: testFile(),
						},
					},
				}

				newJob := &entity.Job{
					UUID:      uuid(),
					CreatedAt: now,
					UpdatedAt: now,
					ExpiresAt: now.Add(cleanupInterval + 4*time.Minute),
					Publications: []entity.Publication{
						{
							UUID:     uuid(),
							Filename: testFile(),
						},
					},
				}

				newJob2Files := &entity.Job{
					UUID:      uuid(),
					CreatedAt: now,
					UpdatedAt: now,
					ExpiresAt: now.Add(cleanupInterval + 4*time.Minute),
					Publications: []entity.Publication{
						{
							UUID:     uuid(),
							Filename: testFile(),
						},
						{
							UUID:     uuid(),
							Filename: testFile(),
						},
					},
				}

				jobs := []*entity.Job{expiredJob, newJob, newJob2Files}

				storer := storage.New(ctx, log, tt.cfg)

				for _, job := range jobs {
					storer.SetJob(ctx, job)

					for _, pub := range job.Publications {
						err := storer.SetPublication(ctx, job.UUID, &pub)
						if err != nil {
							t.Fatalf("failed to set publication: %v", err)
						}
					}
				}

				time.Sleep(cleanupInterval + time.Minute)

				expired := storer.GetJobByID(ctx, expiredJob.UUID)
				if expired != nil {
					t.Fatal("expected job to be cleaned up, but found")
				}

				foundNew := storer.GetJobByID(ctx, newJob.UUID)
				if foundNew == nil {
					t.Fatal("expected new job to be present, but found none")
				}

				job2files := storer.GetJobByID(ctx, newJob2Files.UUID)
				if job2files == nil {
					t.Fatal("expected job to be present, but found none")
				}
			})
		})
	}
}
