//nolint:testpackage // using internal package access to cover private helpers
package depmanager

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"daunrodo/internal/config"
	"daunrodo/internal/observability"
)

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestParseSHASums(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		wantLen  int
		wantHash map[string]string
	}{
		{
			name: "valid sums",
			content: `abc123def456789012345678901234567890123456789012345678901234abcd  yt-dlp_macos
def456abc789012345678901234567890123456789012345678901234567efgh  yt-dlp_linux`,
			wantLen: 2,
			wantHash: map[string]string{
				"yt-dlp_macos": "abc123def456789012345678901234567890123456789012345678901234abcd",
				"yt-dlp_linux": "def456abc789012345678901234567890123456789012345678901234567efgh",
			},
		},
		{
			name:     "empty content",
			content:  "",
			wantLen:  0,
			wantHash: map[string]string{},
		},
		{
			name:     "invalid format",
			content:  "not a valid line",
			wantLen:  0,
			wantHash: map[string]string{},
		},
		{
			name:     "invalid hash length",
			content:  "short  filename",
			wantLen:  0,
			wantHash: map[string]string{},
		},
		{
			name: "mixed valid and invalid",
			content: `abc123def456789012345678901234567890123456789012345678901234abcd  valid_file
invalid line here
def456abc789012345678901234567890123456789012345678901234567efgh  another_valid`,
			wantLen: 2,
			wantHash: map[string]string{
				"valid_file":    "abc123def456789012345678901234567890123456789012345678901234abcd",
				"another_valid": "def456abc789012345678901234567890123456789012345678901234567efgh",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			log := slog.Default()
			cfg := &config.Config{}
			mgr := New(log, cfg, nil)

			err := mgr.ParseSHASums(tc.content)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(mgr.shaSums) != tc.wantLen {
				t.Errorf("got %d sums, want %d", len(mgr.shaSums), tc.wantLen)
			}

			for filename, wantHash := range tc.wantHash {
				if got := mgr.shaSums[filename]; got != wantHash {
					t.Errorf("hash for %s: got %s, want %s", filename, got, wantHash)
				}
			}
		})
	}
}

func TestGetBinaryPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		binary   BinaryName
		os       string
		binsDir  string
		wantPath string
	}{
		{
			name:     "yt-dlp on linux",
			binary:   BinaryYTdlp,
			os:       "linux",
			binsDir:  "/app/bins",
			wantPath: "/app/bins/yt-dlp",
		},
		{
			name:     "yt-dlp on windows",
			binary:   BinaryYTdlp,
			os:       "windows",
			binsDir:  "/app/bins",
			wantPath: "/app/bins/yt-dlp.exe",
		},
		{
			name:     "ffmpeg on darwin",
			binary:   BinaryFFmpeg,
			os:       "darwin",
			binsDir:  "/usr/local/bins",
			wantPath: "/usr/local/bins/ffmpeg",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			log := slog.Default()
			cfg := &config.Config{
				DepManager: config.DepManager{
					BinsDir: tc.binsDir,
				},
			}
			mgr := New(log, cfg, nil)
			mgr.platform.OS = tc.os

			got := mgr.GetBinaryPath(tc.binary)
			if got != tc.wantPath {
				t.Errorf("got %s, want %s", got, tc.wantPath)
			}
		})
	}
}

func TestFetchSHASums(t *testing.T) {
	t.Parallel()

	shaContent := `abc123def456789012345678901234567890123456789012345678901234abcd  yt-dlp_macos
def456abc789012345678901234567890123456789012345678901234567efgh  yt-dlp_linux`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(shaContent))
	}))
	defer server.Close()

	log := slog.Default()
	cfg := &config.Config{
		DepManager: config.DepManager{
			YTdlpSHA256SumsURL: server.URL,
		},
	}

	mgr := New(log, cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := mgr.FetchSHASums(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mgr.shaSums) != 2 {
		t.Errorf("got %d sums, want 2", len(mgr.shaSums))
	}
}

func TestFetchSHASums_ServerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	log := slog.Default()
	cfg := &config.Config{
		DepManager: config.DepManager{
			YTdlpSHA256SumsURL: server.URL,
		},
	}

	mgr := New(log, cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := mgr.FetchSHASums(ctx)
	if err == nil {
		t.Error("expected error for server error response")
	}
}

