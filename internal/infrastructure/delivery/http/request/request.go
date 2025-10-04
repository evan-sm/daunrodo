// Package request defines the HTTP request payloads and their validation logic.
package request

import (
	"daunrodo/internal/errs"
	"daunrodo/pkg/urls"
)

// Enqueue represents the request payload for enqueuing a new download job.
type Enqueue struct {
	URL string `json:"url"`
	// e.g. "mp4", "aac", see: https://github.com/yt-dlp/yt-dlp?tab=readme-ov-file#preset-aliases
	Preset string `json:"preset"`
}

// Validate checks if the Enqueue request has valid fields.
func (e *Enqueue) Validate() error {
	if !urls.IsURLValid(e.URL) {
		return errs.ErrInvalidURL
	}

	if e.Preset == "" {
		return errs.ErrInvalidPreset
	}

	return nil
}
