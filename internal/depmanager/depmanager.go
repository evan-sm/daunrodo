// Package depmanager handles binary dependency management for external tools.
// It downloads and maintains binaries like yt-dlp, ffmpeg, and gallery-dl.
// Checksums are used only to detect when new versions are available, not to verify downloads.
package depmanager

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"daunrodo/internal/config"

	"github.com/ulikunitz/xz"
)

// BinaryName represents the name of a binary dependency.
type BinaryName string

// Binary dependency names.
const (
	BinaryYTdlp     BinaryName = "yt-dlp"
	BinaryFFmpeg    BinaryName = "ffmpeg"
	BinaryFFprobe   BinaryName = "ffprobe"
	BinaryGalleryDL BinaryName = "gallery-dl"
	BinaryDeno      BinaryName = "deno"
)

// Platform operating system names and architectures.
const (
	platformDarwin  = "darwin"
	platformLinux   = "linux"
	platformWindows = "windows"
	archARM64       = "arm64"
	archAMD64       = "amd64"
)

// Internal constants for binary management.
const (
	// downloadTimeout is the HTTP client timeout for downloading binaries.
	downloadTimeout = 10 * time.Minute
	// filePermExecutable is the file permission for executable binaries.
	filePermExecutable = 0o755
	// filePermReadWrite is the file permission for regular files.
	filePermReadWrite = 0o644
	// sha256HexLength is the expected length of SHA256 hex string.
	sha256HexLength = 64
	// sha256SumsFieldCount is the expected field count in SHA256SUMS format.
	sha256SumsFieldCount = 2
	// savedSumsFilename is the filename for saved checksums.
	savedSumsFilename = ".sha256sums.json"
)

// Platform represents the OS and architecture combination.
type Platform struct {
	OS   string
	Arch string
}

// String returns the platform string in format "os/arch".
func (p Platform) String() string {
	return p.OS + "/" + p.Arch
}

// Manager manages binary dependencies.
type Manager struct {
	log      *slog.Logger
	cfg      *config.Config
	platform Platform
	client   *http.Client

	mu        sync.RWMutex
	shaSums   map[string]string     // filename -> sha256 hash (fetched from remote)
	savedSums map[string]string     // filename -> sha256 hash (saved from previous run)
	binPaths  map[BinaryName]string // binary name -> installed path

	isUpdating bool
}

// New creates a new dependency manager.
func New(log *slog.Logger, cfg *config.Config) *Manager {
	return &Manager{
		log: log.With(slog.String("package", "depmanager")),
		cfg: cfg,
		platform: Platform{
			OS:   runtime.GOOS,
			Arch: runtime.GOARCH,
		},
		client: &http.Client{
			Timeout: downloadTimeout,
		},
		shaSums:   make(map[string]string),
		savedSums: make(map[string]string),
		binPaths:  make(map[BinaryName]string),
	}
}

// Start initializes the dependency manager.
func (m *Manager) Start(ctx context.Context) {
	if !m.cfg.DepManager.UseSystemBinaries {
		m.MustInstallAll(ctx)

		go m.StartUpdateChecker(ctx)

		return
	}

	m.MustSetSystemBinaries(ctx)
}

// MustSetSystemBinaries sets system binaries if needed.
// Panics if any binary cannot be set.
func (m *Manager) MustSetSystemBinaries(ctx context.Context) {
	if err := m.SetSystemBinaries(); err != nil {
		m.log.ErrorContext(ctx, "failed to set system binaries", slog.Any("error", err))
		panic(fmt.Sprintf("depmanager: failed to set system binaries: %v", err))
	}
}

// SetSystemBinaries sets system binaries by looking them up in the system PATH.
func (m *Manager) SetSystemBinaries() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	binaries := []BinaryName{BinaryYTdlp, BinaryFFmpeg, BinaryGalleryDL, BinaryDeno}

	for _, binary := range binaries {
		path, err := exec.LookPath(string(binary))
		if err != nil {
			return fmt.Errorf("%s not found in system PATH: %w", binary, err)
		}

		m.binPaths[binary] = path
	}

	return nil
}

// MustInstallAll downloads all required binaries if needed.
// Panics if any binary cannot be installed.
func (m *Manager) MustInstallAll(ctx context.Context) {
	if err := m.InstallAll(ctx); err != nil {
		m.log.ErrorContext(ctx, "failed to install all binaries", slog.Any("error", err))
		panic(fmt.Sprintf("depmanager: failed to install all binaries: %v", err))
	}
}