func TestDownloadDependency(t *testing.T) {
	t.Parallel()

	content := "binary content here"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(content))
	}))
	defer server.Close()

	log := slog.Default()
	tmpDir := t.TempDir()

	cfg := &config.Config{
		DepManager: config.DepManager{
			BinsDir: tmpDir,
		},
	}

	mgr := New(log, cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	paths, err := mgr.downloadDependency(ctx, server.URL, BinaryYTdlp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(paths) != 1 {
		t.Fatalf("expected 1 installed path, got %d", len(paths))
	}

	got, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}

	if string(got) != content {
		t.Errorf("got %q, want %q", string(got), content)
	}
}

func TestSelectURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		platform Platform
		linuxARM string
		linuxAMD string
		want     string
	}{
		{
			name:     "linux/arm64 with config",
			platform: Platform{OS: "linux", Arch: "arm64"},
			linuxARM: "https://example.com/linux-arm64",
			linuxAMD: "https://example.com/linux-amd64",
			want:     "https://example.com/linux-arm64",
		},
		{
			name:     "linux/amd64 with config",
			platform: Platform{OS: "linux", Arch: "amd64"},
			linuxARM: "https://example.com/linux-arm64",
			linuxAMD: "https://example.com/linux-amd64",
			want:     "https://example.com/linux-amd64",
		},
		{
			name:     "unsupported platform falls back to amd64",
			platform: Platform{OS: "freebsd", Arch: "arm"},
			linuxARM: "https://example.com/linux-arm64",
			linuxAMD: "https://example.com/linux-amd64",
			want:     "https://example.com/linux-amd64",
		},
		{
			name:     "darwin falls back to amd64",
			platform: Platform{OS: "darwin", Arch: "arm64"},
			linuxARM: "",
			linuxAMD: "https://example.com/linux-amd64",
			want:     "https://example.com/linux-amd64",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			log := slog.Default()
			cfg := &config.Config{}
			mgr := New(log, cfg, nil)
			mgr.platform = tc.platform

			got := mgr.selectURL(tc.linuxARM, tc.linuxAMD)
			if got != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestGetDownloadFilename(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		binary   BinaryName
		platform Platform
		want     string
	}{
		{
			name:     "yt-dlp linux arm64",
			binary:   BinaryYTdlp,
			platform: Platform{OS: "linux", Arch: "arm64"},
			want:     "yt-dlp_linux_aarch64",
		},
		{
			name:     "yt-dlp linux amd64",
			binary:   BinaryYTdlp,
			platform: Platform{OS: "linux", Arch: "amd64"},
			want:     "yt-dlp_linux",
		},
		{
			name:     "yt-dlp darwin",
			binary:   BinaryYTdlp,
			platform: Platform{OS: "darwin", Arch: "arm64"},
			want:     "yt-dlp",
		},
		{
			name:     "gallery-dl linux",
			binary:   BinaryGalleryDL,
			platform: Platform{OS: "linux", Arch: "amd64"},
			want:     "gallery-dl_linux_amd64",
		},
		{
			name:     "gallery-dl windows",
			binary:   BinaryGalleryDL,
			platform: Platform{OS: "windows", Arch: "amd64"},
			want:     "gallery-dl",
		},
		{
			name:     "ffmpeg linux arm64",
			binary:   BinaryFFmpeg,
			platform: Platform{OS: "linux", Arch: "arm64"},
			want:     "ffmpeg-master-latest-linuxarm64-gpl.tar.xz",
		},
		{
			name:     "ffmpeg linux amd64",
			binary:   BinaryFFmpeg,
			platform: Platform{OS: "linux", Arch: "amd64"},
			want:     "ffmpeg-master-latest-linux64-gpl.tar.xz",
		},
		{
			name:     "deno linux arm64",
			binary:   BinaryDeno,
			platform: Platform{OS: "linux", Arch: "arm64"},
			want:     "deno-aarch64-unknown-linux-gnu.zip",
		},
		{
			name:     "deno linux amd64",
			binary:   BinaryDeno,
			platform: Platform{OS: "linux", Arch: "amd64"},
			want:     "deno-x86_64-unknown-linux-gnu.zip",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			log := slog.Default()
			cfg := &config.Config{}
			mgr := New(log, cfg, nil)
			mgr.platform = tc.platform

			got := mgr.getDownloadFilename(tc.binary)
			if got != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestBinaryExists(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a test binary file
	testBinPath := filepath.Join(tmpDir, "yt-dlp")
	if err := os.WriteFile(testBinPath, []byte("binary content"), 0o755); err != nil {
		t.Fatalf("failed to create test binary: %v", err)
	}

	log := slog.Default()
	cfg := &config.Config{
		DepManager: config.DepManager{
			BinsDir: tmpDir,
		},
	}
	mgr := New(log, cfg, nil)
	mgr.platform.OS = "linux"

	// Test existing binary
	if !mgr.isBinaryExists(BinaryYTdlp) {
		t.Error("expected binary to exist")
	}

	// Test non-existing binary
	if mgr.isBinaryExists(BinaryFFmpeg) {
		t.Error("expected binary to not exist")
	}
}

func TestFindUpdates(t *testing.T) {
	t.Parallel()

	log := slog.Default()
	cfg := &config.Config{}
	mgr := New(log, cfg, nil)
	mgr.platform = Platform{OS: "linux", Arch: "amd64"}

	// Set up saved sums (old)
	mgr.savedSums = map[string]string{
		"yt-dlp_linux": "oldhash1234567890123456789012345678901234567890123456789012",
	}

	// Set up fetched sums (new)
	mgr.shaSums = map[string]string{
		"yt-dlp_linux": "newhash1234567890123456789012345678901234567890123456789012",
	}

	updates := mgr.findUpdates()

	if !slices.Contains(updates, BinaryYTdlp) {
		t.Error("expected yt-dlp to be in updates list")
	}
}

func TestFindUpdates_NoChanges(t *testing.T) {
	t.Parallel()

	log := slog.Default()
	cfg := &config.Config{}
	mgr := New(log, cfg, nil)
	mgr.platform = Platform{OS: "linux", Arch: "amd64"}

	hash := "samehash1234567890123456789012345678901234567890123456789012"

	mgr.savedSums = map[string]string{
		"yt-dlp_linux": hash,
	}

	mgr.shaSums = map[string]string{
		"yt-dlp_linux": hash,
	}

	updates := mgr.findUpdates()

	if len(updates) != 0 {
		t.Errorf("expected no updates, got %v", updates)
	}
}

func TestSaveAndLoadSums(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	log := slog.Default()
	cfg := &config.Config{
		DepManager: config.DepManager{
			BinsDir: tmpDir,
		},
	}
	mgr := New(log, cfg, nil)

	// Set some checksums
	mgr.shaSums = map[string]string{
		"file1": "hash1234567890123456789012345678901234567890123456789012345678",
		"file2": "hash2234567890123456789012345678901234567890123456789012345678",
	}

	// Save checksums
	if err := mgr.saveSums(); err != nil {
		t.Fatalf("failed to save sums: %v", err)
	}

	// Verify file was created
	sumFile := filepath.Join(tmpDir, savedSumsFilename)
	if _, err := os.Stat(sumFile); os.IsNotExist(err) {
		t.Fatal("checksums file was not created")
	}

	// Create new manager and load
	mgr2 := New(log, cfg, nil)
	if err := mgr2.loadSavedSums(); err != nil {
		t.Fatalf("failed to load sums: %v", err)
	}

	// Verify loaded data
	if len(mgr2.savedSums) != 2 {
		t.Errorf("expected 2 saved sums, got %d", len(mgr2.savedSums))
	}

	if mgr2.savedSums["file1"] != mgr.shaSums["file1"] {
		t.Errorf("hash mismatch for file1")
	}
}

func TestCollectSHASumsURLs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     config.DepManager
		wantLen int
		wantErr bool
	}{
		{
			name: "single URL",
			cfg: config.DepManager{
				YTdlpSHA256SumsURL: "https://example.com/sha256sums",
			},
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "multiple URLs with comma",
			cfg: config.DepManager{
				DenoSHA256SumsURL: "https://example.com/sum1,https://example.com/sum2",
			},
			wantLen: 2,
			wantErr: false,
		},
		{
			name: "multiple sources",
			cfg: config.DepManager{
				YTdlpSHA256SumsURL:  "https://example.com/ytdlp",
				FFmpegSHA256SumsURL: "https://example.com/ffmpeg",
				DenoSHA256SumsURL:   "https://example.com/deno",
			},
			wantLen: 3,
			wantErr: false,
		},
		{
			name:    "no URLs configured",
			cfg:     config.DepManager{},
			wantLen: 0,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			log := slog.Default()
			cfg := &config.Config{DepManager: tc.cfg}
			mgr := New(log, cfg, nil)

			urls, err := mgr.CollectSHASumsURLs()
			if (err != nil) != tc.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tc.wantErr)
			}

			if len(urls) != tc.wantLen {
				t.Errorf("got %d URLs, want %d", len(urls), tc.wantLen)
			}
		})
	}
}

