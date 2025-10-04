package config_test

import (
	"bufio"
	"bytes"
	"daunrodo/internal/config"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

//go:embed testdata/.env.custom.dir
var envCustomDir []byte

func parseEnv(r io.Reader) (map[string]string, error) {
	env := make(map[string]string)
	lineNo := 0

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid line %d: %q", lineNo, line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		env[key] = value
	}

	err := scanner.Err()

	return env, fmt.Errorf("scan env: %w", err)
}

func applyEnv(env map[string]string) error {
	os.Clearenv()

	for key, value := range env {
		err := os.Setenv(key, value)
		if err != nil {
			return fmt.Errorf("apply env: %w", err)
		}
	}

	return nil
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string // description of this test case
		env     []byte
		want    *config.Config
		wantErr bool
	}{
		{
			name: "custom dir",
			env:  envCustomDir,
			want: &config.Config{
				Dir: config.Dir{
					Downloads:        "./data/downloads",
					Cache:            "./data/cache",
					CookieFile:       "./data/cookies/cookies.txt",
					FilenameTemplate: "./%(extractor)s - %(title)s [%(id)s].%(ext)s",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := parseEnv(bytes.NewReader(tt.env))
			if err != nil {
				t.Errorf("parseEnv() failed: %v", err)

				return
			}

			if err := applyEnv(env); err != nil {
				t.Errorf("applyEnv() failed: %v", err)

				return
			}

			got, err := config.New()
			if err != nil && !tt.wantErr {
				t.Errorf("New() failed: %v", err)
			}

			if !filepath.IsAbs(got.Dir.Downloads) {
				t.Errorf("expected absolute path, got %s", got.Dir.Downloads)
			}

			if !filepath.IsAbs(got.Dir.Cache) {
				t.Errorf("expected absolute path, got %s", got.Dir.Cache)
			}

			pwd, err := os.Getwd()
			if err != nil {
				t.Errorf("failed to get current working directory: %v", err)
			}

			if _, err := os.Stat(fmt.Sprintf("%s/%s", pwd, got.Dir.CookieFile)); err != nil {
				t.Errorf("expected cookie file to exist, got %s", got.Dir.CookieFile)
			}

			if !filepath.IsAbs(got.Dir.FilenameTemplate) {
				t.Errorf("expected absolute path, got %s", got.Dir.FilenameTemplate)
			}
		})
	}
}
