package downloader_test

import (
	_ "embed"
	"testing"

	"daunrodo/internal/downloader"
)

//go:embed testdata/ytdlp_stdout_single_json_line.json
var ytdlpStdoutSingleJSONLine string

//go:embed testdata/ytdlp_stdout_single_json_line_and_filepath.json
var ytdlpStdoutSingleJSONLineAndFilepath string

//go:embed testdata/ytdlp_stdout_multiple_json_lines_and_random_lines.json
var ytdlpStdoutMultipleJSONLinesAndRandomLines string

func TestParseYtdlpStdout(t *testing.T) {
	tests := []struct {
		name    string
		stdout  string
		want    []downloader.ResultJSON
		wantErr bool
	}{
		{
			name:   "single JSON line",
			stdout: ytdlpStdoutSingleJSONLine,
			want: []downloader.ResultJSON{
				{ID: "one", Title: "First", Filename: "/tmp/first.mp4"},
			},
		},
		{
			name:   "json then filepath on next line assigns filename",
			stdout: ytdlpStdoutSingleJSONLineAndFilepath,
			want: []downloader.ResultJSON{
				{ID: "x1", Title: "With file", Filename: "/tmp/first.mp4"},
			},
		},
		{
			name:   "multiple entries with blanks and stray lines",
			stdout: ytdlpStdoutMultipleJSONLinesAndRandomLines,
			want: []downloader.ResultJSON{
				{ID: "one", Title: "First", Filename: "/tmp/first.mp4"},
				{ID: "two", Title: "Second", Filename: "/tmp/first.mp4"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, gotErr := downloader.ParseYtdlpStdout(tc.stdout)
			if gotErr != nil {
				if !tc.wantErr {
					t.Errorf("ParseYtdlpStdout() failed: %v", gotErr)
				}

				return
			}

			if tc.wantErr {
				t.Fatal("ParseYtdlpStdout() succeeded unexpectedly")
			}

			if len(got) != len(tc.want) {
				t.Fatalf("got %d results, want %d", len(got), len(tc.want))
			}

			for idx, result := range got {
				if result.Title != tc.want[idx].Title {
					t.Errorf("got Title = %q, want %q", result.Title, tc.want[idx].Title)
				}

				if result.ID != tc.want[idx].ID {
					t.Errorf("got ID = %q, want %q", result.ID, tc.want[idx].ID)
				}

				if result.Filename != tc.want[idx].Filename {
					t.Errorf("got Filename = %q, want %q", result.Filename, tc.want[idx].Filename)
				}
			}
		})
	}
}

func TestParseProgress(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantProg float64
		wantOK   bool
	}{
		{
			name:     "standard progress",
			line:     "[download]  50.0% of  100.00MiB at  10.00MiB/s ETA 00:05",
			wantProg: 50.0,
			wantOK:   true,
		},
		{
			name:     "100 percent",
			line:     "[download] 100% of 50.00MiB in 00:05",
			wantProg: 100.0,
			wantOK:   true,
		},
		{
			name:     "no percentage",
			line:     "[youtube] Extracting URL: https://youtube.com/watch?v=abc",
			wantProg: 0,
			wantOK:   false,
		},
		{
			name:     "decimal percentage",
			line:     "[download]   5.5% of ~  50.00MiB at  10.00MiB/s ETA 00:30",
			wantProg: 5.5,
			wantOK:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := downloader.ParseProgress(tc.line)
			if ok != tc.wantOK {
				t.Errorf("ParseProgress() ok = %v, want %v", ok, tc.wantOK)
			}

			if ok && got != tc.wantProg {
				t.Errorf("ParseProgress() = %v, want %v", got, tc.wantProg)
			}
		})
	}
}
