package request

import (
	"daunrodo/internal/errs"
	"daunrodo/pkg/urls"
)

type Enqueue struct {
	URL    string `json:"url"`
	Preset string `json:"preset"` // e.g. "mp4", "aac", see: https://github.com/yt-dlp/yt-dlp?tab=readme-ov-file#preset-aliases
}

func (e *Enqueue) Validate() error {
	if !urls.IsURLValid(e.URL) {
		return errs.ErrInvalidURL
	}
	if e.Preset == "" {
		return errs.ErrInvalidPreset
	}
	return nil
}