func TestCheckAndUpdate_DownloadsNewBinary(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tmpDir := t.TempDir()

		const (
			filename      = "yt-dlp_linux"
			binaryContent = "updated binary"
		)

		newHash := strings.Repeat("a", sha256HexLength)
		oldHash := strings.Repeat("b", sha256HexLength)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/sha":
				fmt.Fprintf(w, "%s  %s\n", newHash, filename)
			case "/bin":
				_, _ = w.Write([]byte(binaryContent))
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		cfg := &config.Config{
			DepManager: config.DepManager{
				BinsDir:            tmpDir,
				YTdlpSHA256SumsURL: server.URL + "/sha",
				YTdlpLinuxAMD64:    server.URL + "/bin",
			},
		}

		metrics := observability.New()
		mgr := New(slog.Default(), cfg, metrics)
		mgr.platform = Platform{OS: platformLinux, Arch: archAMD64}
		mgr.savedSums = map[string]string{filename: oldHash}

		mgr.checkAndUpdate(t.Context())

		binPath := filepath.Join(tmpDir, "yt-dlp")

		data, err := os.ReadFile(binPath)
		if err != nil {
			t.Fatalf("expected binary to be downloaded: %v", err)
		}

		if string(data) != binaryContent {
			t.Fatalf("downloaded binary content mismatch: got %q, want %q", string(data), binaryContent)
		}

		if got := mgr.savedSums[filename]; got != newHash {
			t.Fatalf("saved checksum mismatch: got %s, want %s", got, newHash)
		}

		gotMetrics := binaryDownloadMetrics(t, metrics)
		if got := gotMetrics["yt-dlp|update"]; got != 1 {
			t.Fatalf("update metric mismatch: got %v, want 1", got)
		}
	})
}

