// Package config handles application configuration loading and management.
package config

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config holds the application configuration.
type Config struct {
	HTTP    HTTP
	App     App
	Job     Job
	Dir     Dir
	Storage Storage
	Proxy   Proxy
}

// App holds application-wide configuration.
type App struct {
	LogLevel string `env:"DAUNRODO_APP_LOG_LEVEL" envDefault:"info"`
}

// Job holds job processing configuration.
type Job struct {
	Workers   int           `env:"DAUNRODO_APP_JOB_WORKERS"    envDefault:"2"`
	Timeout   time.Duration `env:"DAUNRODO_APP_JOB_TIMEOUT"    envDefault:"5m"`
	QueueSize int           `env:"DAUNRODO_APP_JOB_QUEUE_SIZE" envDefault:"100"`
}

// Storage holds storage configuration.
type Storage struct {
	TTL             time.Duration `env:"DAUNRODO_APP_STORAGE_TTL"              envDefault:"168h"`
	CleanupInterval time.Duration `env:"DAUNRODO_APP_STORAGE_CLEANUP_INTERVAL" envDefault:"1h"`
}

// Proxy holds proxy configuration.
type Proxy struct {
	// Comma-separated list of proxy URLs (e.g., socks5h://127.0.0.1:1080,socks5h://127.0.0.1:1081)
	URLs           string        `env:"DAUNRODO_PROXY_URLS"             envDefault:""`
	HealthCheck    bool          `env:"DAUNRODO_PROXY_HEALTH_CHECK"     envDefault:"true"`
	HealthTimeout  time.Duration `env:"DAUNRODO_PROXY_HEALTH_TIMEOUT"   envDefault:"5s"`
}

// HTTP holds HTTP server configuration.
type HTTP struct {
	Port            string        `env:"DAUNRODO_HTTP_PORT"             envDefault:":8080"`
	HandlerTimeout  time.Duration `env:"DAUNRODO_HTTP_HANDLER_TIMEOUT"  envDefault:"20s"`
	DownloadTimeout time.Duration `env:"DAUNRODO_HTTP_DOWNLOAD_TIMEOUT" envDefault:"30m"`
	ShutdownTimeout time.Duration `env:"DAUNRODO_HTTP_SHUTDOWN_TIMEOUT" envDefault:"10s"`
}

// Dir holds directory paths for downloads, cache, and cookie file.
type Dir struct {
	Downloads string `env:"DAUNRODO_DIR_DOWNLOAD"          envDefault:"./data/downloads"` // downloads stored here
	Cache     string `env:"DAUNRODO_DIR_CACHE"             envDefault:"./data/cache"`     // yt-dlp cache (meta, sigs)

	// must contain cookies.txt file
	// see: https://github.com/yt-dlp/yt-dlp/wiki/FAQ#how-do-i-pass-cookies-to-yt-dlp
	CookieFile string `env:"DAUNRODO_DIR_COOKIE_FILE" envDefault:""`

	// see: https://github.com/yt-dlp/yt-dlp/blob/2025.09.05/README.md#output-template
	FilenameTemplate string `env:"DAUNRODO_DIR_FILENAME_TEMPLATE" envDefault:"%(extractor)s - %(title)s [%(id)s].%(ext)s"`
}

// SetAbsPaths converts all directory paths to absolute paths.
func (c *Dir) SetAbsPaths() error {
	var err error
	if c.Downloads, err = filepath.Abs(c.Downloads); err != nil {
		return fmt.Errorf("downloads: %w", err)
	}

	if c.Cache, err = filepath.Abs(c.Cache); err != nil {
		return fmt.Errorf("cache: %w", err)
	}

	if c.CookieFile != "" {
		if c.CookieFile, err = filepath.Abs(c.CookieFile); err != nil {
			return fmt.Errorf("cookie file: %w", err)
		}
	}

	if c.FilenameTemplate, err = filepath.Abs(filepath.Join(c.Downloads, c.FilenameTemplate)); err != nil {
		return fmt.Errorf("filename template: %w", err)
	}

	return nil
}

// New loads configuration from environment variables.
func New() (*Config, error) {
	cfg := &Config{}

	err := env.Parse(cfg)
	if err != nil {
		return nil, fmt.Errorf("parse env: %w", err)
	}

	err = cfg.Dir.SetAbsPaths()
	if err != nil {
		return nil, fmt.Errorf("set absolute paths: %w", err)
	}

	return cfg, nil
}
