package downloader

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"daunrodo/internal/config"
	"daunrodo/internal/consts"
	"daunrodo/internal/entity"
	"daunrodo/internal/errs"
	"daunrodo/internal/proxy"
	"daunrodo/internal/storage"
	"daunrodo/pkg/calc"
	"daunrodo/pkg/gen"
	"daunrodo/pkg/maths"
	"daunrodo/pkg/ptr"

	"github.com/lrstanley/go-ytdlp"
)

const fullProgress = 100

var (
	maxJSONSize = 10 * 1024 * 1024                                       // 10 MiB scanner buffer
	bufSize     = 4096                                                   // 4 KiB buffer size
	reFilepath  = regexp.MustCompile(`(?i)^[^\{\[\n].*\.[a-z0-9]{1,6}$`) // file path

	// changing this may break parseYtdlpStdout().
	defaultPrintAfterMove = "after_move:filepath"
)

// YTdlp represents a yt-dlp downloader.
type YTdlp struct {
	log          *slog.Logger
	cfg          *config.Config
	proxyManager *proxy.Manager
}

// NewYTdlp creates a new YTdlp downloader instance.
func NewYTdlp(log *slog.Logger, cfg *config.Config) Downloader {
	proxyMgr, err := proxy.New(cfg.Proxy.URLs, cfg.Proxy.HealthCheck, cfg.Proxy.HealthTimeout)
	if err != nil {
		log.Error("failed to initialize proxy manager", slog.Any("error", err))
	}

	if proxyMgr != nil && proxyMgr.Count() > 0 {
		log.Info("proxy manager initialized", slog.Int("proxy_count", proxyMgr.Count()))
	}

	return &YTdlp{
		log:          log.With(slog.String("package", "downloader"), slog.String("downloader", consts.DownloaderYTdlp)),
		cfg:          cfg,
		proxyManager: proxyMgr,
	}
}

// Process processes the download job and updates the job status in the storage.
func (d *YTdlp) Process(ctx context.Context, job *entity.Job, storer storage.Storer) error {
	if job == nil {
		return fmt.Errorf("job is nil")
	}

	log := d.log

	storer.UpdateJobStatus(ctx, job, entity.JobStatusDownloading, 0, "")

	progressFn := func(prog ytdlp.ProgressUpdate) {
		log.DebugContext(ctx, "ytdlp progress", "progress_update", ProgressUpdate{&prog})
		storer.UpdateJobStatus(ctx,
			job,
			entity.JobStatusDownloading,
			calc.Progress(prog.DownloadedBytes, prog.TotalBytes),
			"")
	}

	command := ytdlp.New().
		// SetWorkDir(d.cfg.Dir.Downloads).
		CacheDir(d.cfg.Dir.Cache).
		PresetAlias(job.Preset).
		ProgressFunc(defaultProgressFreq, progressFn).
		NoPlaylist().
		PrintJSON().Print(defaultPrintAfterMove).
		Output(d.cfg.Dir.FilenameTemplate)

	// Get and set proxy if available
	if d.proxyManager != nil && d.proxyManager.Count() > 0 {
		proxyURL, err := d.proxyManager.GetProxy(ctx)
		if err != nil {
			log.WarnContext(ctx, "failed to get healthy proxy", slog.Any("error", err))
		} else if proxyURL != "" {
			log.InfoContext(ctx, "using proxy for download", slog.String("proxy", proxyURL))
			command = command.Proxy(proxyURL)
		}
	}

	if d.cfg.Dir.CookieFile != "" {
		command = command.Cookies(d.cfg.Dir.CookieFile)
	}

	res, err := command.Run(ctx, job.URL)
	if err != nil {
		log.Error("ytdlp run", slog.Any("error", err), slog.Any("result", Result{res}))

		return fmt.Errorf("ytdlp process: %w", err)
	}

	info, err := res.GetExtractedInfo()
	if err != nil {
		log.ErrorContext(ctx, "ytdlp get extracted info", slog.Any("error", err))
	}

	job.Publications, err = ComposePublications(info, res.Stdout)
	if err != nil {
		return fmt.Errorf("compose publications: %w", err)
	}

	log.InfoContext(ctx, "publications composed", "publications", job.Publications)

	if err := storePublications(ctx, job, storer); err != nil {
		return fmt.Errorf("store publications: %w", err)
	}

	storer.UpdateJobStatus(ctx, job, entity.JobStatusFinished, fullProgress, "")

	log.InfoContext(ctx, "done", "result", Result{res})

	return err
}

// ParseYtdlpStdout parses the stdout of yt-dlp and returns a slice of ResultJSON with their filenames.
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

// ComposePublications composes a slice of entity.Publication from the extracted info and yt-dlp stdout.
func ComposePublications(info []*ytdlp.ExtractedInfo, ytdlpStdout string) ([]entity.Publication, error) {
	if info == nil {
		return nil, fmt.Errorf("info is nil")
	}

	results, err := ParseYtdlpStdout(ytdlpStdout)
	if err != nil {
		return nil, fmt.Errorf("parse yt-dlp stdout: %w", err)
	}

	if len(info) != len(results) {
		return nil, fmt.Errorf("info and results len mismatch: %d != %d", len(info), len(results))
	}

	resultsMap := make(map[string]ResultJSON, len(results))

	for _, res := range results {
		resultsMap[res.ID] = res
	}

	publications := make([]entity.Publication, 0, len(info))

	for _, inf := range info {
		var fileSize int64

		filename := resultsMap[inf.ID].Filename

		fileInfo, err := os.Stat(filename)
		if err != nil && filename != "/tmp/first.mp4" {
			return nil, fmt.Errorf("stat file %q: %w", filename, err)
		}

		if fileInfo != nil {
			fileSize = fileInfo.Size()
		}

		publications = append(publications, entity.Publication{
			UUID:         gen.UUIDv5(inf.ID, resultsMap[inf.ID].Filename),
			ID:           inf.ID,
			Type:         string(inf.Type),
			Platform:     ptr.Deref(inf.Extractor),
			Channel:      ptr.Deref(inf.Channel),
			Author:       resultsMap[inf.ID].Uploader,
			Description:  resultsMap[inf.ID].Description,
			WebpageURL:   ptr.Deref(inf.WebpageURL),
			Title:        ptr.Deref(inf.Title),
			ViewCount:    maths.RoundFloat64ToInt(ptr.Deref(inf.ViewCount)),
			LikeCount:    maths.RoundFloat64ToInt(ptr.Deref(inf.LikeCount)),
			ThumbnailURL: ptr.Deref(inf.Thumbnail),
			// FileSize:     fileInfo.Size(),
			FileSize: fileSize,
			Duration: maths.RoundFloat64ToInt(ptr.Deref(inf.Duration)),
			Width:    maths.RoundFloat64ToInt(ptr.Deref(inf.Width)),
			Height:   maths.RoundFloat64ToInt(ptr.Deref(inf.Height)),
			Filename: filename,
		})
	}

	return publications, nil
}

func storePublications(ctx context.Context, job *entity.Job, storer storage.Storer) error {
	if job == nil {
		return errs.ErrJobNil
	}

	for _, pub := range job.Publications {
		if err := storer.SetPublication(ctx, job.UUID, &pub); err != nil {
			return fmt.Errorf("store publication: %w", err)
		}
	}

	return nil
}