func TestStartUpdateChecker_UsesTicker(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tmpDir := t.TempDir()

		const (
			filename      = "yt-dlp_linux"
			binaryContent = "ticker binary"
		)

		newHash := strings.Repeat("c", sha256HexLength)
		oldHash := strings.Repeat("d", sha256HexLength)

		cfg := &config.Config{
			DepManager: config.DepManager{
				BinsDir:            tmpDir,
				UpdateInterval:     time.Second,
				YTdlpSHA256SumsURL: "/sha",
				YTdlpLinuxAMD64:    "/bin",
			},
		}

		mgr := New(slog.Default(), cfg, nil)
		mgr.platform = Platform{OS: platformLinux, Arch: archAMD64}
		mgr.savedSums = map[string]string{filename: oldHash}

		mgr.client = &http.Client{
			Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
				switch r.URL.Path {
				case "/sha":
					body := fmt.Sprintf("%s  %s\n", newHash, filename)

					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: r}, nil //nolint:lll
				case "/bin":
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(binaryContent)), Header: make(http.Header), Request: r}, nil //nolint:lll
				default:
					return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("nf")), Header: make(http.Header), Request: r}, nil //nolint:lll
				}
			}),
		}

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		mgr.StartUpdateChecker(ctx)

		time.Sleep(cfg.DepManager.UpdateInterval)
		synctest.Wait()

		data, err := os.ReadFile(filepath.Join(tmpDir, "yt-dlp"))
		if err != nil {
			t.Fatalf("expected binary to be downloaded by ticker: %v", err)
		}

		if string(data) != binaryContent {
			t.Fatalf("downloaded binary content mismatch: got %q, want %q", string(data), binaryContent)
		}

		cancel()
		synctest.Wait()

		if got := mgr.savedSums[filename]; got != newHash {
			t.Fatalf("saved checksum mismatch: got %s, want %s", got, newHash)
		}
	})
}

