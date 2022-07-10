package entity

import (
	"log/slog"
	"time"
)

// JobStatus represents the status of a download job
type JobStatus string

const (
	JobStatusStarting       JobStatus = "starting"
	JobStatusDownloading    JobStatus = "downloading"
	JobStatusPostProcessing JobStatus = "post_processing"
	JobStatusError          JobStatus = "error"
	JobStatusFinished       JobStatus = "finished"
)

// Job represents a download job
type Job struct {
	ID           string        `json:"id"`
	URL          string        `json:"url"`
	Preset       string        `json:"preset"`
	Status       JobStatus     `json:"status"`
	Progress     int           `json:"progress"`
	Publications []Publication `json:"publications,omitempty"`
	Error        string        `json:"error,omitempty"`
	EstimatedETA time.Duration `json:"estimatedEta"`
	CreatedAt    time.Time     `json:"createdAt"`
	UpdatedAt    time.Time     `json:"updatedAt"`
	ExpiresAt    time.Time     `json:"expiresAt"`
}

func (j Job) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("id", j.ID),
		slog.String("url", j.URL),
		slog.String("status", string(j.Status)),
		slog.Int("progress", j.Progress),
		slog.Duration("estimatedEta", j.EstimatedETA),
	)
}

// File represents a downloaded file
type File struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	MimeType string `json:"mimeType"`
	Path     string `json:"path"`
	Duration int    `json:"duration"` // Duration in seconds (for videos)
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

// Publication represents a social media publication/post
type Publication struct {
	UUID         string `json:"uuid"`
	ID           string `json:"id"`
	Type         string `json:"type"`
	Platform     string `json:"platform"`
	Channel      string `json:"channel"`
	WebpageURL   string `json:"webpageUrl"` // Original URL
	Title        string `json:"title"`
	Description  string `json:"description"`
	Author       string `json:"author"`
	ViewCount    int    `json:"viewCount"`
	LikeCount    int    `json:"likeCount"`
	ThumbnailURL string `json:"thumbnailUrl"`
	// Files        []File `json:"files"` // Associated files
	Duration int    `json:"duration"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Filename string `json:"filename"`
	// CreatedAt   time.Time `json:"createdAt"`
	// UpdatedAt   time.Time `json:"updatedAt"`
}