// InstallAll downloads all required binaries if needed.
// On first run, if binaries exist, skips all downloads.
func (m *Manager) InstallAll(ctx context.Context) error {
	log := m.log

	err := os.MkdirAll(m.cfg.DepManager.BinsDir, filePermExecutable)
	if err != nil {
		return fmt.Errorf("create bins directory: %w", err)
	}

	// Load saved checksums from previous run
	err = m.loadSavedSums()
	if err != nil {
		log.DebugContext(ctx, "no saved checksums found, first run", slog.Any("error", err))
	}

	binaries := []BinaryName{BinaryFFmpeg, BinaryDeno, BinaryYTdlp, BinaryGalleryDL}

	// Download missing binaries
	for _, binary := range binaries {
		if m.isBinaryExists(binary) {
			m.setBinaryPath(binary)
			log.DebugContext(ctx, "binary already exists", slog.String("binary", string(binary)))

			continue
		}

		err = m.downloadAndInstall(ctx, binary)
		if err != nil {
			return fmt.Errorf("download and install %s: %w", binary, err)
		}
	}

	log.InfoContext(ctx, "all binaries are installed", slog.Any("binaries", m.binPaths))

	// Fetch and save checksums for future update checks
	err = m.FetchSHASums(ctx)
	if err != nil {
		log.WarnContext(ctx, "failed to fetch checksums", slog.Any("error", err))

		return nil
	}

	err = m.saveSums()
	if err != nil {
		log.WarnContext(ctx, "failed to save checksums", slog.Any("error", err))
	}

	log.InfoContext(ctx, "checksums fetched and saved successfully")

	return nil
}

// GetBinaryPath returns the full path to a binary.
//   - /home/user/ + binary => /home/user/binary
func (m *Manager) GetBinaryPath(name BinaryName) string {
	filename := string(name)
	if m.platform.OS == platformWindows {
		filename += ".exe"
	}

	return filepath.Join(m.cfg.DepManager.BinsDir, filename)
}

// GetInstalledPath returns the installed path for a binary, or empty if not installed.
func (m *Manager) GetInstalledPath(name BinaryName) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.binPaths[name]
}

// StartUpdateChecker starts a background goroutine that periodically checks for updates.
// It compares fetched checksums with saved checksums and redownloads if different.
func (m *Manager) StartUpdateChecker(ctx context.Context) {
	if m.cfg.DepManager.UpdateInterval <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(m.cfg.DepManager.UpdateInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.checkAndUpdate(ctx)
			}
		}
	}()
}

// FetchSHASums fetches and parses SHA256 sums from configured URLs.
func (m *Manager) FetchSHASums(ctx context.Context) error {
	sumsURLs, err := m.CollectSHASumsURLs()
	if err != nil {
		return fmt.Errorf("collect SHA sums URLs: %w", err)
	}

	for _, url := range sumsURLs {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}

		resp, err := m.client.Do(req)
		if err != nil {
			return fmt.Errorf("fetch SHA sums: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()

			return fmt.Errorf("unexpected status: %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			return fmt.Errorf("read body: %w", err)
		}

		if err := m.ParseSHASums(string(body)); err != nil {
			return err
		}
	}

	return nil
}

// CollectSHASumsURLs collects SHA256 sums URLs from the configuration.
func (m *Manager) CollectSHASumsURLs() ([]string, error) {
	var sumsURLs []string

	sources := []string{
		m.cfg.DepManager.YTdlpSHA256SumsURL,
		m.cfg.DepManager.FFmpegSHA256SumsURL,
		m.cfg.DepManager.GalleryDLSHA256SumsURL,
		m.cfg.DepManager.DenoSHA256SumsURL,
	}

	for _, raw := range sources {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		if strings.Contains(raw, ",") {
			for part := range strings.SplitSeq(raw, ",") {
				part = strings.TrimSpace(part)
				if part != "" {
					sumsURLs = append(sumsURLs, part)
				}
			}
		} else {
			sumsURLs = append(sumsURLs, raw)
		}
	}

	if len(sumsURLs) == 0 {
		return nil, fmt.Errorf("no SHA256 sums URLs configured")
	}

	return sumsURLs, nil
}

// ParseSHASums parses SHA256 sums from content in the format "hash  filename".
func (m *Manager) ParseSHASums(content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) != sha256SumsFieldCount {
			continue
		}

		hash := parts[0]
		filename := parts[1]

		if len(hash) != sha256HexLength {
			continue
		}

		m.shaSums[filename] = hash
	}

	m.log.Debug("parsed SHA256 sums", slog.Int("count", len(m.shaSums)))

	return nil
}

