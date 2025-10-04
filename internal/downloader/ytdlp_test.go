package downloader_test

import (
	"daunrodo/internal/downloader"
	"daunrodo/internal/entity"
	"daunrodo/pkg/ptr"
	_ "embed"
	"testing"

	"github.com/lrstanley/go-ytdlp"
)

//go:embed testdata/ytdlp_stdout_single_json_line.json
var ytdlpStdoutSingleJSONLine string

//go:embed testdata/ytdlp_stdout_single_json_line_and_filepath.json
var ytdlpStdoutSingleJSONLineAndFilepath string

//go:embed testdata/ytdlp_stdout_multiple_json_lines_and_random_lines.json
var ytdlpStdoutMultipleJSONLinesAndRandomLines string

func Test_parseYtdlpStdout(t *testing.T) {
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := downloader.ParseYtdlpStdout(tt.stdout)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("ParseYtdlpStdout() failed: %v", gotErr)
				}

				return
			}

			if tt.wantErr {
				t.Fatal("ParseYtdlpStdout() succeeded unexpectedly")
			}

			for idx, result := range got {
				if result.Title != tt.want[idx].Title {
					t.Errorf("got = %v, want %v", result.Title, tt.want[idx].Title)
				}

				if result.ID != tt.want[idx].ID {
					t.Errorf("got = %v, want %v", result.ID, tt.want[idx].ID)
				}

				if result.Filename != tt.want[idx].Filename {
					t.Errorf("got = %v, want %v", result.Filename, tt.want[idx].Filename)
				}
			}
		})
	}
}

func Test_composePublications(t *testing.T) {
	info1 := &ytdlp.ExtractedInfo{
		ID:       "one",
		Title:    ptr.Of("First"),
		Filename: ptr.Of("/tmp/first.mp4"),
		ExtractedFormat: &ytdlp.ExtractedFormat{
			Width:  ptr.Of(1920.0),
			Height: ptr.Of(1080.0),
		},
	}

	info2 := &ytdlp.ExtractedInfo{
		ID:       "two",
		Title:    ptr.Of("Second"),
		Filename: ptr.Of("/tmp/second.mp4"),
		ExtractedFormat: &ytdlp.ExtractedFormat{
			Width:  ptr.Of(1920.0),
			Height: ptr.Of(1080.0),
		},
	}

	tests := []struct {
		name        string
		info        []*ytdlp.ExtractedInfo
		ytdlpStdout string
		want        []entity.Publication
		wantErr     bool
	}{
		{
			name:        "test1",
			info:        []*ytdlp.ExtractedInfo{info1},
			ytdlpStdout: ytdlpStdoutSingleJSONLine,
			want: []entity.Publication{
				{
					ID:       "one",
					Title:    "First",
					Filename: "/tmp/first.mp4",
				},
			},
		},
		{
			name:        "test2",
			info:        []*ytdlp.ExtractedInfo{info1, info2},
			ytdlpStdout: ytdlpStdoutMultipleJSONLinesAndRandomLines,
			want: []entity.Publication{
				{
					ID:       "one",
					Title:    "First",
					Filename: "/tmp/first.mp4",
				},
				{
					ID:       "two",
					Title:    "Second",
					Filename: "/tmp/first.mp4",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publications, err := downloader.ComposePublications(tt.info, tt.ytdlpStdout)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("composePublications() failed: %v", err)
				}

				return
			}

			if tt.wantErr {
				t.Fatal("composePublications() succeeded unexpectedly")
			}

			for idx, publication := range publications {
				if publication.ID != tt.want[idx].ID {
					t.Errorf("got = %v, want %v", publication.ID, tt.want[idx].ID)
				}

				if publication.Title != tt.want[idx].Title {
					t.Errorf("got = %v, want %v", publication.Title, tt.want[idx].Title)
				}

				if publication.Filename != tt.want[idx].Filename {
					t.Errorf("got = %v, want %v", publication.Filename, tt.want[idx].Filename)
				}
			}
		})
	}
}
