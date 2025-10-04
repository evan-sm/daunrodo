// Package consts defines application-wide constants.
package consts

import "time"

const (
	// DefaultHandlerTimeout is the default timeout for HTTP handlers.
	DefaultHandlerTimeout = 30 * time.Second
	// DefaultJobTimeout is the default timeout for job processing.
	DefaultJobTimeout = 5 * time.Minute
	// DefaultJobWorkers is the default number of workers for job processing.
	DefaultJobWorkers = 2
	// DefaultQueueSize is the default size of the job queue.
	DefaultQueueSize = 50
	// DefaultSimulateTime is the default time to simulate processing in mock downloader.
	DefaultSimulateTime = 1 * time.Second
	// DefaultJobTTL is the default time-to-live for stored jobs and files.
	DefaultJobTTL = 7 * 24 * time.Hour
)

// HTTP response messages.
const (
	// RespInvalidRequestBody is returned when the request body is invalid.
	RespInvalidRequestBody = "invalid request body"
	// RespQueryParamMissing is returned when a required query parameter is missing or invalid.
	RespQueryParamMissing = "query param missing or invalid"
	// RespUnprocessableEntity is returned when the request cannot be processed.
	RespUnprocessableEntity = "unprocessable entity"
	// RespJobEnqueued is returned when a job is successfully enqueued.
	RespJobEnqueued = "job enqueued"
	// RespJobEnqueueFail is returned when a job cannot be enqueued.
	RespJobEnqueueFail = "job enqueue failed"
	// RespGetJobsFail is returned when fetching all jobs fails.
	RespGetJobsFail = "get all jobs failed"
	// RespGetJobFail is returned when fetching a specific job fails.
	RespGetJobFail = "get job failed"
	// RespNoJobs is returned when there are no jobs available.
	RespNoJobs = "no jobs"
	// RespJobRetrieved is returned when a job is successfully retrieved.
	RespJobRetrieved = "job retrieved"
	// RespJobsRetrieved is returned when jobs are successfully retrieved.
	RespJobsRetrieved = "jobs retrieved"
	// RespJobNotFound is returned when a job is not found.
	RespJobNotFound = "job not found"
	// RespJobAlreadyExists is returned when a job already exists.
	RespJobAlreadyExists = "job already exists"
	// RespPublicationNotFound is returned when a publication is not found.
	RespPublicationNotFound = "publication not found"
	// RespPublicationDownloadFailed is returned when a publication download fails.
	RespPublicationDownloadFailed = "publication download failed"
)

// Downloader identifiers.
const (
	// DownloaderYTdlp is the youtube-dl downloader identifier.
	DownloaderYTdlp = "ytdlp"
	// DownloaderNative is the native downloader identifier.
	DownloaderNative = "native"
	// DownloaderMock is the mock downloader identifier for testing.
	DownloaderMock = "mock"
)

// Files.
const (
	// RespFileNotFound is returned when a file is not found.
	RespFileNotFound = "file not found"
)
