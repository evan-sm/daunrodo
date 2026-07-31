package downloader

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"daunrodo/internal/config"
	"daunrodo/internal/consts"
	"daunrodo/internal/depmanager"
	"daunrodo/internal/entity"
	"daunrodo/internal/errs"
	"daunrodo/internal/observability"
	"daunrodo/internal/proxymgr"
	"daunrodo/internal/storage"
	"daunrodo/pkg/gen"
	"daunrodo/pkg/maths"
)

const (
	instagramUserAgentChrome = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
		"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36"
	instagramSecChUA        = `"Not=A?Brand";v="99", "Google Chrome";v="151", "Chromium";v="151"`
	instagramAcceptDocument = "text/html,application/xhtml+xml,application/xml;q=0.9," +
		"image/avif,image/webp,image/apng,*/*;q=0.8," +
		"application/signed-exchange;v=b3;q=0.7"
	instagramReferer  = "https://www.instagram.com/"
	nativeHTTPTimeout = 2 * time.Minute

	nativeProgressFetch  = 10
	nativeProgressParsed = 25
	nativeProgressVideo  = 40
	nativeProgressAudio  = 65
	nativeProgressRemux  = 80
	nativeWorkDirPerm    = 0o755
	nativeMaxHTMLBytes   = 8 << 20
)

// Native downloads content without yt-dlp (Instagram Reels first).
type Native struct {
	log      *slog.Logger
	cfg      *config.Config
	depMgr   *depmanager.Manager
	proxyMgr *proxymgr.Manager
	metrics  *observability.Metrics
	client   *http.Client
}

// NewNative creates a native downloader instance.
func NewNative(
	log *slog.Logger,
	cfg *config.Config,
	depMgr *depmanager.Manager,
	proxyMgr *proxymgr.Manager,
	metrics *observability.Metrics,
) Downloader {
	return &Native{
		log:      log.With(slog.String("package", "downloader"), slog.String("downloader", consts.DownloaderNative)),
		cfg:      cfg,
		depMgr:   depMgr,
		proxyMgr: proxyMgr,
		metrics:  metrics,
		client:   &http.Client{Timeout: nativeHTTPTimeout},
	}
}

// Process downloads an Instagram Reel via DASH + ffmpeg remux.
func (d *Native) Process(ctx context.Context, job *entity.Job, storer storage.Storer) (retErr error) {
	d.metrics.RecordDownloaderRequest(consts.DownloaderNative, "started")

	defer func() {
		if retErr != nil {
			d.metrics.RecordDownloaderRequest(consts.DownloaderNative, "error")
			d.metrics.RecordDownloaderError(consts.DownloaderNative, classifyProcessingError(retErr))

			return
		}

		d.metrics.RecordDownloaderRequest(consts.DownloaderNative, "success")
	}()

	if job == nil {
		retErr = errs.ErrJobNil

		return retErr
	}

	log := d.log.With(slog.Any("job", job))
	storer.UpdateJobStatus(ctx, job.UUID, entity.JobStatusDownloading, 0, "")

	shortcode, err := InstagramReelShortcode(job.URL)
	if err != nil {
		retErr = err

		return retErr
	}

	storer.UpdateJobStatus(ctx, job.UUID, entity.JobStatusDownloading, nativeProgressFetch, "")

	pageURL := "https://www.instagram.com/reels/" + shortcode + "/"

	page, err := d.fetchHTML(ctx, pageURL)
	if err != nil {
		retErr = err

		return retErr
	}

	storer.UpdateJobStatus(ctx, job.UUID, entity.JobStatusDownloading, nativeProgressParsed, "")

	dash, err := extractInstagramReelDASH(page, shortcode)
	if err != nil {
		retErr = err

		return retErr
	}

	workDir := filepath.Join(d.cfg.Dir.Downloads, "native-"+job.UUID)
	if err := os.MkdirAll(workDir, nativeWorkDirPerm); err != nil {
		retErr = fmt.Errorf("create work dir: %w", err)

		return retErr
	}
	defer os.RemoveAll(workDir)

	videoPath := filepath.Join(workDir, "video.mp4")
	audioPath := filepath.Join(workDir, "audio.mp4")
	outPath := filepath.Join(d.cfg.Dir.Downloads, "instagram - "+shortcode+".mp4")

	storer.UpdateJobStatus(ctx, job.UUID, entity.JobStatusDownloading, nativeProgressVideo, "")

	if err := d.downloadFile(ctx, dash.VideoURL, videoPath); err != nil {
		retErr = fmt.Errorf("download video: %w", err)

		return retErr
	}

	storer.UpdateJobStatus(ctx, job.UUID, entity.JobStatusDownloading, nativeProgressAudio, "")

	if err := d.downloadFile(ctx, dash.AudioURL, audioPath); err != nil {
		retErr = fmt.Errorf("download audio: %w", err)

		return retErr
	}

	storer.UpdateJobStatus(ctx, job.UUID, entity.JobStatusDownloading, nativeProgressRemux, "")

	if err := d.remux(ctx, videoPath, audioPath, outPath); err != nil {
		retErr = err

		return retErr
	}

	info, err := os.Stat(outPath)
	if err != nil {
		retErr = fmt.Errorf("stat output: %w", err)

		return retErr
	}

	pub := entity.Publication{
		UUID:       gen.UUIDv5(shortcode, outPath),
		ID:         shortcode,
		Type:       "video",
		Platform:   "instagram",
		WebpageURL: job.URL,
		Title:      shortcode,
		FileSize:   info.Size(),
		Duration:   maths.RoundFloat64ToInt(dash.Duration.Seconds()),
		Width:      dash.Width,
		Height:     dash.Height,
		Filename:   outPath,
	}
	publications := []entity.Publication{pub}

	if err := storePublications(ctx, job.UUID, publications, storer); err != nil {
		retErr = fmt.Errorf("store publications: %w", err)

		return retErr
	}

	d.metrics.RecordJobDownloadedBytes(totalPublicationSize(publications))
	storer.UpdateJobPublications(ctx, job.UUID, publications)
	storer.UpdateJobStatus(ctx, job.UUID, entity.JobStatusFinished, fullProgress, "")

	log.InfoContext(ctx, "done", slog.String("file", outPath))

	return nil
}