// checkAndUpdate checks for updates and downloads new versions if available.
func (m *Manager) checkAndUpdate(ctx context.Context) {
	if m.isUpdating {
		return
	}

	m.isUpdating = true
	defer func() { m.isUpdating = false }()

	log := m.log

	// Fetch current checksums
	err := m.FetchSHASums(ctx)
	if err != nil {
		log.WarnContext(ctx, "update check: failed to fetch checksums", slog.Any("error", err))

		return
	}

	// Compare with saved checksums
	updates := m.findUpdates()
	if len(updates) == 0 {
		log.DebugContext(ctx, "update check: no updates available")

		return
	}

	log.InfoContext(ctx, "update check: updates available", slog.Any("binaries", updates))

	// Download updated binaries
	for _, binary := range updates {
		if err := m.downloadAndInstall(ctx, binary); err != nil {
			log.ErrorContext(ctx, "update check: failed to update binary",
				slog.String("binary", string(binary)),
				slog.Any("error", err))

			continue
		}

		log.InfoContext(ctx, "update check: binary updated", slog.String("binary", string(binary)))
	}

	// Save new checksums
	if err := m.saveSums(); err != nil {
		log.WarnContext(ctx, "update check: failed to save checksums", slog.Any("error", err))
	}
}

// findUpdates compares fetched checksums with saved checksums and returns binaries that need updating.
func (m *Manager) findUpdates() []BinaryName {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var updates []BinaryName

	// Map of binary to the filenames we care about
	binaryFiles := map[BinaryName][]string{
		BinaryYTdlp:     {m.getDownloadFilename(BinaryYTdlp)},
		BinaryFFmpeg:    {m.getDownloadFilename(BinaryFFmpeg)},
		BinaryGalleryDL: {m.getDownloadFilename(BinaryGalleryDL)},
		BinaryDeno:      {m.getDownloadFilename(BinaryDeno)},
	}

	for binary, files := range binaryFiles {
		for _, filename := range files {
			newHash, hasNew := m.shaSums[filename]
			oldHash, hasOld := m.savedSums[filename]

			// If we have a new hash and it differs from the old one (or no old hash exists)
			if hasNew && (!hasOld || newHash != oldHash) {
				updates = append(updates, binary)

				break
			}
		}
	}

	return updates
}

// isBinaryExists checks if a binary file exists and has non-zero size.
func (m *Manager) isBinaryExists(name BinaryName) bool {
	binPath := m.GetBinaryPath(name)
	info, err := os.Stat(binPath)

	return err == nil && info.Size() > 0
}

func (m *Manager) setBinaryPath(name BinaryName) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.binPaths[name] = m.GetBinaryPath(name)
}

// downloadAndInstall downloads and installs a dependency binary.
func (m *Manager) downloadAndInstall(ctx context.Context, name BinaryName) error {
	log := m.log.With(slog.String("binary", string(name)))

	binPath := m.GetBinaryPath(name)

	url := m.getBinaryURL(name)
	if url == "" {
		return fmt.Errorf("no download URL configured for %s on %s", name, m.platform)
	}

	log.InfoContext(ctx, "downloading binary", slog.String("url", url))

	binPaths, err := m.downloadDependency(ctx, url, name)
	if err != nil {
		return fmt.Errorf("download dependency: %w", err)
	}

	err = m.makeExecutable(binPaths)
	if err != nil {
		return fmt.Errorf("make executable: %w", err)
	}

	for _, path := range binPaths {
		m.setBinaryPath(BinaryName(path))
	}

	log.InfoContext(ctx, "binary installed successfully", slog.String("path", binPath))

	return nil
}

// makeExecutable sets the executable permission on a binary file.
func (m *Manager) makeExecutable(binPaths []string) error {
	for _, path := range binPaths {
		err := os.Chmod(path, filePermExecutable)
		if err != nil {
			return fmt.Errorf("chmod: %w", err)
		}
	}

	return nil
}

