package downloader_test

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"daunrodo/internal/downloader"
	"daunrodo/internal/entity"
	"daunrodo/internal/errs"
	"daunrodo/internal/storage"
)

const testReelURL = "https://www.instagram.com/reel/DYewzkDtDxa/"

type countingDownloader struct {
	calls atomic.Int32
}

func (d *countingDownloader) Process(_ context.Context, _ *entity.Job, _ storage.Storer) error {
	d.calls.Add(1)

	return nil
}

func TestRouterDispatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		url        string
		wantNative bool
	}{
		{
			name:       "instagram reel to native",
			url:        testReelURL,
			wantNative: true,
		},
		{
			name:       "instagram reels to native",
			url:        "https://instagram.com/reels/DYewzkDtDxa/",
			wantNative: true,
		},
		{
			name:       "youtube to default",
			url:        "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			wantNative: false,
		},
		{
			name:       "instagram post to default",
			url:        "https://www.instagram.com/p/ABC123/",
			wantNative: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			native := &countingDownloader{}
			def := &countingDownloader{}
			router := downloader.NewRouter(def, downloader.Rule{
				Match: downloader.IsInstagramReel,
				DL:    native,
			})

			job := &entity.Job{URL: tt.url}
			if err := router.Process(context.Background(), job, nil); err != nil {
				t.Fatalf("Process: %v", err)
			}

			gotNative := native.calls.Load() == 1
			gotDefault := def.calls.Load() == 1

			if gotNative != tt.wantNative {
				t.Fatalf("native called=%v want=%v", gotNative, tt.wantNative)
			}

			if gotDefault == tt.wantNative {
				t.Fatalf("default called=%v want=%v", gotDefault, !tt.wantNative)
			}
		})
	}
}

func TestIsInstagramReel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
		want bool
	}{
		{name: "reel", url: testReelURL, want: true},
		{name: "reels", url: "https://instagram.com/reels/DYewzkDtDxa/?igsh=abc", want: true},
		{name: "instagr.am", url: "https://instagr.am/reel/DYewzkDtDxa/", want: true},
		{name: "post", url: "https://www.instagram.com/p/DYewzkDtDxa/", want: false},
		{name: "youtube", url: "https://youtube.com/watch?v=x", want: false},
		{name: "empty", url: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := downloader.IsInstagramReel(tt.url); got != tt.want {
				t.Fatalf("IsInstagramReel(%q)=%v want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestInstagramReelShortcode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		url     string
		want    string
		wantErr error
	}{
		{
			name: "reel path",
			url:  testReelURL,
			want: "DYewzkDtDxa",
		},
		{
			name: "reels path with query",
			url:  "https://www.instagram.com/reels/Abc_12-3/?utm=1",
			want: "Abc_12-3",
		},
		{
			name:    "post path",
			url:     "https://www.instagram.com/p/DYewzkDtDxa/",
			wantErr: errs.ErrInstagramReelShortcode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := downloader.InstagramReelShortcode(tt.url)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatal("expected error")
				}

				if err != tt.wantErr {
					t.Fatalf("err=%v want %v", err, tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}

			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestExtractInstagramReelDASHFromFixture(t *testing.T) {
	t.Parallel()

	path := filepath.Join("testdata", "instagram_reel_DYewzkDtDxa.html")

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	dash, err := downloader.ExtractInstagramReelDASH(string(raw), "DYewzkDtDxa")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	const wantBandwidth = 4667449
	if dash.Bandwidth != wantBandwidth {
		t.Fatalf("bandwidth=%d want %d", dash.Bandwidth, wantBandwidth)
	}

	if dash.Width != 1080 || dash.Height != 1920 {
		t.Fatalf("dims=%dx%d want 1080x1920", dash.Width, dash.Height)
	}

	if dash.VideoURL == "" || dash.AudioURL == "" {
		t.Fatal("expected video and audio urls")
	}

	if sec := dash.Duration.Seconds(); sec < 26 || sec > 28 {
		t.Fatalf("duration=%v want ~27s", dash.Duration)
	}

	if _, err := downloader.ExtractInstagramReelDASH(string(raw), "missingCODE"); err == nil {
		t.Fatal("expected error for missing code")
	}
}
