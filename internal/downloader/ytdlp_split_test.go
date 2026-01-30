package downloader

import (
	"bufio"
	"reflect"
	"strings"
	"testing"
)

func TestSplitLinesAny(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "mixed line endings",
			input: "a\rb\nc\r\nd",
			want:  []string{"a", "b", "c", "d"},
		},
		{
			name:  "trailing newline",
			input: "one\ntwo\n",
			want:  []string{"one", "two"},
		},
		{
			name:  "single line",
			input: "solo",
			want:  []string{"solo"},
		},
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scanner := bufio.NewScanner(strings.NewReader(tc.input))
			scanner.Split(splitLinesAny)

			var got []string
			for scanner.Scan() {
				got = append(got, scanner.Text())
			}

			if err := scanner.Err(); err != nil {
				t.Fatalf("scanner error: %v", err)
			}

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
