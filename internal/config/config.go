// Package config handles application configuration loading and management.
package config

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config holds the application configuration.
type Config struct {
	HTTP       HTTP
	App        App
	Job        Job
	Dir        Dir
	Storage    Storage
	DepManager DepManager
	Proxy      Proxy
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

	err = cfg.DepManager.SetAbsPaths()
	if err != nil {
		return nil, fmt.Errorf("set dep manager absolute paths: %w", err)
	}

	cfg.Proxy.parseList()

	return cfg, nil
}

// DepManager holds binary dependency management configuration.
type DepManager struct {
	// BinsDir is the directory where binaries are stored
	BinsDir string `env:"DAUNRODO_DEPMANAGER_BINS_DIR" envDefault:"./bins"`
	// UseSystemBinaries indicates whether to use system-installed binaries or download them.
	UseSystemBinaries bool `env:"DAUNRODO_DEPMANAGER_USE_SYSTEM_BINARIES" envDefault:"false"`
	// UpdateInterval is how often to check for binary updates
	UpdateInterval time.Duration `env:"DAUNRODO_DEPMANAGER_UPDATE_INTERVAL" envDefault:"24h"`

	// ffmpeg binary URLs per platform.
	FFmpegSHA256SumsURL string `env:"DAUNRODO_DEPMANAGER_FFMPEG_SHA256SUMS_URL" envDefault:"https://github.com/BtbN/FFmpeg-Builds/releases/latest/download/checksums.sha256"`                        //nolint:lll
	FFmpegLinuxARM64    string `env:"DAUNRODO_DEPMANAGER_FFMPEG_LINUX_ARM64" envDefault:"https://github.com/BtbN/FFmpeg-Builds/releases/latest/download/ffmpeg-master-latest-linuxarm64-gpl.tar.xz"` //nolint:lll
	FFmpegLinuxAMD64    string `env:"DAUNRODO_DEPMANAGER_FFMPEG_LINUX_AMD64" envDefault:"https://github.com/BtbN/FFmpeg-Builds/releases/latest/download/ffmpeg-master-latest-linux64-gpl.tar.xz"`    //nolint:lll

	// yt-dlp binary URLs per platform.
	YTdlpSHA256SumsURL string `env:"DAUNRODO_DEPMANAGER_YTDLP_SHA256SUMS_URL" envDefault:"https://github.com/yt-dlp/yt-dlp/releases/latest/download/SHA2-256SUMS"`      //nolint:lll
	YTdlpLinuxARM64    string `env:"DAUNRODO_DEPMANAGER_YTDLP_LINUX_ARM64" envDefault:"https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux_aarch64"` //nolint:lll // zip: https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux_aarch64.zip
	YTdlpLinuxAMD64    string `env:"DAUNRODO_DEPMANAGER_YTDLP_LINUX_AMD64" envDefault:"https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux"`         //nolint:lll        // zip: https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux.zip

	// gallery-dl binary URLs per platform.
	GalleryDLSHA256SumsURL string `env:"DAUNRODO_DEPMANAGER_GALLERYDL_SHA256SUMS_URL" envDefault:"https://github.com/gallery-dl-builds/gallery-dl-builds/releases/latest/download/SHA256SUMS.txt"`      //nolint:lll
	GalleryDLLinuxARM64    string `env:"DAUNRODO_DEPMANAGER_GALLERYDL_LINUX_ARM64" envDefault:"https://github.com/gallery-dl-builds/gallery-dl-builds/releases/latest/download/gallery-dl_linux_arm64"` //nolint:lll
	GalleryDLLinuxAMD64    string `env:"DAUNRODO_DEPMANAGER_GALLERYDL_LINUX_AMD64" envDefault:"https://github.com/gallery-dl-builds/gallery-dl-builds/releases/latest/download/gallery-dl_linux_amd64"` //nolint:lll

	// deno binary URLs per platform.
	DenoSHA256SumsURL string `env:"DAUNRODO_DEPMANAGER_DENO_SHA256SUMS_URL" envDefault:"https://github.com/denoland/deno/releases/latest/download/deno-aarch64-unknown-linux-gnu.zip.sha256sum,https://github.com/denoland/deno/releases/latest/download/deno-x86_64-unknown-linux-gnu.zip.sha256sum"` //nolint:lll
	DenoLinuxARM64    string `env:"DAUNRODO_DEPMANAGER_DENO_LINUX_ARM64" envDefault:"https://github.com/denoland/deno/releases/latest/download/deno-aarch64-unknown-linux-gnu.zip"`                                                                                                                    //nolint:lll
	DenoLinuxAMD64    string `env:"DAUNRODO_DEPMANAGER_DENO_LINUX_AMD64" envDefault:"https://github.com/denoland/deno/releases/latest/download/deno-x86_64-unknown-linux-gnu.zip"`                                                                                                                     //nolint:lll
}

// SetAbsPaths converts the BinsDir path to an absolute path.
func (d *DepManager) SetAbsPaths() error {
	var err error
	if d.BinsDir, err = filepath.Abs(d.BinsDir); err != nil {
		return fmt.Errorf("bins dir: %w", err)
	}

	return nil
}

// Proxy holds proxy configuration for download requests.
type Proxy struct {
	// List is a comma-separated list of proxy URLs in socks5h format
	List string `env:"DAUNRODO_PROXY_LIST" envDefault:""`
	// HealthCheckInterval is how often to check proxy health
	HealthCheckInterval time.Duration `env:"DAUNRODO_PROXY_HEALTH_CHECK_INTERVAL" envDefault:"5m"`
	// FailureBackoff is the initial backoff duration for failed proxies
	FailureBackoff time.Duration `env:"DAUNRODO_PROXY_FAILURE_BACKOFF" envDefault:"1m"`
	// MaxFailures is the maximum number of failures before a proxy is temporarily removed
	MaxFailures int `env:"DAUNRODO_PROXY_MAX_FAILURES" envDefault:"3"`

	// Proxies is the parsed list of proxy URLs
	Proxies []string `env:"-"`
}

// parseList parses the comma-separated proxy list.
func (p *Proxy) parseList() {
	if p.List == "" {
		return
	}

	for proxy := range strings.SplitSeq(p.List, ",") {
		proxy = strings.TrimSpace(proxy)
		if proxy != "" {
			p.Proxies = append(p.Proxies, proxy)
		}
	}
}
