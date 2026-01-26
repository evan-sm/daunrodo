// Package entity defines the core entities used in the application.
package entity

import (
	"log/slog"
	"time"
)

// JobStatus represents the status of a download job.
type JobStatus string

const (
	// JobStatusStarting indicates that the job is accepted and is about to start.
	JobStatusStarting JobStatus = "starting"
	// JobStatusDownloading indicates that the job is in progress.
	JobStatusDownloading JobStatus = "downloading"
	// JobStatusError indicates that the job has encountered an error.
	JobStatusError JobStatus = "error"
	// JobStatusFinished indicates that the job has finished successfully.
	JobStatusFinished JobStatus = "finished"
	// JobStatusCancelled indicates that the job was cancelled by the user.
	JobStatusCancelled JobStatus = "cancelled"
)

// Job represents a download job.
type Job struct {
	UUID          string        `json:"uuid"`
	URL           string        `json:"url"`
	Preset        string        `json:"preset"`
	Status        JobStatus     `json:"status"`
	Progress      int           `json:"progress"`
	Publications  []Publication `json:"publications,omitempty"`
	Error         string        `json:"error,omitempty"`
	EstimatedETA  time.Duration `json:"estimatedEta"`
	EstimatedSize int64         `json:"estimatedSize,omitempty"` // Estimated total file size in bytes
	TotalSize     int64         `json:"totalSize,omitempty"`     // Actual total size after download
	CreatedAt     time.Time     `json:"createdAt"`
	UpdatedAt     time.Time     `json:"updatedAt"`
	ExpiresAt     time.Time     `json:"expiresAt"`
}

// LogValue implements the slog.LogValuer interface for structured logging.
func (j Job) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("uuid", j.UUID),
		slog.String("url", j.URL),
		slog.String("status", string(j.Status)),
		slog.Int("progress", j.Progress),
		slog.Duration("estimatedEta", j.EstimatedETA),
		slog.Int64("estimatedSize", j.EstimatedSize),
		slog.Int64("totalSize", j.TotalSize),
	)
}

// File represents a downloaded file.
// type File struct {
// 	ID       string `json:"id"`
// 	URL      string `json:"url"`
// 	Filename string `json:"filename"`
// 	Size     int64  `json:"size"`
// 	MimeType string `json:"mimeType"`
// 	Path     string `json:"path"`
// 	Duration int    `json:"duration"` // Duration in seconds (for videos)
// 	Width    int    `json:"width"`
// 	Height   int    `json:"height"`
// }

// Publication represents a social media publication/post.
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
	FileSize int64  `json:"fileSize"`
	Duration int    `json:"duration"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Filename string `json:"filename"`
	// CreatedAt   time.Time `json:"createdAt"`
	// UpdatedAt   time.Time `json:"updatedAt"`
}

// LogValue implements the slog.LogValuer interface for structured logging.
func (p Publication) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("uuid", p.UUID),
		slog.String("id", p.ID),
		slog.String("type", p.Type),
		slog.String("platform", p.Platform),
		slog.String("channel", p.Channel),
		slog.String("webpage_url", p.WebpageURL),
		slog.String("title", p.Title),
		slog.String("description", p.Description),
		slog.String("author", p.Author),
		slog.Int("viewCount", p.ViewCount),
		slog.Int("likeCount", p.LikeCount),
		slog.String("thumbnail_url", p.ThumbnailURL),
		slog.Int64("file_size", p.FileSize),
		slog.Int("duration", p.Duration),
		slog.Int("width", p.Width),
		slog.Int("height", p.Height),
		slog.String("filename", p.Filename),
	)
}
