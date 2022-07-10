package consts

import "time"

const (
	DefaultHandlerTimeout = 30 * time.Second
	DefaultJobTimeout     = 5 * time.Minute
	DefaultJobWorkers     = 2
	DefaultQueueSize      = 50
	DefaultSimulateTime   = 1 * time.Second
	DefaultJobTTL         = 7 * 24 * time.Hour
)

// http
const (
	RespInvalidRequestBody  = "invalid request body"
	RespQueryParamMissing   = "query param missing"
	RespUnprocessableEntity = "unprocessable entity"
	RespJobEnqueued         = "job enqueued"
	RespJobEnqueueFail      = "job enqueue failed"
	RespGetJobsFail         = "get all jobs failed"
	RespGetJobFail          = "get job failed"
	RespNoJobs              = "no jobs"
	RespJobRetrieved        = "job retrieved"
	RespJobsRetrieved       = "jobs retrieved"
	RespJobNotFound         = "job not found"
	RespJobAlreadyExists    = "job already exists"
)

// downloaders
const (
	DownloaderYTdlp  = "ytdlp"
	DownloaderNative = "native"
	DownloaderMock   = "mock"
)

const (
	CookiesFile = "cookies.txt"
)