// loadSavedSums loads saved checksums from file.
func (m *Manager) loadSavedSums() error {
	filePath := filepath.Join(m.cfg.DepManager.BinsDir, savedSumsFilename)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read checksums file: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := json.Unmarshal(data, &m.savedSums); err != nil {
		return fmt.Errorf("unmarshal checksums: %w", err)
	}

	return nil
}

// saveSums saves current checksums to file for future comparison.
func (m *Manager) saveSums() error {
	m.mu.RLock()
	data, err := json.MarshalIndent(m.shaSums, "", "  ")
	m.mu.RUnlock()

	if err != nil {
		return fmt.Errorf("marshal checksums: %w", err)
	}

	filePath := filepath.Join(m.cfg.DepManager.BinsDir, savedSumsFilename)

	if err := os.WriteFile(filePath, data, filePermReadWrite); err != nil {
		return fmt.Errorf("write checksums file: %w", err)
	}

	// Update savedSums to match shaSums
	m.mu.Lock()
	m.savedSums = make(map[string]string)
	maps.Copy(m.savedSums, m.shaSums)
	m.mu.Unlock()

	return nil
}

// getDownloadFilename returns the filename as it appears in SHA256SUMS for a binary.
func (m *Manager) getDownloadFilename(name BinaryName) string {
	switch name {
	case BinaryYTdlp:
		switch {
		case m.platform.OS == platformLinux && m.platform.Arch == archARM64:
			return "yt-dlp_linux_aarch64"
		case m.platform.OS == platformLinux:
			return "yt-dlp_linux"
		default:
			return "yt-dlp"
		}
	case BinaryGalleryDL:
		switch {
		case m.platform.OS == platformLinux && m.platform.Arch == archARM64:
			return "gallery-dl_linux_arm64"
		case m.platform.OS == platformLinux:
			return "gallery-dl_linux_amd64"
		default:
			return "gallery-dl"
		}
	case BinaryFFmpeg:
		switch {
		case m.platform.OS == platformLinux && m.platform.Arch == archARM64:
			return "ffmpeg-master-latest-linuxarm64-gpl.tar.xz"
		case m.platform.OS == platformLinux:
			return "ffmpeg-master-latest-linux64-gpl.tar.xz"
		default:
			return "ffmpeg"
		}
	case BinaryDeno:
		switch {
		case m.platform.OS == platformLinux && m.platform.Arch == archARM64:
			return "deno-aarch64-unknown-linux-gnu.zip"
		case m.platform.OS == platformLinux && m.platform.Arch == archAMD64:
			return "deno-x86_64-unknown-linux-gnu.zip"
		}
	}

	return string(name)
}

func (m *Manager) getBinaryURL(name BinaryName) string {
	cfg := m.cfg.DepManager

	switch name {
	case BinaryYTdlp:
		return m.selectURL(cfg.YTdlpLinuxARM64, cfg.YTdlpLinuxAMD64)
	case BinaryFFmpeg, BinaryFFprobe:
		return m.selectURL(cfg.FFmpegLinuxARM64, cfg.FFmpegLinuxAMD64)
	case BinaryGalleryDL:
		return m.selectURL(cfg.GalleryDLLinuxARM64, cfg.GalleryDLLinuxAMD64)
	case BinaryDeno:
		return m.selectURL(cfg.DenoLinuxARM64, cfg.DenoLinuxAMD64)
	}

	return ""
}

func (m *Manager) selectURL(linuxARM64, linuxAMD64 string) string {
	key := m.platform.OS + "/" + m.platform.Arch

	switch key {
	case "linux/arm64":
		if linuxARM64 != "" {
			return linuxARM64
		}
	case "linux/amd64":
		if linuxAMD64 != "" {
			return linuxAMD64
		}
	}

	return linuxAMD64
}