func TestInstallAll_RecordsInstallMetric(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("yt-dlp binary"))
	}))
	defer server.Close()

	cfg := &config.Config{
		DepManager: config.DepManager{
			BinsDir:         tmpDir,
			YTdlpLinuxAMD64: server.URL,
		},
	}

	for _, name := range []string{"ffmpeg", "deno", "gallery-dl"} {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(name), 0o755); err != nil {
			t.Fatalf("write existing binary %s: %v", name, err)
		}
	}

	metrics := observability.New()
	mgr := New(slog.Default(), cfg, metrics)
	mgr.platform = Platform{OS: platformLinux, Arch: archAMD64}

	if err := mgr.InstallAll(t.Context()); err != nil {
		t.Fatalf("InstallAll() unexpected error: %v", err)
	}

	gotMetrics := binaryDownloadMetrics(t, metrics)
	if got := gotMetrics["yt-dlp|install"]; got != 1 {
		t.Fatalf("install metric mismatch: got %v, want 1", got)
	}
}

func TestInstallAll_SkipExistingBinaryDoesNotRecordMetric(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	for _, name := range []string{"ffmpeg", "deno", "gallery-dl", "yt-dlp"} {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(name), 0o755); err != nil {
			t.Fatalf("write existing binary %s: %v", name, err)
		}
	}

	cfg := &config.Config{
		DepManager: config.DepManager{
			BinsDir: tmpDir,
		},
	}

	metrics := observability.New()
	mgr := New(slog.Default(), cfg, metrics)
	mgr.platform = Platform{OS: platformLinux, Arch: archAMD64}

	if err := mgr.InstallAll(t.Context()); err != nil {
		t.Fatalf("InstallAll() unexpected error: %v", err)
	}

	gotMetrics := binaryDownloadMetrics(t, metrics)
	if len(gotMetrics) != 0 {
		t.Fatalf("expected no metrics for skipped binaries, got %v", gotMetrics)
	}
}

func TestInstallAll_ArchiveBinaryRecordsManagedBinaryOnly(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	archive, err := createZipArchive(map[string]string{
		"ffmpeg":  "ffmpeg binary",
		"ffprobe": "ffprobe binary",
	})
	if err != nil {
		t.Fatalf("create zip archive: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ffmpeg.zip" {
			http.NotFound(w, r)

			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	cfg := &config.Config{
		DepManager: config.DepManager{
			BinsDir:          tmpDir,
			FFmpegLinuxAMD64: server.URL + "/ffmpeg.zip",
		},
	}

	for _, name := range []string{"deno", "gallery-dl", "yt-dlp"} {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(name), 0o755); err != nil {
			t.Fatalf("write existing binary %s: %v", name, err)
		}
	}

	metrics := observability.New()
	mgr := New(slog.Default(), cfg, metrics)
	mgr.platform = Platform{OS: platformLinux, Arch: archAMD64}

	if err := mgr.InstallAll(t.Context()); err != nil {
		t.Fatalf("InstallAll() unexpected error: %v", err)
	}

	gotMetrics := binaryDownloadMetrics(t, metrics)
	if got := gotMetrics["ffmpeg|install"]; got != 1 {
		t.Fatalf("archive install metric mismatch: got %v, want 1", got)
	}

	if _, ok := gotMetrics["ffprobe|install"]; ok {
		t.Fatalf("unexpected ffprobe metric: %v", gotMetrics)
	}
}

func binaryDownloadMetrics(t *testing.T, metrics *observability.Metrics) map[string]float64 {
	t.Helper()

	families, err := metrics.Registry().Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	result := make(map[string]float64)

	for _, family := range families {
		if family.GetName() != "daunrodo_depmanager_binary_downloads_total" {
			continue
		}

		for _, metric := range family.GetMetric() {
			labels := make(map[string]string)

			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}

			key := labels["binary"] + "|" + labels["reason"]
			result[key] = metric.GetCounter().GetValue()
		}
	}

	return result
}

func createZipArchive(files map[string]string) ([]byte, error) {
	var buf bytes.Buffer

	writer := zip.NewWriter(&buf)

	for name, content := range files {
		fileWriter, err := writer.Create(name)
		if err != nil {
			return nil, err
		}

		if _, err := fileWriter.Write([]byte(content)); err != nil {
			return nil, err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
