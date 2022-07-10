package errs

import "errors"

var (
	ErrServiceClosed      = errors.New("service is closed")
	ErrInvalidRequestBody = errors.New("invalid request body")
)

// Valid request errors
var (
	ErrInvalidURL    = errors.New("invalid url field")
	ErrInvalidPreset = errors.New("invalid preset field")
)

var (
	ErrNoJobs           = errors.New("no jobs")
	ErrJobAlreadyExists = errors.New("job already exists")
	ErrJobNotFound      = errors.New("job not found")
)
