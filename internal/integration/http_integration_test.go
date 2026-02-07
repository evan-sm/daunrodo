//go:build integration
// +build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"daunrodo/internal/config"
	"daunrodo/internal/entity"
	httprouter "daunrodo/internal/infrastructure/delivery/http"
	"daunrodo/internal/service"
)

type httpIntegrationFixture struct {
	base   *ytdlpIntegrationFixture
	client *http.Client
	url    string
}

type apiResponse struct {
	Message string          `json:"message"`
	Error   string          `json:"error"`
	Data    json.RawMessage `json:"data"`
}

func newHTTPIntegrationFixture(
	t *testing.T,
	mode string,
	mutateCfg func(cfg *config.Config),
) *httpIntegrationFixture {
	t.Helper()

	base := newYTdlpIntegrationFixture(t, mode)
	if mutateCfg != nil {
		mutateCfg(base.cfg)
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := service.New(base.cfg, log, base.downloader, base.storer, nil)
	workerCtx, cancel := context.WithCancel(t.Context())
	svc.Start(workerCtx)

	router := httprouter.New(log, base.cfg, svc, base.storer, nil)
	server := httptest.NewServer(router)
	client := server.Client()
	client.Timeout = 3 * time.Second

	t.Cleanup(func() {
		cancel()
		server.Close()
	})

	return &httpIntegrationFixture{
		base:   base,
		client: client,
		url:    server.URL,
	}
}

func postEnqueue(t *testing.T, fx *httpIntegrationFixture, url, preset string) (int, apiResponse) {
	t.Helper()

	payload, err := json.Marshal(map[string]string{
		"url":    url,
		"preset": preset,
	})
	if err != nil {
		t.Fatalf("marshal enqueue payload: %v", err)
	}

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		fx.url+"/v1/jobs/enqueue",
		bytes.NewReader(payload),
	)
	if err != nil {
		t.Fatalf("new enqueue request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := fx.client.Do(req)
	if err != nil {
		t.Fatalf("do enqueue request: %v", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode, decodeAPIResponse(t, resp)
}

func getJob(t *testing.T, fx *httpIntegrationFixture, jobID string) (int, entity.Job) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, fx.url+"/v1/jobs/"+jobID, nil)
	if err != nil {
		t.Fatalf("new get job request: %v", err)
	}

	resp, err := fx.client.Do(req)
	if err != nil {
		t.Fatalf("do get job request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, entity.Job{}
	}

	decoded := decodeAPIResponse(t, resp)

	var job entity.Job
	if err := json.Unmarshal(decoded.Data, &job); err != nil {
		t.Fatalf("unmarshal job response data: %v", err)
	}

	return resp.StatusCode, job
}

func waitForJobStatus(
	t *testing.T,
	fx *httpIntegrationFixture,
	jobID string,
	timeout time.Duration,
	want entity.JobStatus,
) entity.Job {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var last entity.Job

	for time.Now().Before(deadline) {
		statusCode, job := getJob(t, fx, jobID)
		if statusCode == http.StatusOK {
			last = job
			if job.Status == want {
				return job
			}
		}

		time.Sleep(75 * time.Millisecond)
	}

	t.Fatalf("wait for job status %q timed out, last status %q", want, last.Status)

	return entity.Job{}
}

func decodeAPIResponse(t *testing.T, resp *http.Response) apiResponse {
	t.Helper()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	var decoded apiResponse
	if len(body) == 0 {
		return decoded
	}

	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal response body: %v body=%q", err, string(body))
	}

	return decoded
}

func decodeJobID(t *testing.T, response apiResponse) string {
	t.Helper()

	var id string
	if err := json.Unmarshal(response.Data, &id); err != nil {
		t.Fatalf("unmarshal job id: %v", err)
	}

	if id == "" {
		t.Fatalf("job id is empty")
	}

	return id
}

func TestHTTPEnqueuePollAndDownload(t *testing.T) {
	fx := newHTTPIntegrationFixture(t, "success", nil)

	statusCode, enqueueResp := postEnqueue(t, fx, "https://example.com/watch?v=vid-123", "mp4")
	if statusCode != http.StatusAccepted {
		t.Fatalf("expected enqueue status %d, got %d", http.StatusAccepted, statusCode)
	}

	jobID := decodeJobID(t, enqueueResp)

	statusCode, duplicateResp := postEnqueue(t, fx, "https://example.com/watch?v=vid-123", "mp4")
	if statusCode != http.StatusOK {
		t.Fatalf("expected duplicate enqueue status %d, got %d", http.StatusOK, statusCode)
	}

	duplicateID := decodeJobID(t, duplicateResp)
	if duplicateID != jobID {
		t.Fatalf("expected duplicate job id %q, got %q", jobID, duplicateID)
	}

	job := waitForJobStatus(t, fx, jobID, 5*time.Second, entity.JobStatusFinished)
	if len(job.Publications) != 1 {
		t.Fatalf("expected 1 publication, got %d", len(job.Publications))
	}

	publication := job.Publications[0]

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, fx.url+"/v1/files/"+publication.UUID, nil)
	if err != nil {
		t.Fatalf("new file download request: %v", err)
	}

	resp, err := fx.client.Do(req)
	if err != nil {
		t.Fatalf("do file download request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected file download status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	contentDisposition := resp.Header.Get("Content-Disposition")
	expectedFileName := filepath.Base(fx.base.outputFile)
	if !strings.Contains(contentDisposition, expectedFileName) {
		t.Fatalf("expected Content-Disposition with %q, got %q", expectedFileName, contentDisposition)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read downloaded body: %v", err)
	}

	if string(body) != "fake-media-bytes" {
		t.Fatalf("unexpected downloaded body %q", string(body))
	}
}

func TestHTTPCancelRunningJob(t *testing.T) {
	fx := newHTTPIntegrationFixture(t, "slow", func(cfg *config.Config) {
		cfg.Job.Timeout = 15 * time.Second
	})

	statusCode, enqueueResp := postEnqueue(t, fx, "https://example.com/watch?v=cancel-me", "mp4")
	if statusCode != http.StatusAccepted {
		t.Fatalf("expected enqueue status %d, got %d", http.StatusAccepted, statusCode)
	}

	jobID := decodeJobID(t, enqueueResp)
	_ = waitForJobStatus(t, fx, jobID, 5*time.Second, entity.JobStatusDownloading)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodDelete, fx.url+"/v1/jobs/"+jobID, nil)
	if err != nil {
		t.Fatalf("new cancel request: %v", err)
	}

	resp, err := fx.client.Do(req)
	if err != nil {
		t.Fatalf("do cancel request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected cancel status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	job := waitForJobStatus(t, fx, jobID, 5*time.Second, entity.JobStatusCancelled)
	if job.Status == entity.JobStatusFinished {
		t.Fatalf("expected canceled job, got finished")
	}
}

func TestHTTPValidationErrors(t *testing.T) {
	fx := newHTTPIntegrationFixture(t, "success", nil)

	tests := []struct {
		name       string
		body       string
		statusCode int
	}{
		{
			name:       "invalid json body",
			body:       "{",
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "invalid url",
			body:       `{"url":"not-a-url","preset":"mp4"}`,
			statusCode: http.StatusUnprocessableEntity,
		},
		{
			name:       "missing preset",
			body:       `{"url":"https://example.com/watch?v=vid-123","preset":""}`,
			statusCode: http.StatusUnprocessableEntity,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				fx.url+"/v1/jobs/enqueue",
				strings.NewReader(test.body),
			)
			if err != nil {
				t.Fatalf("new request: %v", err)
			}

			req.Header.Set("Content-Type", "application/json")

			resp, err := fx.client.Do(req)
			if err != nil {
				t.Fatalf("do request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != test.statusCode {
				t.Fatalf("expected status %d, got %d", test.statusCode, resp.StatusCode)
			}
		})
	}
}

func TestHTTPQueueFull(t *testing.T) {
	fx := newHTTPIntegrationFixture(t, "slow", func(cfg *config.Config) {
		cfg.Job.Workers = 0
		cfg.Job.QueueSize = 1
	})

	statusCode, _ := postEnqueue(t, fx, "https://example.com/watch?v=queue-1", "mp4")
	if statusCode != http.StatusAccepted {
		t.Fatalf("expected first enqueue status %d, got %d", http.StatusAccepted, statusCode)
	}

	statusCode, resp := postEnqueue(t, fx, "https://example.com/watch?v=queue-2", "mp4")
	if statusCode != http.StatusInternalServerError {
		t.Fatalf("expected second enqueue status %d, got %d", http.StatusInternalServerError, statusCode)
	}

	if !strings.Contains(resp.Error, "job queue is full") {
		t.Fatalf("expected queue-full error message, got %q", resp.Error)
	}
}

func TestHTTPDownloadPublicationFileNotFound(t *testing.T) {
	fx := newHTTPIntegrationFixture(t, "success", nil)

	statusCode, enqueueResp := postEnqueue(t, fx, "https://example.com/watch?v=file-not-found", "mp4")
	if statusCode != http.StatusAccepted {
		t.Fatalf("expected enqueue status %d, got %d", http.StatusAccepted, statusCode)
	}

	jobID := decodeJobID(t, enqueueResp)
	job := waitForJobStatus(t, fx, jobID, 5*time.Second, entity.JobStatusFinished)
	if len(job.Publications) != 1 {
		t.Fatalf("expected 1 publication, got %d", len(job.Publications))
	}

	if err := os.Remove(fx.base.outputFile); err != nil {
		t.Fatalf("remove downloaded file: %v", err)
	}

	pubID := job.Publications[0].UUID
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, fx.url+"/v1/files/"+pubID, nil)
	if err != nil {
		t.Fatalf("new file request: %v", err)
	}

	resp, err := fx.client.Do(req)
	if err != nil {
		t.Fatalf("do file request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status %d, got %d body=%s", http.StatusNotFound, resp.StatusCode, string(body))
	}

	decoded := decodeAPIResponse(t, resp)
	if decoded.Message == "" {
		t.Fatalf("expected not found response message")
	}
}