// downloadDependency downloads and installs a binary dependency from a URL. Returns installed paths.
func (m *Manager) downloadDependency(ctx context.Context, url string, name BinaryName) ([]string, error) {
	binPath := m.GetBinaryPath(name)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	needsExtraction := strings.HasSuffix(url, ".zip") ||
		strings.HasSuffix(url, ".tar.xz") ||
		strings.HasSuffix(url, ".tar.gz")

	destDir := filepath.Dir(binPath)

	tmpFile, err := os.CreateTemp(destDir, "download-*")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}

	tmpPath := tmpFile.Name()

	defer func() {
		tmpFile.Close()
		os.Remove(tmpPath)
	}()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("close temp file: %w", err)
	}

	var installedPaths []string

	if needsExtraction {
		targets := m.getFilesNeeded(name)

		err = m.extractFiles(tmpPath, destDir, url, targets)
		if err != nil {
			return nil, fmt.Errorf("extract: %w", err)
		}

		for target := range targets {
			installedPaths = append(installedPaths, filepath.Join(destDir, target))
		}
	}

	if !needsExtraction {
		err = os.Rename(tmpPath, binPath)
		if err != nil {
			return nil, fmt.Errorf("rename: %w", err)
		}

		installedPaths = append(installedPaths, binPath)
	}

	return installedPaths, nil
}

// getFilesNeeded returns the set of files needed from an archive for a given binary.
func (m *Manager) getFilesNeeded(name BinaryName) map[string]struct{} {
	files := make(map[string]struct{})

	switch name {
	case BinaryFFmpeg:
		files["ffmpeg"] = struct{}{}
		files["ffprobe"] = struct{}{}
	case BinaryDeno:
		files["deno"] = struct{}{}
	default:
		files[string(name)] = struct{}{}
	}

	return files
}

func (m *Manager) extractFiles(archivePath, destDir, url string, targets map[string]struct{}) error {
	switch {
	case strings.HasSuffix(url, ".zip"):
		return m.extractFromZip(archivePath, destDir, targets)
	case strings.HasSuffix(url, ".tar.xz"):
		return m.extractFromTarXZ(archivePath, destDir, targets)
	case strings.HasSuffix(url, ".tar.gz"):
		return m.extractFromTarGZ(archivePath, destDir, targets)
	default:
		return fmt.Errorf("unsupported archive format")
	}
}

func (m *Manager) extractFromZip(zipPath, destDir string, targets map[string]struct{}) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer reader.Close()

	extracted := 0

	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		filename := file.FileInfo().Name()
		if _, ok := targets[filename]; !ok {
			continue
		}

		fileReader, err := file.Open()
		if err != nil {
			return fmt.Errorf("open file in zip: %w", err)
		}

		destPath := filepath.Join(destDir, filename)

		outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, filePermExecutable)
		if err != nil {
			fileReader.Close()

			return fmt.Errorf("create dest file: %w", err)
		}

		_, err = io.Copy(outFile, fileReader)
		fileReader.Close()
		outFile.Close()

		if err != nil {
			return fmt.Errorf("extract file: %w", err)
		}

		extracted++

		if extracted == len(targets) {
			return nil
		}
	}

	if extracted == 0 {
		return fmt.Errorf("no target files found in zip archive")
	}

	return nil
}

func (m *Manager) extractFromTarXZ(tarXZPath, destDir string, targets map[string]struct{}) error {
	file, err := os.Open(tarXZPath)
	if err != nil {
		return fmt.Errorf("open tar.xz: %w", err)
	}
	defer file.Close()

	xzReader, err := xz.NewReader(file)
	if err != nil {
		return fmt.Errorf("create xz reader: %w", err)
	}

	return m.extractTarSelected(xzReader, destDir, targets)
}

func (m *Manager) extractFromTarGZ(tarGZPath, destDir string, targets map[string]struct{}) error {
	file, err := os.Open(tarGZPath)
	if err != nil {
		return fmt.Errorf("open tar.gz: %w", err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("create gzip reader: %w", err)
	}
	defer gzReader.Close()

	return m.extractTarSelected(gzReader, destDir, targets)
}

func (m *Manager) extractTarSelected(reader io.Reader, destDir string, targets map[string]struct{}) error {
	tarReader := tar.NewReader(reader)
	extracted := 0

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("read tar header: %w", err)
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		filename := filepath.Base(header.Name)
		if _, ok := targets[filename]; !ok {
			continue
		}

		destPath := filepath.Join(destDir, filename)

		outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, filePermExecutable)
		if err != nil {
			return fmt.Errorf("create dest file: %w", err)
		}

		_, err = io.Copy(outFile, tarReader)
		outFile.Close()

		if err != nil {
			return fmt.Errorf("extract file: %w", err)
		}

		extracted++

		if extracted == len(targets) {
			return nil
		}
	}

	if extracted == 0 {
		return fmt.Errorf("no target files found in tar archive")
	}

	return nil
}
