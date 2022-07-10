package config

import (
	"daunrodo/internal/consts"
	"path/filepath"
	"time"

	"github.com/caarlos0/env/v11"
)

type (
	Config struct {
		HTTP
		App
	}

	App struct {
		LogLevel string `env:"DAUNRODO_APP_LOG_LEVEL" envDefault:"info"`
		HttpPort string `env:"DAUNRODO_APP_HTTP_PORT" envDefault:"8080"`

		Job Job
		Dir Dir
	}

	Job struct {
		Workers    int           `env:"DAUNRODO_APP_JOB_WORKERS" envDefault:"2"`
		StorageTTL time.Duration `env:"DAUNRODO_APP_JOB_STORAGE_TTL" envDefault:"168h"`
		Timeout    time.Duration `env:"DAUNRODO_APP_JOB_TIMEOUT" envDefault:"5m"`
		QueueSize  int           `env:"DAUNRODO_APP_JOB_QUEUE_SIZE" envDefault:"100"`
	}

	HTTP struct {
		HandlerTimeout time.Duration `env:"DAUNRODO_HTTP_HANDLER_TIMEOUT" envDefault:"20s"`
	}
)

type Dir struct {
	Downloads        string `env:"DAUNRODO_DIR_DOWNLOAD" envDefault:"./data/downloads"`                                    // downloads stored here
	Cache            string `env:"DAUNRODO_DIR_CACHE" envDefault:"./data/cache"`                                           // yt-dlp cache (meta, sigs)
	Cookies          string `env:"DAUNRODO_DIR_COOKIES" envDefault:""`                                                     // must contain cookies.txt file | see: https://github.com/yt-dlp/yt-dlp/wiki/FAQ#how-do-i-pass-cookies-to-yt-dlp
	FilenameTemplate string `env:"DAUNRODO_DIR_FILENAME_TEMPLATE" envDefault:"%(extractor)s - %(title)s [%(id)s].%(ext)s"` // see: https://github.com/yt-dlp/yt-dlp/blob/2025.09.05/README.md#output-template
}

func (c *Dir) SetAbsPaths() (err error) {
	if c.Downloads, err = filepath.Abs(c.Downloads); err != nil {
		return err
	}
	if c.Cache, err = filepath.Abs(c.Cache); err != nil {
		return err
	}

	if c.Cookies != "" {
		if c.Cookies, err = filepath.Abs(filepath.Join(c.Cookies, consts.CookiesFile)); err != nil {
			return err
		}
	}

	if c.FilenameTemplate, err = filepath.Abs(filepath.Join(c.Downloads, c.FilenameTemplate)); err != nil {
		return err
	}

	return err
}

func New() (*Config, error) {
	cfg := &Config{}
	err := env.Parse(cfg)
	if err != nil {
		return nil, err
	}

	err = cfg.App.Dir.SetAbsPaths()
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
