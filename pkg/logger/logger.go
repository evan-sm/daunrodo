// Package logger provides logging utilities and configuration.
package logger

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

var (
	errLoggerOptionsRequired = errors.New("logger options are required")
	errUnknownLogLevel       = errors.New("unknown log level")
)

// Options represents the configuration options for the logger.
type Options struct {
	AddSource bool
	Level     string
}

// New creates a new slog.Logger instance based on the provided options.
func New(opt *Options) (*slog.Logger, error) {
	if opt == nil {
		return nil, errLoggerOptionsRequired
	}

	level, err := ParseLevel(opt.Level)
	if err != nil {
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		AddSource: opt.AddSource,
		Level:     level,
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, opts))
	slog.SetDefault(log)

	return log, err
}

// ParseLevel converts a string level to slog.Level.
func ParseLevel(level string) (slog.Level, error) {
	switch strings.TrimSpace(strings.ToLower(level)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		// return slog.LevelInfo, fmt.Errorf("unknown log level: %q", level)
		return slog.LevelInfo, fmt.Errorf("%w: %q", errUnknownLogLevel, level)
	}
}
