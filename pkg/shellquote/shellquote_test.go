package shellquote_test

import (
	"daunrodo/pkg/shellquote"
	"testing"
)

func TestJoin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		bin  string
		args []string
		want string
	}{
		{
			name: "no args",
			bin:  "/usr/bin/yt-dlp",
			args: nil,
			want: "\"/usr/bin/yt-dlp\"",
		},
		{
			name: "simple args",
			bin:  "/usr/bin/yt-dlp",
			args: []string{"--version"},
			want: "\"/usr/bin/yt-dlp\" \"--version\"",
		},
		{
			name: "spaces are preserved via quotes",
			bin:  "/usr/local/bin/yt-dlp",
			args: []string{"-o", "My Video %(title)s.%(ext)s"},
			want: "\"/usr/local/bin/yt-dlp\" \"-o\" \"My Video %(title)s.%(ext)s\"",
		},
		{
			name: "url with query chars",
			bin:  "yt-dlp",
			args: []string{"https://example.com/watch?v=a&b=1"},
			want: "\"yt-dlp\" \"https://example.com/watch?v=a&b=1\"",
		},
		{
			name: "embedded double quote is escaped",
			bin:  "yt-dlp",
			args: []string{"--title", `a"b`},
			want: "\"yt-dlp\" \"--title\" \"a\\\"b\"",
		},
		{
			name: "backslashes are escaped",
			bin:  "yt-dlp",
			args: []string{"--output", `C:\temp\file.%(ext)s`},
			want: "\"yt-dlp\" \"--output\" \"C:\\\\temp\\\\file.%(ext)s\"",
		},
		{
			name: "empty arg",
			bin:  "yt-dlp",
			args: []string{""},
			want: "\"yt-dlp\" \"\"",
		},
		{
			name: "unicode",
			bin:  "yt-dlp",
			args: []string{"--title", "привет"},
			want: "\"yt-dlp\" \"--title\" \"привет\"",
		},
		{
			name: "newline becomes \\n escape sequence (as produced by strconv.Quote)",
			bin:  "yt-dlp",
			args: []string{"--comment", "line1\nline2"},
			want: "\"yt-dlp\" \"--comment\" \"line1\\nline2\"",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := shellquote.Join(tt.bin, tt.args)
			if got != tt.want {
				t.Fatalf("Join() mismatch\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}
