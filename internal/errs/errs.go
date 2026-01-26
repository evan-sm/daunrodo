// Package errs defines common error variables used across the application.
package errs

import "errors"

var (
	// ErrServiceClosed indicates that the service is closed and cannot accept new jobs.
	ErrServiceClosed = errors.New("service is closed")
	// ErrInvalidRequestBody indicates that the request body is invalid or cannot be parsed.
	ErrInvalidRequestBody = errors.New("invalid request body")
)

// Valid request errors.
var (
	// ErrInvalidURL indicates that the URL field in the request is invalid.
	ErrInvalidURL = errors.New("invalid url field")
	// ErrInvalidPreset indicates that the preset field in the request is invalid.
	ErrInvalidPreset = errors.New("invalid preset field")
)

// Job and storage errors.
var (
	// ErrNoJobs indicates that there are no jobs in storage.
	ErrNoJobs = errors.New("no jobs")
	// ErrJobAlreadyExists indicates that the job already exists in storage with the same URL and preset.
	ErrJobAlreadyExists = errors.New("job already exists")
	// ErrJobNotFound indicates that the job is not found in storage.
	ErrJobNotFound = errors.New("job not found")
	// ErrJobNil indicates that the job is nil.
	ErrJobNil = errors.New("job is nil")
	// ErrJobIDEmpty indicates that the job ID is empty.
	ErrJobIDEmpty = errors.New("job_id is empty")
	// ErrJobCancelled indicates that the job was cancelled.
	ErrJobCancelled = errors.New("job cancelled")
	// ErrJobQueueFull indicates that the job queue is full.
	ErrJobQueueFull = errors.New("job queue is full")
)

// Publication errors.
var (
	// ErrPublicationNil indicates that the publication is nil.
	ErrPublicationNil = errors.New("publication is nil")
	// ErrPublicationUUID indicates that the publication UUID is invalid.
	ErrPublicationUUID = errors.New("publication UUID is invalid")
	// ErrPublicationNotFound indicates that the publication was not found in storage.
	ErrPublicationNotFound = errors.New("publication not found")
)

// Downloader errors.
var (
	// ErrDownloadFailed indicates that the download failed.
	ErrDownloadFailed = errors.New("download failed")
	// ErrDownloaderNotFound indicates that no suitable downloader was found.
	ErrDownloaderNotFound = errors.New("no suitable downloader found")
	// ErrBinaryNotFound indicates that the required binary was not found.
	ErrBinaryNotFound = errors.New("binary not found")
	// ErrUnsupportedPlatform indicates that the current platform is not supported.
	ErrUnsupportedPlatform = errors.New("unsupported platform")
)

// Proxy errors.
var (
	// ErrNoProxiesAvailable indicates that no proxies are available.
	ErrNoProxiesAvailable = errors.New("no proxies available")
	// ErrProxyFailed indicates that the proxy request failed.
	ErrProxyFailed = errors.New("proxy failed")
)
