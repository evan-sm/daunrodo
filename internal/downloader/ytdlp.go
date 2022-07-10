package downloader

import (
	"bufio"
	"context"
	"daunrodo/internal/config"
	"daunrodo/internal/consts"
	"daunrodo/internal/entity"
	"daunrodo/pkg/calc"
	"daunrodo/pkg/maths"
	"daunrodo/pkg/ptr"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/lrstanley/go-ytdlp"
)

type YTdlp struct {
	log *slog.Logger
	cfg *config.Config
}

func NewYTdlp(log *slog.Logger, cfg *config.Config) Downloader {
	return &YTdlp{
		log: log.With(slog.String("package", "downloader"), slog.String("downloader", consts.DownloaderYTdlp)),
		cfg: cfg,
	}
}

func (d *YTdlp) Process(ctx context.Context, job *entity.Job, updateStatusFn StatusUpdater) error {
	if job == nil {
		return fmt.Errorf("job is nil")
	}

	log := d.log

	updateStatusFn(ctx, job, entity.JobStatusDownloading, 0, "")

	progressFn := func(prog ytdlp.ProgressUpdate) {
		log.DebugContext(ctx, "ytdlp progress", "progress_update", ProgressUpdate{&prog})
		updateStatusFn(ctx, job, entity.JobStatusDownloading, calc.Progress(prog.DownloadedBytes, prog.TotalBytes), "")
	}

	dl := ytdlp.New().
		// SetWorkDir(d.cfg.Dir.Downloads).
		CacheDir(d.cfg.Dir.Cache).
		CookiesFromBrowser("chrome").
		PresetAlias(job.Preset).
		ProgressFunc(defaultProgressFreq, progressFn).
		PrintJSON().Print("after_move:filepath").
		// DumpSingleJSON().
		// NoSimulate().
		Output(d.cfg.App.Dir.FilenameTemplate)

	if d.cfg.Dir.Cookies != "" {
		dl = dl.Cookies(d.cfg.Dir.Cookies)
	}

	res, err := dl.Run(ctx, job.URL)
	if err != nil {
		log.Error("ytdlp run", slog.Any("error", err), "result", Result{res})

		return err
	}

	info, err := res.GetExtractedInfo()
	if err != nil {
		log.ErrorContext(ctx, "ytdlp get extracted info", slog.Any("error", err))
	}

	job.Publications, err = composePublications(info, res.Stdout)
	if err != nil {
		return fmt.Errorf("compose publications: %w", err)
	}

	log.InfoContext(ctx, "publications composed", "publications", job.Publications)

	updateStatusFn(ctx, job, entity.JobStatusFinished, 100, "")

	log.InfoContext(ctx, "done", "result", Result{res})

	return err

}

// GetResults collects all JSON objects printed by yt-dlp into a slice
func parseYtdlpStdout(stdout string) ([]ResultJSON, error) {
	var (
		res []ResultJSON
		// filePaths []string
	)
	// dec := json.NewDecoder(strings.NewReader(stdout))
	scanner := bufio.NewScanner(strings.NewReader(stdout))
	rePath := regexp.MustCompile(`(?i)^[^\{\[\n].*\.[a-z0-9]{1,6}$`)
	const maxTokenSize = 10 * 1024 * 1024
	scanner.Buffer(make([]byte, 4096), maxTokenSize)

	// for {
	// 	var r ResultJSON
	// 	if err := dec.Decode(&r); err != nil {
	// 		if err.Error() == "invalid character '/' looking for beginning of value" {
	// 			// Handle specific syntax error
	// 			fmt.Println("Syntax error detected")
	// 		}
	// 		if err == io.EOF {
	// 			break
	// 		}
	// 		return nil, fmt.Errorf("decode: %w", err)
	// 	}
	// 	res = append(res, r)
	// }
	lineNo := 0
	resultNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {

			continue
		}

		// attempt to unmarshal a single JSON object
		var r ResultJSON
		if err := json.Unmarshal([]byte(line), &r); err == nil {
			res = append(res, r)
			resultNo++

			continue
		}

		// not valid JSON: maybe it's a path printed by --print after_move:filepath
		// or some other log. Use regex heuristic to detect a file-like line.
		if rePath.MatchString(line) {
			res[resultNo-1].Filename = line

			continue
		}

		// If you want, you can attempt to detect and unmarshal multiline JSON
		// starting at this line — but yt-dlp normally prints one JSON object per line.
		// For debugging, collect the error context.
		// For now we ignore other non-matching lines.
		_ = fmt.Errorf("skipped non-json/non-path stdout line %d: %q", lineNo, line)

	}

	return res, nil
}

func composePublications(info []*ytdlp.ExtractedInfo, ytdlpStdout string) ([]entity.Publication, error) {
	if info == nil {
		return nil, fmt.Errorf("info is nil")
	}

	results, err := parseYtdlpStdout(ytdlpStdout)
	if err != nil {
		return nil, fmt.Errorf("parse yt-dlp stdout: %w", err)
	}

	resultsMap := make(map[string]ResultJSON, len(results))
	for _, res := range results {
		resultsMap[res.ID] = res
	}

	publications := make([]entity.Publication, 0, len(info))
	for _, inf := range info {
		publications = append(publications, entity.Publication{
			ID:           inf.ID,
			Type:         string(inf.Type),
			Platform:     ptr.Deref(inf.Extractor),
			Channel:      ptr.Deref(inf.Channel),
			WebpageURL:   ptr.Deref(inf.WebpageURL),
			Title:        ptr.Deref(inf.Title),
			Description:  ptr.Deref(inf.Description),
			ViewCount:    maths.RoundFloat64ToInt(ptr.Deref(inf.ViewCount)),
			LikeCount:    maths.RoundFloat64ToInt(ptr.Deref(inf.LikeCount)),
			ThumbnailURL: ptr.Deref(inf.Thumbnail),
			Duration:     maths.RoundFloat64ToInt(ptr.Deref(inf.Duration)),
			// Filename:     ptr.Deref(inf.Filename),
			Filename: resultsMap[inf.ID].Filename,
		})
	}

	return publications, nil
}
