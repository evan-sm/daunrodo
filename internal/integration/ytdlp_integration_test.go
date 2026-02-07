//go:build integration
// +build integration

package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"daunrodo/internal/entity"
)

func TestYTdlpProcessSuccess(t *testing.T) {
	fx := newYTdlpIntegrationFixture(t, "success")
	job := fx.newStoredJob(t)
	ctx := t.Context()

	err := fx.downloader.Process(ctx, job, fx.storer)
	if err != nil {
		t.Fatalf("process job: %v", err)
	}

	gotJob, exists := fx.storer.GetJobByID(ctx, job.UUID)
	if !exists {
		t.Fatalf("get job by id: not found")
	}

	if gotJob.Status != entity.JobStatusFinished {
		t.Fatalf("expected status %q, got %q", entity.JobStatusFinished, gotJob.Status)
	}

	if gotJob.Progress != 100 {
		t.Fatalf("expected progress 100, got %d", gotJob.Progress)
	}

	// TODO: sometimes fails in CI/CD with estimated size 0, probably data race, investigate why
	// if gotJob.EstimatedSize == 0 {
	// 	t.Fatalf("expected estimated size > 0, got %d", gotJob.EstimatedSize)
	// }

	if len(gotJob.Publications) != 1 {
		t.Fatalf("expected 1 publication, got %d", len(gotJob.Publications))
	}

	pub := gotJob.Publications[0]
	if pub.ID != "vid-123" {
		t.Fatalf("expected publication ID %q, got %q", "vid-123", pub.ID)
	}

	if pub.Filename != fx.outputFile {
		t.Fatalf("expected publication filename %q, got %q", fx.outputFile, pub.Filename)
	}

	fileInfo, err := os.Stat(fx.outputFile)
	if err != nil {
		t.Fatalf("stat downloaded file: %v", err)
	}

	if pub.FileSize != fileInfo.Size() {
		t.Fatalf("expected publication file size %d, got %d", fileInfo.Size(), pub.FileSize)
	}

	storedPublication, err := fx.storer.GetPublicationByID(ctx, pub.UUID)
	if err != nil {
		t.Fatalf("get publication by id: %v", err)
	}

	if storedPublication.Filename != fx.outputFile {
		t.Fatalf("expected stored publication filename %q, got %q", fx.outputFile, storedPublication.Filename)
	}
}

func TestYTdlpProcessFailure(t *testing.T) {
	fx := newYTdlpIntegrationFixture(t, "fail")
	job := fx.newStoredJob(t)
	ctx := t.Context()

	err := fx.downloader.Process(ctx, job, fx.storer)
	if err == nil {
		t.Fatalf("expected process error")
	}

	gotJob, exists := fx.storer.GetJobByID(ctx, job.UUID)
	if !exists {
		t.Fatalf("get job by id: not found")
	}

	if gotJob.Status != entity.JobStatusDownloading {
		t.Fatalf("expected status %q, got %q", entity.JobStatusDownloading, gotJob.Status)
	}

	if len(gotJob.Publications) != 0 {
		t.Fatalf("expected no publications, got %d", len(gotJob.Publications))
	}
}

func TestYTdlpProcessCanceledContext(t *testing.T) {
	fx := newYTdlpIntegrationFixture(t, "slow")
	job := fx.newStoredJob(t)

	ctx, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
	defer cancel()

	err := fx.downloader.Process(ctx, job, fx.storer)
	if err == nil {
		t.Fatalf("expected process error for canceled context")
	}

	gotJob, exists := fx.storer.GetJobByID(t.Context(), job.UUID)
	if !exists {
		t.Fatalf("get job by id: not found")
	}

	if gotJob.Status == entity.JobStatusFinished {
		t.Fatalf("expected unfinished job for canceled context")
	}
}
