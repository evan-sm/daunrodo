package logger

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

type Options struct {
	AddSource bool
	Level     string
}

func New(opt *Options) (*slog.Logger, error) {
	if opt == nil {
		return nil, fmt.Errorf("logger options are required")
	}

	opts := &slog.HandlerOptions{
		AddSource: opt.AddSource,
	}

	level, err := ParseLevel(opt.Level)
	if err != nil {
		level = slog.LevelInfo
	}
	opts.Level = level

	log := slog.New(slog.NewJSONHandler(os.Stdout, opts))
	slog.SetDefault(log)

	return log, err
}

// ParseLevel converts a string level to slog.Level
func ParseLevel(level string) (slog.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown log level: %q", level)
	}
}
