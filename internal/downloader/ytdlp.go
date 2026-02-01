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
	"daunrodo/pkg/shellquote"
)

const fullProgress = 100

// Size estimation constants.
const (
	// mbPerMinuteAudio is the estimated MB per minute for audio files.
	mbPerMinuteAudio = 1024 * 1024
	// mbPerMinuteVideo is the estimated MB per minute for video files.
	mbPerMinuteVideo = 10 * 1024 * 1024
	// progressMinMatches is the minimum regex matches for progress parsing.
	progressMinMatches = 2
)

var (
	maxJSONSize = 10 * 1024 * 1024                                       // 10 MiB scanner buffer
	bufSize     = 4096                                                   // 4 KiB buffer size
	reFilepath  = regexp.MustCompile(`(?i)^[^\{\[\n].*\.[a-z0-9]{1,6}$`) // file path

	// reProgress is the download progress regex: [download]  50.0%.
	reProgress = regexp.MustCompile(`\[download\]\s+(\d+\.?\d*)%`)

	// changing this may break parseYtdlpStdout().
	defaultPrintAfterMove = "after_move:filepath"
)

// YTdlp represents a yt-dlp downloader using the binary directly.
type YTdlp struct {
	log      *slog.Logger
	cfg      *config.Config
	depMgr   *depmanager.Manager
	proxyMgr *proxymgr.Manager
}

// NewYTdlp creates a new YTdlp downloader instance.
func NewYTdlp(
	log *slog.Logger,
	cfg *config.Config,
	depMgr *depmanager.Manager,
	proxyMgr *proxymgr.Manager,
) Downloader {
	return &YTdlp{
		log:      log.With(slog.String("package", "downloader"), slog.String("downloader", consts.DownloaderYTdlp)),
		cfg:      cfg,
		depMgr:   depMgr,
		proxyMgr: proxyMgr,
	}
}

// Process processes the download job and updates the job status in the storage.
func (d *YTdlp) Process(ctx context.Context, job *entity.Job, storer storage.Storer) error {
	if job == nil {
		return errs.ErrJobNil
	}

	log := d.log.With(slog.Any("job", job))

	storer.UpdateJobStatus(ctx, job.UUID, entity.JobStatusDownloading, 0, "")

	// Get estimated file size before downloading
	estimatedSize, err := d.getEstimatedSize(ctx, job.URL, job.Preset)
	if err != nil {
		log.WarnContext(ctx, "failed to get estimated size", slog.Any("error", err))
	}

	if estimatedSize > 0 {
		storer.UpdateJobEstimatedSize(ctx, job.UUID, estimatedSize)
	}

	// Build command arguments
	args := d.buildArgs(job)

	// Get binary path
	binPath := d.depMgr.GetInstalledPath(depmanager.BinaryYTdlp)
	if binPath == "" {
		binPath = d.depMgr.GetBinaryPath(depmanager.BinaryYTdlp)
	}

	log.DebugContext(ctx, "executing yt-dlp", slog.String("cmd", shellquote.Join(binPath, args)))

	cmd := exec.CommandContext(ctx, binPath, args...)

	// Set up pipes for stdout and stderr
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
		waitGrp   sync.WaitGroup
	)

	// Read stdout (JSON output + progress updates)
	waitGrp.Go(func() {
		d.handleProgress(ctx, stdout, &stdoutBuf, job, storer)
	})

	// Read stderr (error output)
	waitGrp.Go(func() {
		io.Copy(&stderrBuf, stderr)
	})

	waitGrp.Wait()

	if err := cmd.Wait(); err != nil {
		log.ErrorContext(ctx, "yt-dlp command failed",
			slog.Any("error", err),
			slog.String("stderr", stderrBuf.String()))

		return fmt.Errorf("yt-dlp process: %w", err)
	}

	// Parse results
	publications, err := d.composePublications(stdoutBuf.String())
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

// GetEstimatedSize fetches format information and estimates the file size.
func (d *YTdlp) getEstimatedSize(ctx context.Context, url, preset string) (int64, error) {
	binPath := d.depMgr.GetInstalledPath(depmanager.BinaryYTdlp)
	if binPath == "" {
		binPath = d.depMgr.GetBinaryPath(depmanager.BinaryYTdlp)
	}

	args := []string{
		"-F", "--no-playlist", "-J",
		url,
	}

	cmd := exec.CommandContext(ctx, binPath, args...)

	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("get formats: %w", err)
	}

	var info struct {
		Formats []struct {
			FormatID       string `json:"format_id"`
			Filesize       int64  `json:"filesize"`
			FilesizeApprox int64  `json:"filesize_approx"`
		} `json:"formats"`
		Duration float64 `json:"duration"`
	}

	if err := json.Unmarshal(output, &info); err != nil {
		return 0, fmt.Errorf("parse formats: %w", err)
	}

	// Estimate based on preset or use largest format
	var maxSize int64

	for _, f := range info.Formats {
		size := f.Filesize
		if size == 0 {
			size = f.FilesizeApprox
		}

		if size > maxSize {
			maxSize = size
		}
	}

	// If we have duration but no filesize, estimate based on bitrate
	if maxSize == 0 && info.Duration > 0 {
		// Assume ~1MB per minute for audio, ~10MB per minute for video
		switch preset {
		case "mp3", "aac", "audio":
			maxSize = int64(info.Duration / 60 * mbPerMinuteAudio)
		default:
			maxSize = int64(info.Duration / 60 * mbPerMinuteVideo)
		}
	}

	return maxSize, nil
}

