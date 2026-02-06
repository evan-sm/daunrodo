//go:build integration
// +build integration

package integration_test

import (
	_ "embed"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"daunrodo/internal/config"
	"daunrodo/internal/depmanager"
	"daunrodo/internal/downloader"
	"daunrodo/internal/entity"
	"daunrodo/internal/storage"
	"daunrodo/pkg/gen"
)

//go:embed testdata/fake-ytdlp.sh
var fakeYTDLPScript string

type ytdlpIntegrationFixture struct {
	cfg        *config.Config
	storer     storage.Storer
	downloader downloader.Downloader
	outputFile string
}

func newYTdlpIntegrationFixture(t *testing.T, mode string) *ytdlpIntegrationFixture {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("integration fake yt-dlp helper uses shell script")
	}

	baseDir := t.TempDir()
	binsDir := filepath.Join(baseDir, "bins")
	downloadsDir := filepath.Join(baseDir, "downloads")
	cacheDir := filepath.Join(baseDir, "cache")

	if err := os.MkdirAll(binsDir, 0o755); err != nil {
		t.Fatalf("mkdir bins dir: %v", err)
	}

	if err := os.MkdirAll(downloadsDir, 0o755); err != nil {
		t.Fatalf("mkdir downloads dir: %v", err)
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}

	cfg, err := config.New()
	if err != nil {
		t.Fatalf("config new: %v", err)
	}

	cfg.DepManager.BinsDir = binsDir
	cfg.Dir.Downloads = downloadsDir
	cfg.Dir.Cache = cacheDir
	cfg.Dir.CookieFile = ""
	cfg.Dir.FilenameTemplate = filepath.Join(downloadsDir, "%(extractor)s - %(title)s [%(id)s].%(ext)s")
	cfg.Storage.CleanupInterval = time.Hour
	cfg.Job.Timeout = 5 * time.Second

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	depMgr := depmanager.New(log, cfg)
	storer := storage.New(t.Context(), log, cfg)

	fakeBinaryPath := depMgr.GetBinaryPath(depmanager.BinaryYTdlp)
	if err := os.WriteFile(fakeBinaryPath, []byte(fakeYTDLPScript), 0o755); err != nil {
		t.Fatalf("write fake yt-dlp: %v", err)
	}

	outputFile := filepath.Join(downloadsDir, "fake-output.mp4")
	t.Setenv("DAUNRODO_FAKE_MODE", mode)
	t.Setenv("DAUNRODO_FAKE_OUTPUT_FILE", outputFile)

	dl := downloader.NewYTdlp(log, cfg, depMgr, nil)

	return &ytdlpIntegrationFixture{
		cfg:        cfg,
		storer:     storer,
		downloader: dl,
		outputFile: outputFile,
	}
}

func (fx *ytdlpIntegrationFixture) newStoredJob(t *testing.T) *entity.Job {
	t.Helper()

	job := entity.Job{
		UUID:      gen.UUIDv5("https://example.com/watch?v=vid-123", "mp4"),
		URL:       "https://example.com/watch?v=vid-123",
		Preset:    "mp4",
		Status:    entity.JobStatusStarting,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}

	fx.storer.SetJob(t.Context(), job)

	return &job
}
