package downloader

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"daunrodo/internal/config"
	"daunrodo/internal/consts"
	"daunrodo/internal/depmanager"
	"daunrodo/internal/entity"
	"daunrodo/internal/errs"
	"daunrodo/internal/proxymgr"
	"daunrodo/internal/storage"
	"daunrodo/pkg/gen"
)

var (
	// reGalleryProgress is the progress regex: # 1/5.
	reGalleryProgress = regexp.MustCompile(`#\s*(\d+)/(\d+)`)
)

// Constants for progress parsing.
const (
	// progressMatchCount is the minimum regex match count for progress parsing.
	progressMatchCount = 3
	// percentMultiplier for converting fraction to percentage.
	percentMultiplier = 100
)

// GalleryDL represents a gallery-dl downloader for image galleries.
type GalleryDL struct {
	log      *slog.Logger
	cfg      *config.Config
	depMgr   *depmanager.Manager
	proxyMgr *proxymgr.Manager
}

// GalleryDLResult represents the JSON output from gallery-dl.
type GalleryDLResult struct {
	Category    string `json:"category"`
	Subcategory string `json:"subcategory"`
	Filename    string `json:"filename"`
	Extension   string `json:"extension"`
	ID          string `json:"id"`
	Title       string `json:"description"`
	Author      string `json:"author"`
	Date        string `json:"date"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	URL         string `json:"url"`
}

// NewGalleryDL creates a new GalleryDL downloader instance.
func NewGalleryDL(
	log *slog.Logger,
	cfg *config.Config,
	depMgr *depmanager.Manager,
	proxyMgr *proxymgr.Manager,
) Downloader {
	return &GalleryDL{
		log:      log.With(slog.String("package", "downloader"), slog.String("downloader", consts.DownloaderGalleryDL)),
		cfg:      cfg,
		depMgr:   depMgr,
		proxyMgr: proxyMgr,
	}
}

// Process processes the download job using gallery-dl.
func (d *GalleryDL) Process(ctx context.Context, job *entity.Job, storer storage.Storer) error {
	if job == nil {
		return errs.ErrJobNil
	}

	log := d.log.With(slog.Any("job", job))

	storer.UpdateJobStatus(ctx, job.UUID, entity.JobStatusDownloading, 0, "")

	// Build command arguments
	args := d.buildArgs(job)

	// Get binary path
	binPath := d.depMgr.GetInstalledPath(depmanager.BinaryGalleryDL)
	if binPath == "" {
		binPath = d.depMgr.GetBinaryPath(depmanager.BinaryGalleryDL)
	}

	log.DebugContext(ctx, "executing gallery-dl", slog.String("binary", binPath), slog.Any("args", args))

	cmd := exec.CommandContext(ctx, binPath, args...)

	// Set up pipes
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start command: %w", err)
	}

	var (
		stdoutBuf strings.Builder
		stderrBuf strings.Builder
		wg        sync.WaitGroup
	)

	// Read stdout (JSON output)
	wg.Go(func() {
		io.Copy(&stdoutBuf, stdout)
	})

	// Read stderr (progress updates)
	wg.Go(func() {
		d.handleProgress(ctx, stderr, &stderrBuf, job, storer)
	})

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		log.ErrorContext(ctx, "gallery-dl command failed",
			slog.Any("error", err),
			slog.String("stderr", stderrBuf.String()))

		return fmt.Errorf("gallery-dl process: %w", err)
	}

	// Parse results
	publications, err := d.composePublications(ctx, stdoutBuf.String())
	if err != nil {
		return fmt.Errorf("compose publications: %w", err)
	}

	log.InfoContext(ctx, "publications composed", "publications", publications)

	if err := storePublications(ctx, job.UUID, publications, storer); err != nil {
		return fmt.Errorf("store publications: %w", err)
	}

	storer.UpdateJobPublications(ctx, job.UUID, publications)

	storer.UpdateJobStatus(ctx, job.UUID, entity.JobStatusFinished, fullProgress, "")

	log.InfoContext(ctx, "done")

	return nil
}

func (d *GalleryDL) buildArgs(job *entity.Job) []string {
	// Construct output directory based on config
	outputDir := d.cfg.Dir.Downloads

	args := []string{
		"--write-info-json",
		"--dump-json",
		"-D", outputDir,
	}

	// Add proxy if available
	if d.proxyMgr != nil && d.proxyMgr.HasProxies() {
		proxy := d.proxyMgr.GetRandomProxy()
		if proxy != "" {
			args = append(args, "--proxy", proxy)
		}
	}

	// Add cookies if configured
	if d.cfg.Dir.CookieFile != "" {
		args = append(args, "--cookies", d.cfg.Dir.CookieFile)
	}

	// Add URL
	args = append(args, job.URL)

	return args
}

func (d *GalleryDL) handleProgress(
	ctx context.Context,
	reader io.Reader,
	stderrBuf *strings.Builder,
	job *entity.Job,
	storer storage.Storer,
) {
	scanner := bufio.NewScanner(reader)
	lastUpdate := time.Now()

	for scanner.Scan() {
		line := scanner.Text()
		stderrBuf.WriteString(line)
		stderrBuf.WriteString("\n")

		// Parse progress (gallery-dl outputs "# N/M" format)
		matches := reGalleryProgress.FindStringSubmatch(line)
		if len(matches) < progressMatchCount {
			continue
		}

		current, err := strconv.Atoi(matches[1])
		if err != nil {
			continue
		}

		total, err := strconv.Atoi(matches[2])
		if err != nil || total == 0 {
			continue
		}

		progress := float64(current) / float64(total) * percentMultiplier

		// Rate limit updates
		if time.Since(lastUpdate) < defaultProgressFreq {
			continue
		}

		lastUpdate = time.Now()

		d.log.DebugContext(ctx, "download progress",
			slog.Float64("progress", progress),
			slog.Int("current", current),
			slog.Int("total", total))
		storer.UpdateJobStatus(ctx, job.UUID, entity.JobStatusDownloading, int(progress), "")
	}
}

func (d *GalleryDL) composePublications(_ context.Context, stdout string) ([]entity.Publication, error) {
	results := d.parseGalleryDLStdout(stdout)

	if len(results) == 0 {
		return nil, fmt.Errorf("no results parsed from stdout")
	}

	publications := make([]entity.Publication, 0, len(results))

	for _, res := range results {
		var fileSize int64

		filename := res.Filename
		if filename != "" && res.Extension != "" {
			if !strings.HasSuffix(filename, "."+res.Extension) {
				filename = filename + "." + res.Extension
			}
		}

		// Try to get the file path from output directory
		fullPath := filepath.Join(d.cfg.Dir.Downloads, filename)
		if fileInfo, err := os.Stat(fullPath); err == nil {
			fileSize = fileInfo.Size()
			filename = fullPath
		}

		pub := entity.Publication{
			UUID:       gen.UUIDv5(res.ID, filename),
			ID:         res.ID,
			Type:       "image",
			Platform:   res.Category,
			Author:     res.Author,
			Title:      res.Title,
			WebpageURL: res.URL,
			Width:      res.Width,
			Height:     res.Height,
			FileSize:   fileSize,
			Filename:   filename,
		}

		publications = append(publications, pub)
	}

	return publications, nil
}

func (d *GalleryDL) parseGalleryDLStdout(stdout string) []GalleryDLResult {
	scanner := bufio.NewScanner(strings.NewReader(stdout))
	scanner.Buffer(make([]byte, bufSize), maxJSONSize)

	var results []GalleryDLResult

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Try to parse as JSON
		var res GalleryDLResult
		if err := json.Unmarshal([]byte(line), &res); err == nil {
			results = append(results, res)
		}
	}

	return results
}

// IsTikTokImagePost checks if the URL is likely a TikTok image slideshow post.
// This can be used to determine whether to use GalleryDL vs YTdlp.
func IsTikTokImagePost(url string) bool {
	// TikTok image slideshows typically have /photo/ in the URL
	// or specific patterns that indicate slideshow content
	return strings.Contains(url, "tiktok.com") && strings.Contains(url, "/photo/")
}