func (d *YTdlp) buildArgs(job *entity.Job) []string {
	args := []string{
		"--ffmpeg-location", d.depMgr.GetInstalledPath(depmanager.BinaryFFmpeg),
		"--remote-components", "ejs:github",
		"--js-runtimes", fmt.Sprintf("deno:%s", d.depMgr.GetInstalledPath(depmanager.BinaryDeno)),
		"--cache-dir", d.cfg.Dir.Cache,
		"--no-playlist",
		"--print-json",
		"--print", defaultPrintAfterMove,
		"-o", d.cfg.Dir.FilenameTemplate,
		"--progress",
		"--newline",
	}

	// Add preset/format
	if job.Preset != "" {
		args = append(args, "-t", job.Preset)
	}

	// Add cookies if configured
	if d.cfg.Dir.CookieFile != "" {
		args = append(args, "--cookies", d.cfg.Dir.CookieFile)
	}

	// Add proxy if available
	if d.proxyMgr != nil && d.proxyMgr.HasProxies() {
		proxy := d.proxyMgr.GetRandomProxy()
		if proxy != "" {
			args = append(args, "--proxy", proxy)
		}
	}

	// Add URL
	args = append(args, job.URL)

	return args
}

// ParseProgress extracts the progress percentage from a yt-dlp stderr line.
// Returns the progress percentage and true if found, 0 and false otherwise.
func ParseProgress(line string) (float64, bool) {
	matches := reProgress.FindStringSubmatch(line)
	if len(matches) < progressMinMatches {
		return 0, false
	}

	progress, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, false
	}

	return progress, true
}

// SplitLinesAny is a bufio.SplitFunc that splits on both \n and \r\n line endings.
func SplitLinesAny(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for i := range data {
		switch data[i] {
		case '\n':
			return i + 1, data[:i], nil
		case '\r':
			if i+1 < len(data) && data[i+1] == '\n' {
				return i + 2, data[:i], nil //nolint:mnd
			}

			return i + 1, data[:i], nil
		}
	}

	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}

	return 0, nil, nil
}

func (d *YTdlp) handleProgress(
	ctx context.Context,
	reader io.Reader,
	stdoutBuf *strings.Builder,
	job *entity.Job,
	storer storage.Storer,
) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, bufSize), maxJSONSize)
	scanner.Split(SplitLinesAny)

	lastUpdate := time.Now()

	for scanner.Scan() {
		line := scanner.Text()
		stdoutBuf.WriteString(line)
		stdoutBuf.WriteString("\n")

		// Parse progress
		progress, ok := ParseProgress(line)
		if !ok {
			continue
		}

		// Rate limit updates
		if time.Since(lastUpdate) < defaultProgressFreq {
			continue
		}

		lastUpdate = time.Now()

		d.log.DebugContext(ctx, "download progress", slog.Float64("progress", progress))
		storer.UpdateJobStatus(ctx, job.UUID, entity.JobStatusDownloading, int(progress), "")
	}
}

func (d *YTdlp) composePublications(stdout string) ([]entity.Publication, error) {
	results, err := ParseYtdlpStdout(stdout)
	if err != nil {
		return nil, fmt.Errorf("parse yt-dlp stdout: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no results parsed from stdout")
	}

	publications := make([]entity.Publication, 0, len(results))

	for _, res := range results {
		var fileSize int64

		if res.Filename != "" {
			fileInfo, err := os.Stat(res.Filename)
			if err == nil {
				fileSize = fileInfo.Size()
			}
		}

		pub := entity.Publication{
			UUID:         gen.UUIDv5(res.ID, res.Filename),
			ID:           res.ID,
			Type:         res.Type,
			Platform:     res.Extractor,
			Channel:      res.Channel,
			Author:       res.Uploader,
			Description:  res.Description,
			WebpageURL:   res.WebpageURL,
			Title:        res.Title,
			ViewCount:    res.GetViewCount(),
			LikeCount:    res.LikeCount,
			ThumbnailURL: res.GetThumbnail(),
			FileSize:     fileSize,
			Duration:     res.Timestamp,
			Filename:     res.Filename,
		}

		publications = append(publications, pub)
	}

	return publications, nil
}

// ParseYtdlpStdout parses the stdout of yt-dlp and returns a slice of ResultJSON with filenames.
func ParseYtdlpStdout(stdout string) ([]ResultJSON, error) {
	scanner := bufio.NewScanner(strings.NewReader(stdout))
	scanner.Buffer(make([]byte, bufSize), maxJSONSize)

	var (
		lineNo   int
		resultNo int
		res      []ResultJSON
	)

	for scanner.Scan() {
		lineNo++

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var r ResultJSON
		if err := json.Unmarshal([]byte(line), &r); err == nil {
			res = append(res, r)
			resultNo++

			continue
		}

		if reFilepath.MatchString(line) {
			if resultNo > 0 {
				res[resultNo-1].Filename = line
			}

			continue
		}
	}

	return res, nil
}

func storePublications(ctx context.Context, jobID string, publications []entity.Publication, storer storage.Storer) error { //nolint:lll
	if jobID == "" {
		return errs.ErrJobIDEmpty
	}

	for i := range publications {
		pub := publications[i]
		if err := storer.SetPublication(ctx, jobID, &pub); err != nil {
			return fmt.Errorf("store publication: %w", err)
		}
	}

	return nil
}
