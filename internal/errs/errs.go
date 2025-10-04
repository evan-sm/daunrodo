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
	// ErrPublicationNil indicates that the publication is nil.
	ErrPublicationNil = errors.New("publication is nil")
	// ErrPublicationUUID indicates that the publication UUID is invalid.
	ErrPublicationUUID = errors.New("publication UUID is invalid")
	// ErrPublicationNotFound indicates that the publication was not found in storage.
	ErrPublicationNotFound = errors.New("publication not found")
	// ErrJobQueueFull indicates that the job queue is full.
	ErrJobQueueFull = errors.New("job queue is full")
)