func (d *Native) fetchHTML(ctx context.Context, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("%w: %v", errs.ErrInstagramReelPage, err)
	}

	d.setDocumentHeaders(req)

	res, err := d.httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", errs.ErrInstagramReelPage, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: status %d", errs.ErrInstagramReelPage, res.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, nativeMaxHTMLBytes))
	if err != nil {
		return "", fmt.Errorf("%w: %v", errs.ErrInstagramReelPage, err)
	}

	return string(body), nil
}

func (d *Native) downloadFile(ctx context.Context, fileURL, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("%w: %v", errs.ErrDownloadFailed, err)
	}

	d.setCDNHeaders(req)

	res, err := d.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", errs.ErrDownloadFailed, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status %d", errs.ErrDownloadFailed, res.StatusCode)
	}

	outFile, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("%w: %v", errs.ErrDownloadFailed, err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, res.Body); err != nil {
		return fmt.Errorf("%w: %v", errs.ErrDownloadFailed, err)
	}

	return nil
}

func (d *Native) remux(ctx context.Context, videoPath, audioPath, outPath string) error {
	ffmpeg := d.depMgr.GetInstalledPath(depmanager.BinaryFFmpeg)
	if ffmpeg == "" {
		ffmpeg = d.depMgr.GetBinaryPath(depmanager.BinaryFFmpeg)
	}

	if ffmpeg == "" {
		return errs.ErrBinaryNotFound
	}

	cmd := exec.CommandContext(ctx, ffmpeg,
		"-y",
		"-i", videoPath,
		"-i", audioPath,
		"-map", "0:v:0",
		"-map", "1:a:0",
		"-c", "copy",
		"-movflags", "+faststart",
		outPath,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg remux: %w: %s", err, string(out))
	}

	return nil
}

// setDocumentHeaders mirrors a Chrome incognito document navigation (no cookies/Referer).
func (d *Native) setDocumentHeaders(req *http.Request) {
	req.Header.Set("Accept", instagramAcceptDocument)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Dnt", "1")
	req.Header.Set("Priority", "u=0, i")
	req.Header.Set("Sec-Ch-Ua", instagramSecChUA)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"macOS"`)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", instagramUserAgentChrome)
}

// setCDNHeaders is used for media BaseURL downloads.
func (d *Native) setCDNHeaders(req *http.Request) {
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Dnt", "1")
	req.Header.Set("Referer", instagramReferer)
	req.Header.Set("Sec-Ch-Ua", instagramSecChUA)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"macOS"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("User-Agent", instagramUserAgentChrome)
}

func (d *Native) httpClient() *http.Client {
	if d.proxyMgr == nil || !d.proxyMgr.HasProxies() {
		return d.client
	}

	proxyURL := d.proxyMgr.GetRandomProxy()
	if proxyURL == "" {
		return d.client
	}

	parsedProxy, err := url.Parse(proxyURL)
	if err != nil {
		return d.client
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyURL(parsedProxy)

	return &http.Client{
		Timeout:   d.client.Timeout,
		Transport: transport,
	}
}
